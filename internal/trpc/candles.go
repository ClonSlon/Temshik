package trpc

// candles.go fetches real OHLCV data from Binance public API.

type candlesListInput struct {
	ExchangeCode  string `json:"exchangeCode"`
	Symbol        string `json:"symbol"`
	BarSize       string `json:"barSize"`
	Since         int64  `json:"since"`
	Limit         *int   `json:"limit"`
	IsDemoAccount bool   `json:"isDemoAccount"`
}

func (h *Handler) candlesList(input any) (any, *TRPCError) {
	var in candlesListInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	limit := 200
	if in.Limit != nil {
		limit = clampInt(*in.Limit, 1, 1000)
	}

	interval := binanceInterval(in.BarSize)
	if interval == "" {
		return nil, &TRPCError{Code: CodeBadRequest, Message: "unsupported barSize"}
	}

	candles, err := fetchBinanceCandles(in.Symbol, interval, limit, in.Since)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	return candles, nil
}

func binanceInterval(bar string) string {
	switch bar {
	case "1m":
		return "1m"
	case "5m":
		return "5m"
	case "15m":
		return "15m"
	case "1h":
		return "1h"
	case "4h":
		return "4h"
	case "1d":
		return "1d"
	case "1w":
		return "1w"
	case "1M":
		return "1M"
	case "3M":
		return "3M"
	default:
		return ""
	}
}
