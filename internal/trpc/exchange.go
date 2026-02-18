package trpc

type exchangeAssetsInput struct {
	ExchangeAccountID int64 `json:"exchangeAccountId"`
}

type exchangeTickerInput struct {
	ExchangeCode  string `json:"exchangeCode"`
	IsDemoAccount bool   `json:"isDemoAccount"`
	Symbol        string `json:"symbol"`
}

func (h *Handler) exchangeGetAssets(input any) (any, *TRPCError) {
	if h.db == nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: "db not configured"}
	}

	var in exchangeAssetsInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	// Ensure account exists.
	accAny, trpcErr := h.exchangeAccountGetOne(in.ExchangeAccountID)
	if trpcErr != nil {
		return nil, trpcErr
	}
	acc, _ := accAny.(map[string]any)

	// If paper account: read balances from PaperAsset table.
	if toBool(acc["isPaperAccount"]) {
		rows, err := h.db.Query(`SELECT currency, balance FROM "PaperAsset" ORDER BY currency ASC`)
		if err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		defer rows.Close()

		var out []any
		for rows.Next() {
			var currency string
			var balance float64
			if err := rows.Scan(&currency, &balance); err != nil {
				return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
			}
			out = append(out, map[string]any{
				"currency":         currency,
				"balance":          balance,
				"availableBalance": balance,
			})
		}
		if err := rows.Err(); err != nil {
			return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
		}
		return out, nil
	}

	// Live exchange account balances are not yet supported in Go rewrite.
	return nil, &TRPCError{Code: CodeNotImplemented, Message: "exchange assets require live exchange integration (not yet ported)"}
}

func (h *Handler) exchangeGetTicker(input any) (any, *TRPCError) {
	var in exchangeTickerInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	t, err := fetchBinanceTicker(in.Symbol)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	return t, nil
}
