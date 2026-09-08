"""Immutable checkpoint operations for Bridge 2.1 Work Runtime."""

from __future__ import annotations

from typing import Any

from .common import bounded_json, decode_row, new_id, now


class CheckpointOps:
    store: Any

    def checkpoint_create(
        self,
        task_id: str,
        important_findings: list[Any] | None = None,
        decisions: list[Any] | None = None,
        next_action: str | None = None,
        artifact_ids: list[str] | None = None,
        audit_id: str | None = None,
    ) -> dict[str, Any]:
        with self.store.connect() as conn:
            task = self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))
            step_rows = conn.execute("SELECT step_id,status FROM task_steps WHERE task_id = ? ORDER BY position", (task_id,)).fetchall()
            completed = [str(item["step_id"]) for item in step_rows if item["status"] in {"done", "skipped"}]
            pending = [str(item["step_id"]) for item in step_rows if item["status"] in {"todo", "doing"}]
            blocked = [str(item["step_id"]) for item in step_rows if item["status"] == "blocked"]
            latest = conn.execute(
                "SELECT checkpoint_id,sequence FROM checkpoints WHERE task_id = ? ORDER BY sequence DESC LIMIT 1", (task_id,)
            ).fetchone()
            sequence = int(latest["sequence"]) + 1 if latest else 1
            parent = str(latest["checkpoint_id"]) if latest else None
            checkpoint_id = new_id("chk")
            conn.execute(
                """INSERT INTO checkpoints
                (checkpoint_id,task_id,sequence,parent_checkpoint_id,current_step_id,task_status,task_revision,
                 completed_step_ids,pending_step_ids,blocked_step_ids,important_findings,decisions,next_action,artifact_ids,audit_id,created_at)
                VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (
                    checkpoint_id, task_id, sequence, parent, task["current_step_id"], task["status"], task["revision"],
                    bounded_json(completed), bounded_json(pending), bounded_json(blocked),
                    bounded_json(important_findings or []), bounded_json(decisions or []), next_action,
                    bounded_json(artifact_ids or []), audit_id, now(),
                ),
            )
            return {"ok": True, "checkpoint": self.store.one(conn, "checkpoints", "SELECT * FROM checkpoints WHERE checkpoint_id = ?", (checkpoint_id,))}

    def checkpoint_get(self, checkpoint_id: str) -> dict[str, Any]:
        with self.store.connect() as conn:
            return {"ok": True, "checkpoint": self.store.one(conn, "checkpoints", "SELECT * FROM checkpoints WHERE checkpoint_id = ?", (checkpoint_id,))}

    def checkpoint_list(self, task_id: str, limit: int = 50, offset: int = 0) -> dict[str, Any]:
        with self.store.connect() as conn:
            rows = conn.execute(
                "SELECT * FROM checkpoints WHERE task_id = ? ORDER BY sequence DESC LIMIT ? OFFSET ?",
                (task_id, max(1, min(int(limit), 200)), max(0, int(offset))),
            ).fetchall()
            return {"ok": True, "checkpoints": [decode_row("checkpoints", item) for item in rows], "count": len(rows)}
