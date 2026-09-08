"""Runtime configuration for the standalone CodexPro Bridge."""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path
from urllib.parse import urlparse

from .errors import BridgeError


def _env_bool(name: str, default: bool = False) -> bool:
    raw = os.environ.get(name)
    if raw is None:
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


def _env_modules(name: str = "CODEXPRO_BRIDGE_MODULES") -> tuple[str, ...] | None:
    raw = os.environ.get(name)
    if raw is None or raw.strip() in {"", "*", "all", "auto"}:
        return None
    values = tuple(dict.fromkeys(item.strip() for item in raw.split(",") if item.strip()))
    if not values:
        return None
    return values


def _env_int(name: str, default: int, minimum: int, maximum: int) -> int:
    raw = os.environ.get(name)
    if raw is None:
        return default
    try:
        value = int(raw)
    except ValueError as exc:
        raise BridgeError("invalid_config", f"{name} must be an integer") from exc
    if not minimum <= value <= maximum:
        raise BridgeError("invalid_config", f"{name} must be between {minimum} and {maximum}")
    return value


@dataclass(slots=True)
class BridgeConfig:
    """Bridge process configuration.

    ``http_token`` is excluded from ``repr`` so diagnostics cannot disclose it.
    The Bridge is intentionally local-first: its shared Skill root and MCP
    endpoint are independent infrastructure, not properties of any agent.
    """

    host: str = "127.0.0.1"
    port: int = 18787
    http_token: str | None = field(default=None, repr=False)
    allow_anonymous: bool = False
    skills_root: Path = Path("/home/agent/.skills-manager/skills")
    mcp_url: str = "http://127.0.0.1:19090/mcp"
    mcp_optional_urls: tuple[str, ...] = ()
    mcp_timeout_seconds: int = 180
    obsidian_mcp_command: Path = Path("/home/agent/.local/share/agent-stack/memory/obsidian-mcp.sh")
    memos_mcp_command: Path = Path("/home/agent/.local/share/agent-stack/memory/memos-api-mcp.sh")
    memory_timeout_seconds: int = 180
    work_db_path: Path = Path("/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3")
    enabled_modules: tuple[str, ...] | None = None
    max_output_chars: int = 120_000

    @classmethod
    def from_env(cls) -> "BridgeConfig":
        token = os.environ.get("CODEXPRO_BRIDGE_HTTP_TOKEN") or os.environ.get("CODEXPRO_HTTP_TOKEN")
        config = cls(
            host=os.environ.get("CODEXPRO_BRIDGE_HOST", "127.0.0.1"),
            port=_env_int("CODEXPRO_BRIDGE_PORT", 18787, 1024, 65535),
            http_token=token,
            allow_anonymous=_env_bool("CODEXPRO_BRIDGE_ALLOW_ANONYMOUS", False),
            skills_root=Path(os.environ.get("CODEXPRO_BRIDGE_SKILLS_ROOT", "/home/agent/.skills-manager/skills")),
            mcp_url=os.environ.get("CODEXPRO_BRIDGE_MCP_URL", "http://127.0.0.1:19090/mcp"),
            mcp_optional_urls=tuple(
                item.strip()
                for item in os.environ.get("CODEXPRO_BRIDGE_MCP_OPTIONAL_URLS", "").split(",")
                if item.strip()
            ),
            mcp_timeout_seconds=_env_int("CODEXPRO_BRIDGE_MCP_TIMEOUT", 180, 5, 600),
            obsidian_mcp_command=Path(
                os.environ.get(
                    "CODEXPRO_BRIDGE_OBSIDIAN_MCP_COMMAND",
                    "/home/agent/.local/share/agent-stack/memory/obsidian-mcp.sh",
                )
            ),
            memos_mcp_command=Path(
                os.environ.get(
                    "CODEXPRO_BRIDGE_MEMOS_MCP_COMMAND",
                    "/home/agent/.local/share/agent-stack/memory/memos-api-mcp.sh",
                )
            ),
            memory_timeout_seconds=_env_int("CODEXPRO_BRIDGE_MEMORY_TIMEOUT", 180, 5, 600),
            work_db_path=Path(
                os.environ.get(
                    "CODEXPRO_BRIDGE_WORK_DB_PATH",
                    "/home/agent/.local/share/codexpro-bridge/work-runtime.sqlite3",
                )
            ),
            enabled_modules=_env_modules(),
            max_output_chars=_env_int("CODEXPRO_BRIDGE_MAX_OUTPUT_CHARS", 120_000, 4_096, 500_000),
        )
        config.validate()
        return config

    def validate(self) -> None:
        if self.host not in {"127.0.0.1", "::1", "localhost"}:
            raise BridgeError("invalid_config", "CodexPro Bridge must listen on loopback only")
        if not self.allow_anonymous and not self.http_token:
            raise BridgeError("missing_auth", "Bridge HTTP authentication token is not configured")
        if self.http_token is not None and len(self.http_token.encode()) < 24:
            raise BridgeError("weak_auth", "Bridge HTTP authentication token is too short")
        if not self.skills_root.is_absolute():
            raise BridgeError("invalid_config", "Shared Skill root must be an absolute path")
        for endpoint in (self.mcp_url, *self.mcp_optional_urls):
            parsed = urlparse(endpoint)
            if parsed.scheme not in {"http", "https"} or parsed.hostname not in {"127.0.0.1", "::1", "localhost"}:
                raise BridgeError("invalid_config", "Shared MCP endpoints must be loopback HTTP(S) URLs")
        for command in (self.obsidian_mcp_command, self.memos_mcp_command):
            if not command.is_absolute():
                raise BridgeError("invalid_config", "Shared memory MCP commands must be absolute paths")
        if not self.work_db_path.is_absolute():
            raise BridgeError("invalid_config", "Work Runtime database path must be absolute")

    def public_dict(self) -> dict[str, object]:
        return {
            "host": self.host,
            "port": self.port,
            "auth_required": not self.allow_anonymous,
            "skills_root": str(self.skills_root),
            "mcp_url": self.mcp_url,
            "mcp_optional_urls": list(self.mcp_optional_urls),
            "mcp_timeout_seconds": self.mcp_timeout_seconds,
            "obsidian_mcp_command": str(self.obsidian_mcp_command),
            "memos_mcp_command": str(self.memos_mcp_command),
            "memory_timeout_seconds": self.memory_timeout_seconds,
            "work_db_path": str(self.work_db_path),
            "enabled_modules": "auto" if self.enabled_modules is None else list(self.enabled_modules),
            "max_output_chars": self.max_output_chars,
        }
