package work

import (
	"fmt"
	"strings"
	"testing"

	"github.com/codexpro/bridge/core"
)

func TestReceiptIdempotencyAndResumeCapsule(t *testing.T) {
	r := newTestRuntime(t)
	must := mustMap(t)
	task := must(r.TaskCreate("deploy bridge", "finish production cutover", "", "high", false, nil))
	taskID := task["task"].(map[string]any)["task_id"].(string)
	must(r.TaskTransition(taskID, "running"))
	step := must(r.TaskStepAdd(taskID, "production switch", "", nil))
	stepID := step["step"].(map[string]any)["step_id"].(string)
	must(r.TaskStepUpdate(stepID, map[string]any{"status": "doing"}))

	first := must(r.ReceiptPrepare("op-prod-switch", taskID, stepID, "deploy", map[string]any{"target": "prod"}, true, nil))
	if first["existing"] != false || first["do_not_replay"] != false {
		t.Fatalf("unexpected first receipt: %#v", first)
	}
	receiptID := first["receipt"].(map[string]any)["receipt_id"].(string)

	second := must(r.ReceiptPrepare("op-prod-switch", taskID, stepID, "deploy", map[string]any{"target": "prod"}, true, nil))
	if second["existing"] != true || second["do_not_replay"] != true || second["replay_policy"] != "VERIFY_BEFORE_ANY_RETRY" {
		t.Fatalf("prepared receipt did not fail closed on replay: %#v", second)
	}

	final := must(r.ReceiptFinalize(receiptID, "confirmed", "returned", ""))
	if final["replay_policy"] != "DO_NOT_REPLAY" {
		t.Fatalf("confirmed mutation replay policy drifted: %#v", final)
	}

	third := must(r.ReceiptPrepare("op-prod-switch", taskID, stepID, "deploy", map[string]any{"target": "prod"}, true, nil))
	if third["replay_policy"] != "DO_NOT_REPLAY" {
		t.Fatalf("confirmed operation was not deduplicated: %#v", third)
	}

	if _, err := r.ReceiptPrepare("op-prod-switch", taskID, stepID, "deploy", map[string]any{"target": "other"}, true, nil); err == nil {
		t.Fatal("operation id reused with different arguments should fail")
	} else if be, ok := err.(core.BridgeError); !ok || be.Code != "idempotency_conflict" {
		t.Fatalf("wrong idempotency error: %#v", err)
	}

	capsule := must(r.ResumeCapsule(taskID))
	last := capsule["last_successful_operation"].(map[string]any)
	if last["operation_id"] != "op-prod-switch" || last["replay_policy"] != "DO_NOT_REPLAY" {
		t.Fatalf("resume capsule lost receipt state: %#v", capsule)
	}
	if len(capsule["do_not_replay"].([]map[string]any)) != 1 {
		t.Fatalf("resume capsule missing no-replay ledger: %#v", capsule["do_not_replay"])
	}
}

func TestMutationReceiptRequiresOperationIDAndFinalizeIsImmutable(t *testing.T) {
	r := newTestRuntime(t)
	if _, err := r.ReceiptPrepare("", "", "", "deploy", map[string]any{"target": "prod"}, true, nil); err == nil {
		t.Fatal("mutation receipt without operation_id should fail")
	} else if be, ok := err.(core.BridgeError); !ok || be.Code != "operation_id_required" {
		t.Fatalf("unexpected missing operation id error: %#v", err)
	}

	prepared, err := r.ReceiptPrepare("op-final", "", "", "deploy", map[string]any{"target": "prod"}, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	receiptID := prepared["receipt"].(map[string]any)["receipt_id"].(string)
	first, err := r.ReceiptFinalize(receiptID, "confirmed", "returned", "")
	if err != nil || first["existing"] != false {
		t.Fatalf("first finalize failed: %#v %v", first, err)
	}
	second, err := r.ReceiptFinalize(receiptID, "confirmed", "returned", "")
	if err != nil || second["existing"] != true {
		t.Fatalf("idempotent finalize failed: %#v %v", second, err)
	}
	if _, err := r.ReceiptFinalize(receiptID, "unknown", "unknown", ""); err == nil {
		t.Fatal("finalized receipt was rewritten")
	} else if be, ok := err.(core.BridgeError); !ok || be.Code != "receipt_finalized" {
		t.Fatalf("unexpected rewrite error: %#v", err)
	}

	read, err := r.ReceiptPrepare("", "", "", "read", map[string]any{"q": "x"}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	readAgain, err := r.ReceiptPrepare(read["receipt"].(map[string]any)["operation_id"].(string), "", "", "read", map[string]any{"q": "x"}, false, nil)
	if err != nil || readAgain["do_not_replay"] != false || readAgain["replay_policy"] != "SAFE_TO_REPEAT_READ_ONLY_IF_NEEDED" {
		t.Fatalf("read-only replay policy drifted: %#v %v", readAgain, err)
	}
}

func TestConversationAffinityStoresOnlyFingerprint(t *testing.T) {
	r := newTestRuntime(t)
	must := mustMap(t)
	task := must(r.TaskCreate("resume me", "", "", "normal", false, nil))
	taskID := task["task"].(map[string]any)["task_id"].(string)
	firstMessage := "  Bridge Web   Full Power Upgrade  "
	bound := must(r.AffinityBind(firstMessage, taskID))
	fingerprint := bound["conversation_fingerprint"].(string)
	if !strings.HasPrefix(fingerprint, "conv_") || strings.Contains(strings.ToLower(fingerprint), "bridge") {
		t.Fatalf("conversation fingerprint leaked raw content: %q", fingerprint)
	}
	resolved := must(r.AffinityResolve("bridge web full power upgrade"))
	if resolved["matched"] != true || resolved["affinity"].(map[string]any)["task_id"] != taskID {
		t.Fatalf("normalized affinity did not resolve: %#v", resolved)
	}
	removed := must(r.AffinityUnbind(firstMessage))
	if removed["removed"] != true {
		t.Fatalf("affinity was not removed: %#v", removed)
	}
}

func TestResultStoreRoundTripAndBound(t *testing.T) {
	r := newTestRuntime(t)
	must := mustMap(t)
	task := must(r.TaskCreate("results", "", "", "normal", false, nil))
	taskID := task["task"].(map[string]any)["task_id"].(string)
	stored := must(r.ResultStore(taskID, "", map[string]any{"items": []any{1.0, 2.0, 3.0}, "secret": "safe-value"}))
	resultID := stored["result"].(map[string]any)["result_id"].(string)
	got := must(r.ResultGet(resultID))
	row := got["result"].(map[string]any)
	content, ok := row["content_json"].(map[string]any)
	if !ok || len(content["items"].([]any)) != 3 {
		t.Fatalf("stored result did not round trip: %#v", row)
	}

	tooLarge := strings.Repeat("x", MaxStoredResultBytes+1)
	if _, err := r.ResultStore(taskID, "", map[string]any{"data": tooLarge}); err == nil {
		t.Fatal("oversized result should be rejected")
	}
	must(r.ResultDelete(resultID))
}

func TestDeterministicEventCheckpoints(t *testing.T) {
	r := newTestRuntime(t)
	must := mustMap(t)
	task := must(r.TaskCreate("events", "", "", "normal", false, nil))
	taskID := task["task"].(map[string]any)["task_id"].(string)
	must(r.TaskTransition(taskID, "running"))
	step := must(r.TaskStepAdd(taskID, "step", "", nil))
	stepID := step["step"].(map[string]any)["step_id"].(string)
	must(r.TaskStepUpdate(stepID, map[string]any{"status": "done"}))

	artifact := must(r.ArtifactRegister("a", "/tmp/a", "other", taskID, "", "", nil, "declared", "", nil))
	artifactID := artifact["artifact"].(map[string]any)["artifact_id"].(string)
	must(r.ArtifactFinalize(artifactID))

	must(r.TaskTransition(taskID, "audit"))
	audit := must(r.AuditCreate(taskID, []map[string]any{{"name": "ok", "expected": true}}))
	auditID := audit["audit"].(map[string]any)["audit_id"].(string)
	checkID := audit["checks"].([]map[string]any)[0]["check_id"].(string)
	must(r.AuditRecord(auditID, checkID, true, "verified_structured", "test", "go test", nil))
	must(r.AuditEvaluate(auditID))

	list := must(r.CheckpointList(taskID, 20, 0))
	rows := list["checkpoints"].([]map[string]any)
	seen := map[string]bool{}
	for _, row := range rows {
		seen[fmt.Sprint(row["event_type"])] = true
	}
	for _, event := range []string{"step.complete", "artifact.finalized", "audit.complete"} {
		if !seen[event] {
			t.Fatalf("missing deterministic checkpoint event %s: %#v", event, rows)
		}
	}
}
