"""Artifact-reference registry for Bridge 2.1 Work Runtime."""

from __future__ import annotations

from typing import Any

from codexpro_bridge.core.errors import BridgeError
from .common import ARTIFACT_STATES, VERIFICATION_STATES, bounded_json, decode_row, new_id, now


class ArtifactOps:
    store: Any

    def artifact_register(
        self,
        name: str,
        path_or_ref: str,
        type: str = "other",
        task_id: str | None = None,
        project_id: str | None = None,
        sha256: str | None = None,
        size: int | None = None,
        verification_status: str = "declared",
        verification_evidence_ref: str | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        if not name.strip() or not path_or_ref.strip():
            raise BridgeError("invalid_work_input", "Artifact name and path/reference are required")
        if verification_status not in VERIFICATION_STATES:
            raise BridgeError("invalid_work_input", "Unknown artifact verification status")
        if verification_status == "verified" and not verification_evidence_ref:
            raise BridgeError("invalid_work_input", "Verified artifact requires verification evidence reference")
        artifact_id = new_id("art")
        stamp = now()
        with self.store.connect() as conn:
            if task_id:
                task = self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))
                if project_id is None:
                    project_id = task.get("project_id")
            if project_id:
                self.store.one(conn, "projects", "SELECT * FROM projects WHERE project_id = ?", (project_id,))
            conn.execute(
                """INSERT INTO artifacts
                (artifact_id,task_id,project_id,name,type,path_or_ref,status,sha256,size,verification_status,verification_evidence_ref,created_at,updated_at,metadata)
                VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (
                    artifact_id, task_id, project_id, name.strip(), type, path_or_ref, "draft", sha256, size,
                    verification_status, verification_evidence_ref, stamp, stamp, bounded_json(metadata or {}),
                ),
            )
            if task_id:
                self.store.touch_task_revision(conn, task_id)
            return {"ok": True, "artifact": self.store.one(conn, "artifacts", "SELECT * FROM artifacts WHERE artifact_id = ?", (artifact_id,))}

    def artifact_get(self, artifact_id: str) -> dict[str, Any]:
        with self.store.connect() as conn:
            return {"ok": True, "artifact": self.store.one(conn, "artifacts", "SELECT * FROM artifacts WHERE artifact_id = ?", (artifact_id,))}

    def artifact_list(
        self,
        task_id: str | None = None,
        project_id: str | None = None,
        status: str | None = None,
        limit: int = 100,
        offset: int = 0,
    ) -> dict[str, Any]:
        if status is not None and status not in ARTIFACT_STATES:
            raise BridgeError("invalid_work_input", "Unknown artifact status")
        clauses: list[str] = []
        params: list[Any] = []
        if task_id:
            clauses.append("task_id = ?")
            params.append(task_id)
        if project_id:
            clauses.append("project_id = ?")
            params.append(project_id)
        if status:
            clauses.append("status = ?")
            params.append(status)
        sql = "SELECT * FROM artifacts"
        if clauses:
            sql += " WHERE " + " AND ".join(clauses)
        sql += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
        params.extend([max(1, min(int(limit), 300)), max(0, int(offset))])
        with self.store.connect() as conn:
            rows = conn.execute(sql, tuple(params)).fetchall()
            return {"ok": True, "artifacts": [decode_row("artifacts", item) for item in rows], "count": len(rows)}

    def _artifact_state(self, artifact_id: str, target: str) -> dict[str, Any]:
        with self.store.connect() as conn:
            artifact = self.store.one(conn, "artifacts", "SELECT * FROM artifacts WHERE artifact_id = ?", (artifact_id,))
            current = str(artifact["status"])
            legal = (current == "draft" and target in {"final", "archived"}) or (current == "final" and target == "archived")
            if not legal:
                raise BridgeError("invalid_artifact_transition", f"Artifact cannot transition from {current} to {target}")
            conn.execute("UPDATE artifacts SET status = ?, updated_at = ? WHERE artifact_id = ?", (target, now(), artifact_id))
            if artifact.get("task_id"):
                self.store.touch_task_revision(conn, str(artifact["task_id"]))
            return {"ok": True, "artifact": self.store.one(conn, "artifacts", "SELECT * FROM artifacts WHERE artifact_id = ?", (artifact_id,))}

    def artifact_finalize(self, artifact_id: str) -> dict[str, Any]:
        return self._artifact_state(artifact_id, "final")

    def artifact_archive(self, artifact_id: str) -> dict[str, Any]:
        return self._artifact_state(artifact_id, "archived")
