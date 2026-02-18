package trpc

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func parseNullableDBTime(v any) (*time.Time, error) {
	if v == nil {
		return nil, nil
	}
	t, err := parseDBTime(v)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseJSONOrEmptyObject(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return map[string]any{}
	}
	var out any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func toFloat64Ptr(v any) (*float64, error) {
	if v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case float64:
		return &t, nil
	case int64:
		f := float64(t)
		return &f, nil
	case int:
		f := float64(t)
		return &f, nil
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return nil, err
		}
		return &f, nil
	case []byte:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil {
			return nil, err
		}
		return &f, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return nil, err
		}
		return &f, nil
	default:
		return nil, fmt.Errorf("unsupported float value: %T", v)
	}
}

func mapClone(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

func asInt64Ptr(v any) (*int64, error) {
	if v == nil {
		return nil, nil
	}
	switch t := v.(type) {
	case int64:
		return &t, nil
	case int:
		i := int64(t)
		return &i, nil
	case float64:
		i := int64(t)
		return &i, nil
	case json.Number:
		i, err := t.Int64()
		if err != nil {
			return nil, err
		}
		return &i, nil
	case []byte:
		i, err := strconv.ParseInt(string(t), 10, 64)
		if err != nil {
			return nil, err
		}
		return &i, nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return nil, err
		}
		return &i, nil
	default:
		return nil, fmt.Errorf("unsupported int value: %T", v)
	}
}

func hasKey(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}

func stableHash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func roundToDecimals(v float64, decimals int) float64 {
	if decimals <= 0 {
		return math.Round(v)
	}
	pow := math.Pow10(decimals)
	return math.Round(v*pow) / pow
}

// httpJSON performs a GET request and decodes JSON into out.
func httpJSON(url string, out any) error {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	return dec.Decode(out)
}

func toSymbolID(exchangeCode, pair string) string {
	return strings.TrimSpace(exchangeCode) + ":" + strings.TrimSpace(pair)
}

// getExponentAbs returns number of decimal places implied by a tick/step (e.g. 0.001 -> 3).
func getExponentAbs(step float64) int {
	if step == 0 {
		return 0
	}
	exp := 0
	for step < 1 {
		step *= 10
		exp++
		if exp > 18 {
			break
		}
	}
	return exp
}
