"""Read-only runtime over the Skills Manager central library."""

from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path, PurePosixPath, PureWindowsPath
from typing import Any

import yaml

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError

_RESOURCE_ROOTS = frozenset({"references", "templates", "scripts", "assets", "agents", "prompts", "evals"})
_SENSITIVE_COMPONENT = re.compile(
    r"(?:^|[_-])(?:env|secret|secrets|credential|credentials|password|passwd|private[_-]?key|ssh)(?:$|[_-])",
    re.IGNORECASE,
)
_SENSITIVE_SUFFIXES = frozenset({".pem", ".key", ".p12", ".pfx", ".kdbx"})


def _sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def _frontmatter(text: str) -> dict[str, Any]:
    if not text.startswith("---"):
        return {}
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}
    try:
        end = next(i for i in range(1, len(lines)) if lines[i].strip() == "---")
    except StopIteration:
        return {}
    try:
        parsed = yaml.safe_load("\n".join(lines[1:end])) or {}
    except yaml.YAMLError:
        return {}
    return parsed if isinstance(parsed, dict) else {}


def _tags(metadata: dict[str, Any]) -> list[str]:
    value = metadata.get("tags")
    return [str(item) for item in value] if isinstance(value, list) else []


class SharedSkillsRuntime:
    """Use Skills Manager metadata as membership authority and files as content authority."""

    def __init__(self, config: BridgeConfig):
        self.config = config

    @property
    def root(self) -> Path:
        return self.config.skills_root

    @property
    def metadata_root(self) -> Path:
        return self.root / ".skills-manager" / "skills"

    def _safe_skill_dir(self, relative: str) -> Path:
        posix = PurePosixPath(relative.replace("\\", "/"))
        windows = PureWindowsPath(relative)
        if posix.is_absolute() or windows.is_absolute() or windows.drive or ".." in posix.parts or not posix.parts:
            raise BridgeError("skills_unavailable", "Skills Manager metadata contains an invalid path")
        root = self.root.resolve()
        candidate = (root / Path(*posix.parts)).resolve()
        if candidate != root and root not in candidate.parents:
            raise BridgeError("skills_unavailable", "Skill path escapes the shared Skill root")
        return candidate

    def _manager_records(self) -> list[dict[str, Any]]:
        if not self.root.is_dir() or not self.metadata_root.is_dir():
            raise BridgeError("skills_unavailable", "Skills Manager central library is unavailable")
        records: list[dict[str, Any]] = []
        for metadata_file in sorted(self.metadata_root.glob("*.json")):
            try:
                raw = json.loads(metadata_file.read_text(encoding="utf-8"))
            except (OSError, UnicodeError, json.JSONDecodeError) as exc:
                raise BridgeError("skills_unavailable", "Skills Manager metadata is unreadable") from exc
            if not isinstance(raw, dict) or not raw.get("enabled", True):
                continue
            relative = raw.get("path")
            if not isinstance(relative, str) or not relative.strip():
                continue
            skill_dir = self._safe_skill_dir(relative.strip())
            skill_file = skill_dir / "SKILL.md"
            if not skill_file.is_file():
                continue
            try:
                text = skill_file.read_text(encoding="utf-8")
            except (OSError, UnicodeError) as exc:
                raise BridgeError("skills_unavailable", "A managed SKILL.md is unreadable") from exc
            metadata = _frontmatter(text)
            name = str(metadata.get("name") or skill_dir.name).strip()
            if not name:
                continue
            records.append(
                {
                    "id": str(raw.get("skill_id") or metadata_file.stem),
                    "name": name,
                    "description": str(metadata.get("description") or ""),
                    "version": metadata.get("version"),
                    "tags": _tags(metadata),
                    "category": metadata.get("category"),
                    "path": str(skill_dir.relative_to(self.root.resolve())),
                    "sha256": _sha256(skill_file),
                    "_file": skill_file,
                    "_dir": skill_dir,
                }
            )
        records.sort(key=lambda item: item["name"].casefold())
        return records

    @staticmethod
    def _public(record: dict[str, Any]) -> dict[str, Any]:
        return {key: value for key, value in record.items() if not key.startswith("_") and value is not None}

    def list(self, category: str | None = None) -> dict[str, Any]:
        if category is not None and (not isinstance(category, str) or len(category) > 128):
            raise BridgeError("invalid_category", "Skill category is invalid")
        records = self._manager_records()
        if category is not None:
            records = [item for item in records if item.get("category") == category]
        skills = [self._public(item) for item in records]
        categories = sorted({str(item["category"]) for item in records if item.get("category")})
        return {"ok": True, "skills": skills, "categories": categories, "count": len(skills), "source": "skills-manager"}

    def _record(self, name: str) -> dict[str, Any]:
        if not isinstance(name, str) or not name.strip() or len(name) > 128:
            raise BridgeError("invalid_skill", "Skill name is invalid")
        cleaned = name.strip()
        path = PurePosixPath(cleaned)
        windows = PureWindowsPath(cleaned)
        if path.is_absolute() or windows.is_absolute() or windows.drive or ".." in path.parts or "/" in cleaned or "\\" in cleaned:
            raise BridgeError("invalid_skill", "Skill name must be a managed Skill name")
        for record in self._manager_records():
            if record["name"] == cleaned:
                return record
        raise BridgeError("skill_unavailable", "Skill is not enabled in Skills Manager", {"name": cleaned})

    @staticmethod
    def _validate_resource_path(resource_path: str) -> str:
        if not isinstance(resource_path, str) or not resource_path.strip() or len(resource_path) > 512:
            raise BridgeError("invalid_resource", "Skill resource path is invalid")
        raw = resource_path.strip().replace("\\", "/")
        posix = PurePosixPath(raw)
        windows = PureWindowsPath(resource_path)
        if (
            posix.is_absolute()
            or windows.is_absolute()
            or windows.drive
            or ".." in posix.parts
            or not posix.parts
            or posix.parts[0] not in _RESOURCE_ROOTS
        ):
            raise BridgeError("invalid_resource", "Resource must stay below an allowed Skill resource directory")
        for component in posix.parts:
            if component.startswith(".") or _SENSITIVE_COMPONENT.search(component.casefold()):
                raise BridgeError("sensitive_resource", "Sensitive Skill resources cannot be returned")
        if posix.suffix.casefold() in _SENSITIVE_SUFFIXES:
            raise BridgeError("sensitive_resource", "Sensitive Skill resources cannot be returned")
        return str(posix)

    def load(self, name: str) -> dict[str, Any]:
        record = self._record(name)
        path: Path = record["_file"]
        text = path.read_text(encoding="utf-8")
        return {
            "ok": True,
            "skill": {
                **self._public(record),
                "text": text,
                "bytes": len(text.encode("utf-8")),
            },
        }

    def resource(self, name: str, resource_path: str) -> dict[str, Any]:
        record = self._record(name)
        safe_path = self._validate_resource_path(resource_path)
        skill_dir: Path = record["_dir"]
        root = skill_dir.resolve()
        target = (root / safe_path).resolve()
        if root not in target.parents or not target.is_file():
            raise BridgeError("resource_unavailable", "Requested Skill resource is unavailable")
        try:
            text = target.read_text(encoding="utf-8")
        except (OSError, UnicodeError) as exc:
            raise BridgeError("resource_unavailable", "Requested Skill resource is not readable text") from exc
        return {
            "ok": True,
            "resource": {
                "skill": record["name"],
                "path": safe_path,
                "text": text,
                "bytes": len(text.encode("utf-8")),
                "sha256": _sha256(target),
            },
        }
