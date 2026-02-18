package trpc

import (
	"database/sql"
	"encoding/json"
	"time"
)

type botLogsInput struct {
	BotID  int64  `json:"botId"`
	Limit  *int   `json:"limit"`
	Cursor *int64 `json:"cursor"`
}

type botLogScanner interface {
	Scan(dest ...any) error
}

func scanBotLog(row botLogScanner) (map[string]any, error) {
	var (
		id               int64
		action           string
		triggerEventType sql.NullString
		contextRaw       sql.NullString
		errorRaw         sql.NullString
		startedAtRaw     any
		endedAtRaw       any
		createdAtRaw     any
		botID            int64
	)

	if err := row.Scan(
		&id,
		&action,
		&triggerEventType,
		&contextRaw,
		&errorRaw,
		&startedAtRaw,
		&endedAtRaw,
		&createdAtRaw,
		&botID,
	); err != nil {
		return nil, err
	}

	startedAt, err := parseDBTime(startedAtRaw)
	if err != nil {
		return nil, err
	}
	endedAt, err := parseDBTime(endedAtRaw)
	if err != nil {
		return nil, err
	}
	createdAt, err := parseDBTime(createdAtRaw)
	if err != nil {
		return nil, err
	}

	out := map[string]any{
		"id":               id,
		"action":           action,
		"triggerEventType": nullString(triggerEventType),
		"startedAt":        startedAt,
		"endedAt":          endedAt,
		"createdAt":        createdAt,
		"botId":            botID,
	}

	if contextRaw.Valid && contextRaw.String != "" {
		var v any
		if err := json.Unmarshal([]byte(contextRaw.String), &v); err == nil {
			out["context"] = v
		}
	}
	if errorRaw.Valid && errorRaw.String != "" {
		var v any
		if err := json.Unmarshal([]byte(errorRaw.String), &v); err == nil {
			out["error"] = v
		}
	}

	return out, nil
}

func (h *Handler) botGetBotLogs(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in botLogsInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	limit := 50
	if in.Limit != nil {
		limit = clampInt(*in.Limit, 1, 100)
	}

	where := `botId = ?`
	args := []any{in.BotID}

	if in.Cursor != nil {
		var cursorCreatedAtRaw any
		err := h.db.QueryRow(`SELECT createdAt FROM "BotLog" WHERE id=?`, *in.Cursor).Scan(&cursorCreatedAtRaw)
		if err == nil {
			// Use raw value for stable SQLite comparisons.
			where += ` AND (createdAt < ? OR (createdAt = ? AND id <= ?))`
			args = append(args, cursorCreatedAtRaw, cursorCreatedAtRaw, *in.Cursor)
		}
	}

	rows, err := h.db.Query(
		`SELECT id, action, triggerEventType, context, error, startedAt, endedAt, createdAt, botId
FROM "BotLog"
WHERE `+where+`
ORDER BY createdAt DESC, id DESC
LIMIT ?`,
		append(args, limit+1)...,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	var items []map[string]any
	for rows.Next() {
		m, err := scanBotLog(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	var nextCursor any = nil
	if len(items) > limit {
		nextCursor = items[len(items)-1]["id"]
		items = items[:len(items)-1]
	}

	outItems := make([]any, 0, len(items))
	for _, it := range items {
		// Ensure JS client sees time fields as SuperJSON Dates.
		if t, ok := it["startedAt"].(time.Time); ok {
			it["startedAt"] = t
		}
		if t, ok := it["endedAt"].(time.Time); ok {
			it["endedAt"] = t
		}
		if t, ok := it["createdAt"].(time.Time); ok {
			it["createdAt"] = t
		}
		outItems = append(outItems, it)
	}

	return map[string]any{
		"items":      outItems,
		"nextCursor": nextCursor,
	}, nil
}
