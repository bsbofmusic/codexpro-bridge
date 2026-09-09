package work

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/codexpro/bridge/core"
)

func (r *Runtime) auditBundle(auditID string, dbRows bool) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return auditBundleDB(db, auditID)
}

func auditBundleDB(db DBTX, auditID string) (map[string]any, error) {
	audit, err := oneDBTX(db, "audits", "SELECT * FROM audits WHERE audit_id = ?", auditID)
	if err != nil {
		return nil, err
	}
	checks, err := queryRowsDBTX(db, "audit_checks", "SELECT * FROM audit_checks WHERE audit_id = ? ORDER BY rowid", auditID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "audit": audit, "checks": checks}, nil
}

func (r *Runtime) AuditCreate(taskID string, checks []map[string]any) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	task, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", taskID)
	if err != nil {
		return nil, err
	}
	auditID := newID("aud")
	stamp := now()
	if _, err := db.Exec("INSERT INTO audits (audit_id,task_id,status,audited_task_revision,created_at,started_at,completed_at) VALUES (?,?,?,?,?,?,?)", auditID, taskID, "UNCERTAIN", task["revision"], stamp, stamp, nil); err != nil {
		return nil, err
	}
	for _, item := range checks {
		name := strings.TrimSpace(valueString(item["name"]))
		if name == "" {
			return nil, core.Err("invalid_work_input", "Audit check name is required")
		}
		expected, err := boundedJSON(item["expected"])
		if err != nil {
			return nil, err
		}
		manualAllowed := toBool(item["manual_allowed"])
		if _, err := db.Exec(`INSERT INTO audit_checks
(check_id,audit_id,name,expected,actual,status,evidence_type,evidence_ref,command_or_tool,exit_code,timestamp,manual_allowed)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, newID("ack"), auditID, name, expected, nil, "UNCERTAIN", nil, nil, nil, nil, nil, boolInt(manualAllowed)); err != nil {
			return nil, err
		}
	}
	return auditBundleDB(db, auditID)
}

func (r *Runtime) AuditRecord(auditID, checkID string, actual any, evidenceType, evidenceRef, commandOrTool string, exitCode *int) (map[string]any, error) {
	if !evidenceTypes[evidenceType] {
		return nil, core.Err("invalid_work_input", "Unknown audit evidence type")
	}
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err := one(db, "audit_checks", "SELECT * FROM audit_checks WHERE check_id = ? AND audit_id = ?", checkID, auditID); err != nil {
		return nil, err
	}
	encoded, err := boundedJSON(actual)
	if err != nil {
		return nil, err
	}
	var code any
	if exitCode != nil {
		code = *exitCode
	}
	_, err = db.Exec(`UPDATE audit_checks SET actual = ?, evidence_type = ?, evidence_ref = ?, command_or_tool = ?, exit_code = ?, timestamp = ?, status = 'UNCERTAIN'
WHERE check_id = ? AND audit_id = ?`, encoded, evidenceType, nullableText(evidenceRef), nullableText(commandOrTool), code, now(), checkID, auditID)
	if err != nil {
		return nil, err
	}
	check, err := one(db, "audit_checks", "SELECT * FROM audit_checks WHERE check_id = ?", checkID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "check": check}, nil
}

func (r *Runtime) AuditEvaluate(auditID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	audit, err := one(db, "audits", "SELECT * FROM audits WHERE audit_id = ?", auditID)
	if err != nil {
		return nil, err
	}
	task, err := one(db, "tasks", "SELECT * FROM tasks WHERE task_id = ?", audit["task_id"])
	if err != nil {
		return nil, err
	}
	checks, err := queryRows(db, "audit_checks", "SELECT * FROM audit_checks WHERE audit_id = ? ORDER BY rowid", auditID)
	if err != nil {
		return nil, err
	}
	statuses := make([]string, 0, len(checks))
	for _, check := range checks {
		status := "UNCERTAIN"
		if check["actual"] != nil {
			if valueString(check["evidence_type"]) == "manual" && !toBool(check["manual_allowed"]) {
				status = "UNCERTAIN"
			} else {
				a, _ := canonicalJSON(check["actual"])
				e, _ := canonicalJSON(check["expected"])
				if a == e {
					status = "PASS"
				} else {
					status = "FAIL"
				}
			}
		}
		if _, err := db.Exec("UPDATE audit_checks SET status = ? WHERE check_id = ?", status, check["check_id"]); err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	stale := toInt(audit["audited_task_revision"]) != toInt(task["revision"])
	overall := "UNCERTAIN"
	if !stale && len(statuses) > 0 {
		overall = "PASS"
		for _, status := range statuses {
			if status == "FAIL" {
				overall = "FAIL"
				break
			}
			if status == "UNCERTAIN" {
				overall = "UNCERTAIN"
			}
		}
	}
	var completed any
	if overall == "PASS" || overall == "FAIL" {
		completed = now()
	}
	if _, err := db.Exec("UPDATE audits SET status = ?, completed_at = ? WHERE audit_id = ?", overall, completed, auditID); err != nil {
		return nil, err
	}
	if overall == "PASS" || overall == "FAIL" {
		if _, err := checkpointCreateDB(db, valueString(audit["task_id"]), nil, []any{map[string]any{"event": "audit.complete", "audit_id": auditID, "status": overall}}, "", nil, auditID, "audit.complete", ""); err != nil {
			return nil, err
		}
	}
	result, err := auditBundleDB(db, auditID)
	if err != nil {
		return nil, err
	}
	result["stale"] = stale
	return result, nil
}

func (r *Runtime) AuditGet(auditID string) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	return auditBundleDB(db, auditID)
}

func (r *Runtime) AuditList(taskID string, limit, offset int) (map[string]any, error) {
	db, err := r.Store.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := queryRows(db, "audits", "SELECT * FROM audits WHERE task_id = ? ORDER BY created_at DESC, rowid DESC LIMIT ? OFFSET ?", taskID, clamp(limit, 1, 200), maxInt(offset, 0))
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "audits": rows, "count": len(rows)}, nil
}

// DBTX is the tiny common surface used by *sql.DB in audit helpers.
type DBTX interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func queryRowsDBTX(db DBTX, table, query string, args ...any) ([]map[string]any, error) {
	rs, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, col := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			if jsonColumns[table][col] && v != nil {
				v = decodeJSONText(v)
			}
			if col == "audit_required" || col == "manual_allowed" {
				v = toBool(v)
			}
			row[col] = v
		}
		out = append(out, row)
	}
	return out, rs.Err()
}

func oneDBTX(db DBTX, table, query string, args ...any) (map[string]any, error) {
	rows, err := queryRowsDBTX(db, table, query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, core.Err("work_not_found", fmt.Sprintf("%s not found", table))
	}
	return rows[0], nil
}
