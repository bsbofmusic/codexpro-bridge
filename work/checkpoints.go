package work

import (
	"database/sql"
	"fmt"
)

func (r *Runtime) CheckpointCreate(taskID string, importantFindings, decisions []any, nextAction string, artifactIDs []string, auditID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return checkpointCreateDB(db, taskID, importantFindings, decisions, nextAction, artifactIDs, auditID, "", "")
}

func (r *Runtime) CheckpointCreateEvent(taskID, eventType, receiptID, nextAction string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return checkpointCreateDB(db, taskID, nil, []any{map[string]any{"event": eventType}}, nextAction, nil, "", eventType, receiptID)
}

func checkpointCreateDB(db *sql.DB, taskID string, importantFindings, decisions []any, nextAction string, artifactIDs []string, auditID, eventType, receiptID string) (map[string]any, error) {
	task, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID)
	if err != nil {
		return nil, err
	}
	steps, err := queryRows(db, "task_steps", "SELECT step_id,status FROM task_steps WHERE task_id = ? ORDER BY position", taskID)
	if err != nil {
		return nil, err
	}
	completed := []string{}
	pending := []string{}
	blocked := []string{}
	for _, step := range steps {
		id := fmt.Sprint(step["step_id"])
		switch fmt.Sprint(step["status"]) {
		case "done", "skipped":
			completed = append(completed, id)
		case "todo", "doing":
			pending = append(pending, id)
		case "blocked":
			blocked = append(blocked, id)
		}
	}
	latest, err := queryRows(db, "checkpoints", "SELECT checkpoint_id,sequence FROM checkpoints WHERE task_id = ? ORDER BY sequence DESC LIMIT 1", taskID)
	if err != nil {
		return nil, err
	}
	sequence := 1
	var parent any
	if len(latest) > 0 {
		sequence = toInt(latest[0]["sequence"]) + 1
		parent = latest[0]["checkpoint_id"]
	}
	if importantFindings == nil {
		importantFindings = []any{}
	}
	if decisions == nil {
		decisions = []any{}
	}
	if artifactIDs == nil {
		artifactIDs = []string{}
	}
	completedJSON, err := boundedJSON(completed)
	if err != nil {
		return nil, err
	}
	pendingJSON, err := boundedJSON(pending)
	if err != nil {
		return nil, err
	}
	blockedJSON, err := boundedJSON(blocked)
	if err != nil {
		return nil, err
	}
	findingsJSON, err := boundedJSON(importantFindings)
	if err != nil {
		return nil, err
	}
	decisionsJSON, err := boundedJSON(decisions)
	if err != nil {
		return nil, err
	}
	artifactsJSON, err := boundedJSON(artifactIDs)
	if err != nil {
		return nil, err
	}
	id := newID("chk")
	_, err = db.Exec(`INSERT INTO checkpoints
(checkpoint_id,task_id,sequence,parent_checkpoint_id,current_step_id,task_status,task_revision,
 completed_step_ids,pending_step_ids,blocked_step_ids,important_findings,decisions,next_action,artifact_ids,audit_id,event_type,receipt_id,created_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, taskID, sequence, parent, task["current_step_id"], task["status"], task["revision"],
		completedJSON, pendingJSON, blockedJSON, findingsJSON, decisionsJSON, nullableText(nextAction), artifactsJSON, nullableText(auditID), nullableText(eventType), nullableText(receiptID), now())
	if err != nil {
		return nil, err
	}
	checkpoint, err := one(db, "checkpoints", "SELECT * FROM checkpoints WHERE checkpoint_id = ?", id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "checkpoint": checkpoint}, nil
}

func (r *Runtime) CheckpointGet(checkpointID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	checkpoint, err := one(db, "checkpoints", "SELECT * FROM checkpoints WHERE checkpoint_id = ?", checkpointID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "checkpoint": checkpoint}, nil
}

func (r *Runtime) CheckpointList(taskID string, limit, offset int) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := queryRows(db, "checkpoints", "SELECT * FROM checkpoints WHERE task_id = ? ORDER BY sequence DESC LIMIT ? OFFSET ?", taskID, clamp(limit, 1, 200), maxInt(offset, 0))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "checkpoints": rows, "count": len(rows)}, nil
}

func nullableText(v string) any {
	if v == "" {
		return nil
	}
	return v
}
