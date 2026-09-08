"""Lazy SQLite storage and schema migration for Bridge 2.1 Work Runtime."""

from __future__ import annotations

import sqlite3
from typing import Any

from codexpro_bridge.core.errors import BridgeError
from .common import SCHEMA_VERSION, decode_row, now


class WorkStore:
    def __init__(self, db_path):
        self.db_path = db_path

    def connect(self) -> sqlite3.Connection:
        try:
            self.db_path.parent.mkdir(parents=True, exist_ok=True)
            conn = sqlite3.connect(self.db_path, timeout=5.0)
            conn.row_factory = sqlite3.Row
            conn.execute("PRAGMA foreign_keys = ON")
            conn.execute("PRAGMA busy_timeout = 5000")
            conn.execute("PRAGMA journal_mode = DELETE")
            if int(conn.execute("PRAGMA foreign_keys").fetchone()[0]) != 1:
                conn.close()
                raise BridgeError("work_db_invalid", "SQLite foreign key enforcement is unavailable")
            self.migrate(conn)
            return conn
        except BridgeError:
            raise
        except Exception as exc:
            raise BridgeError("work_db_unavailable", "Work Runtime database is unavailable") from exc

    def migrate(self, conn: sqlite3.Connection) -> None:
        version = int(conn.execute("PRAGMA user_version").fetchone()[0])
        if version > SCHEMA_VERSION:
            raise BridgeError("work_schema_newer", "Work Runtime database schema is newer than this Bridge")
        if version == SCHEMA_VERSION:
            return
        if version != 0:
            raise BridgeError("work_schema_unknown", "Unsupported Work Runtime database schema version")
        conn.executescript(
            """
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
            CREATE INDEX idx_tasks_project_status ON tasks(project_id, status);
            CREATE INDEX idx_steps_task_position ON task_steps(task_id, position);
            CREATE INDEX idx_checkpoints_task_sequence ON checkpoints(task_id, sequence DESC);
            CREATE INDEX idx_audits_task_created ON audits(task_id, created_at DESC);
            CREATE INDEX idx_artifacts_task_status ON artifacts(task_id, status);
            """
        )
        conn.execute(f"PRAGMA user_version = {SCHEMA_VERSION}")
        conn.commit()

    def one(self, conn: sqlite3.Connection, table: str, query: str, params: tuple[Any, ...]) -> dict[str, Any]:
        item = decode_row(table, conn.execute(query, params).fetchone())
        if item is None:
            label = table[:-1].replace("_", " ").capitalize()
            raise BridgeError("work_not_found", f"{label} not found")
        return item

    def touch_task_revision(self, conn: sqlite3.Connection, task_id: str) -> None:
        cur = conn.execute(
            "UPDATE tasks SET revision = revision + 1, updated_at = ? WHERE task_id = ?",
            (now(), task_id),
        )
        if cur.rowcount != 1:
            raise BridgeError("work_not_found", "Task not found")

    def health(self) -> dict[str, Any]:
        result: dict[str, Any] = {
            "ok": False,
            "degraded": True,
            "enabled": True,
            "db_path": str(self.db_path),
            "schema_version": None,
            "task_count": None,
            "open_task_count": None,
            "latest_checkpoint": None,
        }
        try:
            with self.connect() as conn:
                result["schema_version"] = int(conn.execute("PRAGMA user_version").fetchone()[0])
                result["task_count"] = int(conn.execute("SELECT COUNT(*) FROM tasks").fetchone()[0])
                result["open_task_count"] = int(conn.execute(
                    "SELECT COUNT(*) FROM tasks WHERE status IN ('pending','running','blocked','audit')"
                ).fetchone()[0])
                latest = conn.execute(
                    "SELECT checkpoint_id, task_id, sequence, created_at FROM checkpoints ORDER BY created_at DESC, rowid DESC LIMIT 1"
                ).fetchone()
                result["latest_checkpoint"] = dict(latest) if latest else None
            result["ok"] = True
            result["degraded"] = False
            return result
        except BridgeError as exc:
            result["error"] = {"code": exc.code, "message": exc.message}
            return result
        except Exception:
            result["error"] = {"code": "work_health_unavailable", "message": "Work Runtime health check failed"}
            return result
