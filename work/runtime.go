package work

import (
	"fmt"
	"sync"

	"github.com/codexpro/bridge/core"
)

const Version = "2.0.0"

type Runtime struct {
	Store Store
}

func New(path string) *Runtime {
	return &Runtime{Store: Store{Path: path, initMu: &sync.Mutex{}}}
}

func (r *Runtime) Health() map[string]any {
	return r.Store.Health()
}

func (r *Runtime) Dispatch(operation string, arguments map[string]any) (map[string]any, error) {
	if !containsOperation(operation) {
		return nil, core.Err("unsupported_work_operation", "Unsupported Work Runtime operation")
	}
	args := arguments
	if args == nil {
		args = map[string]any{}
	}
	switch operation {
	case "task.create":
		return r.TaskCreate(strArg(args, "title"), strArg(args, "description"), strArg(args, "project_id"), defaultString(args, "priority", "normal"), boolArg(args, "audit_required"), mapArg(args, "metadata"))
	case "task.get":
		return r.TaskGet(strArg(args, "task_id"))
	case "task.list":
		return r.TaskList(strArg(args, "project_id"), strArg(args, "status"), intDefault(args, "limit", 50), intDefault(args, "offset", 0))
	case "task.update":
		return r.TaskUpdate(strArg(args, "task_id"), without(args, "task_id"))
	case "task.transition":
		return r.TaskTransition(strArg(args, "task_id"), strArg(args, "status"))
	case "task.step_add":
		return r.TaskStepAdd(strArg(args, "task_id"), strArg(args, "title"), strArg(args, "description"), optionalInt(args, "position"))
	case "task.step_update":
		return r.TaskStepUpdate(strArg(args, "step_id"), without(args, "step_id"))
	case "task.summary":
		return r.TaskSummary(strArg(args, "task_id"))
	case "task.resume":
		return r.TaskResume(strArg(args, "task_id"), strArg(args, "project_id"), strArg(args, "title"))
	case "checkpoint.create":
		return r.CheckpointCreate(strArg(args, "task_id"), anySliceArg(args, "important_findings"), anySliceArg(args, "decisions"), strArg(args, "next_action"), stringSliceArg(args, "artifact_ids"), strArg(args, "audit_id"))
	case "checkpoint.get":
		return r.CheckpointGet(strArg(args, "checkpoint_id"))
	case "checkpoint.list":
		return r.CheckpointList(strArg(args, "task_id"), intDefault(args, "limit", 50), intDefault(args, "offset", 0))
	case "project.create":
		return r.ProjectCreate(strArg(args, "name"), strArg(args, "description"), strArg(args, "workspace_ref"), stringSliceArg(args, "relevant_skills"), stringSliceArg(args, "relevant_mcp"), stringSliceArg(args, "memory_queries"), stringSliceArg(args, "project_rules"), stringSliceArg(args, "known_risks"))
	case "project.get":
		return r.ProjectGet(strArg(args, "project_id"))
	case "project.list":
		return r.ProjectList(intDefault(args, "limit", 50), intDefault(args, "offset", 0))
	case "project.context":
		return r.ProjectContext(strArg(args, "project_id"))
	case "project.refresh":
		return r.ProjectRefresh(strArg(args, "project_id"), without(args, "project_id"))
	case "audit.create":
		return r.AuditCreate(strArg(args, "task_id"), mapSliceArg(args, "checks"))
	case "audit.record":
		return r.AuditRecord(strArg(args, "audit_id"), strArg(args, "check_id"), args["actual"], strArg(args, "evidence_type"), strArg(args, "evidence_ref"), strArg(args, "command_or_tool"), optionalInt(args, "exit_code"))
	case "audit.evaluate":
		return r.AuditEvaluate(strArg(args, "audit_id"))
	case "audit.get":
		return r.AuditGet(strArg(args, "audit_id"))
	case "audit.list":
		return r.AuditList(strArg(args, "task_id"), intDefault(args, "limit", 50), intDefault(args, "offset", 0))
	case "artifact.register":
		return r.ArtifactRegister(strArg(args, "name"), strArg(args, "path_or_ref"), defaultString(args, "type", "other"), strArg(args, "task_id"), strArg(args, "project_id"), strArg(args, "sha256"), optionalInt(args, "size"), defaultString(args, "verification_status", "declared"), strArg(args, "verification_evidence_ref"), mapArg(args, "metadata"))
	case "artifact.get":
		return r.ArtifactGet(strArg(args, "artifact_id"))
	case "artifact.list":
		return r.ArtifactList(strArg(args, "task_id"), strArg(args, "project_id"), strArg(args, "status"), intDefault(args, "limit", 100), intDefault(args, "offset", 0))
	case "artifact.finalize":
		return r.ArtifactFinalize(strArg(args, "artifact_id"))
	case "artifact.archive":
		return r.ArtifactArchive(strArg(args, "artifact_id"))
	default:
		return nil, core.Err("unsupported_work_operation", "Unsupported Work Runtime operation")
	}
}

func containsOperation(op string) bool {
	for _, item := range WorkOperations {
		if item == op {
			return true
		}
	}
	return false
}

func strArg(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func defaultString(args map[string]any, key, fallback string) string {
	v := strArg(args, key)
	if v == "" {
		return fallback
	}
	return v
}

func intDefault(args map[string]any, key string, fallback int) int {
	v, ok := args[key]
	if !ok || v == nil {
		return fallback
	}
	return toInt(v)
}

func optionalInt(args map[string]any, key string) *int {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	n := toInt(v)
	return &n
}

func boolArg(args map[string]any, key string) bool {
	return toBool(args[key])
}

func mapArg(args map[string]any, key string) map[string]any {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func anySliceArg(args map[string]any, key string) []any {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	if a, ok := v.([]any); ok {
		return a
	}
	return nil
}

func stringSliceArg(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	switch a := v.(type) {
	case []string:
		return a
	case []any:
		out := make([]string, 0, len(a))
		for _, item := range a {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}

func mapSliceArg(args map[string]any, key string) []map[string]any {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	if typed, ok := v.([]map[string]any); ok {
		return typed
	}
	a, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(a))
	for _, item := range a {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func without(args map[string]any, keys ...string) map[string]any {
	drop := setOf(keys...)
	out := make(map[string]any, len(args))
	for key, value := range args {
		if !drop[key] {
			out[key] = value
		}
	}
	return out
}
