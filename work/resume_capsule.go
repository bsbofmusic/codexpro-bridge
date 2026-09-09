package work

import "fmt"

func (r *Runtime) ResumeCapsule(taskID string) (map[string]any, error) {
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
	checkpoints, err := queryRows(db, "checkpoints", "SELECT * FROM checkpoints WHERE task_id = ? ORDER BY sequence DESC LIMIT 1", taskID)
	if err != nil {
		return nil, err
	}
	audits, err := queryRows(db, "audits", "SELECT * FROM audits WHERE task_id = ? ORDER BY created_at DESC, rowid DESC LIMIT 1", taskID)
	if err != nil {
		return nil, err
	}
	artifacts, err := queryRows(db, "artifacts", "SELECT artifact_id,name,type,status,verification_status,updated_at FROM artifacts WHERE task_id = ? ORDER BY updated_at DESC LIMIT 20", taskID)
	if err != nil {
		return nil, err
	}
	receipts, err := queryRows(db, "operation_receipts", "SELECT * FROM operation_receipts WHERE task_id = ? ORDER BY started_at DESC, rowid DESC LIMIT 20", taskID)
	if err != nil {
		return nil, err
	}

	counts := map[string]int{}
	pending := []map[string]any{}
	var current map[string]any
	currentID := valueString(task["current_step_id"])
	for _, step := range steps {
		state := fmt.Sprint(step["status"])
		counts[state]++
		if currentID != "" && fmt.Sprint(step["step_id"]) == currentID {
			current = stepBrief(step)
		}
		if state != "done" && state != "skipped" {
			pending = append(pending, stepBrief(step))
			if current == nil && (state == "doing" || state == "blocked" || state == "todo") {
				current = stepBrief(step)
			}
		}
	}
	if len(pending) > 12 {
		pending = pending[:12]
	}

	var latestCheckpoint, latestAudit any
	var nextAction any
	if len(checkpoints) > 0 {
		latestCheckpoint = checkpoints[0]
		nextAction = checkpoints[0]["next_action"]
	}
	if len(audits) > 0 {
		latestAudit = audits[0]
	}
	if nextAction == nil && current != nil {
		nextAction = "Continue step: " + fmt.Sprint(current["title"])
	}

	var lastSuccess, lastFailure any
	uncertain := []map[string]any{}
	doNotReplay := []map[string]any{}
	for _, receipt := range receipts {
		policy := replayPolicy(receipt)
		receipt["replay_policy"] = policy
		delivery := fmt.Sprint(receipt["delivery_state"])
		result := fmt.Sprint(receipt["result_state"])
		if lastSuccess == nil && delivery == "confirmed" && result == "returned" {
			lastSuccess = receiptBrief(receipt)
		}
		if lastFailure == nil && result == "failed" {
			lastFailure = receiptBrief(receipt)
		}
		if delivery == "unknown" || delivery == "prepared" {
			uncertain = append(uncertain, receiptBrief(receipt))
		}
		if toBool(receipt["mutation"]) && policy != "SAFE_TO_REPEAT_READ_ONLY_IF_NEEDED" {
			doNotReplay = append(doNotReplay, receiptBrief(receipt))
		}
	}
	if len(uncertain) > 8 {
		uncertain = uncertain[:8]
	}
	if len(doNotReplay) > 12 {
		doNotReplay = doNotReplay[:12]
	}

	return map[string]any{
		"ok":              true,
		"capsule_version": 1,
		"task": map[string]any{
			"task_id": task["task_id"], "title": task["title"], "description": task["description"],
			"status": task["status"], "revision": task["revision"], "updated_at": task["updated_at"],
		},
		"progress":                  map[string]any{"total_steps": len(steps), "by_status": counts, "pending_count": len(pending)},
		"current":                   current,
		"pending":                   pending,
		"last_successful_operation": lastSuccess,
		"last_failure":              lastFailure,
		"uncertain_delivery":        uncertain,
		"do_not_replay":             doNotReplay,
		"latest_checkpoint":         latestCheckpoint,
		"audit":                     latestAudit,
		"artifacts":                 artifacts,
		"artifact_count":            len(artifacts),
		"next_action":               nextAction,
		"resume_from": func() any {
			if current != nil {
				return current["step_id"]
			}
			return nil
		}(),
		"mutation_replayed": false,
		"resume_semantics":  "state_only",
	}, nil
}

func stepBrief(step map[string]any) map[string]any {
	return map[string]any{
		"step_id": step["step_id"], "position": step["position"], "title": step["title"],
		"status": step["status"], "blocker": step["blocker"], "updated_at": step["updated_at"],
	}
}

func receiptBrief(receipt map[string]any) map[string]any {
	return map[string]any{
		"receipt_id": receipt["receipt_id"], "operation_id": receipt["operation_id"], "tool_name": receipt["tool_name"],
		"step_id": receipt["step_id"], "mutation": receipt["mutation"], "delivery_state": receipt["delivery_state"],
		"result_state": receipt["result_state"], "result_ref": receipt["result_ref"], "started_at": receipt["started_at"],
		"finished_at": receipt["finished_at"], "replay_policy": replayPolicy(receipt),
	}
}
