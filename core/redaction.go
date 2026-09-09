package core

import (
	"encoding/json"
	"regexp"
	"strings"
)

var secretPattern = regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._~+/=-]{8,}|([?&][^=]*(token|key|secret|password)[^=]*=)[^&#\s]+`)
var secretKeyPattern = regexp.MustCompile(`(?i)(authorization|cookie|set-cookie|token|api[_-]?key|secret|password|passwd|credential|private[_-]?key)`)

func RedactText(s string) string {
	return secretPattern.ReplaceAllStringFunc(s, func(match string) string {
		if strings.HasPrefix(strings.ToLower(match), "bearer ") {
			return "Bearer [REDACTED]"
		}
		if i := strings.Index(match, "="); i >= 0 {
			return match[:i+1] + "[REDACTED]"
		}
		return "[REDACTED]"
	})
}

func RedactValue(v any) any {
	switch x := v.(type) {
	case string:
		return RedactText(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for key, value := range x {
			if secretKeyPattern.MatchString(key) {
				out[key] = "[REDACTED]"
			} else {
				out[key] = RedactValue(value)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, value := range x {
			out[i] = RedactValue(value)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(x))
		for i, value := range x {
			redacted, _ := RedactValue(value).(map[string]any)
			out[i] = redacted
		}
		return out
	default:
		return v
	}
}

func Bounded(v any, max int) any {
	v = RedactValue(v)
	b, err := json.Marshal(v)
	if err != nil || len(b) > max {
		return map[string]any{"ok": false, "error": map[string]any{"code": "output_truncated", "message": "Result exceeded the Bridge output limit"}}
	}
	return v
}
