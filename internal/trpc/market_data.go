package trpc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type binanceSymbol struct {
	Symbol              string `json:"symbol"`
	Status              string `json:"status"`
	BaseAsset           string `json:"baseAsset"`
	QuoteAsset          string `json:"quoteAsset"`
	BaseAssetPrecision  int    `json:"baseAssetPrecision"`
	QuoteAssetPrecision int    `json:"quoteAssetPrecision"`
	Filters             []struct {
		FilterType  string `json:"filterType"`
		MinPrice    string `json:"minPrice"`
		MaxPrice    string `json:"maxPrice"`
		TickSize    string `json:"tickSize"`
		MinQty      string `json:"minQty"`
		MaxQty      string `json:"maxQty"`
		StepSize    string `json:"stepSize"`
		MinNotional string `json:"minNotional"`
	} `json:"filters"`
}

type binanceExchangeInfo struct {
	Symbols []binanceSymbol `json:"symbols"`
}

type binanceTicker struct {
	Symbol      string `json:"symbol"`
	PriceChange string `json:"priceChange"`
	LastPrice   string `json:"lastPrice"`
	BidPrice    string `json:"bidPrice"`
	AskPrice    string `json:"askPrice"`
	OpenPrice   string `json:"openPrice"`
	HighPrice   string `json:"highPrice"`
	LowPrice    string `json:"lowPrice"`
	Volume      string `json:"volume"`
	QuoteVolume string `json:"quoteVolume"`
}

type binanceCandle []any

type symbolsCache struct {
	sync.Mutex
	data []map[string]any
	till time.Time
}

var (
	globalSymbolsCache symbolsCache
)

func fetchBinanceSymbols(exchangeCode string) ([]map[string]any, error) {
	globalSymbolsCache.Lock()
	defer globalSymbolsCache.Unlock()
	if time.Now().Before(globalSymbolsCache.till) && len(globalSymbolsCache.data) > 0 {
		return cloneSymbolSlice(globalSymbolsCache.data), nil
	}

	var info binanceExchangeInfo
	if err := httpJSON("https://api.binance.com/api/v3/exchangeInfo", &info); err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0, len(info.Symbols))
	for _, s := range info.Symbols {
		if s.Status != "TRADING" {
			continue
		}
		pair := s.BaseAsset + "/" + s.QuoteAsset
		tickSize := parseFloatDefault(s.Filters, "PRICE_FILTER", "tickSize", 0.01)
		minPrice := parseFloatDefault(s.Filters, "PRICE_FILTER", "minPrice", 0)
		maxPrice := parseFloatDefault(s.Filters, "PRICE_FILTER", "maxPrice", 0)
		stepSize := parseFloatDefault(s.Filters, "LOT_SIZE", "stepSize", 0.000001)
		minQty := parseFloatDefault(s.Filters, "LOT_SIZE", "minQty", 0)
		maxQty := parseFloatDefault(s.Filters, "LOT_SIZE", "maxQty", 0)

		out = append(out, map[string]any{
			"symbolId":         toSymbolID(exchangeCode, pair),
			"currencyPair":     pair,
			"exchangeCode":     exchangeCode,
			"exchangeSymbolId": s.Symbol,
			"baseCurrency":     s.BaseAsset,
			"quoteCurrency":    s.QuoteAsset,
			"filters": map[string]any{
				"precision": map[string]any{
					"amount": stepSize,
					"price":  tickSize,
				},
				"decimals": map[string]any{
					"amount": getExponentAbs(stepSize),
					"price":  getExponentAbs(tickSize),
				},
				"limits": map[string]any{
					"amount": map[string]any{
						"min": minQty,
						"max": maxQty,
					},
					"price": map[string]any{
						"min": minPrice,
						"max": maxPrice,
					},
				},
			},
		})
	}

	globalSymbolsCache.data = cloneSymbolSlice(out)
	globalSymbolsCache.till = time.Now().Add(10 * time.Minute)
	return out, nil
}

func parseFloatDefault(filters []struct {
	FilterType  string `json:"filterType"`
	MinPrice    string `json:"minPrice"`
	MaxPrice    string `json:"maxPrice"`
	TickSize    string `json:"tickSize"`
	MinQty      string `json:"minQty"`
	MaxQty      string `json:"maxQty"`
	StepSize    string `json:"stepSize"`
	MinNotional string `json:"minNotional"`
}, filterType, field string, def float64) float64 {
	for _, f := range filters {
		if f.FilterType != filterType {
			continue
		}
		var val string
		switch field {
		case "minPrice":
			val = f.MinPrice
		case "maxPrice":
			val = f.MaxPrice
		case "tickSize":
			val = f.TickSize
		case "minQty":
			val = f.MinQty
		case "maxQty":
			val = f.MaxQty
		case "stepSize":
			val = f.StepSize
		case "minNotional":
			val = f.MinNotional
		}
		if val == "" {
			continue
		}
		if v, err := strconv.ParseFloat(val, 64); err == nil {
			return v
		}
	}
	return def
}

func cloneSymbolSlice(in []map[string]any) []map[string]any {
	out := make([]map[string]any, len(in))
	for i, m := range in {
		out[i] = mapClone(m)
	}
	return out
}

func fetchBinancePrice(pair string) (float64, error) {
	symbol := strings.ReplaceAll(pair, "/", "")
	var res struct {
		Price string `json:"price"`
	}
	if err := httpJSON("https://api.binance.com/api/v3/ticker/price?symbol="+symbol, &res); err != nil {
		return 0, err
	}
	return strconv.ParseFloat(res.Price, 64)
}

func fetchBinanceTicker(pair string) (map[string]any, error) {
	symbol := strings.ReplaceAll(pair, "/", "")
	var t binanceTicker
	if err := httpJSON("https://api.binance.com/api/v3/ticker/24hr?symbol="+symbol, &t); err != nil {
		return nil, err
	}

	price := parseFloatStr(t.LastPrice, 0)
	bid := parseFloatStr(t.BidPrice, 0)
	ask := parseFloatStr(t.AskPrice, 0)
	open := parseFloatStr(t.OpenPrice, 0)
	high := parseFloatStr(t.HighPrice, 0)
	low := parseFloatStr(t.LowPrice, 0)
	vol := parseFloatStr(t.Volume, 0)
	qVol := parseFloatStr(t.QuoteVolume, 0)

	return map[string]any{
		"symbol":      pair,
		"timestamp":   time.Now().UnixMilli(),
		"bid":         bid,
		"ask":         ask,
		"last":        price,
		"open":        open,
		"high":        high,
		"low":         low,
		"close":       price,
		"baseVolume":  vol,
		"quoteVolume": qVol,
	}, nil
}

func fetchBinanceCandles(pair string, interval string, limit int, since int64) ([]any, error) {
	if limit <= 0 {
		limit = 200
	}
	symbol := strings.ReplaceAll(pair, "/", "")
	url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&limit=%d", symbol, interval, limit)
	if since > 0 {
		url += fmt.Sprintf("&startTime=%d", since)
	}

	var data []binanceCandle
	if err := httpJSON(url, &data); err != nil {
		return nil, err
	}

	out := make([]any, 0, len(data))
	for _, c := range data {
		if len(c) < 6 {
			continue
		}
		ts, _ := toInt64(c[0])
		open := parseFloatIdx(c, 1)
		high := parseFloatIdx(c, 2)
		low := parseFloatIdx(c, 3)
		close := parseFloatIdx(c, 4)
		vol := parseFloatIdx(c, 5)
		out = append(out, []any{ts, open, high, low, close, vol})
	}
	return out, nil
}

func parseFloatIdx(arr []any, idx int) float64 {
	if len(arr) <= idx {
		return 0
	}
	switch v := arr[idx].(type) {
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}

func parseFloatStr(s string, def float64) float64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

func toInt64(v any) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case float64:
		return int64(t), nil
	case string:
		return strconv.ParseInt(t, 10, 64)
	case json.Number:
		return t.Int64()
	default:
		return 0, fmt.Errorf("unsupported int type %T", v)
	}
}
