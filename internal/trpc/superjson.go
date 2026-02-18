package trpc

import (
	"strconv"
	"strings"
	"time"
)

func superJSONSerialize(v any) map[string]any {
	transformed, annotations := transformForSuperJSON(v, nil)

	out := map[string]any{
		"json": transformed,
	}
	if annotations != nil {
		out["meta"] = map[string]any{
			"values": annotations,
		}
	}
	return out
}

func transformForSuperJSON(v any, path []string) (any, any) {
	switch vv := v.(type) {
	case time.Time:
		return vv.UTC().Format("2006-01-02T15:04:05.000Z"), annotationAt(path, []any{"Date"})
	case *time.Time:
		if vv == nil {
			return nil, nil
		}
		return vv.UTC().Format("2006-01-02T15:04:05.000Z"), annotationAt(path, []any{"Date"})
	case map[string]any:
		out := make(map[string]any, len(vv))
		ann := map[string]any{}
		for k, val := range vv {
			tv, ta := transformForSuperJSON(val, append(path, k))
			out[k] = tv
			mergeAnnotations(ann, ta)
		}
		if len(ann) == 0 {
			return out, nil
		}
		return out, ann
	case []any:
		out := make([]any, len(vv))
		ann := map[string]any{}
		for i, val := range vv {
			tv, ta := transformForSuperJSON(val, append(path, strconv.Itoa(i)))
			out[i] = tv
			mergeAnnotations(ann, ta)
		}
		if len(ann) == 0 {
			return out, nil
		}
		return out, ann
	default:
		return v, nil
	}
}

func annotationAt(path []string, annotation any) any {
	if len(path) == 0 {
		return annotation
	}
	return map[string]any{
		pathKey(path): annotation,
	}
}

func mergeAnnotations(dst map[string]any, annotations any) {
	if annotations == nil {
		return
	}
	if m, ok := annotations.(map[string]any); ok {
		for k, v := range m {
			dst[k] = v
		}
	}
}

func pathKey(segments []string) string {
	escaped := make([]string, 0, len(segments))
	for _, s := range segments {
		escaped = append(escaped, strings.ReplaceAll(s, ".", "\\."))
	}
	return strings.Join(escaped, ".")
}
