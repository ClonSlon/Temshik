package trpc

import (
	"database/sql"
	"strings"
)

type smartTradeScanner interface {
	Scan(dest ...any) error
}

func scanSmartTrade(row smartTradeScanner) (map[string]any, error) {
	var (
		id                int64
		typ               string
		entryType         string
		takeProfitType    string
		symbol            string
		ref               sql.NullString
		exchangeAccountID int64
		botID             sql.NullInt64
		ownerID           int64
		createdAtRaw      any
		updatedAtRaw      any
	)

	if err := row.Scan(
		&id,
		&typ,
		&entryType,
		&takeProfitType,
		&symbol,
		&ref,
		&exchangeAccountID,
		&botID,
		&ownerID,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return nil, err
	}

	createdAt, err := parseDBTime(createdAtRaw)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseDBTime(updatedAtRaw)
	if err != nil {
		return nil, err
	}

	var bot any = nil
	if botID.Valid {
		bot = botID.Int64
	}

	return map[string]any{
		"id":                id,
		"type":              typ,
		"entryType":         entryType,
		"takeProfitType":    takeProfitType,
		"symbol":            symbol,
		"ref":               nullString(ref),
		"exchangeAccountId": exchangeAccountID,
		"botId":             bot,
		"ownerId":           ownerID,
		"createdAt":         createdAt,
		"updatedAt":         updatedAt,
	}, nil
}

type smartTradeListInput struct {
	BotID *int64 `json:"botId"`
}

func (h *Handler) smartTradeList(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in smartTradeListInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	where := `ownerId = ?`
	args := []any{adminUserID}
	if in.BotID != nil {
		where += ` AND botId = ?`
		args = append(args, *in.BotID)
	}

	rows, err := h.db.Query(
		`SELECT id, type, entryType, takeProfitType, symbol, ref, exchangeAccountId, botId, ownerId, createdAt, updatedAt
FROM "SmartTrade"
WHERE `+where+`
LIMIT 1000`,
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

type smartTradeInfiniteListInput struct {
	BotID  *int64 `json:"botId"`
	Limit  *int   `json:"limit"`
	Cursor *int64 `json:"cursor"`
}

func (h *Handler) smartTradeInfiniteList(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in smartTradeInfiniteListInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	limit := 50
	if in.Limit != nil {
		limit = clampInt(*in.Limit, 1, 100)
	}

	where := `ownerId = ?`
	args := []any{adminUserID}
	if in.BotID != nil {
		where += ` AND botId = ?`
		args = append(args, *in.BotID)
	}

	if in.Cursor != nil {
		var cursorUpdatedAtRaw any
		err := h.db.QueryRow(`SELECT updatedAt FROM "SmartTrade" WHERE id=?`, *in.Cursor).Scan(&cursorUpdatedAtRaw)
		if err == nil {
			// Use raw value for stable SQLite comparisons.
			where += ` AND (updatedAt < ? OR (updatedAt = ? AND id <= ?))`
			args = append(args, cursorUpdatedAtRaw, cursorUpdatedAtRaw, *in.Cursor)
		}
	}

	rows, err := h.db.Query(
		`SELECT id, type, entryType, takeProfitType, symbol, ref, exchangeAccountId, botId, ownerId, createdAt, updatedAt
FROM "SmartTrade"
WHERE `+where+`
ORDER BY updatedAt DESC, id DESC
LIMIT ?`,
		append(args, limit+1)...,
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

	var nextCursor any = nil
	if len(trades) > limit {
		next := trades[len(trades)-1]
		nextCursor = next["id"]
		trades = trades[:len(trades)-1]
	}

	ordersByTrade, trpcErr := h.fetchOrdersBySmartTradeIDs(smartTradeIDs)
	if trpcErr != nil {
		return nil, trpcErr
	}
	accountsByID, trpcErr := h.fetchExchangeAccountsByIDs(exchangeAccountIDs)
	if trpcErr != nil {
		return nil, trpcErr
	}

	items := make([]any, 0, len(trades))
	for _, t := range trades {
		id := t["id"].(int64)
		t["orders"] = ordersByTrade[id]
		if acc, ok := accountsByID[t["exchangeAccountId"].(int64)]; ok {
			t["exchangeAccount"] = acc
		} else {
			t["exchangeAccount"] = nil
		}
		items = append(items, toSmartTradeEntity(t))
	}

	return map[string]any{
		"items":      items,
		"nextCursor": nextCursor,
	}, nil
}

func (h *Handler) smartTradeGetOne(id int64) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	row := h.db.QueryRow(
		`SELECT id, type, entryType, takeProfitType, symbol, ref, exchangeAccountId, botId, ownerId, createdAt, updatedAt
FROM "SmartTrade"
WHERE id = ?`,
		id,
	)

	t, err := scanSmartTrade(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &TRPCError{Code: CodeNotFound, Message: "smartTrade not found"}
		}
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	ordersByTrade, trpcErr := h.fetchOrdersBySmartTradeIDs([]int64{id})
	if trpcErr != nil {
		return nil, trpcErr
	}
	accountsByID, trpcErr := h.fetchExchangeAccountsByIDs([]int64{t["exchangeAccountId"].(int64)})
	if trpcErr != nil {
		return nil, trpcErr
	}

	t["orders"] = ordersByTrade[id]
	if acc, ok := accountsByID[t["exchangeAccountId"].(int64)]; ok {
		t["exchangeAccount"] = acc
	} else {
		t["exchangeAccount"] = nil
	}

	return toSmartTradeEntity(t), nil
}

func buildInClause(ids []int64) (string, []any) {
	uniq := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return "", nil
	}

	var b strings.Builder
	args := make([]any, 0, len(uniq))
	for i, id := range uniq {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("?")
		args = append(args, id)
	}
	return b.String(), args
}
