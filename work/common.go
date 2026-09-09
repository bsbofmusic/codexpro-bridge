package work

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/codexpro/bridge/core"
	"github.com/google/uuid"
)

const SchemaVersion = 2

var WorkOperations = []string{
	"task.create", "task.get", "task.list", "task.update", "task.transition",
	"task.step_add", "task.step_update", "task.summary", "task.resume",
	"checkpoint.create", "checkpoint.get", "checkpoint.list",
	"project.create", "project.get", "project.list", "project.context", "project.refresh",
	"audit.create", "audit.record", "audit.evaluate", "audit.get", "audit.list",
	"artifact.register", "artifact.get", "artifact.list", "artifact.finalize", "artifact.archive",
}

var taskStates = setOf("pending", "running", "blocked", "audit", "done", "failed", "cancelled")
var stepStates = setOf("todo", "doing", "blocked", "done", "failed", "skipped")
var artifactStates = setOf("draft", "final", "archived")
var verificationStates = setOf("declared", "verified", "stale")
var evidenceTypes = setOf("verified_structured", "manual", "external_reference")

var taskTransitions = map[string]map[string]bool{
	"pending":   setOf("running", "cancelled"),
	"running":   setOf("blocked", "audit", "failed", "cancelled"),
	"blocked":   setOf("running", "cancelled"),
	"audit":     setOf("done", "running", "blocked"),
	"done":      {},
	"failed":    {},
	"cancelled": {},
}

var jsonColumns = map[string]map[string]bool{
	"projects":           setOf("relevant_skills", "relevant_mcp", "memory_queries", "project_rules", "known_risks"),
	"tasks":              setOf("metadata"),
	"checkpoints":        setOf("completed_step_ids", "pending_step_ids", "blocked_step_ids", "important_findings", "decisions", "artifact_ids"),
	"audit_checks":       setOf("expected", "actual"),
	"artifacts":          setOf("metadata"),
	"operation_receipts": setOf("metadata"),
	"result_blobs":       setOf("content_json"),
}

func setOf(values ...string) map[string]bool {
	m := make(map[string]bool, len(values))
	for _, v := range values {
		m[v] = true
	}
	return m
}

func now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func newID(prefix string) string {
	hex := strings.ReplaceAll(uuid.NewString(), "-", "")
	return prefix + "_" + hex[:20]
}

func canonicalJSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

func boundedJSON(v any) (string, error) {
	s, err := canonicalJSON(v)
	if err != nil {
		return "", core.Err("invalid_work_input", "Work Runtime JSON field is invalid")
	}
	if len([]rune(s)) > 20_000 {
		return "", core.Err("invalid_work_input", "Work Runtime JSON field is too large")
	}
	return s, nil
}

func decodeJSONText(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	var out any
	if json.Unmarshal([]byte(s), &out) == nil {
		return out
	}
	return v
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case int32:
		return int(x)
	case float64:
		return int(x)
	case []byte:
		var n int
		fmt.Sscanf(string(x), "%d", &n)
		return n
	case string:
		var n int
		fmt.Sscanf(x, "%d", &n)
		return n
	default:
		return 0
	}
}

func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	case []byte:
		return string(x) != "0" && string(x) != ""
	case string:
		return x != "0" && x != "" && strings.ToLower(x) != "false"
	default:
		return false
	}
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
