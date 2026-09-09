package work

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/codexpro/bridge/core"
)

func ConversationFingerprint(firstMessage string) (string, error) {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(firstMessage)), " "))
	if normalized == "" || len([]rune(normalized)) > 8_000 {
		return "", core.Err("invalid_work_input", "Conversation first message is invalid")
	}
	sum := sha256.Sum256([]byte(normalized))
	return "conv_" + hex.EncodeToString(sum[:]), nil
}

func (r *Runtime) AffinityBind(firstMessage, taskID string) (map[string]any, error) {
	fingerprint, err := ConversationFingerprint(firstMessage)
	if err != nil {
		return nil, err
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID); err != nil {
		return nil, err
	}
	stamp := now()
	_, err = db.Exec(`INSERT INTO conversation_affinity (conversation_fingerprint,task_id,created_at,updated_at)
VALUES (?,?,?,?)
ON CONFLICT(conversation_fingerprint) DO UPDATE SET task_id = excluded.task_id, updated_at = excluded.updated_at`, fingerprint, taskID, stamp, stamp)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "conversation_fingerprint": fingerprint, "task_id": taskID, "raw_conversation_stored": false}, nil
}

func (r *Runtime) AffinityResolve(firstMessage string) (map[string]any, error) {
	fingerprint, err := ConversationFingerprint(firstMessage)
	if err != nil {
		return nil, err
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := queryRows(db, "conversation_affinity", `SELECT a.conversation_fingerprint,a.task_id,a.created_at,a.updated_at,t.title,t.status
FROM conversation_affinity a JOIN tasks t ON t.task_id = a.task_id
WHERE a.conversation_fingerprint = ? LIMIT 1`, fingerprint)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return map[string]any{"ok": false, "matched": false, "conversation_fingerprint": fingerprint}, nil
	}
	return map[string]any{"ok": true, "matched": true, "affinity": rows[0]}, nil
}

func (r *Runtime) AffinityUnbind(firstMessage string) (map[string]any, error) {
	fingerprint, err := ConversationFingerprint(firstMessage)
	if err != nil {
		return nil, err
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	res, err := db.Exec("DELETE FROM conversation_affinity WHERE conversation_fingerprint = ?", fingerprint)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	return map[string]any{"ok": true, "conversation_fingerprint": fingerprint, "removed": n == 1}, nil
}
