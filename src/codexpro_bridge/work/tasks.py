"""Task and step lifecycle operations for Bridge 2.1 Work Runtime."""

from __future__ import annotations

import sqlite3
from typing import Any

from codexpro_bridge.core.errors import BridgeError
from .common import STEP_STATES, TASK_STATES, TASK_TRANSITIONS, bounded_json, decode_row, new_id, now


class TaskOps:
    store: Any

    def task_create(
        self,
        title: str,
        description: str = "",
        project_id: str | None = None,
        priority: str = "normal",
        audit_required: bool = False,
        metadata: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        if not title.strip():
            raise BridgeError("invalid_work_input", "Task title is required")
        task_id = new_id("tsk")
        stamp = now()
        with self.store.connect() as conn:
            if project_id is not None:
                self.store.one(conn, "projects", "SELECT * FROM projects WHERE project_id = ?", (project_id,))
            conn.execute(
                """INSERT INTO tasks
                (task_id,project_id,title,description,status,priority,audit_required,current_step_id,revision,created_at,updated_at,completed_at,metadata)
                VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (task_id, project_id, title.strip(), description, "pending", priority, int(bool(audit_required)), None, 1, stamp, stamp, None, bounded_json(metadata or {})),
            )
            return {"ok": True, "task": self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))}

    def task_get(self, task_id: str) -> dict[str, Any]:
        with self.store.connect() as conn:
            task = self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))
            steps = [decode_row("task_steps", item) for item in conn.execute(
                "SELECT * FROM task_steps WHERE task_id = ? ORDER BY position", (task_id,)
            ).fetchall()]
            return {"ok": True, "task": task, "steps": steps}

    def task_list(self, project_id: str | None = None, status: str | None = None, limit: int = 50, offset: int = 0) -> dict[str, Any]:
        if status is not None and status not in TASK_STATES:
            raise BridgeError("invalid_work_input", "Unknown task status")
        clauses: list[str] = []
        params: list[Any] = []
        if project_id is not None:
            clauses.append("project_id = ?")
            params.append(project_id)
        if status is not None:
            clauses.append("status = ?")
            params.append(status)
        sql = "SELECT * FROM tasks"
        if clauses:
            sql += " WHERE " + " AND ".join(clauses)
        sql += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
        params.extend([max(1, min(int(limit), 200)), max(0, int(offset))])
        with self.store.connect() as conn:
            rows = conn.execute(sql, tuple(params)).fetchall()
            return {"ok": True, "tasks": [decode_row("tasks", item) for item in rows], "count": len(rows)}

    def task_update(self, task_id: str, **changes: Any) -> dict[str, Any]:
        allowed = {"project_id", "title", "description", "priority", "audit_required", "metadata"}
        unknown = set(changes) - allowed
        if unknown:
            raise BridgeError("invalid_work_input", "Unsupported task fields", {"fields": sorted(unknown)})
        if not changes:
            return self.task_get(task_id)
        sets: list[str] = []
        params: list[Any] = []
        for key, value in changes.items():
            if key == "metadata":
                value = bounded_json(value or {})
            elif key == "audit_required":
                value = int(bool(value))
            sets.append(f"{key} = ?")
            params.append(value)
        sets.extend(["revision = revision + 1", "updated_at = ?"])
        params.extend([now(), task_id])
        with self.store.connect() as conn:
            cur = conn.execute(f"UPDATE tasks SET {', '.join(sets)} WHERE task_id = ?", tuple(params))
            if cur.rowcount != 1:
                raise BridgeError("work_not_found", "Task not found")
            return {"ok": True, "task": self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))}

    def task_transition(self, task_id: str, status: str) -> dict[str, Any]:
        if status not in TASK_STATES:
            raise BridgeError("invalid_work_input", "Unknown task status")
        with self.store.connect() as conn:
            task = self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))
            current = str(task["status"])
            if status not in TASK_TRANSITIONS[current]:
                raise BridgeError("invalid_task_transition", f"Task cannot transition from {current} to {status}")
            if status == "done" and bool(task["audit_required"]):
                audit = decode_row("audits", conn.execute(
                    "SELECT * FROM audits WHERE task_id = ? ORDER BY created_at DESC, rowid DESC LIMIT 1", (task_id,)
                ).fetchone())
                if not audit or audit["status"] != "PASS" or int(audit["audited_task_revision"]) != int(task["revision"]):
                    raise BridgeError("audit_gate_blocked", "Task completion requires a PASS audit for the current task revision")
            stamp = now()
            conn.execute(
                "UPDATE tasks SET status = ?, completed_at = ?, updated_at = ? WHERE task_id = ?",
                (status, stamp if status == "done" else None, stamp, task_id),
            )
            return {"ok": True, "task": self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))}

    def task_step_add(self, task_id: str, title: str, description: str = "", position: int | None = None) -> dict[str, Any]:
        if not title.strip():
            raise BridgeError("invalid_work_input", "Step title is required")
        with self.store.connect() as conn:
            self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))
            if position is None:
                position = int(conn.execute("SELECT COALESCE(MAX(position),0)+1 FROM task_steps WHERE task_id = ?", (task_id,)).fetchone()[0])
            step_id = new_id("stp")
            stamp = now()
            try:
                conn.execute(
                    """INSERT INTO task_steps
                    (step_id,task_id,position,title,description,status,blocker,created_at,updated_at,completed_at)
                    VALUES (?,?,?,?,?,?,?,?,?,?)""",
                    (step_id, task_id, int(position), title.strip(), description, "todo", None, stamp, stamp, None),
                )
            except sqlite3.IntegrityError as exc:
                raise BridgeError("invalid_work_input", "Step position is already in use") from exc
            self.store.touch_task_revision(conn, task_id)
            return {"ok": True, "step": self.store.one(conn, "task_steps", "SELECT * FROM task_steps WHERE step_id = ?", (step_id,))}

    def task_step_update(self, step_id: str, **changes: Any) -> dict[str, Any]:
        allowed = {"title", "description", "status", "blocker", "position"}
        unknown = set(changes) - allowed
        if unknown:
            raise BridgeError("invalid_work_input", "Unsupported step fields", {"fields": sorted(unknown)})
        with self.store.connect() as conn:
            step = self.store.one(conn, "task_steps", "SELECT * FROM task_steps WHERE step_id = ?", (step_id,))
            if not changes:
                return {"ok": True, "step": step}
            if "status" in changes and changes["status"] not in STEP_STATES:
                raise BridgeError("invalid_work_input", "Unknown step status")
            sets: list[str] = []
            params: list[Any] = []
            for key, value in changes.items():
                sets.append(f"{key} = ?")
                params.append(value)
            target_status = str(changes.get("status", step["status"]))
            sets.extend(["updated_at = ?", "completed_at = ?"])
            params.extend([now(), now() if target_status in {"done", "failed", "skipped"} else None, step_id])
            try:
                conn.execute(f"UPDATE task_steps SET {', '.join(sets)} WHERE step_id = ?", tuple(params))
            except sqlite3.IntegrityError as exc:
                raise BridgeError("invalid_work_input", "Step update violates task ordering constraints") from exc
            task_id = str(step["task_id"])
            self.store.touch_task_revision(conn, task_id)
            if target_status == "doing":
                conn.execute("UPDATE tasks SET current_step_id = ? WHERE task_id = ?", (step_id, task_id))
            return {"ok": True, "step": self.store.one(conn, "task_steps", "SELECT * FROM task_steps WHERE step_id = ?", (step_id,))}

    def task_summary(self, task_id: str) -> dict[str, Any]:
        with self.store.connect() as conn:
            task = self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))
            project = None
            if task.get("project_id"):
                project = decode_row("projects", conn.execute("SELECT * FROM projects WHERE project_id = ?", (task["project_id"],)).fetchone())
            steps = [decode_row("task_steps", item) for item in conn.execute(
                "SELECT * FROM task_steps WHERE task_id = ? ORDER BY position", (task_id,)
            ).fetchall()]
            counts = {state: 0 for state in STEP_STATES}
            for step in steps:
                if step:
                    counts[str(step["status"])] += 1
            checkpoint = decode_row("checkpoints", conn.execute(
                "SELECT * FROM checkpoints WHERE task_id = ? ORDER BY sequence DESC LIMIT 1", (task_id,)
            ).fetchone())
            audit = decode_row("audits", conn.execute(
                "SELECT * FROM audits WHERE task_id = ? ORDER BY created_at DESC, rowid DESC LIMIT 1", (task_id,)
            ).fetchone())
            artifacts = [decode_row("artifacts", item) for item in conn.execute(
                "SELECT * FROM artifacts WHERE task_id = ? ORDER BY updated_at DESC", (task_id,)
            ).fetchall()]
            next_action = checkpoint.get("next_action") if checkpoint else None
            if not next_action:
                next_step = next((step for step in steps if step and step["status"] not in {"done", "skipped"}), None)
                if next_step:
                    next_action = f"Continue step: {next_step['title']}"
            return {
                "ok": True,
                "task": task,
                "project": project,
                "progress": {"total_steps": len(steps), "by_status": counts},
                "latest_checkpoint": checkpoint,
                "latest_audit": audit,
                "artifacts": artifacts,
                "next_action": next_action,
            }

    def task_resume(self, task_id: str | None = None, project_id: str | None = None, title: str | None = None) -> dict[str, Any]:
        with self.store.connect() as conn:
            if task_id:
                task = self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))
            else:
                clauses = ["status IN ('pending','running','blocked','audit')"]
                params: list[Any] = []
                if project_id:
                    clauses.append("project_id = ?")
                    params.append(project_id)
                if title:
                    clauses.append("LOWER(title) LIKE LOWER(?)")
                    params.append(f"%{title}%")
                rows = conn.execute(
                    "SELECT * FROM tasks WHERE " + " AND ".join(clauses) + " ORDER BY updated_at DESC LIMIT 20",
                    tuple(params),
                ).fetchall()
                candidates = [decode_row("tasks", item) for item in rows]
                if not candidates:
                    raise BridgeError("work_not_found", "No resumable task matched")
                if len(candidates) > 1:
                    return {
                        "ok": False,
                        "error": {"code": "ambiguous_task", "message": "Multiple resumable tasks matched; choose a task_id"},
                        "candidates": [
                            {"task_id": item["task_id"], "project_id": item["project_id"], "title": item["title"], "status": item["status"], "updated_at": item["updated_at"]}
                            for item in candidates if item
                        ],
                        "mutation_replayed": False,
                    }
                task = candidates[0]
            checkpoint = decode_row("checkpoints", conn.execute(
                "SELECT * FROM checkpoints WHERE task_id = ? ORDER BY sequence DESC LIMIT 1", (task["task_id"],)
            ).fetchone())
            project_context = None
            if task.get("project_id"):
                project = decode_row("projects", conn.execute("SELECT * FROM projects WHERE project_id = ?", (task["project_id"],)).fetchone())
                if project:
                    project_context = {
                        "project_id": project["project_id"], "workspace_ref": project["workspace_ref"],
                        "relevant_skills": project["relevant_skills"], "relevant_mcp": project["relevant_mcp"],
                        "memory_queries": project["memory_queries"], "project_rules": project["project_rules"],
                        "known_risks": project["known_risks"],
                    }
            return {
                "ok": True,
                "task": task,
                "latest_checkpoint": checkpoint,
                "project_context": project_context,
                "next_action": checkpoint.get("next_action") if checkpoint else None,
                "mutation_replayed": False,
                "resume_semantics": "state_only",
            }
