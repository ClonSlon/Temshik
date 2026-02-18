package trpc

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Code string

const (
	CodeParseError           Code = "PARSE_ERROR"
	CodeBadRequest           Code = "BAD_REQUEST"
	CodeInternalServerError  Code = "INTERNAL_SERVER_ERROR"
	CodeUnauthorized         Code = "UNAUTHORIZED"
	CodeForbidden            Code = "FORBIDDEN"
	CodeNotFound             Code = "NOT_FOUND"
	CodeConflict             Code = "CONFLICT"
	CodeMethodNotSupported   Code = "METHOD_NOT_SUPPORTED"
	CodeNotImplemented       Code = "NOT_IMPLEMENTED"
	CodeUnsupportedMediaType Code = "UNSUPPORTED_MEDIA_TYPE"
)

func jsonRPCCode(code Code) int {
	switch code {
	case CodeParseError:
		return -32700
	case CodeBadRequest:
		return -32600
	case CodeUnauthorized:
		return -32001
	case CodeForbidden:
		return -32003
	case CodeNotFound:
		return -32004
	case CodeConflict:
		return -32009
	case CodeMethodNotSupported:
		return -32005
	case CodeNotImplemented:
		return -32603
	case CodeUnsupportedMediaType:
		return -32015
	default:
		return -32603
	}
}

func httpStatus(code Code) int {
	switch code {
	case CodeParseError, CodeBadRequest:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeMethodNotSupported:
		return http.StatusMethodNotAllowed
	case CodeUnsupportedMediaType:
		return http.StatusUnsupportedMediaType
	default:
		return http.StatusInternalServerError
	}
}

type TRPCError struct {
	Code    Code
	Message string
}

func (e *TRPCError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return string(e.Code)
}

func (e *TRPCError) shape(path string) map[string]any {
	msg := e.Message
	if msg == "" {
		msg = string(e.Code)
	}
	return map[string]any{
		"message": msg,
		"code":    jsonRPCCode(e.Code),
		"data": map[string]any{
			"code":       string(e.Code),
			"httpStatus": httpStatus(e.Code),
			"path":       path,
		},
	}
}

type Handler struct {
	adminPassword string
	db            *sql.DB
}

func NewHandler(adminPassword string, db *sql.DB) *Handler {
	return &Handler{adminPassword: adminPassword, db: db}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/trpc/")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": (&TRPCError{Code: CodeNotFound, Message: "missing procedure path"}).shape(""),
		})
		return
	}

	paths := strings.Split(trimmed, ",")
	isBatch := len(paths) > 1 || r.URL.Query().Get("batch") == "1"

	var rawInput any
	if r.Method == http.MethodGet {
		inputParam := r.URL.Query().Get("input")
		if inputParam != "" {
			if err := json.Unmarshal([]byte(inputParam), &rawInput); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"error": (&TRPCError{Code: CodeParseError, Message: "invalid input"}).shape(strings.Join(paths, ",")),
				})
				return
			}
		}
	} else if r.Method == http.MethodPost {
		if ct := r.Header.Get("content-type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{
				"error": (&TRPCError{Code: CodeUnsupportedMediaType, Message: "content-type must be application/json"}).shape(strings.Join(paths, ",")),
			})
			return
		}
		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&rawInput); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": (&TRPCError{Code: CodeParseError, Message: "invalid json body"}).shape(strings.Join(paths, ",")),
			})
			return
		}
	} else if r.Method == http.MethodOptions {
		w.Header().Set("access-control-allow-origin", "*")
		w.Header().Set("access-control-allow-methods", "GET,POST,OPTIONS")
		w.Header().Set("access-control-allow-headers", "content-type,authorization")
		w.WriteHeader(http.StatusNoContent)
		return
	} else {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error": (&TRPCError{Code: CodeMethodNotSupported}).shape(strings.Join(paths, ",")),
		})
		return
	}

	authorized := r.Header.Get("Authorization") != "" && r.Header.Get("Authorization") == h.adminPassword

	if isBatch {
		inputsMap, _ := rawInput.(map[string]any)
		responses := make([]any, len(paths))
		for i, p := range paths {
			key := strconv.Itoa(i)
			var in any
			if inputsMap != nil {
				in = inputsMap[key]
			}
			responses[i] = h.dispatch(p, in, authorized)
		}
		writeJSON(w, http.StatusOK, responses)
		return
	}

	resp := h.dispatch(paths[0], rawInput, authorized)

	if m, ok := resp.(map[string]any); ok {
		if errObj, hasError := m["error"]; hasError && errObj != nil {
			if errMap, ok := errObj.(map[string]any); ok {
				if data, ok := errMap["data"].(map[string]any); ok {
					if hs, ok := data["httpStatus"].(int); ok {
						writeJSON(w, hs, resp)
						return
					}
					if hs, ok := data["httpStatus"].(json.Number); ok {
						if v, err := hs.Int64(); err == nil {
							writeJSON(w, int(v), resp)
							return
						}
					}
				}
			}
			writeJSON(w, http.StatusInternalServerError, resp)
			return
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) dispatch(path string, input any, authorized bool) any {
	input = unwrapInput(input)

	if path == "public.healhcheck" {
		return success(map[string]any{
			"status": "ok",
			"time":   time.Now().UnixMilli(),
		})
	}

	if !authorized {
		return map[string]any{
			"error": (&TRPCError{Code: CodeUnauthorized}).shape(path),
		}
	}

	switch path {
	case "exchangeAccount.list":
		out, err := h.exchangeAccountList()
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "exchangeAccount.getOne":
		var id int64
		if err := decodeInput(input, &id); err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		out, err := h.exchangeAccountGetOne(id)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "exchangeAccount.create":
		out, err := h.exchangeAccountCreate(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "exchangeAccount.update":
		out, err := h.exchangeAccountUpdate(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "exchangeAccount.delete":
		out, err := h.exchangeAccountDelete(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "exchangeAccount.check":
		out, err := h.exchangeAccountCheck(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.list":
		out, err := h.botList()
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.getOne":
		var id int64
		if err := decodeInput(input, &id); err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		out, err := h.botGetOne(id)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.create":
		out, err := h.botCreate(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.update":
		out, err := h.botUpdate(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.delete":
		out, err := h.botDelete(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.start":
		out, err := h.botStart(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.stop":
		out, err := h.botStop(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.backtest":
		out, err := h.botBacktest(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.openSmartTrades":
		out, err := h.botOpenSmartTrades(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.activeSmartTrades":
		out, err := h.botActiveSmartTrades(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.pendingSmartTrades":
		out, err := h.botPendingSmartTrades(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.completedSmartTrades":
		out, err := h.botCompletedSmartTrades(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.orders":
		out, err := h.botOrders(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.getBotLogs":
		out, err := h.botGetBotLogs(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "bot.getStrategies":
		out, err := h.botGetStrategies()
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "gridBot.list":
		out, err := h.gridBotList()
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "gridBot.getOne":
		var id int64
		if err := decodeInput(input, &id); err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		out, err := h.gridBotGetOne(id)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "gridBot.create":
		out, err := h.gridBotCreate(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "gridBot.update":
		out, err := h.gridBotUpdate(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "gridBot.formOptions":
		out, err := h.gridBotFormOptions(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "dcaBot.list":
		out, err := h.dcaBotList()
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "dcaBot.getOne":
		var id int64
		if err := decodeInput(input, &id); err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		out, err := h.dcaBotGetOne(id)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "dcaBot.create":
		out, err := h.dcaBotCreate(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "dcaBot.update":
		out, err := h.dcaBotUpdate(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "dcaBot.getTrades":
		out, err := h.dcaBotGetTrades(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "dcaBot.formOptions":
		out, err := h.dcaBotFormOptions(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "smartTrade.list":
		out, err := h.smartTradeList(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "smartTrade.infiniteList":
		out, err := h.smartTradeInfiniteList(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "smartTrade.getOne":
		var id int64
		if err := decodeInput(input, &id); err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		out, err := h.smartTradeGetOne(id)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "order.openOrders":
		out, err := h.orderOpenOrders(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "order.closedOrders":
		out, err := h.orderClosedOrders(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "order.infiniteOrders":
		out, err := h.orderInfiniteOrders(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "symbol.list":
		out, err := h.symbolList(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "symbol.getOne":
		out, err := h.symbolGetOne(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "symbol.price":
		out, err := h.symbolPrice(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "candles.list":
		out, err := h.candlesList(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "exchange.getAssets":
		out, err := h.exchangeGetAssets(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	case "exchange.getTicker":
		out, err := h.exchangeGetTicker(input)
		if err != nil {
			return map[string]any{"error": err.shape(path)}
		}
		return success(out)
	default:
		return map[string]any{
			"error": (&TRPCError{Code: CodeNotFound, Message: "procedure not implemented"}).shape(path),
		}
	}
}

func success(data any) map[string]any {
	return map[string]any{
		"result": map[string]any{
			"data": superJSONSerialize(data),
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func unwrapInput(input any) any {
	m, ok := input.(map[string]any)
	if !ok {
		return input
	}
	if jsonVal, ok := m["json"]; ok {
		return jsonVal
	}
	return input
}
