package trpc

import (
	"database/sql"
	"encoding/json"
	"time"
)

type gridBotCreateInput struct {
	ExchangeAccountID int64 `json:"exchangeAccountId"`
	Data              struct {
		Name     string `json:"name"`
		Symbol   string `json:"symbol"`
		Settings any    `json:"settings"`
	} `json:"data"`
}

type gridBotUpdateInput struct {
	BotID int64 `json:"botId"`
	Data  struct {
		Name              string `json:"name"`
		Symbol            string `json:"symbol"`
		Settings          any    `json:"settings"`
		ExchangeAccountID int64  `json:"exchangeAccountId"`
	} `json:"data"`
}

type gridBotFormOptionsInput struct {
	SymbolID      string `json:"symbolId"`
	IsDemoAccount bool   `json:"isDemoAccount"`
}

func (h *Handler) gridBotList() (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	rows, err := h.db.Query(
		`SELECT id, type, name, label, symbol, enabled, logging, template, timeframe, processing, createdAt, settings, state, exchangeAccountId, ownerId
FROM "Bot"
WHERE ownerId = ? AND type = ?
ORDER BY createdAt DESC`,
		adminUserID,
		"GridBot",
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

func (h *Handler) gridBotGetOne(id int64) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	row := h.db.QueryRow(
		`SELECT id, type, name, label, symbol, enabled, logging, template, timeframe, processing, createdAt, settings, state, exchangeAccountId, ownerId
FROM "Bot"
WHERE id = ? AND ownerId = ? AND type = ?`,
		id,
		adminUserID,
		"GridBot",
	)

	m, err := scanBot(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &TRPCError{Code: CodeNotFound, Message: "grid bot not found"}
		}
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	return m, nil
}

func (h *Handler) gridBotCreate(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in gridBotCreateInput
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
		"GridBot",
		in.Data.Name,
		nil,
		in.Data.Symbol,
		false,
		true,
		"gridBot",
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

	return h.gridBotGetOne(id)
}

func (h *Handler) gridBotUpdate(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in gridBotUpdateInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	existingAny, trpcErr := h.gridBotGetOne(in.BotID)
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
		"GridBot",
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return h.gridBotGetOne(in.BotID)
}

func (h *Handler) gridBotFormOptions(input any) (any, *TRPCError) {
	var in gridBotFormOptionsInput
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
	highPrice := price + price*0.3
	lowPrice := price - price*0.3

	return map[string]any{
		"highPrice": highPrice,
		"lowPrice":  lowPrice,
	}, nil
}
