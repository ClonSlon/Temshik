package trpc

import (
	"encoding/json"
	"sort"
)

func toOrderEntity(order map[string]any) map[string]any {
	out := mapClone(order)

	typ := asString(out["type"])
	status := asString(out["status"])

	relativePrice := out["relativePrice"]
	hasRelative := relativePrice != nil

	switch typ {
	case "Limit":
		switch status {
		case "Idle":
			// price required unless relative orders (then allow -1 placeholder)
			if hasRelative && out["price"] == nil {
				out["price"] = float64(-1)
			}
			out["filledPrice"] = nil
			out["filledAt"] = nil
			out["placedAt"] = nil
		case "Placed":
			out["filledPrice"] = nil
			out["filledAt"] = nil
			// placedAt kept as-is (expected non-null)
		case "Filled":
			// keep all as-is (expected non-null filledPrice/filledAt/placedAt)
		case "Canceled":
			out["filledPrice"] = nil
			out["filledAt"] = nil
			// placedAt kept as-is (expected non-null)
		case "Revoked", "Deleted":
			if hasRelative && out["price"] == nil {
				out["price"] = float64(-1)
			}
			out["filledPrice"] = nil
			out["filledAt"] = nil
			out["placedAt"] = nil
		default:
			// Unknown status: keep as-is.
		}
	case "Market":
		out["price"] = nil
		switch status {
		case "Idle":
			out["filledPrice"] = nil
			out["filledAt"] = nil
			out["placedAt"] = nil
		case "Placed":
			out["filledPrice"] = nil
			out["filledAt"] = nil
			// placedAt kept as-is (expected non-null)
		case "Filled":
			// keep filledPrice/filledAt/placedAt as-is (expected non-null)
		case "Canceled":
			out["filledPrice"] = nil
			out["filledAt"] = nil
			// placedAt kept as-is
		case "Revoked", "Deleted":
			out["filledPrice"] = nil
			out["filledAt"] = nil
			out["placedAt"] = nil
		default:
			// Unknown status: keep as-is.
		}
	default:
		// Unknown type: keep as-is.
	}

	return out
}

func toSmartTradeEntity(entity map[string]any) map[string]any {
	out := mapClone(entity)

	ordersAny, _ := out["orders"].([]any)
	orders := make([]map[string]any, 0, len(ordersAny))
	for _, o := range ordersAny {
		if m, ok := o.(map[string]any); ok {
			orders = append(orders, m)
		}
	}

	entryType := asString(out["entryType"])
	takeProfitType := asString(out["takeProfitType"])

	findByEntityType := func(entityType string) []map[string]any {
		var res []map[string]any
		for _, o := range orders {
			if asString(o["entityType"]) == entityType {
				res = append(res, o)
			}
		}
		return res
	}

	entryOrders := findByEntityType("EntryOrder")
	tpOrders := findByEntityType("TakeProfitOrder")
	safetyOrders := findByEntityType("SafetyOrder")
	slOrders := findByEntityType("StopLossOrder")

	// Deterministic ordering (Prisma returns stable order by primary key implicitly).
	sort.Slice(entryOrders, func(i, j int) bool { return asInt(entryOrders[i]["id"]) < asInt(entryOrders[j]["id"]) })
	sort.Slice(tpOrders, func(i, j int) bool { return asInt(tpOrders[i]["id"]) < asInt(tpOrders[j]["id"]) })
	sort.Slice(safetyOrders, func(i, j int) bool { return asInt(safetyOrders[i]["id"]) < asInt(safetyOrders[j]["id"]) })
	sort.Slice(slOrders, func(i, j int) bool { return asInt(slOrders[i]["id"]) < asInt(slOrders[j]["id"]) })

	toEntitySlice := func(in []map[string]any) []any {
		res := make([]any, 0, len(in))
		for _, o := range in {
			res = append(res, toOrderEntity(o))
		}
		return res
	}

	out["safetyOrders"] = toEntitySlice(safetyOrders)
	if len(slOrders) > 0 {
		out["stopLossOrder"] = toOrderEntity(slOrders[0])
	} else {
		out["stopLossOrder"] = nil
	}

	if entryType == "Order" {
		if len(entryOrders) > 0 {
			out["entryOrder"] = toOrderEntity(entryOrders[0])
		} else {
			out["entryOrder"] = nil
		}
	} else {
		out["entryOrders"] = toEntitySlice(entryOrders)
	}

	switch takeProfitType {
	case "None":
		out["takeProfitOrder"] = nil
	case "Order":
		if len(tpOrders) > 0 {
			out["takeProfitOrder"] = toOrderEntity(tpOrders[0])
		} else {
			out["takeProfitOrder"] = nil
		}
	case "Ladder":
		out["takeProfitOrders"] = toEntitySlice(tpOrders)
	default:
		// Unknown takeProfitType: leave unchanged.
	}

	return out
}

func asInt(v any) int64 {
	if v == nil {
		return 0
	}
	if i, ok := v.(int64); ok {
		return i
	}
	if i, ok := v.(int); ok {
		return int64(i)
	}
	if f, ok := v.(float64); ok {
		return int64(f)
	}
	if n, ok := v.(json.Number); ok {
		i, _ := n.Int64()
		return i
	}
	return 0
}
