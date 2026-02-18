package trpc

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const adminUserID int64 = 1

type exchangeAccountCreateInput struct {
	ExchangeCode   string  `json:"exchangeCode"`
	Name           string  `json:"name"`
	Label          *string `json:"label"`
	APIKey         string  `json:"apiKey"`
	SecretKey      string  `json:"secretKey"`
	Password       *string `json:"password"`
	IsDemoAccount  bool    `json:"isDemoAccount"`
	IsPaperAccount bool    `json:"isPaperAccount"`
}

type exchangeAccountUpdateInput struct {
	ID   int64 `json:"id"`
	Body struct {
		ExchangeCode   string  `json:"exchangeCode"`
		Name           string  `json:"name"`
		APIKey         string  `json:"apiKey"`
		SecretKey      string  `json:"secretKey"`
		Password       *string `json:"password"`
		IsDemoAccount  bool    `json:"isDemoAccount"`
		IsPaperAccount bool    `json:"isPaperAccount"`
	} `json:"body"`
}

type exchangeAccountDeleteInput struct {
	ID int64 `json:"id"`
}

type exchangeAccountCheckInput struct {
	ID int64 `json:"id"`
}

func (h *Handler) exchangeAccountList() (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	rows, err := h.db.Query(
		`SELECT id, name, label, exchangeCode, apiKey, secretKey, password, isDemoAccount, isPaperAccount, ownerId, createdAt, updatedAt, expired
FROM "ExchangeAccount"
WHERE ownerId = ?
ORDER BY id ASC`,
		adminUserID,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	defer rows.Close()

	var out []any
	for rows.Next() {
		m, err := scanExchangeAccount(rows)
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

func (h *Handler) exchangeAccountGetOne(id int64) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	row := h.db.QueryRow(
		`SELECT id, name, label, exchangeCode, apiKey, secretKey, password, isDemoAccount, isPaperAccount, ownerId, createdAt, updatedAt, expired
FROM "ExchangeAccount"
WHERE id = ? AND ownerId = ?`,
		id,
		adminUserID,
	)

	m, err := scanExchangeAccount(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &TRPCError{Code: CodeNotFound, Message: "exchange account not found"}
		}
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	return m, nil
}

func (h *Handler) exchangeAccountCreate(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in exchangeAccountCreateInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := h.db.Exec(
		`INSERT INTO "ExchangeAccount" ("name","label","exchangeCode","apiKey","secretKey","password","isDemoAccount","isPaperAccount","ownerId","createdAt","updatedAt","expired")
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name,
		nullableString(in.Label),
		in.ExchangeCode,
		in.APIKey,
		in.SecretKey,
		nullableString(in.Password),
		in.IsDemoAccount,
		in.IsPaperAccount,
		adminUserID,
		now,
		now,
		false,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return h.exchangeAccountGetOne(id)
}

func (h *Handler) exchangeAccountUpdate(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in exchangeAccountUpdateInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := h.db.Exec(
		`UPDATE "ExchangeAccount"
SET "exchangeCode"=?, "name"=?, "apiKey"=?, "secretKey"=?, "password"=?, "isDemoAccount"=?, "isPaperAccount"=?, "updatedAt"=?
WHERE id=? AND ownerId=?`,
		in.Body.ExchangeCode,
		in.Body.Name,
		in.Body.APIKey,
		in.Body.SecretKey,
		nullableString(in.Body.Password),
		in.Body.IsDemoAccount,
		in.Body.IsPaperAccount,
		now,
		in.ID,
		adminUserID,
	)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return h.exchangeAccountGetOne(in.ID)
}

func (h *Handler) exchangeAccountDelete(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in exchangeAccountDeleteInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	existing, trpcErr := h.exchangeAccountGetOne(in.ID)
	if trpcErr != nil {
		return nil, trpcErr
	}

	if _, err := h.db.Exec(`DELETE FROM "ExchangeAccount" WHERE id=? AND ownerId=?`, in.ID, adminUserID); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return existing, nil
}

func (h *Handler) exchangeAccountCheck(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in exchangeAccountCheckInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	// @todo implement real exchange credentials check in the Go rewrite.
	if _, err := h.db.Exec(`UPDATE "ExchangeAccount" SET "expired"=? WHERE id=? AND ownerId=?`, false, in.ID, adminUserID); err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}

	return map[string]any{
		"valid":   true,
		"message": "Credentials check is not implemented yet (Go rewrite); marked as valid",
	}, nil
}

type exchangeAccountScanner interface {
	Scan(dest ...any) error
}

func scanExchangeAccount(row exchangeAccountScanner) (map[string]any, error) {
	var (
		id           int64
		name         string
		label        sql.NullString
		exchangeCode string
		apiKey       string
		secretKey    string
		password     sql.NullString
		isDemo       any
		isPaper      any
		ownerID      int64
		createdAtRaw any
		updatedAtRaw any
		expiredRaw   any
	)

	if err := row.Scan(
		&id,
		&name,
		&label,
		&exchangeCode,
		&apiKey,
		&secretKey,
		&password,
		&isDemo,
		&isPaper,
		&ownerID,
		&createdAtRaw,
		&updatedAtRaw,
		&expiredRaw,
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

	return map[string]any{
		"id":             id,
		"name":           name,
		"label":          nullString(label),
		"exchangeCode":   exchangeCode,
		"apiKey":         apiKey,
		"secretKey":      secretKey,
		"password":       nullString(password),
		"isDemoAccount":  toBool(isDemo),
		"isPaperAccount": toBool(isPaper),
		"credentials": map[string]any{
			"code":           exchangeCode,
			"apiKey":         apiKey,
			"secretKey":      secretKey,
			"password":       nullString(password),
			"isDemoAccount":  toBool(isDemo),
			"isPaperAccount": toBool(isPaper),
		},
		"ownerId":   ownerID,
		"createdAt": createdAt,
		"updatedAt": updatedAt,
		"expired":   toBool(expiredRaw),
	}, nil
}

func decodeInput(input any, out any) *TRPCError {
	b, err := json.Marshal(input)
	if err != nil {
		return &TRPCError{Code: CodeBadRequest, Message: "invalid input"}
	}
	if err := json.Unmarshal(b, out); err != nil {
		return &TRPCError{Code: CodeBadRequest, Message: err.Error()}
	}
	return nil
}

func parseDBTime(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t, nil
	case string:
		return parseTimeString(t)
	case []byte:
		return parseTimeString(string(t))
	default:
		return time.Time{}, fmt.Errorf("unsupported time value: %T", v)
	}
}

func parseTimeString(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05.000000000",
		"2006-01-02 15:04:05.000000",
		"2006-01-02 15:04:05.000",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts, nil
		}
	}

	return time.Time{}, fmt.Errorf("unparseable time: %q", s)
}

func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	if strings.TrimSpace(*s) == "" {
		return nil
	}
	return *s
}

func nullString(ns sql.NullString) any {
	if !ns.Valid {
		return nil
	}
	return ns.String
}

func toBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case int:
		return t != 0
	case []byte:
		return string(t) != "0"
	case string:
		return t != "0" && strings.ToLower(t) != "false"
	default:
		return false
	}
}
