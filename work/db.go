package work

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/codexpro/bridge/core"
	_ "modernc.org/sqlite"
)

type Store struct {
	Path   string
	initMu *sync.Mutex
}

func (s Store) Connect() (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return nil, core.Err("work_db_unavailable", "Work Runtime database is unavailable")
	}
	db, err := sql.Open("sqlite", s.Path)
	if err != nil {
		return nil, core.Err("work_db_unavailable", "Work Runtime database is unavailable")
	}
	if s.initMu != nil {
		s.initMu.Lock()
		defer s.initMu.Unlock()
	}
	// SQLite PRAGMAs are connection-scoped. Work Runtime is deliberately small,
	// so one connection keeps the contract deterministic and avoids hidden pool state.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	for _, q := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = DELETE",
	} {
		if _, err := db.Exec(q); err != nil {
			db.Close()
			return nil, core.Err("work_db_unavailable", "Work Runtime database is unavailable")
		}
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		db.Close()
		return nil, core.Err("work_db_invalid", "SQLite foreign key enforcement is unavailable")
	}
	if err := s.migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (s Store) migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return core.Err("work_db_unavailable", "Work Runtime database is unavailable")
	}
	if version > SchemaVersion {
		return core.Err("work_schema_newer", "Work Runtime database schema is newer than this Bridge")
	}
	if version == SchemaVersion {
		return nil
	}
	if version == 1 {
		return s.migrateV1ToV2(db)
	}
	if version != 0 {
		return core.Err("work_schema_unknown", "Unsupported Work Runtime database schema version")
	}

	schema := `
CREATE TABLE projects (
    project_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    workspace_ref TEXT,
    relevant_skills TEXT NOT NULL DEFAULT '[]',
    relevant_mcp TEXT NOT NULL DEFAULT '[]',
    memory_queries TEXT NOT NULL DEFAULT '[]',
    project_rules TEXT NOT NULL DEFAULT '[]',
    known_risks TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE tasks (
    task_id TEXT PRIMARY KEY,
    project_id TEXT REFERENCES projects(project_id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    priority TEXT NOT NULL DEFAULT 'normal',
    audit_required INTEGER NOT NULL DEFAULT 0 CHECK(audit_required IN (0,1)),
    current_step_id TEXT,
    revision INTEGER NOT NULL DEFAULT 1 CHECK(revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    completed_at TEXT,
    metadata TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE task_steps (
    step_id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK(position >= 1),
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    blocker TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    completed_at TEXT,
    UNIQUE(task_id, position)
);
CREATE TABLE checkpoints (
    checkpoint_id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK(sequence >= 1),
    parent_checkpoint_id TEXT REFERENCES checkpoints(checkpoint_id),
    current_step_id TEXT,
    task_status TEXT NOT NULL,
    task_revision INTEGER NOT NULL CHECK(task_revision >= 1),
    completed_step_ids TEXT NOT NULL DEFAULT '[]',
    pending_step_ids TEXT NOT NULL DEFAULT '[]',
    blocked_step_ids TEXT NOT NULL DEFAULT '[]',
    important_findings TEXT NOT NULL DEFAULT '[]',
    decisions TEXT NOT NULL DEFAULT '[]',
    next_action TEXT,
    artifact_ids TEXT NOT NULL DEFAULT '[]',
    audit_id TEXT,
    event_type TEXT,
    receipt_id TEXT,
    created_at TEXT NOT NULL,
    UNIQUE(task_id, sequence)
);
CREATE TABLE audits (
    audit_id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    audited_task_revision INTEGER NOT NULL CHECK(audited_task_revision >= 1),
    created_at TEXT NOT NULL,
    started_at TEXT NOT NULL,
    completed_at TEXT
);
CREATE TABLE audit_checks (
    check_id TEXT PRIMARY KEY,
    audit_id TEXT NOT NULL REFERENCES audits(audit_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    expected TEXT,
    actual TEXT,
    status TEXT NOT NULL,
    evidence_type TEXT,
    evidence_ref TEXT,
    command_or_tool TEXT,
    exit_code INTEGER,
    timestamp TEXT,
    manual_allowed INTEGER NOT NULL DEFAULT 0 CHECK(manual_allowed IN (0,1))
);
CREATE TABLE artifacts (
    artifact_id TEXT PRIMARY KEY,
    task_id TEXT REFERENCES tasks(task_id) ON DELETE SET NULL,
    project_id TEXT REFERENCES projects(project_id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'other',
    path_or_ref TEXT NOT NULL,
    status TEXT NOT NULL,
    sha256 TEXT,
    size INTEGER,
    verification_status TEXT NOT NULL,
    verification_evidence_ref TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE operation_receipts (
    receipt_id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL UNIQUE,
    task_id TEXT REFERENCES tasks(task_id) ON DELETE SET NULL,
    step_id TEXT REFERENCES task_steps(step_id) ON DELETE SET NULL,
    tool_name TEXT NOT NULL,
    arguments_hash TEXT NOT NULL,
    mutation INTEGER NOT NULL DEFAULT 0 CHECK(mutation IN (0,1)),
    delivery_state TEXT NOT NULL,
    result_state TEXT NOT NULL,
    result_ref TEXT,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    metadata TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE conversation_affinity (
    conversation_fingerprint TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE TABLE result_blobs (
    result_id TEXT PRIMARY KEY,
    task_id TEXT REFERENCES tasks(task_id) ON DELETE SET NULL,
    receipt_id TEXT REFERENCES operation_receipts(receipt_id) ON DELETE SET NULL,
    content_json TEXT NOT NULL,
    sha256 TEXT NOT NULL,
    size INTEGER NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX idx_steps_task_position ON task_steps(task_id, position);
CREATE INDEX idx_checkpoints_task_sequence ON checkpoints(task_id, sequence DESC);
CREATE INDEX idx_audits_task_created ON audits(task_id, created_at DESC);
CREATE INDEX idx_artifacts_task_status ON artifacts(task_id, status);
CREATE INDEX idx_receipts_task_started ON operation_receipts(task_id, started_at DESC);
CREATE INDEX idx_receipts_step_started ON operation_receipts(step_id, started_at DESC);
CREATE INDEX idx_results_task_created ON result_blobs(task_id, created_at DESC);
PRAGMA user_version = 2;
`
	if _, err := db.Exec(schema); err != nil {
		return core.Err("work_db_unavailable", "Work Runtime database is unavailable")
	}
	return nil
}

func (s Store) migrateV1ToV2(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return core.Err("work_db_unavailable", "Work Runtime database migration failed")
	}
	defer tx.Rollback()
	statements := []string{
		"ALTER TABLE checkpoints ADD COLUMN event_type TEXT",
		"ALTER TABLE checkpoints ADD COLUMN receipt_id TEXT",
		`CREATE TABLE operation_receipts (
            receipt_id TEXT PRIMARY KEY,
            operation_id TEXT NOT NULL UNIQUE,
            task_id TEXT REFERENCES tasks(task_id) ON DELETE SET NULL,
            step_id TEXT REFERENCES task_steps(step_id) ON DELETE SET NULL,
            tool_name TEXT NOT NULL,
            arguments_hash TEXT NOT NULL,
            mutation INTEGER NOT NULL DEFAULT 0 CHECK(mutation IN (0,1)),
            delivery_state TEXT NOT NULL,
            result_state TEXT NOT NULL,
            result_ref TEXT,
            started_at TEXT NOT NULL,
            finished_at TEXT,
            metadata TEXT NOT NULL DEFAULT '{}'
        )`,
		`CREATE TABLE conversation_affinity (
            conversation_fingerprint TEXT PRIMARY KEY,
            task_id TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`CREATE TABLE result_blobs (
            result_id TEXT PRIMARY KEY,
            task_id TEXT REFERENCES tasks(task_id) ON DELETE SET NULL,
            receipt_id TEXT REFERENCES operation_receipts(receipt_id) ON DELETE SET NULL,
            content_json TEXT NOT NULL,
            sha256 TEXT NOT NULL,
            size INTEGER NOT NULL,
            created_at TEXT NOT NULL
        )`,
		"CREATE INDEX idx_receipts_task_started ON operation_receipts(task_id, started_at DESC)",
		"CREATE INDEX idx_receipts_step_started ON operation_receipts(step_id, started_at DESC)",
		"CREATE INDEX idx_results_task_created ON result_blobs(task_id, created_at DESC)",
		"PRAGMA user_version = 2",
	}
	for _, statement := range statements {
		if _, err := tx.Exec(statement); err != nil {
			return core.Err("work_db_unavailable", "Work Runtime database migration failed")
		}
	}
	if err := tx.Commit(); err != nil {
		return core.Err("work_db_unavailable", "Work Runtime database migration failed")
	}
	return nil
}

func queryRows(db *sql.DB, table, query string, args ...any) ([]map[string]any, error) {
	rs, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	for rs.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			if jsonColumns[table][col] && v != nil {
				v = decodeJSONText(v)
			}
			if col == "audit_required" || col == "manual_allowed" || col == "mutation" {
				v = toBool(v)
			}
			row[col] = v
		}
		out = append(out, row)
	}
	return out, rs.Err()
}

func one(db *sql.DB, table, query string, args ...any) (map[string]any, error) {
	rows, err := queryRows(db, table, query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		labels := map[string]string{
			"projects": "Project", "tasks": "Task", "task_steps": "Task step",
			"checkpoints": "Checkpoint", "audits": "Audit", "audit_checks": "Audit check", "artifacts": "Artifact",
		}
		label := labels[table]
		if label == "" {
			label = strings.TrimSuffix(strings.ReplaceAll(table, "_", " "), "s")
			label = strings.ToUpper(label[:1]) + label[1:]
		}
		return nil, core.Err("work_not_found", label+" not found")
	}
	return rows[0], nil
}

func touchTaskRevision(db *sql.DB, taskID string) error {
	res, err := db.Exec("UPDATE tasks SET revision = revision + 1, updated_at = ? WHERE task_id = ?", now(), taskID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return core.Err("work_not_found", "Task not found")
	}
	return nil
}

func (s Store) Health() map[string]any {
	result := map[string]any{
		"ok": false, "degraded": true, "enabled": true, "db_path": s.Path,
		"schema_version": nil, "task_count": nil, "open_task_count": nil, "latest_checkpoint": nil,
	}
	db, err := s.Connect()
	if err != nil {
		if be, ok := err.(core.BridgeError); ok {
			result["error"] = map[string]any{"code": be.Code, "message": be.Message}
		} else {
			result["error"] = map[string]any{"code": "work_health_unavailable", "message": "Work Runtime health check failed"}
		}
		return result
	}
	defer db.Close()
	var version, tasks, open int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		result["error"] = map[string]any{"code": "work_health_unavailable", "message": "Work Runtime health check failed"}
		return result
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&tasks); err != nil {
		result["error"] = map[string]any{"code": "work_health_unavailable", "message": "Work Runtime health check failed"}
		return result
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status IN ('pending','running','blocked','audit')").Scan(&open); err != nil {
		result["error"] = map[string]any{"code": "work_health_unavailable", "message": "Work Runtime health check failed"}
		return result
	}
	latest, err := queryRows(db, "checkpoints", "SELECT checkpoint_id, task_id, sequence, created_at FROM checkpoints ORDER BY created_at DESC, rowid DESC LIMIT 1")
	if err != nil {
		result["error"] = map[string]any{"code": "work_health_unavailable", "message": "Work Runtime health check failed"}
		return result
	}
	result["schema_version"] = version
	result["task_count"] = tasks
	result["open_task_count"] = open
	if len(latest) > 0 {
		result["latest_checkpoint"] = latest[0]
	}
	result["ok"] = true
	result["degraded"] = false
	return result
}

func sqliteConstraint(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "constraint") || strings.Contains(s, "unique")
}

func sqlNullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func debugSQLValue(v any) string { return fmt.Sprint(v) }
