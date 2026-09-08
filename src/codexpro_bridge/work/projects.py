"""Project context operations for Bridge 2.1 Work Runtime."""

from __future__ import annotations

from typing import Any

from codexpro_bridge.core.errors import BridgeError
from .common import bounded_json, decode_row, new_id, now


class ProjectOps:
    store: Any

    def project_create(
        self,
        name: str,
        description: str = "",
        workspace_ref: str | None = None,
        relevant_skills: list[str] | None = None,
        relevant_mcp: list[str] | None = None,
        memory_queries: list[str] | None = None,
        project_rules: list[str] | None = None,
        known_risks: list[str] | None = None,
    ) -> dict[str, Any]:
        if not name.strip():
            raise BridgeError("invalid_work_input", "Project name is required")
        project_id = new_id("prj")
        stamp = now()
        with self.store.connect() as conn:
            conn.execute(
                """INSERT INTO projects
                (project_id,name,description,workspace_ref,relevant_skills,relevant_mcp,memory_queries,project_rules,known_risks,created_at,updated_at)
                VALUES (?,?,?,?,?,?,?,?,?,?,?)""",
                (
                    project_id, name.strip(), description, workspace_ref,
                    bounded_json(relevant_skills or []), bounded_json(relevant_mcp or []),
                    bounded_json(memory_queries or []), bounded_json(project_rules or []),
                    bounded_json(known_risks or []), stamp, stamp,
                ),
            )
            return {"ok": True, "project": self.store.one(conn, "projects", "SELECT * FROM projects WHERE project_id = ?", (project_id,))}

    def project_get(self, project_id: str) -> dict[str, Any]:
        with self.store.connect() as conn:
            return {"ok": True, "project": self.store.one(conn, "projects", "SELECT * FROM projects WHERE project_id = ?", (project_id,))}

    def project_list(self, limit: int = 50, offset: int = 0) -> dict[str, Any]:
        limit = max(1, min(int(limit), 200))
        offset = max(0, int(offset))
        with self.store.connect() as conn:
            rows = conn.execute("SELECT * FROM projects ORDER BY updated_at DESC LIMIT ? OFFSET ?", (limit, offset)).fetchall()
            return {"ok": True, "projects": [decode_row("projects", item) for item in rows], "count": len(rows)}

    def project_refresh(self, project_id: str, **changes: Any) -> dict[str, Any]:
        allowed = {"name", "description", "workspace_ref", "relevant_skills", "relevant_mcp", "memory_queries", "project_rules", "known_risks"}
        unknown = set(changes) - allowed
        if unknown:
            raise BridgeError("invalid_work_input", "Unsupported project fields", {"fields": sorted(unknown)})
        if not changes:
            return self.project_get(project_id)
        sets: list[str] = []
        params: list[Any] = []
        for key, value in changes.items():
            if key in {"relevant_skills", "relevant_mcp", "memory_queries", "project_rules", "known_risks"}:
                value = bounded_json(value or [])
            sets.append(f"{key} = ?")
            params.append(value)
        sets.append("updated_at = ?")
        params.extend([now(), project_id])
        with self.store.connect() as conn:
            cur = conn.execute(f"UPDATE projects SET {', '.join(sets)} WHERE project_id = ?", tuple(params))
            if cur.rowcount != 1:
                raise BridgeError("work_not_found", "Project not found")
            return {"ok": True, "project": self.store.one(conn, "projects", "SELECT * FROM projects WHERE project_id = ?", (project_id,))}

    def project_context(self, project_id: str) -> dict[str, Any]:
        with self.store.connect() as conn:
            project = self.store.one(conn, "projects", "SELECT * FROM projects WHERE project_id = ?", (project_id,))
            tasks = [decode_row("tasks", item) for item in conn.execute(
                "SELECT * FROM tasks WHERE project_id = ? ORDER BY updated_at DESC LIMIT 50", (project_id,)
            ).fetchall()]
            latest = conn.execute(
                """SELECT c.* FROM checkpoints c JOIN tasks t ON t.task_id = c.task_id
                WHERE t.project_id = ? ORDER BY c.created_at DESC LIMIT 1""", (project_id,)
            ).fetchone()
            artifacts = [decode_row("artifacts", item) for item in conn.execute(
                "SELECT * FROM artifacts WHERE project_id = ? ORDER BY updated_at DESC LIMIT 100", (project_id,)
            ).fetchall()]
            return {
                "ok": True,
                "project": project,
                "open_task_ids": [t["task_id"] for t in tasks if t and t["status"] in {"pending", "running", "blocked", "audit"}],
                "recent_task_ids": [t["task_id"] for t in tasks if t][:20],
                "latest_checkpoint": decode_row("checkpoints", latest),
                "artifact_ids": [a["artifact_id"] for a in artifacts if a],
                "workspace_action": "Use the separate official CodexPro connection for workspace/files/Bash/Git/edits/tests.",
            }
