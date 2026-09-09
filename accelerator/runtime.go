package accelerator

import (
	"context"
	"fmt"
	"sync"

	"github.com/codexpro/bridge/core"
	"github.com/codexpro/bridge/shared_mcp"
	"github.com/codexpro/bridge/work"
)

const Version = "1.0.0"
const MaxBatchCalls = 8
const MaxParallel = 4

var Operations = []string{
	"conversation.bind", "conversation.resolve", "conversation.unbind",
	"resume.capsule",
	"mcp.call", "mcp.batch",
	"receipt.prepare", "receipt.finalize", "receipt.get", "receipt.list",
	"result.read", "result.delete",
}

type Runtime struct {
	MCP  *shared_mcp.Runtime
	Work *work.Runtime
}

func New(mcpRuntime *shared_mcp.Runtime, workRuntime *work.Runtime) *Runtime {
	return &Runtime{MCP: mcpRuntime, Work: workRuntime}
}

func (r *Runtime) Health() map[string]any {
	return map[string]any{
		"ok": true, "mode": "deterministic", "ai_runtime": false,
		"operations": append([]string(nil), Operations...),
		"depends_on": []string{"shared_mcp", "work_runtime"},
	}
}

func (r *Runtime) Dispatch(ctx context.Context, operation string, args map[string]any) (map[string]any, error) {
	if args == nil {
		args = map[string]any{}
	}
	switch operation {
	case "conversation.bind":
		return r.Work.AffinityBind(str(args, "conversation_first_message"), str(args, "task_id"))
	case "conversation.resolve":
		return r.Work.AffinityResolve(str(args, "conversation_first_message"))
	case "conversation.unbind":
		return r.Work.AffinityUnbind(str(args, "conversation_first_message"))
	case "resume.capsule":
		return r.resumeCapsule(args)
	case "mcp.call":
		return r.mcpCall(ctx, args)
	case "mcp.batch":
		return r.mcpBatch(ctx, args)
	case "receipt.prepare":
		return r.receiptPrepare(args)
	case "receipt.finalize":
		return r.receiptFinalize(args)
	case "receipt.get":
		return r.Work.ReceiptGet(str(args, "receipt_id"), str(args, "operation_id"))
	case "receipt.list":
		return r.Work.ReceiptList(str(args, "task_id"), str(args, "step_id"), integer(args, "limit", 50), integer(args, "offset", 0))
	case "result.read":
		return r.resultRead(args)
	case "result.delete":
		return r.Work.ResultDelete(str(args, "result_ref"))
	default:
		return nil, core.Err("unsupported_accelerator_operation", "Unsupported Web Accelerator operation")
	}
}

func (r *Runtime) resumeCapsule(args map[string]any) (map[string]any, error) {
	taskID := str(args, "task_id")
	if taskID == "" {
		first := str(args, "conversation_first_message")
		resolved, err := r.Work.AffinityResolve(first)
		if err != nil {
			return nil, err
		}
		if resolved["matched"] != true {
			return map[string]any{
				"ok":                false,
				"error":             map[string]any{"code": "affinity_not_found", "message": "No active task is bound to this conversation fingerprint"},
				"mutation_replayed": false,
			}, nil
		}
		affinity, _ := resolved["affinity"].(map[string]any)
		taskID = str(affinity, "task_id")
	}
	return r.Work.ResumeCapsule(taskID)
}

func (r *Runtime) mcpCall(ctx context.Context, args map[string]any) (map[string]any, error) {
	name := str(args, "tool")
	resolved, err := r.MCP.ResolveTools(ctx, []string{name})
	if err != nil {
		return nil, err
	}
	selected := resolved[name]
	if selected == nil {
		return nil, core.Err("mcp_denied", "MCP tool is not exposed by the shared gateway")
	}
	spec := callSpecFromMap(args)
	return r.executeResolved(ctx, selected, spec)
}

type callSpec struct {
	Tool        string
	Arguments   map[string]any
	TaskID      string
	StepID      string
	OperationID string
	Shape       ShapeOptions
}

func callSpecFromMap(args map[string]any) callSpec {
	return callSpec{
		Tool:        str(args, "tool"),
		Arguments:   object(args, "arguments"),
		TaskID:      str(args, "task_id"),
		StepID:      str(args, "step_id"),
		OperationID: str(args, "operation_id"),
		Shape:       shapeOptions(object(args, "shape")),
	}
}

func (r *Runtime) executeResolved(ctx context.Context, selected *shared_mcp.ResolvedTool, spec callSpec) (map[string]any, error) {
	mutation := !selected.ReadOnly()
	prepared, err := r.Work.ReceiptPrepare(spec.OperationID, spec.TaskID, spec.StepID, spec.Tool, spec.Arguments, mutation, map[string]any{"source": "web_accelerator"})
	if err != nil {
		return nil, err
	}
	if prepared["existing"] == true {
		return map[string]any{
			"ok": true, "deduplicated": true, "executed": false,
			"receipt": prepared["receipt"], "replay_policy": prepared["replay_policy"],
		}, nil
	}
	receipt, _ := prepared["receipt"].(map[string]any)
	receiptID := str(receipt, "receipt_id")
	taskID := str(receipt, "task_id")
	if mutation && taskID != "" {
		if _, err := r.Work.CheckpointCreateEvent(taskID, "mutation.before", receiptID, "Verify the operation receipt before any retry if delivery becomes uncertain"); err != nil {
			return nil, err
		}
	}

	raw, callErr := r.MCP.CallResolved(ctx, selected, spec.Arguments)
	if callErr != nil {
		delivery := "prepared"
		resultState := "failed"
		event := "mutation.failed_before_delivery"
		if be, ok := callErr.(core.BridgeError); ok && be.Code == "delivery_unknown" {
			delivery = "unknown"
			resultState = "unknown"
			event = "mutation.delivery_unknown"
		}
		finalized, finalizeErr := r.Work.ReceiptFinalize(receiptID, delivery, resultState, "")
		if finalizeErr != nil {
			return nil, finalizeErr
		}
		if mutation && taskID != "" {
			_, _ = r.Work.CheckpointCreateEvent(taskID, event, receiptID, "Verify current external state; do not replay automatically")
		}
		errMap := core.ErrorMap(callErr)
		return map[string]any{
			"ok": false, "executed": true, "error": errMap["error"],
			"receipt": finalized["receipt"], "replay_policy": finalized["replay_policy"],
		}, nil
	}

	full := raw["result"]
	shaped, err := Shape(full, spec.Shape)
	if err != nil {
		finalized, _ := r.Work.ReceiptFinalize(receiptID, "confirmed", "returned", "")
		return map[string]any{
			"ok": false, "executed": true, "error": core.ErrorMap(err)["error"],
			"receipt": finalized["receipt"], "replay_policy": finalized["replay_policy"],
		}, nil
	}
	resultRef := ""
	if shaped.Truncated || spec.Shape.StoreFullResult {
		stored, storeErr := r.Work.ResultStore(taskID, receiptID, shaped.Original)
		if storeErr != nil {
			return nil, storeErr
		}
		storedMeta, _ := stored["result"].(map[string]any)
		resultRef = str(storedMeta, "result_id")
	}
	finalized, err := r.Work.ReceiptFinalize(receiptID, "confirmed", "returned", resultRef)
	if err != nil {
		return nil, err
	}
	if mutation && taskID != "" {
		if _, err := r.Work.CheckpointCreateEvent(taskID, "mutation.after", receiptID, "Continue from the next explicit task step"); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"ok": true, "executed": true, "deduplicated": false, "tool": spec.Tool,
		"mutation": mutation, "result": shaped.Value, "truncated": shaped.Truncated,
		"result_ref": nullable(resultRef), "receipt": finalized["receipt"], "replay_policy": finalized["replay_policy"],
	}, nil
}

func (r *Runtime) mcpBatch(ctx context.Context, args map[string]any) (map[string]any, error) {
	mode := str(args, "mode")
	if mode == "" {
		mode = "parallel"
	}
	if mode != "parallel" && mode != "sequential" {
		return nil, core.Err("invalid_batch", "Batch mode must be parallel or sequential")
	}
	stopOnError := boolean(args, "stop_on_error")
	if mode == "parallel" && stopOnError {
		return nil, core.Err("invalid_batch", "stop_on_error is supported only in sequential mode")
	}
	callsRaw, ok := args["calls"].([]any)
	if !ok || len(callsRaw) == 0 || len(callsRaw) > MaxBatchCalls {
		return nil, core.Err("invalid_batch", "Batch calls must contain between 1 and 8 items")
	}
	baseTaskID, baseStepID := str(args, "task_id"), str(args, "step_id")
	specs := make([]callSpec, 0, len(callsRaw))
	names := make([]string, 0, len(callsRaw))
	for _, item := range callsRaw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, core.Err("invalid_batch", "Each batch call must be an object")
		}
		spec := callSpecFromMap(m)
		if spec.TaskID == "" {
			spec.TaskID = baseTaskID
		}
		if spec.StepID == "" {
			spec.StepID = baseStepID
		}
		if spec.Tool == "" {
			return nil, core.Err("invalid_batch", "Each batch call requires a tool")
		}
		specs = append(specs, spec)
		names = append(names, spec.Tool)
	}
	resolved, err := r.MCP.ResolveTools(ctx, names)
	if err != nil {
		return nil, err
	}
	for _, spec := range specs {
		tool := resolved[spec.Tool]
		if tool == nil {
			return nil, core.Err("mcp_denied", "A batch tool is not exposed by the shared gateway")
		}
		if mode == "parallel" && !tool.ReadOnly() {
			return nil, core.Err("parallel_mutation_denied", "Parallel batch requires every tool to explicitly advertise readOnlyHint=true")
		}
	}

	results := make([]map[string]any, len(specs))
	if mode == "sequential" {
		for i, spec := range specs {
			result, err := r.executeResolved(ctx, resolved[spec.Tool], spec)
			if err != nil {
				return nil, err
			}
			results[i] = result
			if stopOnError && result["ok"] != true {
				results = results[:i+1]
				break
			}
		}
		return map[string]any{"ok": batchOK(results), "mode": mode, "results": results, "count": len(results)}, nil
	}

	concurrency := integer(args, "concurrency", MaxParallel)
	if concurrency < 1 || concurrency > MaxParallel {
		return nil, core.Err("invalid_batch", "Parallel concurrency must be between 1 and 4")
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var fatalMu sync.Mutex
	var fatal error
	for i, spec := range specs {
		i, spec := i, spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				fatalMu.Lock()
				if fatal == nil {
					fatal = ctx.Err()
				}
				fatalMu.Unlock()
				return
			}
			defer func() { <-sem }()
			result, err := r.executeResolved(ctx, resolved[spec.Tool], spec)
			if err != nil {
				fatalMu.Lock()
				if fatal == nil {
					fatal = err
				}
				fatalMu.Unlock()
				return
			}
			results[i] = result
		}()
	}
	wg.Wait()
	if fatal != nil {
		return nil, fatal
	}
	return map[string]any{"ok": batchOK(results), "mode": mode, "concurrency": concurrency, "results": results, "count": len(results)}, nil
}

func (r *Runtime) receiptPrepare(args map[string]any) (map[string]any, error) {
	prepared, err := r.Work.ReceiptPrepare(str(args, "operation_id"), str(args, "task_id"), str(args, "step_id"), str(args, "tool_name"), args["arguments"], boolean(args, "mutation"), map[string]any{"source": "external_execution_plane"})
	if err != nil || prepared["existing"] == true {
		return prepared, err
	}
	receipt, _ := prepared["receipt"].(map[string]any)
	if boolean(args, "mutation") && str(receipt, "task_id") != "" {
		_, err = r.Work.CheckpointCreateEvent(str(receipt, "task_id"), "mutation.before", str(receipt, "receipt_id"), "External executor may proceed once; verify receipt before any retry")
		if err != nil {
			return nil, err
		}
	}
	return prepared, nil
}

func (r *Runtime) receiptFinalize(args map[string]any) (map[string]any, error) {
	receiptID := str(args, "receipt_id")
	before, err := r.Work.ReceiptGet(receiptID, "")
	if err != nil {
		return nil, err
	}
	finalized, err := r.Work.ReceiptFinalize(receiptID, str(args, "delivery_state"), str(args, "result_state"), str(args, "result_ref"))
	if err != nil {
		return nil, err
	}
	receipt, _ := before["receipt"].(map[string]any)
	if boolean(receipt, "mutation") && str(receipt, "task_id") != "" {
		event := "mutation.after"
		if str(args, "delivery_state") == "unknown" {
			event = "mutation.delivery_unknown"
		}
		_, _ = r.Work.CheckpointCreateEvent(str(receipt, "task_id"), event, receiptID, "Verify current external state before any retry")
	}
	return finalized, nil
}

func (r *Runtime) resultRead(args map[string]any) (map[string]any, error) {
	got, err := r.Work.ResultGet(str(args, "result_ref"))
	if err != nil {
		return nil, err
	}
	row, _ := got["result"].(map[string]any)
	shaped, err := Shape(row["content_json"], shapeOptions(object(args, "shape")))
	if err != nil {
		return nil, err
	}
	meta := map[string]any{}
	for _, key := range []string{"result_id", "task_id", "receipt_id", "sha256", "size", "created_at"} {
		meta[key] = row[key]
	}
	return map[string]any{"ok": true, "result": shaped.Value, "truncated": shaped.Truncated, "metadata": meta}, nil
}

func shapeOptions(args map[string]any) ShapeOptions {
	return ShapeOptions{
		JSONPath:        str(args, "json_path"),
		Fields:          stringsList(args["fields"]),
		Offset:          integer(args, "offset", 0),
		Limit:           integer(args, "limit", 0),
		MaxChars:        integer(args, "max_chars", DefaultPreviewChars),
		StoreFullResult: boolean(args, "store_full_result"),
	}
}

func batchOK(results []map[string]any) bool {
	for _, result := range results {
		if result == nil || result["ok"] != true {
			return false
		}
	}
	return true
}

func str(args map[string]any, key string) string {
	if args == nil || args[key] == nil {
		return ""
	}
	return fmt.Sprint(args[key])
}

func integer(args map[string]any, key string, fallback int) int {
	if args == nil || args[key] == nil {
		return fallback
	}
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return fallback
	}
}

func boolean(args map[string]any, key string) bool {
	if args == nil || args[key] == nil {
		return false
	}
	value, _ := args[key].(bool)
	return value
}

func object(args map[string]any, key string) map[string]any {
	if args == nil || args[key] == nil {
		return map[string]any{}
	}
	value, _ := args[key].(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}

func stringsList(value any) []string {
	if value == nil {
		return nil
	}
	if typed, ok := value.([]string); ok {
		return append([]string(nil), typed...)
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil {
			out = append(out, fmt.Sprint(item))
		}
	}
	return out
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
