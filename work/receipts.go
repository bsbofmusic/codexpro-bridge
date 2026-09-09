package work

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/codexpro/bridge/core"
)

var receiptDeliveryStates = setOf("prepared", "confirmed", "unknown")
var receiptResultStates = setOf("pending", "returned", "failed", "unknown")

func ArgumentsHash(arguments any) (string, error) {
	encoded, err := canonicalJSON(arguments)
	if err != nil {
		return "", core.Err("invalid_work_input", "Operation arguments are not JSON serializable")
	}
	sum := sha256.Sum256([]byte(encoded))
	return hex.EncodeToString(sum[:]), nil
}

func (r *Runtime) ReceiptPrepare(operationID, taskID, stepID, toolName string, arguments any, mutation bool, metadata map[string]any) (map[string]any, error) {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" && mutation {
		return nil, core.Err("operation_id_required", "Mutation receipts require an explicit operation_id for no-replay safety")
	}
	if operationID == "" {
		operationID = newID("op")
	}
	if len(operationID) > 160 || strings.TrimSpace(toolName) == "" || len(toolName) > 256 {
		return nil, core.Err("invalid_work_input", "Operation receipt identity is invalid")
	}
	hash, err := ArgumentsHash(arguments)
	if err != nil {
		return nil, err
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

	existing, err := queryRows(db, "operation_receipts", "SELECT * FROM operation_receipts WHERE operation_id = ? LIMIT 1", operationID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		receipt := existing[0]
		if fmt.Sprint(receipt["arguments_hash"]) != hash || fmt.Sprint(receipt["tool_name"]) != toolName {
			return nil, core.Err("idempotency_conflict", "Operation ID is already bound to different arguments or tool")
		}
		return map[string]any{
			"ok": true, "existing": true, "receipt": receipt,
			"do_not_replay": toBool(receipt["mutation"]), "replay_policy": replayPolicy(receipt),
		}, nil
	}

	if stepID != "" {
		step, err := one(db, "task_steps", "SELECT * FROM task_steps WHERE step_id = ?", stepID)
		if err != nil {
			return nil, err
		}
		stepTask := fmt.Sprint(step["task_id"])
		if taskID == "" {
			taskID = stepTask
		} else if taskID != stepTask {
			return nil, core.Err("invalid_work_input", "Receipt task_id and step_id do not belong together")
		}
	}
	if taskID != "" {
		if _, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID); err != nil {
			return nil, err
		}
	}

	receiptID := newID("rcp")
	_, err = db.Exec(`INSERT INTO operation_receipts
(receipt_id,operation_id,task_id,step_id,tool_name,arguments_hash,mutation,delivery_state,result_state,result_ref,started_at,finished_at,metadata)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, receiptID, operationID, sqlNullableString(taskID), sqlNullableString(stepID), toolName, hash, boolInt(mutation), "prepared", "pending", nil, now(), nil, meta)
	if err != nil {
		return nil, err
	}
	receipt, err := one(db, "operation_receipts", "SELECT * FROM operation_receipts WHERE receipt_id = ?", receiptID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "existing": false, "receipt": receipt, "do_not_replay": false}, nil
}

func (r *Runtime) ReceiptFinalize(receiptID, deliveryState, resultState, resultRef string) (map[string]any, error) {
	if !receiptDeliveryStates[deliveryState] || !receiptResultStates[resultState] {
		return nil, core.Err("invalid_work_input", "Unknown operation receipt state")
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	existing, err := one(db, "operation_receipts", "SELECT * FROM operation_receipts WHERE receipt_id = ?", receiptID)
	if err != nil {
		return nil, err
	}
	if existing["finished_at"] != nil {
		if fmt.Sprint(existing["delivery_state"]) == deliveryState && fmt.Sprint(existing["result_state"]) == resultState && valueString(existing["result_ref"]) == resultRef {
			return map[string]any{
				"ok": true, "existing": true, "receipt": existing,
				"do_not_replay": toBool(existing["mutation"]), "replay_policy": replayPolicy(existing),
			}, nil
		}
		return nil, core.Err("receipt_finalized", "Operation receipt is already finalized and cannot be rewritten")
	}

	if _, err := db.Exec("UPDATE operation_receipts SET delivery_state = ?, result_state = ?, result_ref = ?, finished_at = ? WHERE receipt_id = ?", deliveryState, resultState, sqlNullableString(resultRef), now(), receiptID); err != nil {
		return nil, err
	}
	receipt, err := one(db, "operation_receipts", "SELECT * FROM operation_receipts WHERE receipt_id = ?", receiptID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "existing": false, "receipt": receipt, "do_not_replay": toBool(receipt["mutation"]), "replay_policy": replayPolicy(receipt)}, nil
}

func (r *Runtime) ReceiptGet(receiptID, operationID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var receipt map[string]any
	if receiptID != "" {
		receipt, err = one(db, "operation_receipts", "SELECT * FROM operation_receipts WHERE receipt_id = ?", receiptID)
	} else if operationID != "" {
		receipt, err = one(db, "operation_receipts", "SELECT * FROM operation_receipts WHERE operation_id = ?", operationID)
	} else {
		return nil, core.Err("invalid_work_input", "receipt_id or operation_id is required")
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "receipt": receipt, "do_not_replay": toBool(receipt["mutation"]), "replay_policy": replayPolicy(receipt)}, nil
}

func (r *Runtime) ReceiptList(taskID, stepID string, limit, offset int) (map[string]any, error) {
	clauses := []string{}
	args := []any{}
	if taskID != "" {
		clauses = append(clauses, "task_id = ?")
		args = append(args, taskID)
	}
	if stepID != "" {
		clauses = append(clauses, "step_id = ?")
		args = append(args, stepID)
	}
	query := "SELECT * FROM operation_receipts"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY started_at DESC, rowid DESC LIMIT ? OFFSET ?"
	args = append(args, clamp(limit, 1, 200), maxInt(offset, 0))
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := queryRows(db, "operation_receipts", query, args...)
	if err != nil {
		return nil, err
	}
	for _, receipt := range rows {
		receipt["replay_policy"] = replayPolicy(receipt)
		receipt["do_not_replay"] = toBool(receipt["mutation"])
	}
	return map[string]any{"ok": true, "receipts": rows, "count": len(rows)}, nil
}

func replayPolicy(receipt map[string]any) string {
	if !toBool(receipt["mutation"]) {
		return "SAFE_TO_REPEAT_READ_ONLY_IF_NEEDED"
	}
	delivery := fmt.Sprint(receipt["delivery_state"])
	result := fmt.Sprint(receipt["result_state"])
	switch {
	case delivery == "confirmed" && result == "returned":
		return "DO_NOT_REPLAY"
	case delivery == "unknown" || delivery == "prepared":
		return "VERIFY_BEFORE_ANY_RETRY"
	case result == "failed":
		return "RETRY_ONLY_BY_EXPLICIT_DECISION"
	default:
		return "VERIFY_BEFORE_ANY_RETRY"
	}
}
