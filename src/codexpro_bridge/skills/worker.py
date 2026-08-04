"""Short-lived, stdout-clean worker around Hermes's public Skill API.

This module is intentionally run as a separate process for each Bridge Skill
request.  It imports no Bridge runtime state and writes one JSON document to
stdout, which keeps Hermes's in-process cache and environment side effects out
of the long-lived public MCP server.
"""

from __future__ import annotations

import contextlib
import io
import json
import os
import sys
from typing import Any


def _response_from_hermes(raw: str) -> dict[str, Any]:
    try:
        parsed = json.loads(raw)
    except (TypeError, json.JSONDecodeError):
        return {"success": False, "error": "Hermes returned an invalid Skill response"}
    if isinstance(parsed, dict):
        return parsed
    return {"success": False, "error": "Hermes returned an invalid Skill response"}


def execute(request: dict[str, Any]) -> dict[str, Any]:
    # Import and incidental prints are quarantined so stdout remains an MCP-safe
    # one-document channel.  Stderr is captured by the parent and discarded.
    with contextlib.redirect_stdout(io.StringIO()):
        from tools.skills_tool import skill_view, skills_list

        operation = request.get("operation")
        if operation == "list":
            return _response_from_hermes(skills_list(category=request.get("category")))
        if operation == "load":
            return _response_from_hermes(
                skill_view(str(request["name"]), preprocess=False)
            )
        if operation == "resource":
            return _response_from_hermes(
                skill_view(
                    str(request["name"]),
                    file_path=str(request["resource_path"]),
                    preprocess=False,
                )
            )
    return {"success": False, "error": "Unsupported Skill worker operation"}


def main() -> int:
    try:
        request = json.load(sys.stdin)
        if not isinstance(request, dict):
            raise ValueError("request must be an object")
        result = execute(request)
    except Exception:
        # Do not emit source paths, process arguments, environment values, or
        # raw exception text from the worker boundary.
        result = {"success": False, "error": "Hermes Skill worker failed"}
    print(json.dumps(result, ensure_ascii=False, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
