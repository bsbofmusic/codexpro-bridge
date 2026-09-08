"""Shared constants and helpers for the Bridge 2.1 Work Runtime."""

from __future__ import annotations

import json
from datetime import datetime, timezone
from typing import Any
from uuid import uuid4

from codexpro_bridge.core.errors import BridgeError

SCHEMA_VERSION = 1

WORK_OPERATIONS = (
    "task.create", "task.get", "task.list", "task.update", "task.transition",
    "task.step_add", "task.step_update", "task.summary", "task.resume",
    "checkpoint.create", "checkpoint.get", "checkpoint.list",
    "project.create", "project.get", "project.list", "project.context", "project.refresh",
    "audit.create", "audit.record", "audit.evaluate", "audit.get", "audit.list",
    "artifact.register", "artifact.get", "artifact.list", "artifact.finalize", "artifact.archive",
)

TASK_STATES = {"pending", "running", "blocked", "audit", "done", "failed", "cancelled"}
STEP_STATES = {"todo", "doing", "blocked", "done", "failed", "skipped"}
ARTIFACT_STATES = {"draft", "final", "archived"}
VERIFICATION_STATES = {"declared", "verified", "stale"}
EVIDENCE_TYPES = {"verified_structured", "manual", "external_reference"}

TASK_TRANSITIONS: dict[str, set[str]] = {
    "pending": {"running", "cancelled"},
    "running": {"blocked", "audit", "failed", "cancelled"},
    "blocked": {"running", "cancelled"},
    "audit": {"done", "running", "blocked"},
    "done": set(), "failed": set(), "cancelled": set(),
}

JSON_COLUMNS = {
    "projects": {"relevant_skills", "relevant_mcp", "memory_queries", "project_rules", "known_risks"},
    "tasks": {"metadata"},
    "checkpoints": {"completed_step_ids", "pending_step_ids", "blocked_step_ids", "important_findings", "decisions", "artifact_ids"},
    "audit_checks": {"expected", "actual"},
    "artifacts": {"metadata"},
}


def now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid4().hex[:20]}"


def dump(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def load(value: Any) -> Any:
    if value is None or not isinstance(value, str):
        return value
    try:
        return json.loads(value)
    except Exception:
        return value


def bounded_json(value: Any, max_chars: int = 20_000) -> str:
    encoded = dump(value)
    if len(encoded) > max_chars:
        raise BridgeError("invalid_work_input", "Work Runtime JSON field is too large")
    return encoded


def decode_row(table: str, row: Any) -> dict[str, Any] | None:
    if row is None:
        return None
    result = dict(row)
    for column in JSON_COLUMNS.get(table, set()):
        if column in result:
            result[column] = load(result[column])
    for key in ("audit_required", "manual_allowed"):
        if key in result:
            result[key] = bool(result[key])
    return result
