package trpc

import (
	"database/sql"
	"encoding/json"
	"time"
)

type dcaBotCreateInput struct {
	ExchangeAccountID int64 `json:"exchangeAccountId"`
	Data              struct {
		Name     string `json:"name"`
		Symbol   string `json:"symbol"`
		Settings any    `json:"settings"`
	} `json:"data"`
}

type dcaBotUpdateInput struct {
	BotID int64 `json:"botId"`
	Data  struct {
		Name              string `json:"name"`
		Symbol            string `json:"symbol"`
		Settings          any    `json:"settings"`
		ExchangeAccountID int64  `json:"exchangeAccountId"`
	} `json:"data"`
}

type dcaBotTradesInput struct {
	BotID *int64 `json:"botId"`
}

type dcaBotFormOptionsInput struct {
	SymbolID      string `json:"symbolId"`
	IsDemoAccount bool   `json:"isDemoAccount"`
}

func (h *Handler) dcaBotList() (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	rows, err := h.db.Query(
		`SELECT id, type, name, label, symbol, enabled, logging, template, timeframe, processing, createdAt, settings, state, exchangeAccountId, ownerId
FROM "Bot"
WHERE ownerId = ? AND type = ?
ORDER BY createdAt DESC`,
		adminUserID,
		"DcaBot",
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	var out []any
	for rows.Next() {
		m, err := scanBot(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return out, nil
}

func (h *Handler) dcaBotGetOne(id int64) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	row := h.db.QueryRow(
		`SELECT id, type, name, label, symbol, enabled, logging, template, timeframe, processing, createdAt, settings, state, exchangeAccountId, ownerId
FROM "Bot"
WHERE id = ? AND ownerId = ? AND type = ?`,
		id,
		adminUserID,
		"DcaBot",
	)

	m, err := scanBot(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &TRPCError{Code: CodeNotFound, Message: "dca bot not found"}
		}
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	return m, nil
}

func (h *Handler) dcaBotCreate(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in dcaBotCreateInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	if _, trpcErr := h.exchangeAccountGetOne(in.ExchangeAccountID); trpcErr != nil {
		return nil, trpcErr
	}

	settingsBytes, _ := json.Marshal(in.Data.Settings)
	if len(settingsBytes) == 0 {
		settingsBytes = []byte(`{}`)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := h.db.Exec(
		`INSERT INTO "Bot" ("type","name","label","symbol","enabled","logging","template","timeframe","processing","createdAt","settings","state","exchangeAccountId","ownerId")
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"DcaBot",
		in.Data.Name,
		nil,
		in.Data.Symbol,
		false,
		true,
		"dca",
		nil,
		false,
		now,
		string(settingsBytes),
		"{}",
		in.ExchangeAccountID,
		adminUserID,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return h.dcaBotGetOne(id)
}

func (h *Handler) dcaBotUpdate(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in dcaBotUpdateInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	existingAny, trpcErr := h.dcaBotGetOne(in.BotID)
	if trpcErr != nil {
		return nil, trpcErr
	}
	existing, _ := existingAny.(map[string]any)
	if toBool(existing["enabled"]) {
		return nil, &TRPCError{Code: CodeConflict, Message: "Bot already running. Please stop the bot first."}
	}
	if toBool(existing["processing"]) {
		return nil, &TRPCError{Code: CodeConflict, Message: "The bot is busy with the previous processing job"}
	}

	if _, trpcErr := h.exchangeAccountGetOne(in.Data.ExchangeAccountID); trpcErr != nil {
		return nil, trpcErr
	}

	settingsBytes, _ := json.Marshal(in.Data.Settings)
	if len(settingsBytes) == 0 {
		settingsBytes = []byte(`{}`)
	}

	_, err := h.db.Exec(
		`UPDATE "Bot"
SET "name"=?, "symbol"=?, "settings"=?, "exchangeAccountId"=?
WHERE id=? AND ownerId=? AND type=?`,
		in.Data.Name,
		in.Data.Symbol,
		string(settingsBytes),
		in.Data.ExchangeAccountID,
		in.BotID,
		adminUserID,
		"DcaBot",
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return h.dcaBotGetOne(in.BotID)
}

func (h *Handler) dcaBotGetTrades(input any) (any, *TRPCError) {
	var in dcaBotTradesInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	where := `ownerId = ? AND type = ?`
	args := []any{adminUserID, "DCA"}
	if in.BotID != nil {
		where += ` AND botId = ?`
		args = append(args, *in.BotID)
	}

	rows, err := h.db.Query(
		`SELECT id, type, entryType, takeProfitType, symbol, ref, exchangeAccountId, botId, ownerId, createdAt, updatedAt
FROM "SmartTrade"
WHERE `+where+`
ORDER BY createdAt DESC`,
		args...,
	)
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

	out := make([]any, 0, len(trades))
	for _, t := range trades {
		id := t["id"].(int64)
		t["orders"] = ordersByTrade[id]
		if acc, ok := accountsByID[t["exchangeAccountId"].(int64)]; ok {
			t["exchangeAccount"] = acc
		} else {
			t["exchangeAccount"] = nil
		}
		out = append(out, toSmartTradeEntity(t))
	}

	return out, nil
}

func (h *Handler) dcaBotFormOptions(input any) (any, *TRPCError) {
	var in dcaBotFormOptionsInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	_, pair, err := decomposeSymbolID(in.SymbolID)
	if err != nil {
		return nil, &TRPCError{Code: CodeBadRequest, Message: err.Error()}
	}

	price, err := fetchBinancePrice(pair)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	return map[string]any{"price": price}, nil
}
