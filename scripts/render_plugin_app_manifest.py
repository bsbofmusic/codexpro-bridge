#!/usr/bin/env python3
"""Render the deployment-local Plugin app manifest from its checked-in template."""

from __future__ import annotations

import argparse
import json
import re
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
PLUGIN_ROOT = ROOT / "plugin" / "codexpro-bridge"
DEFAULT_TEMPLATE = PLUGIN_ROOT / ".app.json.template"
DEFAULT_OUTPUT = PLUGIN_ROOT / ".app.json"
_CONNECTION_ID = re.compile(r"[A-Za-z0-9][A-Za-z0-9._-]{2,127}\Z")
_TEMPLATE_IDS = {
    "codexpro": "__CODEXPRO_CONNECTION_ID__",
    "bridge": "__CODEXPRO_BRIDGE_CONNECTION_ID__",
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Render a local CodexPro Bridge app manifest without printing IDs."
    )
    parser.add_argument("--codexpro-connection-id", required=True)
    parser.add_argument("--bridge-connection-id", required=True)
    parser.add_argument("--template", type=Path, default=DEFAULT_TEMPLATE)
    parser.add_argument("--output", type=Path, default=DEFAULT_OUTPUT)
    return parser.parse_args()


def _read_template(path: Path) -> dict[str, Any]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError("Plugin app-manifest template is invalid") from exc
    if not isinstance(payload, dict) or set(payload) != {"apps"}:
        raise ValueError("Plugin app-manifest template has an invalid shape")
    apps = payload["apps"]
    if not isinstance(apps, dict) or set(apps) != set(_TEMPLATE_IDS):
        raise ValueError("Plugin app-manifest template must define the two required apps")
    for app_name, placeholder in _TEMPLATE_IDS.items():
        if apps.get(app_name) != {"id": placeholder}:
            raise ValueError("Plugin app-manifest template contains an unexpected app entry")
    return payload


def _validate_connection_id(value: str, label: str) -> str:
    if not _CONNECTION_ID.fullmatch(value):
        raise ValueError(f"{label} must be an opaque technical connection ID")
    if value in _TEMPLATE_IDS.values():
        raise ValueError(f"{label} must not be a template placeholder")
    return value


def render(
    *,
    codexpro_connection_id: str,
    bridge_connection_id: str,
    template: Path = DEFAULT_TEMPLATE,
    output: Path = DEFAULT_OUTPUT,
) -> None:
    codexpro_id = _validate_connection_id(codexpro_connection_id, "CodexPro connection ID")
    bridge_id = _validate_connection_id(bridge_connection_id, "Bridge connection ID")
    if codexpro_id == bridge_id:
        raise ValueError("CodexPro and Bridge connection IDs must be distinct")
    payload = _read_template(template)
    payload["apps"]["codexpro"]["id"] = codexpro_id
    payload["apps"]["bridge"]["id"] = bridge_id
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def main() -> None:
    args = parse_args()
    try:
        render(
            codexpro_connection_id=args.codexpro_connection_id,
            bridge_connection_id=args.bridge_connection_id,
            template=args.template,
            output=args.output,
        )
    except ValueError as exc:
        raise SystemExit(str(exc)) from exc
    print(f"Rendered Plugin app manifest: {args.output}")


if __name__ == "__main__":
    main()
