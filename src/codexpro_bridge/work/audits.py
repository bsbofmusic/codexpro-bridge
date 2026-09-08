"""Deterministic mechanical-audit state for Bridge 2.1 Work Runtime."""

from __future__ import annotations

from typing import Any

from codexpro_bridge.core.errors import BridgeError
from .common import EVIDENCE_TYPES, bounded_json, decode_row, dump, new_id, now


class AuditOps:
    store: Any

    def _audit_bundle(self, conn, audit_id: str) -> dict[str, Any]:
        audit = self.store.one(conn, "audits", "SELECT * FROM audits WHERE audit_id = ?", (audit_id,))
        checks = [decode_row("audit_checks", item) for item in conn.execute(
            "SELECT * FROM audit_checks WHERE audit_id = ? ORDER BY rowid", (audit_id,)
        ).fetchall()]
        return {"ok": True, "audit": audit, "checks": checks}

    def audit_create(self, task_id: str, checks: list[dict[str, Any]] | None = None) -> dict[str, Any]:
        audit_id = new_id("aud")
        stamp = now()
        with self.store.connect() as conn:
            task = self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (task_id,))
            conn.execute(
                "INSERT INTO audits (audit_id,task_id,status,audited_task_revision,created_at,started_at,completed_at) VALUES (?,?,?,?,?,?,?)",
                (audit_id, task_id, "UNCERTAIN", task["revision"], stamp, stamp, None),
            )
            for item in checks or []:
                name = str(item.get("name") or "").strip()
                if not name:
                    raise BridgeError("invalid_work_input", "Audit check name is required")
                conn.execute(
                    """INSERT INTO audit_checks
                    (check_id,audit_id,name,expected,actual,status,evidence_type,evidence_ref,command_or_tool,exit_code,timestamp,manual_allowed)
                    VALUES (?,?,?,?,?,?,?,?,?,?,?,?)""",
                    (
                        new_id("ack"), audit_id, name, bounded_json(item.get("expected")), None, "UNCERTAIN",
                        None, None, None, None, None, int(bool(item.get("manual_allowed", False))),
                    ),
                )
            return self._audit_bundle(conn, audit_id)

    def audit_record(
        self,
        audit_id: str,
        check_id: str,
        actual: Any,
        evidence_type: str,
        evidence_ref: str | None = None,
        command_or_tool: str | None = None,
        exit_code: int | None = None,
    ) -> dict[str, Any]:
        if evidence_type not in EVIDENCE_TYPES:
            raise BridgeError("invalid_work_input", "Unknown audit evidence type")
        with self.store.connect() as conn:
            self.store.one(conn, "audit_checks", "SELECT * FROM audit_checks WHERE check_id = ? AND audit_id = ?", (check_id, audit_id))
            conn.execute(
                """UPDATE audit_checks SET actual = ?, evidence_type = ?, evidence_ref = ?, command_or_tool = ?, exit_code = ?, timestamp = ?, status = 'UNCERTAIN'
                WHERE check_id = ? AND audit_id = ?""",
                (bounded_json(actual), evidence_type, evidence_ref, command_or_tool, exit_code, now(), check_id, audit_id),
            )
            return {"ok": True, "check": self.store.one(conn, "audit_checks", "SELECT * FROM audit_checks WHERE check_id = ?", (check_id,))}

    def audit_evaluate(self, audit_id: str) -> dict[str, Any]:
        with self.store.connect() as conn:
            audit = self.store.one(conn, "audits", "SELECT * FROM audits WHERE audit_id = ?", (audit_id,))
            task = self.store.one(conn, "tasks", "SELECT * FROM tasks WHERE task_id = ?", (audit["task_id"],))
            checks = conn.execute("SELECT * FROM audit_checks WHERE audit_id = ? ORDER BY rowid", (audit_id,)).fetchall()
            statuses: list[str] = []
            for raw in checks:
                check = decode_row("audit_checks", raw)
                assert check is not None
                status = "UNCERTAIN"
                if check["actual"] is not None:
                    if check["evidence_type"] == "manual" and not check["manual_allowed"]:
                        status = "UNCERTAIN"
                    else:
                        status = "PASS" if dump(check["actual"]) == dump(check["expected"]) else "FAIL"
                conn.execute("UPDATE audit_checks SET status = ? WHERE check_id = ?", (status, check["check_id"]))
                statuses.append(status)
            stale = int(audit["audited_task_revision"]) != int(task["revision"])
            if stale or not statuses:
                overall = "UNCERTAIN"
            elif "FAIL" in statuses:
                overall = "FAIL"
            elif "UNCERTAIN" in statuses:
                overall = "UNCERTAIN"
            else:
                overall = "PASS"
            completed = now() if overall in {"PASS", "FAIL"} else None
            conn.execute("UPDATE audits SET status = ?, completed_at = ? WHERE audit_id = ?", (overall, completed, audit_id))
            result = self._audit_bundle(conn, audit_id)
            result["stale"] = stale
            return result

    def audit_get(self, audit_id: str) -> dict[str, Any]:
        with self.store.connect() as conn:
            return self._audit_bundle(conn, audit_id)

    def audit_list(self, task_id: str, limit: int = 50, offset: int = 0) -> dict[str, Any]:
        with self.store.connect() as conn:
            rows = conn.execute(
                "SELECT * FROM audits WHERE task_id = ? ORDER BY created_at DESC, rowid DESC LIMIT ? OFFSET ?",
                (task_id, max(1, min(int(limit), 200)), max(0, int(offset))),
            ).fetchall()
            return {"ok": True, "audits": [decode_row("audits", item) for item in rows], "count": len(rows)}
