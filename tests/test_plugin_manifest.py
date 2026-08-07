from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tomllib
from pathlib import Path

from codexpro_bridge import __version__
from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.server.app import build_server


ROOT = Path(__file__).resolve().parents[1]
PLUGIN = ROOT / "plugin" / "codexpro-bridge"
RENDERER = ROOT / "scripts" / "render_plugin_app_manifest.py"


def test_plugin_manifest_declares_one_skill_and_two_pre_registered_apps() -> None:
    manifest = json.loads((PLUGIN / ".codex-plugin" / "plugin.json").read_text())

    assert manifest["name"] == PLUGIN.name
    assert manifest["version"] == "0.1.1"
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


def test_release_version_is_consistent() -> None:
    manifest = json.loads((PLUGIN / ".codex-plugin" / "plugin.json").read_text())
    project = tomllib.loads((ROOT / "pyproject.toml").read_text())
    server, capabilities = build_server(BridgeConfig(allow_anonymous=True))
    try:
        assert __version__ == "0.1.1"
        assert project["project"]["version"] == __version__
        assert manifest["version"] == __version__
        assert server._mcp_server.version == __version__
        assert {module["version"] for module in capabilities.registry.manifest()} == {
            __version__
        }
    finally:
        capabilities.shutdown()


def test_renderer_creates_a_validator_accepted_local_manifest_without_template_ids(
    tmp_path: Path,
) -> None:
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
    validator = os.environ.get("CODEX_PLUGIN_VALIDATOR")
    if validator:
        validated = subprocess.run(
            [sys.executable, validator, str(rendered_plugin)],
            capture_output=True,
            text=True,
            check=False,
        )
        assert validated.returncode == 0, validated.stdout + validated.stderr
    else:
        assert json.loads(rendered) == {
            "apps": {
                "codexpro": {"id": "conn_codexpro_123"},
                "bridge": {"id": "conn_bridge_456"},
            }
        }


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
