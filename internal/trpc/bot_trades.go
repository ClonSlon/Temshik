package trpc

import (
	"encoding/json"
	"sort"
	"time"
)

type botIDInput struct {
	BotID int64 `json:"botId"`
}

func (h *Handler) botOpenSmartTrades(input any) (any, *TRPCError) {
	return h.botSmartTradesByQuery(
		input,
		`SELECT id, type, entryType, takeProfitType, symbol, ref, exchangeAccountId, botId, ownerId, createdAt, updatedAt
FROM "SmartTrade"
WHERE ownerId=? AND botId=? AND type='Trade' AND entryType='Order' AND takeProfitType IN ('Order','None') AND ref IS NOT NULL`,
		true,
	)
}

func (h *Handler) botActiveSmartTrades(input any) (any, *TRPCError) {
	return h.botSmartTradesByQuery(
		input,
		`SELECT id, type, entryType, takeProfitType, symbol, ref, exchangeAccountId, botId, ownerId, createdAt, updatedAt
FROM "SmartTrade"
WHERE ownerId=? AND botId=? AND type='Trade' AND entryType='Order' AND takeProfitType='Order' AND ref IS NOT NULL`,
		true,
	)
}

func (h *Handler) botCompletedSmartTrades(input any) (any, *TRPCError) {
	return h.botSmartTradesByQuery(
		input,
		`SELECT id, type, entryType, takeProfitType, symbol, ref, exchangeAccountId, botId, ownerId, createdAt, updatedAt
FROM "SmartTrade" st
WHERE st.ownerId=? AND st.botId=? AND st.entryType='Order' AND st.takeProfitType='Order'
  AND EXISTS (
    SELECT 1 FROM "Order" o
    WHERE o.smartTradeId = st.id
      AND (
        (o.entityType='TakeProfitOrder' AND o.status='Filled') OR
        (o.entityType='StopLossOrder' AND o.status='Filled')
      )
  )
ORDER BY st.updatedAt DESC`,
		false,
	)
}

func (h *Handler) botPendingSmartTrades(input any) (any, *TRPCError) {
	return h.botSmartTradesByQuery(
		input,
		`SELECT id, type, entryType, takeProfitType, symbol, ref, exchangeAccountId, botId, ownerId, createdAt, updatedAt
FROM "SmartTrade" st
WHERE st.ownerId=? AND st.botId=? AND st.type='Trade' AND st.entryType='Order' AND st.takeProfitType='Order' AND st.ref IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM "Order" o
    WHERE o.smartTradeId = st.id AND o.status NOT IN ('Idle','Filled')
  )`,
		true,
	)
}

func (h *Handler) botSmartTradesByQuery(input any, sqlQuery string, sortByEntryPrice bool) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in botIDInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	rows, err := h.db.Query(sqlQuery, adminUserID, in.BotID)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	var trades []map[string]any
	var smartTradeIDs []int64
	var exchangeAccountIDs []int64
	for rows.Next() {
		m, err := scanSmartTrade(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		trades = append(trades, m)
		smartTradeIDs = append(smartTradeIDs, m["id"].(int64))
		exchangeAccountIDs = append(exchangeAccountIDs, m["exchangeAccountId"].(int64))
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	ordersByTrade, trpcErr := h.fetchOrdersBySmartTradeIDs(smartTradeIDs)
	if trpcErr != nil {
		return nil, trpcErr
	}
	accountsByID, trpcErr := h.fetchExchangeAccountsByIDs(exchangeAccountIDs)
	if trpcErr != nil {
		return nil, trpcErr
	}

	entities := make([]map[string]any, 0, len(trades))
	for _, t := range trades {
		id := t["id"].(int64)
		t["orders"] = ordersByTrade[id]
		if acc, ok := accountsByID[t["exchangeAccountId"].(int64)]; ok {
			t["exchangeAccount"] = acc
		} else {
			t["exchangeAccount"] = nil
		}
		entities = append(entities, toSmartTradeEntity(t))
	}

	if sortByEntryPrice {
		sort.SliceStable(entities, func(i, j int) bool {
			return entryOrderPrice(entities[i]) > entryOrderPrice(entities[j])
		})
	}

	out := make([]any, 0, len(entities))
	for _, e := range entities {
		out = append(out, e)
	}
	return out, nil
}

func entryOrderPrice(st map[string]any) float64 {
	eo, ok := st["entryOrder"].(map[string]any)
	if !ok {
		return 0
	}
	switch p := eo["price"].(type) {
	case float64:
		return p
	case int64:
		return float64(p)
	case json.Number:
		f, _ := p.Float64()
		return f
	default:
		return 0
	}
}

func (h *Handler) botOrders(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in botIDInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	rows, err := h.db.Query(
		`SELECT o.id, o.status, o.type, o.entityType, o.side, o.price, o.stopPrice, o.relativePrice, o.filledPrice, o.fee, o.symbol, o.exchangeAccountId, o.exchangeOrderId, o.quantity, o.smartTradeId, o.createdAt, o.placedAt, o.syncedAt, o.filledAt, o.updatedAt
FROM "Order" o
JOIN "SmartTrade" st ON st.id = o.smartTradeId
WHERE st.ownerId=? AND st.botId=? AND st.type='Trade'
  AND NOT EXISTS (SELECT 1 FROM "Order" o2 WHERE o2.smartTradeId = st.id AND o2.status <> 'Filled')`,
		adminUserID,
		in.BotID,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	var orders []map[string]any
	for rows.Next() {
		m, err := scanOrder(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		// Ensure filledAt is present (TS backend throws otherwise).
		if m["filledAt"] == nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: "order missing filledAt"}
		}
		if t, ok := m["filledAt"].(*time.Time); ok && t != nil {
			m["filledAt"] = *t
		}
		orders = append(orders, m)
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	sort.SliceStable(orders, func(i, j int) bool {
		ti, _ := orders[i]["filledAt"].(time.Time)
		tj, _ := orders[j]["filledAt"].(time.Time)
		return ti.Before(tj)
	})

	out := make([]any, 0, len(orders))
	for _, o := range orders {
		out = append(out, o)
	}
	return out, nil
}
