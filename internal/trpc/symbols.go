package trpc

import (
	"fmt"
	"strings"
	"time"
)

type symbolListInput struct {
	ExchangeCode  string `json:"exchangeCode"`
	IsDemoAccount bool   `json:"isDemoAccount"`
}

type symbolGetOneInput struct {
	SymbolID      string `json:"symbolId"`
	IsDemoAccount bool   `json:"isDemoAccount"`
}

type symbolPriceInput struct {
	SymbolID      string `json:"symbolId"`
	IsDemoAccount bool   `json:"isDemoAccount"`
}

func decomposeSymbolID(symbolID string) (exchangeCode string, currencyPair string, err error) {
	symbolID = strings.TrimSpace(symbolID)
	if symbolID == "" {
		return "", "", fmt.Errorf("symbolId is empty")
	}

	parts := strings.SplitN(symbolID, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid symbolId: %q", symbolID)
	}
	exchangeCode = strings.TrimSpace(parts[0])
	currencyPair = strings.TrimSpace(parts[1])
	if exchangeCode == "" || currencyPair == "" {
		return "", "", fmt.Errorf("invalid symbolId: %q", symbolID)
	}
	return exchangeCode, currencyPair, nil
}

func (h *Handler) symbolList(input any) (any, *TRPCError) {
	var in symbolListInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	syms, err := fetchBinanceSymbols(in.ExchangeCode)
	if err != nil {
		return nil, &TRPCError{Code: CodeInternalServerError, Message: err.Error()}
	}
	out := make([]any, 0, len(syms))
	for _, s := range syms {
		out = append(out, s)
	}
	return out, nil
}

func (h *Handler) symbolGetOne(input any) (any, *TRPCError) {
	var in symbolGetOneInput
	if err := decodeInput(input, &in); err != nil {
		return nil, err
	}

	ex, pair, err := decomposeSymbolID(in.SymbolID)
	if err != nil {
		return nil, &TRPCError{Code: CodeBadRequest, Message: err.Error()}
	}

	syms, err := fetchBinanceSymbols(ex)
	if err == nil {
		for _, s := range syms {
			if strings.EqualFold(s["symbolId"].(string), toSymbolID(ex, pair)) {
				return s, nil
			}
		}
	}

	// Fallback minimal info
	return map[string]any{
		"symbolId":         toSymbolID(ex, pair),
		"currencyPair":     pair,
		"exchangeCode":     ex,
		"exchangeSymbolId": strings.ReplaceAll(pair, "/", ""),
		"baseCurrency":     strings.Split(pair, "/")[0],
		"quoteCurrency":    strings.Split(pair, "/")[1],
		"filters":          map[string]any{},
	}, nil
}

func (h *Handler) symbolPrice(input any) (any, *TRPCError) {
	var in symbolPriceInput
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

	return map[string]any{
		"symbol":    pair,
		"price":     price,
		"timestamp": time.Now().UnixMilli(),
	}, nil
}
