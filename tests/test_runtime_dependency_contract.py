from __future__ import annotations

from pathlib import Path
import shlex
import subprocess
import tomllib


ROOT = Path(__file__).resolve().parents[1]
SERVICE = ROOT / "deploy/codexpro-bridge.service"


def _production_python() -> str:
    exec_start = next(
        line for line in SERVICE.read_text(encoding="utf-8").splitlines() if line.startswith("ExecStart=")
    )
    return shlex.split(exec_start.removeprefix("ExecStart="))[0]


def test_declared_mcp_pin_matches_production_runtime() -> None:
    project = tomllib.loads((ROOT / "pyproject.toml").read_text(encoding="utf-8"))
    dependencies = project["project"]["dependencies"]
    pins = [dependency for dependency in dependencies if dependency.startswith("mcp==")]

    assert len(pins) == 1
    runtime_version = subprocess.check_output(
        [
            _production_python(),
            "-c",
            "from importlib import metadata; print(metadata.version('mcp'))",
        ],
        text=True,
    ).strip()
    assert pins[0].removeprefix("mcp==") == runtime_version
