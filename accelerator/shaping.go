package accelerator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/codexpro/bridge/core"
)

const DefaultPreviewChars = 12_000

type ShapeOptions struct {
	JSONPath        string
	Fields          []string
	Offset          int
	Limit           int
	MaxChars        int
	StoreFullResult bool
}

type ShapeResult struct {
	Value     any
	Truncated bool
	Original  any
}

func Shape(value any, options ShapeOptions) (ShapeResult, error) {
	original := core.RedactValue(value)
	current := original
	var err error
	if strings.TrimSpace(options.JSONPath) != "" && strings.TrimSpace(options.JSONPath) != "$" {
		current, err = selectPath(current, options.JSONPath)
		if err != nil {
			return ShapeResult{}, err
		}
	}
	if len(options.Fields) > 0 {
		m, ok := current.(map[string]any)
		if !ok {
			return ShapeResult{}, core.Err("invalid_shape", "fields requires an object result")
		}
		selected := make(map[string]any, len(options.Fields))
		for _, field := range options.Fields {
			field = strings.TrimSpace(field)
			if field == "" {
				continue
			}
			if v, ok := m[field]; ok {
				selected[field] = v
			}
		}
		current = selected
	}
	if list, ok := current.([]any); ok {
		offset := options.Offset
		if offset < 0 {
			return ShapeResult{}, core.Err("invalid_shape", "offset must be non-negative")
		}
		if offset > len(list) {
			offset = len(list)
		}
		end := len(list)
		if options.Limit > 0 && offset+options.Limit < end {
			end = offset + options.Limit
		}
		current = list[offset:end]
	} else if options.Offset != 0 || options.Limit > 0 {
		return ShapeResult{}, core.Err("invalid_shape", "offset/limit requires an array result")
	}
	maxChars := options.MaxChars
	if maxChars <= 0 {
		maxChars = DefaultPreviewChars
	}
	if maxChars < 256 || maxChars > 120_000 {
		return ShapeResult{}, core.Err("invalid_shape", "max_chars must be between 256 and 120000")
	}
	encoded, err := json.Marshal(current)
	if err != nil {
		return ShapeResult{}, core.Err("invalid_shape", "Result cannot be encoded as JSON")
	}
	if utf8.RuneCount(encoded) <= maxChars {
		return ShapeResult{Value: current, Original: original}, nil
	}
	preview := truncateRunes(string(encoded), maxChars)
	return ShapeResult{
		Value: map[string]any{
			"preview_json": preview,
			"truncated":    true,
			"max_chars":    maxChars,
		},
		Truncated: true,
		Original:  original,
	}, nil
}

func selectPath(value any, path string) (any, error) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "$") {
		return nil, core.Err("invalid_shape", "json_path must start with $")
	}
	current := value
	for i := 1; i < len(path); {
		switch path[i] {
		case '.':
			i++
			start := i
			for i < len(path) && path[i] != '.' && path[i] != '[' {
				i++
			}
			if start == i {
				return nil, core.Err("invalid_shape", "json_path contains an empty field")
			}
			m, ok := current.(map[string]any)
			if !ok {
				return nil, core.Err("shape_not_found", "json_path field is not available")
			}
			field := path[start:i]
			var exists bool
			current, exists = m[field]
			if !exists {
				return nil, core.Err("shape_not_found", "json_path field is not available")
			}
		case '[':
			close := strings.IndexByte(path[i:], ']')
			if close < 0 {
				return nil, core.Err("invalid_shape", "json_path index is not closed")
			}
			close += i
			index, err := strconv.Atoi(path[i+1 : close])
			if err != nil || index < 0 {
				return nil, core.Err("invalid_shape", "json_path only supports non-negative numeric indexes")
			}
			list, ok := current.([]any)
			if !ok || index >= len(list) {
				return nil, core.Err("shape_not_found", fmt.Sprintf("json_path index %d is not available", index))
			}
			current = list[index]
			i = close + 1
		default:
			return nil, core.Err("invalid_shape", "json_path supports only .field and [index]")
		}
	}
	return current, nil
}

func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}
