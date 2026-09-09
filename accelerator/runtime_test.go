package accelerator

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/codexpro/bridge/core"
	"github.com/codexpro/bridge/shared_mcp"
	"github.com/codexpro/bridge/work"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeTransport struct {
	mu      sync.Mutex
	tools   []*mcp.Tool
	calls   map[string]int
	errors  map[string]error
	results map[string]*mcp.CallToolResult
}

func (f *fakeTransport) ListTools(context.Context) ([]*mcp.Tool, error) { return f.tools, nil }
func (f *fakeTransport) CallTool(_ context.Context, name string, _ map[string]any) (*mcp.CallToolResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[name]++
	if err := f.errors[name]; err != nil {
		return nil, err
	}
	return f.results[name], nil
}
func (f *fakeTransport) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[name]
}

func newRuntime(t *testing.T, transport shared_mcp.Transport) *Runtime {
	t.Helper()
	mcpRuntime := &shared_mcp.Runtime{Primary: transport, Timeout: time.Second, MaxOutput: 120000}
	workRuntime := work.New(filepath.Join(t.TempDir(), "work.sqlite3"))
	return New(mcpRuntime, workRuntime)
}

func tool(name string, readOnly bool) *mcp.Tool {
	return &mcp.Tool{Name: name, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly}}
}

func okResult(value any) *mcp.CallToolResult {
	return &mcp.CallToolResult{StructuredContent: value}
}

func mustMap(t *testing.T) func(map[string]any, error) map[string]any {
	t.Helper()
	return func(value map[string]any, err error) map[string]any {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
}

func TestMCPCallAutomaticallyReceiptsAndDeduplicatesMutation(t *testing.T) {
	must := mustMap(t)
	f := &fakeTransport{
		tools:   []*mcp.Tool{tool("deploy", false)},
		results: map[string]*mcp.CallToolResult{"deploy": okResult(map[string]any{"status": "ok"})},
		errors:  map[string]error{},
	}
	r := newRuntime(t, f)
	task := must(r.Work.TaskCreate("deploy", "", "", "normal", false, nil))
	taskID := task["task"].(map[string]any)["task_id"].(string)
	must(r.Work.TaskTransition(taskID, "running"))

	args := map[string]any{
		"tool": "deploy", "arguments": map[string]any{"target": "prod"},
		"task_id": taskID, "operation_id": "op-one",
	}
	first := must(r.Dispatch(context.Background(), "mcp.call", args))
	if first["ok"] != true || first["replay_policy"] != "DO_NOT_REPLAY" || first["mutation"] != true {
		t.Fatalf("unexpected first mutation call: %#v", first)
	}
	second := must(r.Dispatch(context.Background(), "mcp.call", args))
	if second["deduplicated"] != true || second["executed"] != false || f.count("deploy") != 1 {
		t.Fatalf("mutation replay was not blocked: %#v calls=%d", second, f.count("deploy"))
	}
	capsule := must(r.Dispatch(context.Background(), "resume.capsule", map[string]any{"task_id": taskID}))
	if len(capsule["do_not_replay"].([]map[string]any)) != 1 {
		t.Fatalf("resume capsule lost mutation receipt: %#v", capsule)
	}
}

func TestDeliveryUnknownReturnsReceiptWithoutReplay(t *testing.T) {
	must := mustMap(t)
	f := &fakeTransport{
		tools:   []*mcp.Tool{tool("mutate", false)},
		results: map[string]*mcp.CallToolResult{},
		errors:  map[string]error{"mutate": core.Err("delivery_unknown", "unknown")},
	}
	r := newRuntime(t, f)
	got := must(r.Dispatch(context.Background(), "mcp.call", map[string]any{"tool": "mutate", "operation_id": "op-unknown"}))
	if got["ok"] != false || got["replay_policy"] != "VERIFY_BEFORE_ANY_RETRY" {
		t.Fatalf("delivery unknown policy drifted: %#v", got)
	}
	again := must(r.Dispatch(context.Background(), "mcp.call", map[string]any{"tool": "mutate", "operation_id": "op-unknown"}))
	if again["executed"] != false || f.count("mutate") != 1 {
		t.Fatalf("delivery-unknown mutation was replayed: %#v calls=%d", again, f.count("mutate"))
	}
}

func TestParallelBatchRequiresExplicitReadOnlyAndPreservesOrder(t *testing.T) {
	must := mustMap(t)
	f := &fakeTransport{
		tools: []*mcp.Tool{tool("a", true), tool("b", true), tool("write", false)},
		results: map[string]*mcp.CallToolResult{
			"a":     okResult(map[string]any{"value": "A"}),
			"b":     okResult(map[string]any{"value": "B"}),
			"write": okResult(map[string]any{"value": "W"}),
		},
		errors: map[string]error{},
	}
	r := newRuntime(t, f)
	got := must(r.Dispatch(context.Background(), "mcp.batch", map[string]any{
		"mode": "parallel", "concurrency": 2,
		"calls": []any{
			map[string]any{"tool": "a", "operation_id": "a1"},
			map[string]any{"tool": "b", "operation_id": "b1"},
		},
	}))
	if got["ok"] != true || got["count"] != 2 {
		t.Fatalf("parallel read batch failed: %#v", got)
	}
	results := got["results"].([]map[string]any)
	if results[0]["tool"] != "a" || results[1]["tool"] != "b" {
		t.Fatalf("parallel result order drifted: %#v", results)
	}
	_, err := r.Dispatch(context.Background(), "mcp.batch", map[string]any{
		"mode": "parallel", "calls": []any{map[string]any{"tool": "write", "operation_id": "w1"}},
	})
	be, ok := err.(core.BridgeError)
	if !ok || be.Code != "parallel_mutation_denied" || f.count("write") != 0 {
		t.Fatalf("parallel mutation was not rejected before execution: %#v calls=%d", err, f.count("write"))
	}
}

func TestSequentialStopOnErrorAndNoRetry(t *testing.T) {
	must := mustMap(t)
	f := &fakeTransport{
		tools: []*mcp.Tool{tool("one", true), tool("two", true), tool("three", true)},
		results: map[string]*mcp.CallToolResult{
			"one":   okResult(map[string]any{"ok": 1}),
			"three": okResult(map[string]any{"ok": 3}),
		},
		errors: map[string]error{"two": errors.New("transport lost")},
	}
	r := newRuntime(t, f)
	got := must(r.Dispatch(context.Background(), "mcp.batch", map[string]any{
		"mode": "sequential", "stop_on_error": true,
		"calls": []any{
			map[string]any{"tool": "one", "operation_id": "s1"},
			map[string]any{"tool": "two", "operation_id": "s2"},
			map[string]any{"tool": "three", "operation_id": "s3"},
		},
	}))
	if got["ok"] != false || got["count"] != 2 || f.count("three") != 0 {
		t.Fatalf("sequential stop_on_error failed: %#v", got)
	}
}

func TestResultShapingStoresFullResultAndReadsByReference(t *testing.T) {
	must := mustMap(t)
	f := &fakeTransport{
		tools: []*mcp.Tool{tool("read", true)},
		results: map[string]*mcp.CallToolResult{"read": okResult(map[string]any{
			"items": []any{
				map[string]any{"id": 1, "name": "alpha", "extra": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"},
				map[string]any{"id": 2, "name": "beta", "extra": "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy"},
			},
		})},
		errors: map[string]error{},
	}
	r := newRuntime(t, f)
	got := must(r.Dispatch(context.Background(), "mcp.call", map[string]any{
		"tool": "read", "operation_id": "shape-1",
		"shape": map[string]any{"json_path": "$.structured_content.items", "offset": 0, "limit": 1, "max_chars": 256, "store_full_result": true},
	}))
	if got["ok"] != true || got["result_ref"] == nil {
		t.Fatalf("result ref was not created: %#v", got)
	}
	ref := got["result_ref"].(string)
	read := must(r.Dispatch(context.Background(), "result.read", map[string]any{
		"result_ref": ref, "shape": map[string]any{"json_path": "$.structured_content.items", "offset": 1, "limit": 1, "max_chars": 256},
	}))
	items := read["result"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["id"].(float64) != 2 {
		t.Fatalf("deferred result range read failed: %#v", read)
	}
}

func TestConversationAffinityResume(t *testing.T) {
	must := mustMap(t)
	f := &fakeTransport{tools: []*mcp.Tool{}, results: map[string]*mcp.CallToolResult{}, errors: map[string]error{}}
	r := newRuntime(t, f)
	task := must(r.Work.TaskCreate("continue", "", "", "normal", false, nil))
	taskID := task["task"].(map[string]any)["task_id"].(string)
	must(r.Dispatch(context.Background(), "conversation.bind", map[string]any{"conversation_first_message": "My Bridge Job", "task_id": taskID}))
	capsule := must(r.Dispatch(context.Background(), "resume.capsule", map[string]any{"conversation_first_message": "my bridge job"}))
	if capsule["task"].(map[string]any)["task_id"] != taskID || capsule["mutation_replayed"] != false {
		t.Fatalf("affinity resume failed: %#v", capsule)
	}
}
