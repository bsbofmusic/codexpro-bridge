package work

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/codexpro/bridge/core"
)

func newTestRuntime(t *testing.T) *Runtime {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "work.sqlite3"))
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

func TestWorkLifecycleAndOperationFamilies(t *testing.T) {
	r := newTestRuntime(t)
	must := mustMap(t)
	project := must(r.ProjectCreate("Go bridge", "migration", "/tmp/workspace", []string{"bridge"}, []string{"search"}, []string{"prior"}, []string{"keep thin"}, []string{"schema drift"}))
	projectID := project["project"].(map[string]any)["project_id"].(string)

	task := must(r.TaskCreate("Migrate", "full parity", projectID, "high", true, map[string]any{"phase": 3}))
	taskID := task["task"].(map[string]any)["task_id"].(string)
	must(r.TaskTransition(taskID, "running"))

	step := must(r.TaskStepAdd(taskID, "Port Work", "", nil))
	stepID := step["step"].(map[string]any)["step_id"].(string)
	must(r.TaskStepUpdate(stepID, map[string]any{"status": "doing"}))
	must(r.TaskStepUpdate(stepID, map[string]any{"status": "done"}))

	size := 10
	artifact := must(r.ArtifactRegister("binary", "/tmp/bridge", "binary", taskID, "", "abc", &size, "declared", "", map[string]any{"kind": "test"}))
	artifactID := artifact["artifact"].(map[string]any)["artifact_id"].(string)
	must(r.ArtifactFinalize(artifactID))

	checkpoint := must(r.CheckpointCreate(taskID, []any{"ported"}, []any{"keep schema"}, "audit", []string{artifactID}, ""))
	checkpointID := checkpoint["checkpoint"].(map[string]any)["checkpoint_id"].(string)
	if got := must(r.CheckpointGet(checkpointID)); got["checkpoint"] == nil {
		t.Fatal("checkpoint get failed")
	}

	must(r.TaskTransition(taskID, "audit"))
	audit := must(r.AuditCreate(taskID, []map[string]any{{"name": "tests", "expected": map[string]any{"ok": true}}}))
	auditID := audit["audit"].(map[string]any)["audit_id"].(string)
	checks := audit["checks"].([]map[string]any)
	if len(checks) != 1 {
		t.Fatalf("unexpected checks: %#v", checks)
	}
	checkID := checks[0]["check_id"].(string)
	must(r.AuditRecord(auditID, checkID, map[string]any{"ok": true}, "verified_structured", "test:go", "go test", nil))
	evaluated := must(r.AuditEvaluate(auditID))
	if evaluated["audit"].(map[string]any)["status"] != "PASS" || evaluated["stale"] != false {
		t.Fatalf("audit did not pass: %#v", evaluated)
	}
	must(r.TaskTransition(taskID, "done"))

	summary := must(r.TaskSummary(taskID))
	if summary["task"].(map[string]any)["status"] != "done" {
		t.Fatalf("summary status drifted: %#v", summary)
	}
	resume := must(r.TaskResume(taskID, "", ""))
	if resume["mutation_replayed"] != false || resume["resume_semantics"] != "state_only" {
		t.Fatalf("resume semantics drifted: %#v", resume)
	}
	context := must(r.ProjectContext(projectID))
	if context["project"] == nil {
		t.Fatalf("project context missing: %#v", context)
	}

	for operation, args := range map[string]map[string]any{
		"project.get": {"project_id": projectID}, "project.list": {}, "task.get": {"task_id": taskID},
		"task.list": {"project_id": projectID}, "task.summary": {"task_id": taskID},
		"checkpoint.get": {"checkpoint_id": checkpointID}, "checkpoint.list": {"task_id": taskID},
		"audit.get": {"audit_id": auditID}, "audit.list": {"task_id": taskID},
		"artifact.get": {"artifact_id": artifactID}, "artifact.list": {"task_id": taskID},
	} {
		if _, err := r.Dispatch(operation, args); err != nil {
			t.Fatalf("dispatch %s failed: %v", operation, err)
		}
	}
}

func TestCurrentRevisionAuditGateAndArtifactVerification(t *testing.T) {
	r := newTestRuntime(t)
	must := mustMap(t)
	if _, err := r.ArtifactRegister("x", "/tmp/x", "other", "", "", "", nil, "verified", "", nil); err == nil {
		t.Fatal("verified artifact without evidence should fail")
	}

	task := must(r.TaskCreate("audit", "", "", "normal", true, nil))
	taskID := task["task"].(map[string]any)["task_id"].(string)
	must(r.TaskTransition(taskID, "running"))
	must(r.TaskTransition(taskID, "audit"))
	audit := must(r.AuditCreate(taskID, []map[string]any{{"name": "gate", "expected": true}}))
	auditID := audit["audit"].(map[string]any)["audit_id"].(string)
	checkID := audit["checks"].([]map[string]any)[0]["check_id"].(string)
	must(r.AuditRecord(auditID, checkID, true, "verified_structured", "test", "", nil))
	must(r.AuditEvaluate(auditID))
	must(r.TaskUpdate(taskID, map[string]any{"description": "changed after pass"}))
	_, err := r.TaskTransition(taskID, "done")
	be, ok := err.(core.BridgeError)
	if !ok || be.Code != "audit_gate_blocked" {
		t.Fatalf("stale audit was accepted: %#v", err)
	}
}

func TestAmbiguousResumeReturnsCandidatesWithoutReplay(t *testing.T) {
	r := newTestRuntime(t)
	must := mustMap(t)
	must(r.TaskCreate("same task", "", "", "normal", false, nil))
	must(r.TaskCreate("same task", "", "", "normal", false, nil))
	got, err := r.TaskResume("", "", "same")
	if err != nil {
		t.Fatal(err)
	}
	if got["ok"] != false || got["mutation_replayed"] != false {
		t.Fatalf("ambiguous resume semantics drifted: %#v", got)
	}
	if len(got["candidates"].([]map[string]any)) != 2 {
		t.Fatalf("expected two candidates: %#v", got)
	}
}

func TestProductionDBCopyCompatibility(t *testing.T) {
	const production = "/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3"
	data, err := os.ReadFile(production)
	if os.IsNotExist(err) {
		t.Skip("production Work Runtime DB is not present")
	}
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), "production-copy.sqlite3")
	if err := os.WriteFile(copyPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	r := New(copyPath)
	health := r.Health()
	if health["ok"] != true || health["schema_version"] != SchemaVersion {
		t.Fatalf("production DB copy incompatible after additive migration: %#v", health)
	}
	created, err := r.TaskCreate("copy-only probe", "must not touch production", "", "normal", false, nil)
	if err != nil || created["ok"] != true {
		t.Fatalf("production DB copy write failed: %#v %v", created, err)
	}
}

func TestDeclaredOperationsAreUnique(t *testing.T) {
	if len(WorkOperations) == 0 {
		t.Fatal("Work operation catalog is empty")
	}
	seen := map[string]bool{}
	for _, operation := range WorkOperations {
		if seen[operation] {
			t.Fatalf("duplicate operation: %s", operation)
		}
		seen[operation] = true
	}
}
