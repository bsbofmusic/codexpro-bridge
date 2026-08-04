"""Runtime configuration with secret-safe representations."""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path

from .errors import BridgeError


_DEFAULT_HERMES_HOME = Path("/hermes/hermes_data")
_DEFAULT_HERMES_WORKDIR = Path("/opt/hermes")
_DEFAULT_HERMES_PYTHON = _DEFAULT_HERMES_WORKDIR / ".venv" / "bin" / "python"


def _env_bool(name: str, default: bool = False) -> bool:
    raw = os.environ.get(name)
    if raw is None:
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


def _env_int(name: str, default: int, minimum: int, maximum: int) -> int:
    raw = os.environ.get(name)
    if raw is None:
        return default
    try:
        value = int(raw)
    except ValueError as exc:
        raise BridgeError("invalid_config", f"{name} must be an integer") from exc
    if not minimum <= value <= maximum:
        raise BridgeError(
            "invalid_config",
            f"{name} must be between {minimum} and {maximum}",
        )
    return value


@dataclass(slots=True)
class BridgeConfig:
    """Bridge process configuration.

    ``http_token`` is deliberately excluded from ``repr`` so diagnostics and
    test failures cannot accidentally disclose it.
    """

    host: str = "127.0.0.1"
    port: int = 18787
    http_token: str | None = field(default=None, repr=False)
    allow_anonymous: bool = False
    hermes_home: Path = _DEFAULT_HERMES_HOME
    hermes_workdir: Path = _DEFAULT_HERMES_WORKDIR
    hermes_python: Path = _DEFAULT_HERMES_PYTHON
    memos_command: Path = field(
        default_factory=lambda: Path.home() / ".local" / "bin" / "memos-api-mcp"
    )
    skill_timeout_seconds: int = 30
    mcp_timeout_seconds: int = 180
    max_output_chars: int = 120_000

    @classmethod
    def from_env(cls) -> "BridgeConfig":
        # The CODEXPRO_HTTP_TOKEN fallback lets a protected systemd environment
        # reference be reused without creating a second human-readable secret.
        token = os.environ.get("CODEXPRO_BRIDGE_HTTP_TOKEN") or os.environ.get(
            "CODEXPRO_HTTP_TOKEN"
        )
        hermes_home = Path(
            os.environ.get("CODEXPRO_BRIDGE_HERMES_HOME", str(_DEFAULT_HERMES_HOME))
        )
        hermes_workdir = Path(
            os.environ.get("CODEXPRO_BRIDGE_HERMES_WORKDIR", str(_DEFAULT_HERMES_WORKDIR))
        )
        hermes_python = Path(
            os.environ.get(
                "CODEXPRO_BRIDGE_HERMES_PYTHON",
                str(hermes_workdir / ".venv" / "bin" / "python"),
            )
        )
        memos_command = Path(
            os.environ.get(
                "CODEXPRO_BRIDGE_MEMOS_COMMAND",
                str(Path.home() / ".local" / "bin" / "memos-api-mcp"),
            )
        )
        config = cls(
            host=os.environ.get("CODEXPRO_BRIDGE_HOST", "127.0.0.1"),
            port=_env_int("CODEXPRO_BRIDGE_PORT", 18787, 1024, 65535),
            http_token=token,
            allow_anonymous=_env_bool("CODEXPRO_BRIDGE_ALLOW_ANONYMOUS", False),
            hermes_home=hermes_home,
            hermes_workdir=hermes_workdir,
            hermes_python=hermes_python,
            memos_command=memos_command,
            skill_timeout_seconds=_env_int(
                "CODEXPRO_BRIDGE_SKILL_TIMEOUT", 30, 5, 120
            ),
            mcp_timeout_seconds=_env_int(
                "CODEXPRO_BRIDGE_MCP_TIMEOUT", 180, 5, 600
            ),
            max_output_chars=_env_int(
                "CODEXPRO_BRIDGE_MAX_OUTPUT_CHARS", 120_000, 4_096, 500_000
            ),
        )
        config.validate()
        return config

    def validate(self) -> None:
        if self.host not in {"127.0.0.1", "::1", "localhost"}:
            raise BridgeError(
                "invalid_config", "CodexPro Bridge must listen on loopback only"
            )
        if not self.allow_anonymous and not self.http_token:
            raise BridgeError(
                "missing_auth",
                "Bridge HTTP authentication token is not configured",
            )
        if self.http_token is not None and len(self.http_token.encode()) < 24:
            raise BridgeError(
                "weak_auth", "Bridge HTTP authentication token is too short"
            )

    def public_dict(self) -> dict[str, object]:
        return {
            "host": self.host,
            "port": self.port,
            "auth_required": not self.allow_anonymous,
            "hermes_home": str(self.hermes_home),
            "hermes_workdir": str(self.hermes_workdir),
            "skill_timeout_seconds": self.skill_timeout_seconds,
            "mcp_timeout_seconds": self.mcp_timeout_seconds,
            "max_output_chars": self.max_output_chars,
        }
