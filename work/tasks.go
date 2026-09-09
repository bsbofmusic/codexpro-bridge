package work

import (
	"fmt"
	"strings"

	"github.com/codexpro/bridge/core"
)

func (r *Runtime) TaskCreate(title, description, projectID, priority string, auditRequired bool, metadata map[string]any) (map[string]any, error) {
	if strings.TrimSpace(title) == "" {
		return nil, core.Err("invalid_work_input", "Task title is required")
	}
	if priority == "" {
		priority = "normal"
	}
	meta, err := boundedJSON(defaultMap(metadata))
	if err != nil {
		return nil, err
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if projectID != "" {
		if _, err := one(db, "projects", "SELECT * FROM projects WHERE project_id = ?", projectID); err != nil {
			return nil, err
		}
	}
	id := newID("tsk")
	stamp := now()
	_, err = db.Exec(`INSERT INTO tasks
(task_id,project_id,title,description,status,priority,audit_required,current_step_id,revision,created_at,updated_at,completed_at,metadata)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, sqlNullableString(projectID), strings.TrimSpace(title), description, "pending", priority, boolInt(auditRequired), nil, 1, stamp, stamp, nil, meta)
	if err != nil {
		return nil, err
	}
	task, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "task": task}, nil
}

func (r *Runtime) TaskGet(taskID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	task, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID)
	if err != nil {
		return nil, err
	}
	steps, err := queryRows(db, "task_steps", "SELECT * FROM task_steps WHERE task_id = ? ORDER BY position", taskID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "task": task, "steps": steps}, nil
}

func (r *Runtime) TaskList(projectID, status string, limit, offset int) (map[string]any, error) {
	if status != "" && !taskStates[status] {
		return nil, core.Err("invalid_work_input", "Unknown task status")
	}
	clauses := []string{}
	args := []any{}
	if projectID != "" {
		clauses = append(clauses, "project_id = ?")
		args = append(args, projectID)
	}
	if status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, status)
	}
	query := "SELECT * FROM tasks"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, clamp(limit, 1, 200), maxInt(offset, 0))
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := queryRows(db, "tasks", query, args...)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "tasks": rows, "count": len(rows)}, nil
}

func (r *Runtime) TaskUpdate(taskID string, changes map[string]any) (map[string]any, error) {
	allowed := setOf("project_id", "title", "description", "priority", "audit_required", "metadata")
	if len(changes) == 0 {
		return r.TaskGet(taskID)
	}
	sets := []string{}
	args := []any{}
	for key, value := range changes {
		if !allowed[key] {
			return nil, core.Err("invalid_work_input", "Unsupported task fields")
		}
		if key == "metadata" {
			var err error
			value, err = boundedJSON(defaultMapValue(value))
			if err != nil {
				return nil, err
			}
		} else if key == "audit_required" {
			value = boolInt(toBool(value))
		} else if key == "project_id" && value == nil {
			value = nil
		}
		sets = append(sets, key+" = ?")
		args = append(args, value)
	}
	sets = append(sets, "revision = revision + 1", "updated_at = ?")
	args = append(args, now(), taskID)
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	res, err := db.Exec("UPDATE tasks SET "+strings.Join(sets, ", ")+" WHERE task_id = ?", args...)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return nil, core.Err("work_not_found", "Task not found")
	}
	task, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "task": task}, nil
}

func (r *Runtime) TaskTransition(taskID, status string) (map[string]any, error) {
	if !taskStates[status] {
		return nil, core.Err("invalid_work_input", "Unknown task status")
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	task, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID)
	if err != nil {
		return nil, err
	}
	current := fmt.Sprint(task["status"])
	if !taskTransitions[current][status] {
		return nil, core.Err("invalid_task_transition", fmt.Sprintf("Task cannot transition from %s to %s", current, status))
	}
	if status == "done" && toBool(task["audit_required"]) {
		audits, err := queryRows(db, "audits", "SELECT * FROM audits WHERE task_id = ? ORDER BY created_at DESC, rowid DESC LIMIT 1", taskID)
		if err != nil {
			return nil, err
		}
		if len(audits) == 0 || fmt.Sprint(audits[0]["status"]) != "PASS" || toInt(audits[0]["audited_task_revision"]) != toInt(task["revision"]) {
			return nil, core.Err("audit_gate_blocked", "Task completion requires a PASS audit for the current task revision")
		}
	}
	stamp := now()
	var completed any
	if status == "done" {
		completed = stamp
	}
	if _, err := db.Exec("UPDATE tasks SET status = ?, completed_at = ?, updated_at = ? WHERE task_id = ?", status, completed, stamp, taskID); err != nil {
		return nil, err
	}
	task, err = one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "task": task}, nil
}

func (r *Runtime) TaskStepAdd(taskID, title, description string, position *int) (map[string]any, error) {
	if strings.TrimSpace(title) == "" {
		return nil, core.Err("invalid_work_input", "Step title is required")
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID); err != nil {
		return nil, err
	}
	pos := 0
	if position == nil {
		if err := db.QueryRow("SELECT COALESCE(MAX(position),0)+1 FROM task_steps WHERE task_id = ?", taskID).Scan(&pos); err != nil {
			return nil, err
		}
	} else {
		pos = *position
	}
	id := newID("stp")
	stamp := now()
	_, err = db.Exec(`INSERT INTO task_steps
(step_id,task_id,position,title,description,status,blocker,created_at,updated_at,completed_at)
VALUES (?,?,?,?,?,?,?,?,?,?)`, id, taskID, pos, strings.TrimSpace(title), description, "todo", nil, stamp, stamp, nil)
	if err != nil {
		if sqliteConstraint(err) {
			return nil, core.Err("invalid_work_input", "Step position is already in use")
		}
		return nil, err
	}
	if err := touchTaskRevision(db, taskID); err != nil {
		return nil, err
	}
	step, err := one(db, "task_steps", "SELECT * FROM task_steps WHERE step_id = ?", id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "step": step}, nil
}

func (r *Runtime) TaskStepUpdate(stepID string, changes map[string]any) (map[string]any, error) {
	allowed := setOf("title", "description", "status", "blocker", "position")
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	step, err := one(db, "task_steps", "SELECT * FROM task_steps WHERE step_id = ?", stepID)
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 {
		return map[string]any{"ok": true, "step": step}, nil
	}
	for key := range changes {
		if !allowed[key] {
			return nil, core.Err("invalid_work_input", "Unsupported step fields")
		}
	}
	if raw, ok := changes["status"]; ok && !stepStates[fmt.Sprint(raw)] {
		return nil, core.Err("invalid_work_input", "Unknown step status")
	}
	sets := []string{}
	args := []any{}
	for key, value := range changes {
		sets = append(sets, key+" = ?")
		args = append(args, value)
	}
	targetStatus := fmt.Sprint(step["status"])
	if v, ok := changes["status"]; ok {
		targetStatus = fmt.Sprint(v)
	}
	stamp := now()
	sets = append(sets, "updated_at = ?", "completed_at = ?")
	args = append(args, stamp, terminalStepTime(targetStatus, stamp), stepID)
	_, err = db.Exec("UPDATE task_steps SET "+strings.Join(sets, ", ")+" WHERE step_id = ?", args...)
	if err != nil {
		if sqliteConstraint(err) {
			return nil, core.Err("invalid_work_input", "Step update violates task ordering constraints")
		}
		return nil, err
	}
	taskID := fmt.Sprint(step["task_id"])
	if err := touchTaskRevision(db, taskID); err != nil {
		return nil, err
	}
	if targetStatus == "doing" {
		if _, err := db.Exec("UPDATE tasks SET current_step_id = ? WHERE task_id = ?", stepID, taskID); err != nil {
			return nil, err
		}
	}
	updated, err := one(db, "task_steps", "SELECT * FROM task_steps WHERE step_id = ?", stepID)
	if err != nil {
		return nil, err
	}
	if targetStatus == "done" {
		if _, err := checkpointCreateDB(db, taskID, nil, []any{map[string]any{"event": "step.complete", "step_id": stepID}}, "", nil, "", "step.complete", ""); err != nil {
			return nil, err
		}
	}
	return map[string]any{"ok": true, "step": updated}, nil
}

func (r *Runtime) TaskSummary(taskID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	task, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID)
	if err != nil {
		return nil, err
	}
	var project any
	if pid := valueString(task["project_id"]); pid != "" {
		p, _ := one(db, "projects", "SELECT * FROM projects WHERE project_id = ?", pid)
		project = p
	}
	steps, err := queryRows(db, "task_steps", "SELECT * FROM task_steps WHERE task_id = ? ORDER BY position", taskID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for state := range stepStates {
		counts[state] = 0
	}
	for _, step := range steps {
		counts[fmt.Sprint(step["status"])]++
	}
	checkpoints, err := queryRows(db, "checkpoints", "SELECT * FROM checkpoints WHERE task_id = ? ORDER BY sequence DESC LIMIT 1", taskID)
	if err != nil {
		return nil, err
	}
	audits, err := queryRows(db, "audits", "SELECT * FROM audits WHERE task_id = ? ORDER BY created_at DESC, rowid DESC LIMIT 1", taskID)
	if err != nil {
		return nil, err
	}
	artifacts, err := queryRows(db, "artifacts", "SELECT * FROM artifacts WHERE task_id = ? ORDER BY updated_at DESC", taskID)
	if err != nil {
		return nil, err
	}
	var checkpoint, audit any
	var nextAction any
	if len(checkpoints) > 0 {
		checkpoint = checkpoints[0]
		if v := checkpoints[0]["next_action"]; v != nil && fmt.Sprint(v) != "" {
			nextAction = v
		}
	}
	if len(audits) > 0 {
		audit = audits[0]
	}
	if nextAction == nil {
		for _, step := range steps {
			st := fmt.Sprint(step["status"])
			if st != "done" && st != "skipped" {
				nextAction = "Continue step: " + fmt.Sprint(step["title"])
				break
			}
		}
	}
	return map[string]any{
		"ok": true, "task": task, "project": project,
		"progress":          map[string]any{"total_steps": len(steps), "by_status": counts},
		"latest_checkpoint": checkpoint, "latest_audit": audit,
		"artifacts": artifacts, "next_action": nextAction,
	}, nil
}

func (r *Runtime) TaskResume(taskID, projectID, title string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var task map[string]any
	if taskID != "" {
		task, err = one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID)
		if err != nil {
			return nil, err
		}
	} else {
		clauses := []string{"status IN ('pending','running','blocked','audit')"}
		args := []any{}
		if projectID != "" {
			clauses = append(clauses, "project_id = ?")
			args = append(args, projectID)
		}
		if title != "" {
			clauses = append(clauses, "LOWER(title) LIKE LOWER(?)")
			args = append(args, "%"+title+"%")
		}
		candidates, err := queryRows(db, "tasks", "SELECT * FROM tasks WHERE "+strings.Join(clauses, " AND ")+" ORDER BY updated_at DESC LIMIT 20", args...)
		if err != nil {
			return nil, err
		}
		if len(candidates) == 0 {
			return nil, core.Err("work_not_found", "No resumable task matched")
		}
		if len(candidates) > 1 {
			brief := make([]map[string]any, 0, len(candidates))
			for _, c := range candidates {
				brief = append(brief, map[string]any{
					"task_id": c["task_id"], "project_id": c["project_id"], "title": c["title"],
					"status": c["status"], "updated_at": c["updated_at"],
				})
			}
			return map[string]any{
				"ok":         false,
				"error":      map[string]any{"code": "ambiguous_task", "message": "Multiple resumable tasks matched; choose a task_id"},
				"candidates": brief, "mutation_replayed": false,
			}, nil
		}
		task = candidates[0]
	}
	checkpoints, err := queryRows(db, "checkpoints", "SELECT * FROM checkpoints WHERE task_id = ? ORDER BY sequence DESC LIMIT 1", task["task_id"])
	if err != nil {
		return nil, err
	}
	var checkpoint any
	var nextAction any
	if len(checkpoints) > 0 {
		checkpoint = checkpoints[0]
		nextAction = checkpoints[0]["next_action"]
	}
	var projectContext any
	if pid := valueString(task["project_id"]); pid != "" {
		project, err := one(db, "projects", "SELECT * FROM projects WHERE project_id = ?", pid)
		if err == nil {
			projectContext = map[string]any{
				"project_id": project["project_id"], "workspace_ref": project["workspace_ref"],
				"relevant_skills": project["relevant_skills"], "relevant_mcp": project["relevant_mcp"],
				"memory_queries": project["memory_queries"], "project_rules": project["project_rules"],
				"known_risks": project["known_risks"],
			}
		}
	}
	return map[string]any{
		"ok": true, "task": task, "latest_checkpoint": checkpoint, "project_context": projectContext,
		"next_action": nextAction, "mutation_replayed": false, "resume_semantics": "state_only",
	}, nil
}

func terminalStepTime(status, stamp string) any {
	if status == "done" || status == "failed" || status == "skipped" {
		return stamp
	}
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func defaultMap(v map[string]any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	return v
}

func defaultMapValue(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}

func valueString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
