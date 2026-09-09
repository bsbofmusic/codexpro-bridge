package work

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/codexpro/bridge/core"
)

const MaxStoredResultBytes = 1_000_000

func (r *Runtime) ResultStore(taskID, receiptID string, value any) (map[string]any, error) {
	redacted := core.RedactValue(value)
	content, err := canonicalJSON(redacted)
	if err != nil {
		return nil, core.Err("invalid_work_input", "Result is not JSON serializable")
	}
	if len([]byte(content)) > MaxStoredResultBytes {
		return nil, core.Err("result_too_large", "Result exceeds the bounded persistent result store limit")
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if taskID != "" {
		if _, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID); err != nil {
			return nil, err
		}
	}
	if receiptID != "" {
		receipt, err := one(db, "operation_receipts", "SELECT * FROM operation_receipts WHERE receipt_id = ?", receiptID)
		if err != nil {
			return nil, err
		}
		if taskID == "" {
			taskID = valueString(receipt["task_id"])
		}
	}
	sum := sha256.Sum256([]byte(content))
	resultID := newID("res")
	_, err = db.Exec(`INSERT INTO result_blobs (result_id,task_id,receipt_id,content_json,sha256,size,created_at)
VALUES (?,?,?,?,?,?,?)`, resultID, sqlNullableString(taskID), sqlNullableString(receiptID), content, hex.EncodeToString(sum[:]), len([]byte(content)), now())
	if err != nil {
		return nil, err
	}
	row, err := one(db, "result_blobs", "SELECT result_id,task_id,receipt_id,sha256,size,created_at FROM result_blobs WHERE result_id = ?", resultID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "result": row}, nil
}

func (r *Runtime) ResultGet(resultID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	row, err := one(db, "result_blobs", "SELECT * FROM result_blobs WHERE result_id = ?", resultID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "result": row}, nil
}

func (r *Runtime) ResultDelete(resultID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	res, err := db.Exec("DELETE FROM result_blobs WHERE result_id = ?", resultID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	return map[string]any{"ok": true, "result_id": resultID, "removed": n == 1}, nil
}

func decodeStoredResult(row map[string]any) any {
	if row == nil {
		return nil
	}
	value := row["content_json"]
	if value == nil {
		return nil
	}
	if _, ok := value.(map[string]any); ok {
		return value
	}
	if _, ok := value.([]any); ok {
		return value
	}
	if s, ok := value.(string); ok {
		var out any
		if json.Unmarshal([]byte(s), &out) == nil {
			return out
		}
	}
	return value
}
