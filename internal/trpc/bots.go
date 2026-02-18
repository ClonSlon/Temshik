package trpc

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type botScanner interface {
	Scan(dest ...any) error
}

func scanBot(row botScanner) (map[string]any, error) {
	var (
		id                int64
		typ               string
		name              string
		label             sql.NullString
		symbol            string
		enabledRaw        any
		loggingRaw        any
		template          string
		timeframe         sql.NullString
		processingRaw     any
		createdAtRaw      any
		settingsRaw       string
		stateRaw          string
		exchangeAccountID int64
		ownerID           int64
	)

	if err := row.Scan(
		&id,
		&typ,
		&name,
		&label,
		&symbol,
		&enabledRaw,
		&loggingRaw,
		&template,
		&timeframe,
		&processingRaw,
		&createdAtRaw,
		&settingsRaw,
		&stateRaw,
		&exchangeAccountID,
		&ownerID,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseDBTime(createdAtRaw)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"id":                id,
		"type":              typ,
		"name":              name,
		"label":             nullString(label),
		"symbol":            symbol,
		"enabled":           toBool(enabledRaw),
		"logging":           toBool(loggingRaw),
		"template":          template,
		"timeframe":         nullString(timeframe),
		"processing":        toBool(processingRaw),
		"createdAt":         createdAt,
		"settings":          parseJSONOrEmptyObject(settingsRaw),
		"state":             parseJSONOrEmptyObject(stateRaw),
		"exchangeAccountId": exchangeAccountID,
		"ownerId":           ownerID,
	}, nil
}

func (h *Handler) botList() (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	rows, err := h.db.Query(
		`SELECT id, type, name, label, symbol, enabled, logging, template, timeframe, processing, createdAt, settings, state, exchangeAccountId, ownerId
FROM "Bot"
WHERE ownerId = ?
ORDER BY createdAt DESC`,
		adminUserID,
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

func (h *Handler) botGetOne(id int64) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	row := h.db.QueryRow(
		`SELECT id, type, name, label, symbol, enabled, logging, template, timeframe, processing, createdAt, settings, state, exchangeAccountId, ownerId
FROM "Bot"
WHERE id = ? AND ownerId = ?`,
		id,
		adminUserID,
	)

	m, err := scanBot(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &TRPCError{Code: CodeNotFound, Message: "bot not found"}
		}
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	return m, nil
}

type botCreateInput struct {
	ExchangeAccountID int64 `json:"exchangeAccountId"`
	Data              struct {
		Name      string  `json:"name"`
		Symbol    string  `json:"symbol"`
		Settings  any     `json:"settings"`
		Template  string  `json:"template"`
		Timeframe *string `json:"timeframe"`
		Logging   bool    `json:"logging"`
	} `json:"data"`
}

func (h *Handler) botCreate(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in botCreateInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	exAcc, trpcErr := h.exchangeAccountGetOne(in.ExchangeAccountID)
	if trpcErr != nil {
		return nil, trpcErr
	}

	settingsBytes, _ := json.Marshal(in.Data.Settings)
	if len(settingsBytes) == 0 {
		settingsBytes = []byte(`{}`)
	}

	state := "{}"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	res, err := h.db.Exec(
		`INSERT INTO "Bot" ("type","name","label","symbol","enabled","logging","template","timeframe","processing","createdAt","settings","state","exchangeAccountId","ownerId")
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Bot",
		in.Data.Name,
		nil,
		in.Data.Symbol,
		false,
		in.Data.Logging,
		in.Data.Template,
		nullableString(in.Data.Timeframe),
		false,
		now,
		string(settingsBytes),
		state,
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

	bot, trpcErr := h.botGetOne(id)
	if trpcErr != nil {
		return nil, trpcErr
	}
	if bm, ok := bot.(map[string]any); ok {
		bm["exchangeAccount"] = exAcc
		return bm, nil
	}
	return bot, nil
}

type botUpdateInput struct {
	BotID int64 `json:"botId"`
	Data  struct {
		Name              string  `json:"name"`
		Symbol            string  `json:"symbol"`
		Settings          any     `json:"settings"`
		Template          string  `json:"template"`
		Timeframe         *string `json:"timeframe"`
		ExchangeAccountID int64   `json:"exchangeAccountId"`
		Logging           bool    `json:"logging"`
	} `json:"data"`
}

func (h *Handler) botUpdate(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in botUpdateInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	existingAny, trpcErr := h.botGetOne(in.BotID)
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
SET "name"=?, "symbol"=?, "settings"=?, "template"=?, "timeframe"=?, "exchangeAccountId"=?, "logging"=?
WHERE id=? AND ownerId=?`,
		in.Data.Name,
		in.Data.Symbol,
		string(settingsBytes),
		in.Data.Template,
		nullableString(in.Data.Timeframe),
		in.Data.ExchangeAccountID,
		in.Data.Logging,
		in.BotID,
		adminUserID,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return h.botGetOne(in.BotID)
}

type botDeleteInput struct {
	BotID int64 `json:"botId"`
}

func (h *Handler) botDelete(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in botDeleteInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	existingAny, trpcErr := h.botGetOne(in.BotID)
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

	if _, err := h.db.Exec(`DELETE FROM "Bot" WHERE id=? AND ownerId=?`, in.BotID, adminUserID); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return map[string]any{
		"message":    "Deleted successfully",
		"deletedBot": existingAny,
	}, nil
}

type botStartStopInput struct {
	BotID int64 `json:"botId"`
}

func (h *Handler) botStart(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in botStartStopInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	existingAny, trpcErr := h.botGetOne(in.BotID)
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

	if _, err := h.db.Exec(`UPDATE "Bot" SET "enabled"=? WHERE id=? AND ownerId=?`, true, in.BotID, adminUserID); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return map[string]any{"ok": true}, nil
}

func (h *Handler) botStop(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in botStartStopInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	existingAny, trpcErr := h.botGetOne(in.BotID)
	if trpcErr != nil {
		return nil, trpcErr
	}
	existing, _ := existingAny.(map[string]any)
	if !toBool(existing["enabled"]) {
		return nil, &TRPCError{Code: CodeConflict, Message: "Bot already stopped"}
	}

	if _, err := h.db.Exec(`UPDATE "Bot" SET "enabled"=? WHERE id=? AND ownerId=?`, false, in.BotID, adminUserID); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return map[string]any{"ok": true}, nil
}

func (h *Handler) botBacktest(_ any) (any, *TRPCError) {
	// Keep parity with the TS backend: it returns null (deprecated).
	return nil, nil
}

func (h *Handler) botGetStrategies() (any, *TRPCError) {
	// Minimal list to keep UI functional; schema is JSON Schema (draft-07-ish).
	emptyObjectSchema := map[string]any{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": true,
	}

	return map[string]any{
		"grid": map[string]any{
			"name":            "grid",
			"schema":          emptyObjectSchema,
			"isCustom":        false,
			"displayName":     "Grid Bot",
			"description":     "Grid trading strategy (built-in).",
			"hidden":          true,
			"runPolicy":       map[string]any{"onTradeCompleted": true},
			"watchers":        map[string]any{},
			"requiredHistory": nil,
		},
		"dca": map[string]any{
			"name":            "dca",
			"schema":          emptyObjectSchema,
			"isCustom":        false,
			"displayName":     "DCA Bot",
			"description":     "Dollar-cost averaging strategy (built-in).",
			"hidden":          true,
			"runPolicy":       map[string]any{"onOrderFilled": true, "onCandleClosed": true},
			"watchers":        map[string]any{},
			"requiredHistory": nil,
		},
		"rsi": map[string]any{
			"name":            "rsi",
			"schema":          emptyObjectSchema,
			"isCustom":        false,
			"displayName":     "RSI Strategy",
			"description":     "RSI-based example strategy.",
			"hidden":          false,
			"runPolicy":       map[string]any{"onCandleClosed": true},
			"watchers":        map[string]any{},
			"requiredHistory": 15,
		},
	}, nil
}

func (h *Handler) botAssertOwned(botID int64) (map[string]any, *TRPCError) {
	anyBot, err := h.botGetOne(botID)
	if err != nil {
		return nil, err
	}
	m, ok := anyBot.(map[string]any)
	if !ok {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: fmt.Sprintf("unexpected bot type: %T", anyBot)}
	}
	return m, nil
}
