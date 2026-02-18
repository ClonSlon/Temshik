package trpc

import (
	"database/sql"
	"strings"
)

func (h *Handler) fetchExchangeAccountsByIDs(ids []int64) (map[int64]map[string]any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

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
		return map[int64]map[string]any{}, nil
	}

	placeholders := strings.Repeat("?,", len(uniq))
	placeholders = strings.TrimSuffix(placeholders, ",")

	args := make([]any, 0, len(uniq)+1)
	for _, id := range uniq {
		args = append(args, id)
	}
	args = append(args, adminUserID)

	rows, err := h.db.Query(
		`SELECT id, name, label, exchangeCode, apiKey, secretKey, password, isDemoAccount, isPaperAccount, ownerId, createdAt, updatedAt, expired
FROM "ExchangeAccount"
WHERE id IN (`+placeholders+`) AND ownerId = ?`,
		args...,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	out := map[int64]map[string]any{}
	for rows.Next() {
		m, err := scanExchangeAccount(rows)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		if idv, ok := m["id"].(int64); ok {
			out[idv] = m
		}
	}
	if err := rows.Err(); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	// Ensure stable shape even if some ids missing (deleted).
	for _, id := range uniq {
		if _, ok := out[id]; !ok {
			out[id] = nil
		}
	}

	return out, nil
}

func (h *Handler) exchangeAccountExists(id int64) (bool, error) {
	if h.db == nil {
		return false, sql.ErrConnDone
	}
	var v int
	err := h.db.QueryRow(`SELECT 1 FROM "ExchangeAccount" WHERE id=? AND ownerId=?`, id, adminUserID).Scan(&v)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
