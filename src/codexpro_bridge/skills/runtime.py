"""Hermes public Skill API adapter with a hard read-only resource boundary."""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from pathlib import Path, PurePosixPath, PureWindowsPath
from typing import Any

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.core.redaction import redact_value, safe_exception_message

_RESOURCE_ROOTS = frozenset({"references", "templates", "scripts", "assets"})
_SENSITIVE_COMPONENT = re.compile(
    r"(?:^|[_-])(?:env|secret|secrets|credential|credentials|password|passwd|"
    r"private[_-]?key|ssh)(?:$|[_-])",
    re.IGNORECASE,
)
_SENSITIVE_SUFFIXES = frozenset({".pem", ".key", ".p12", ".pfx", ".kdbx"})


class HermesSkillsRuntime:
    """Read live Skill metadata/content through Hermes's supported functions."""

    def __init__(self, config: BridgeConfig):
        self.config = config
        self._source_root = Path(__file__).resolve().parents[2]

    def _worker_env(self) -> dict[str, str]:
        # The Skill worker never inherits the Bridge HTTP credential or any
        # unrelated process secret. Hermes loads its own profile data only as
        # required by its established public APIs.
        allowed = {
            key: value
            for key, value in os.environ.items()
            if key
            in {
                "PATH",
                "HOME",
                "USER",
                "LANG",
                "LC_ALL",
                "TERM",
                "SHELL",
                "TMPDIR",
            }
            or key.startswith("XDG_")
        }
        allowed["HERMES_HOME"] = str(self.config.hermes_home)
        allowed["PYTHONPATH"] = os.pathsep.join(
            [str(self._source_root), str(self.config.hermes_workdir)]
        )
        allowed["PYTHONUNBUFFERED"] = "1"
        return allowed

    def _invoke_worker(self, request: dict[str, Any]) -> dict[str, Any]:
        if not self.config.hermes_python.is_file():
            raise BridgeError("skills_unavailable", "Hermes Python runtime is unavailable")
        try:
            completed = subprocess.run(
                [str(self.config.hermes_python), "-m", "codexpro_bridge.skills.worker"],
                input=json.dumps(request, ensure_ascii=False),
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.DEVNULL,
                cwd=self.config.hermes_workdir,
                env=self._worker_env(),
                timeout=self.config.skill_timeout_seconds,
                check=False,
            )
        except subprocess.TimeoutExpired as exc:
            raise BridgeError("skills_timeout", "Hermes Skill request timed out") from exc
        except OSError as exc:
            raise BridgeError("skills_unavailable", "Hermes Skill worker could not start") from exc

        if completed.returncode != 0:
            raise BridgeError("skills_unavailable", "Hermes Skill worker failed")
        try:
            result = json.loads(completed.stdout)
        except json.JSONDecodeError as exc:
            raise BridgeError("skills_unavailable", "Hermes Skill worker returned invalid JSON") from exc
        if not isinstance(result, dict):
            raise BridgeError("skills_unavailable", "Hermes Skill worker returned invalid data")
        return redact_value(result)

    @staticmethod
    def _catalog_or_error(result: dict[str, Any]) -> list[dict[str, Any]]:
        if not result.get("success"):
            raise BridgeError("skills_unavailable", "Hermes Skill catalog is unavailable")
        skills = result.get("skills")
        if not isinstance(skills, list):
            raise BridgeError("skills_unavailable", "Hermes Skill catalog is invalid")
        return [item for item in skills if isinstance(item, dict) and item.get("name")]

    def list(self, category: str | None = None) -> dict[str, Any]:
        if category is not None and (not isinstance(category, str) or len(category) > 128):
            raise BridgeError("invalid_category", "Skill category is invalid")
        result = self._invoke_worker({"operation": "list", "category": category})
        skills = self._catalog_or_error(result)
        # Preserve Hermes metadata exactly where it is safe to do so.  The
        # public tool does not invent an index, ranking, or copied route table.
        return {
            "ok": True,
            "skills": skills,
            "categories": result.get("categories") if isinstance(result.get("categories"), list) else [],
            "count": len(skills),
        }

    def _validate_active_name(self, name: str) -> str:
        if not isinstance(name, str) or not name.strip() or len(name) > 128:
            raise BridgeError("invalid_skill", "Skill name is invalid")
        cleaned = name.strip()
        path = PurePosixPath(cleaned)
        windows_path = PureWindowsPath(cleaned)
        if (
            path.is_absolute()
            or windows_path.is_absolute()
            or windows_path.drive
            or ".." in path.parts
            or "/" in cleaned
            or "\\" in cleaned
            or ":" in cleaned
        ):
            raise BridgeError("invalid_skill", "Skill name must be an active Hermes Skill name")
        names = {str(item["name"]) for item in self.list()["skills"]}
        if cleaned not in names:
            raise BridgeError(
                "skill_unavailable",
                "Skill is not in the current active Hermes catalog",
                {"name": cleaned},
            )
        return cleaned

    @staticmethod
    def _validate_resource_path(resource_path: str) -> str:
        if not isinstance(resource_path, str) or not resource_path.strip() or len(resource_path) > 512:
            raise BridgeError("invalid_resource", "Skill resource path is invalid")
        raw = resource_path.strip().replace("\\", "/")
        posix_path = PurePosixPath(raw)
        windows_path = PureWindowsPath(resource_path)
        if (
            posix_path.is_absolute()
            or windows_path.is_absolute()
            or windows_path.drive
            or ".." in posix_path.parts
            or not posix_path.parts
            or posix_path.parts[0] not in _RESOURCE_ROOTS
        ):
            raise BridgeError(
                "invalid_resource",
                "Resource must be a relative file below references, templates, scripts, or assets",
            )
        for component in posix_path.parts:
            lowered = component.casefold()
            if component.startswith(".") or _SENSITIVE_COMPONENT.search(lowered):
                raise BridgeError("sensitive_resource", "Sensitive Skill resources cannot be returned")
        if posix_path.suffix.casefold() in _SENSITIVE_SUFFIXES:
            raise BridgeError("sensitive_resource", "Sensitive Skill resources cannot be returned")
        return str(posix_path)

    def load(self, name: str) -> dict[str, Any]:
        active_name = self._validate_active_name(name)
        result = self._invoke_worker({"operation": "load", "name": active_name})
        if not result.get("success"):
            raise BridgeError("skill_unavailable", "Hermes could not load the requested Skill")
        return {"ok": True, "skill": result}

    def resource(self, name: str, resource_path: str) -> dict[str, Any]:
        active_name = self._validate_active_name(name)
        safe_path = self._validate_resource_path(resource_path)
        result = self._invoke_worker(
            {"operation": "resource", "name": active_name, "resource_path": safe_path}
        )
        if not result.get("success"):
            raise BridgeError("resource_unavailable", "Hermes could not load the requested Skill resource")
        return {"ok": True, "resource": result}
