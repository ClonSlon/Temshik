package trpc

import (
	"database/sql"
	"fmt"
	"strings"
)

type orderScanner interface {
	Scan(dest ...any) error
}

func scanOrder(row orderScanner) (map[string]any, error) {
	var (
		id                int64
		status            string
		typ               string
		entityType        string
		side              string
		priceRaw          any
		stopPriceRaw      any
		relativePriceRaw  any
		filledPriceRaw    any
		feeRaw            any
		symbol            string
		exchangeAccountID int64
		exchangeOrderID   sql.NullString
		quantity          float64
		smartTradeID      int64
		createdAtRaw      any
		placedAtRaw       any
		syncedAtRaw       any
		filledAtRaw       any
		updatedAtRaw      any
	)

	if err := row.Scan(
		&id,
		&status,
		&typ,
		&entityType,
		&side,
		&priceRaw,
		&stopPriceRaw,
		&relativePriceRaw,
		&filledPriceRaw,
		&feeRaw,
		&symbol,
		&exchangeAccountID,
		&exchangeOrderID,
		&quantity,
		&smartTradeID,
		&createdAtRaw,
		&placedAtRaw,
		&syncedAtRaw,
		&filledAtRaw,
		&updatedAtRaw,
	); err != nil {
		return nil, err
	}

	price, err := toFloat64Ptr(priceRaw)
	if err != nil {
		return nil, err
	}
	stopPrice, err := toFloat64Ptr(stopPriceRaw)
	if err != nil {
		return nil, err
	}
	relativePrice, err := toFloat64Ptr(relativePriceRaw)
	if err != nil {
		return nil, err
	}
	filledPrice, err := toFloat64Ptr(filledPriceRaw)
	if err != nil {
		return nil, err
	}
	fee, err := toFloat64Ptr(feeRaw)
	if err != nil {
		return nil, err
	}

	createdAt, err := parseDBTime(createdAtRaw)
	if err != nil {
		return nil, err
	}
	placedAt, err := parseNullableDBTime(placedAtRaw)
	if err != nil {
		return nil, err
	}
	syncedAt, err := parseNullableDBTime(syncedAtRaw)
	if err != nil {
		return nil, err
	}
	filledAt, err := parseNullableDBTime(filledAtRaw)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseDBTime(updatedAtRaw)
	if err != nil {
		return nil, err
	}

	var exOrder any = nil
	if exchangeOrderID.Valid {
		exOrder = exchangeOrderID.String
	}

	out := map[string]any{
		"id":                id,
		"status":            status,
		"type":              typ,
		"entityType":        entityType,
		"side":              side,
		"price":             nil,
		"stopPrice":         nil,
		"relativePrice":     nil,
		"filledPrice":       nil,
		"fee":               nil,
		"symbol":            symbol,
		"exchangeAccountId": exchangeAccountID,
		"exchangeOrderId":   exOrder,
		"quantity":          quantity,
		"smartTradeId":      smartTradeID,
		"createdAt":         createdAt,
		"placedAt":          placedAt,
		"syncedAt":          syncedAt,
		"filledAt":          filledAt,
		"updatedAt":         updatedAt,
	}

	if price != nil {
		out["price"] = *price
	}
	if stopPrice != nil {
		out["stopPrice"] = *stopPrice
	}
	if relativePrice != nil {
		out["relativePrice"] = *relativePrice
	}
	if filledPrice != nil {
		out["filledPrice"] = *filledPrice
	}
	if fee != nil {
		out["fee"] = *fee
	}

	return out, nil
}

func (h *Handler) fetchOrdersBySmartTradeIDs(ids []int64) (map[int64][]any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	inClause, args := buildInClause(ids)
	if inClause == "" {
		return map[int64][]any{}, nil
	}

	rows, err := h.db.Query(
		`SELECT id, status, type, entityType, side, price, stopPrice, relativePrice, filledPrice, fee, symbol, exchangeAccountId, exchangeOrderId, quantity, smartTradeId, createdAt, placedAt, syncedAt, filledAt, updatedAt
FROM "Order"
WHERE smartTradeId IN (`+inClause+`)
ORDER BY id ASC`,
		args...,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	out := map[int64][]any{}
	for rows.Next() {
		m, err := scanOrder(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		tid, _ := m["smartTradeId"].(int64)
		out[tid] = append(out[tid], m)
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	// Ensure all requested ids exist in map.
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			out[id] = []any{}
		}
	}
	return out, nil
}

func (h *Handler) fetchSmartTradesByIDs(ids []int64) (map[int64]map[string]any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	inClause, args := buildInClause(ids)
	if inClause == "" {
		return map[int64]map[string]any{}, nil
	}

	rows, err := h.db.Query(
		`SELECT id, type, entryType, takeProfitType, symbol, ref, exchangeAccountId, botId, ownerId, createdAt, updatedAt
FROM "SmartTrade"
WHERE id IN (`+inClause+`)`,
		args...,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	out := map[int64]map[string]any{}
	for rows.Next() {
		m, err := scanSmartTrade(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		idv, _ := m["id"].(int64)
		out[idv] = m
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	for _, id := range ids {
		if _, ok := out[id]; !ok {
			out[id] = nil
		}
	}
	return out, nil
}

type ordersBotInput struct {
	BotID int64 `json:"botId"`
}

type ordersStatusesInput struct {
	BotID    int64    `json:"botId"`
	Statuses []string `json:"statuses"`
	Limit    *int     `json:"limit"`
	Cursor   *int64   `json:"cursor"`
}

func (h *Handler) orderOpenOrders(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in ordersBotInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	rows, err := h.db.Query(
		`SELECT o.id, o.status, o.type, o.entityType, o.side, o.price, o.stopPrice, o.relativePrice, o.filledPrice, o.fee, o.symbol, o.exchangeAccountId, o.exchangeOrderId, o.quantity, o.smartTradeId, o.createdAt, o.placedAt, o.syncedAt, o.filledAt, o.updatedAt
FROM "Order" o
JOIN "SmartTrade" st ON st.id = o.smartTradeId
WHERE st.ownerId=? AND st.botId=? AND o.status=?
ORDER BY o.placedAt DESC, o.id DESC`,
		adminUserID,
		in.BotID,
		"Placed",
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	var orders []map[string]any
	var smartTradeIDs []int64
	for rows.Next() {
		m, err := scanOrder(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		orders = append(orders, m)
		smartTradeIDs = append(smartTradeIDs, m["smartTradeId"].(int64))
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return h.buildOrdersWithSmartTrades(orders, smartTradeIDs)
}

func (h *Handler) orderClosedOrders(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in ordersStatusesInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	if len(in.Statuses) == 0 {
		in.Statuses = []string{"Filled"}
	}

	placeholders := strings.Repeat("?,", len(in.Statuses))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := []any{adminUserID, in.BotID}
	for _, s := range in.Statuses {
		args = append(args, s)
	}

	rows, err := h.db.Query(
		`SELECT o.id, o.status, o.type, o.entityType, o.side, o.price, o.stopPrice, o.relativePrice, o.filledPrice, o.fee, o.symbol, o.exchangeAccountId, o.exchangeOrderId, o.quantity, o.smartTradeId, o.createdAt, o.placedAt, o.syncedAt, o.filledAt, o.updatedAt
FROM "Order" o
JOIN "SmartTrade" st ON st.id = o.smartTradeId
WHERE st.ownerId=? AND st.botId=? AND o.status IN (`+placeholders+`)
ORDER BY o.updatedAt DESC, o.id DESC`,
		args...,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	var orders []map[string]any
	var smartTradeIDs []int64
	for rows.Next() {
		m, err := scanOrder(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		orders = append(orders, m)
		smartTradeIDs = append(smartTradeIDs, m["smartTradeId"].(int64))
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return h.buildOrdersWithSmartTrades(orders, smartTradeIDs)
}

func (h *Handler) orderInfiniteOrders(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in ordersStatusesInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}
	if len(in.Statuses) == 0 {
		in.Statuses = []string{"Idle", "Placed", "Filled", "Canceled", "Revoked", "Deleted"}
	}

	limit := 50
	if in.Limit != nil {
		limit = clampInt(*in.Limit, 1, 100)
	}

	where := `st.ownerId=? AND st.botId=?`
	args := []any{adminUserID, in.BotID}

	placeholders := strings.Repeat("?,", len(in.Statuses))
	placeholders = strings.TrimSuffix(placeholders, ",")
	where += ` AND o.status IN (` + placeholders + `)`
	for _, s := range in.Statuses {
		args = append(args, s)
	}

	if in.Cursor != nil {
		var cursorUpdatedAtRaw any
		err := h.db.QueryRow(`SELECT updatedAt FROM "Order" WHERE id=?`, *in.Cursor).Scan(&cursorUpdatedAtRaw)
		if err == nil {
			where += ` AND (o.updatedAt < ? OR (o.updatedAt = ? AND o.id <= ?))`
			args = append(args, cursorUpdatedAtRaw, cursorUpdatedAtRaw, *in.Cursor)
		}
	}

	rows, err := h.db.Query(
		`SELECT o.id, o.status, o.type, o.entityType, o.side, o.price, o.stopPrice, o.relativePrice, o.filledPrice, o.fee, o.symbol, o.exchangeAccountId, o.exchangeOrderId, o.quantity, o.smartTradeId, o.createdAt, o.placedAt, o.syncedAt, o.filledAt, o.updatedAt
FROM "Order" o
JOIN "SmartTrade" st ON st.id = o.smartTradeId
WHERE `+where+`
ORDER BY o.updatedAt DESC, o.id DESC
LIMIT ?`,
		append(args, limit+1)...,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	var orders []map[string]any
	var smartTradeIDs []int64
	for rows.Next() {
		m, err := scanOrder(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		orders = append(orders, m)
		smartTradeIDs = append(smartTradeIDs, m["smartTradeId"].(int64))
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	var nextCursor any = nil
	if len(orders) > limit {
		nextCursor = orders[len(orders)-1]["id"]
		orders = orders[:len(orders)-1]
	}

	items, trpcErr := h.buildOrdersWithSmartTrades(orders, smartTradeIDs)
	if trpcErr != nil {
		return nil, trpcErr
	}

	return map[string]any{
		"items":      items,
		"nextCursor": nextCursor,
	}, nil
}

func (h *Handler) buildOrdersWithSmartTrades(orders []map[string]any, smartTradeIDs []int64) (any, *TRPCError) {
	ordersByTrade, trpcErr := h.fetchOrdersBySmartTradeIDs(smartTradeIDs)
	if trpcErr != nil {
		return nil, trpcErr
	}
	smartTradesByID, trpcErr := h.fetchSmartTradesByIDs(smartTradeIDs)
	if trpcErr != nil {
		return nil, trpcErr
	}

	var exchangeAccountIDs []int64
	for _, st := range smartTradesByID {
		if st == nil {
			continue
		}
		exchangeAccountIDs = append(exchangeAccountIDs, st["exchangeAccountId"].(int64))
	}
	accountsByID, trpcErr := h.fetchExchangeAccountsByIDs(exchangeAccountIDs)
	if trpcErr != nil {
		return nil, trpcErr
	}

	smartTradeEntities := map[int64]map[string]any{}
	for id, st := range smartTradesByID {
		if st == nil {
			continue
		}
		st["orders"] = ordersByTrade[id]
		if acc, ok := accountsByID[st["exchangeAccountId"].(int64)]; ok {
			st["exchangeAccount"] = acc
		} else {
			st["exchangeAccount"] = nil
		}
		smartTradeEntities[id] = toSmartTradeEntity(st)
	}

	out := make([]any, 0, len(orders))
	for _, o := range orders {
		tid, _ := o["smartTradeId"].(int64)
		st := smartTradeEntities[tid]
		if st == nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: fmt.Sprintf("smartTrade %d not found", tid)}
		}
		out = append(out, map[string]any{
			// toOrderEntity keeps the same keys and adjusts nullable fields like TS.
			"smartTrade": st,
			// merge order fields after smartTrade for consistent property order (not important for JSON).
			// We'll add order fields now:
			"id":                o["id"],
			"status":            o["status"],
			"type":              o["type"],
			"entityType":        o["entityType"],
			"side":              o["side"],
			"price":             toOrderEntity(o)["price"],
			"stopPrice":         o["stopPrice"],
			"relativePrice":     o["relativePrice"],
			"filledPrice":       toOrderEntity(o)["filledPrice"],
			"fee":               o["fee"],
			"symbol":            o["symbol"],
			"exchangeAccountId": o["exchangeAccountId"],
			"exchangeOrderId":   o["exchangeOrderId"],
			"quantity":          o["quantity"],
			"smartTradeId":      o["smartTradeId"],
			"createdAt":         o["createdAt"],
			"placedAt":          toOrderEntity(o)["placedAt"],
			"syncedAt":          o["syncedAt"],
			"filledAt":          toOrderEntity(o)["filledAt"],
			"updatedAt":         o["updatedAt"],
		})
	}
	return out, nil
}
