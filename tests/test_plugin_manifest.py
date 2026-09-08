from __future__ import annotations

import json
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PLUGIN = ROOT / "plugin" / "codexpro-bridge"
RENDERER = ROOT / "scripts" / "render_plugin_app_manifest.py"
VALIDATOR = Path("/home/agent/.codex/skills/.system/plugin-creator/scripts/validate_plugin.py")


def test_plugin_manifest_declares_one_skill_and_two_pre_registered_apps() -> None:
    manifest = json.loads((PLUGIN / ".codex-plugin" / "plugin.json").read_text())

    assert manifest["name"] == PLUGIN.name
    assert manifest["version"] == "2.1.0"
    assert manifest["skills"] == "./skills/"
    assert manifest["apps"] == "./.app.json"
    assert "mcpServers" not in manifest
    assert not (PLUGIN / ".app.json").exists()

    template = json.loads((PLUGIN / ".app.json.template").read_text())
    assert template == {
        "apps": {
            "codexpro": {"id": "__CODEXPRO_CONNECTION_ID__"},
            "bridge": {"id": "__CODEXPRO_BRIDGE_CONNECTION_ID__"},
        }
    }


def test_renderer_creates_a_validator_accepted_local_manifest_without_template_ids(tmp_path: Path) -> None:
    rendered_plugin = tmp_path / "codexpro-bridge"
    shutil.copytree(PLUGIN, rendered_plugin)
    output = rendered_plugin / ".app.json"
    command = [
        sys.executable,
        str(RENDERER),
        "--codexpro-connection-id",
        "conn_codexpro_123",
        "--bridge-connection-id",
        "conn_bridge_456",
        "--output",
        str(output),
    ]

    completed = subprocess.run(command, capture_output=True, text=True, check=False)
    assert completed.returncode == 0, completed.stderr
    assert "conn_" not in completed.stdout
    rendered = output.read_text()
    assert "__" not in rendered
    assert "conn_codexpro_123" in rendered

    if VALIDATOR.is_file():
        validated = subprocess.run(
            [sys.executable, str(VALIDATOR), str(rendered_plugin)],
            capture_output=True,
            text=True,
            check=False,
        )
        assert validated.returncode == 0, validated.stdout + validated.stderr


def test_renderer_rejects_duplicate_or_non_opaque_connection_ids(tmp_path: Path) -> None:
    output = tmp_path / ".app.json"
    duplicate = subprocess.run(
        [
            sys.executable,
            str(RENDERER),
            "--codexpro-connection-id",
            "conn_same_123",
            "--bridge-connection-id",
            "conn_same_123",
            "--output",
            str(output),
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    malformed = subprocess.run(
        [
            sys.executable,
            str(RENDERER),
            "--codexpro-connection-id",
            "https://not-an-id.invalid",
            "--bridge-connection-id",
            "conn_bridge_456",
            "--output",
            str(output),
        ],
        capture_output=True,
        text=True,
        check=False,
    )

    assert duplicate.returncode != 0
    assert "distinct" in duplicate.stderr
    assert malformed.returncode != 0
    assert "opaque technical connection ID" in malformed.stderr
