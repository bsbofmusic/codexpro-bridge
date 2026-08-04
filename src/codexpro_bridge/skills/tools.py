"""Public Skill-tool behavior, independent from the HTTP/MCP transport."""

from __future__ import annotations

from typing import Any, Protocol

from codexpro_bridge.core.errors import BridgeError

from .runtime import HermesSkillsRuntime


class MemoryRecall(Protocol):
    def recall(self, task: str) -> dict[str, Any]: ...


class SkillTools:
    """Expose a live catalog and progressive disclosure without reranking it."""

    def __init__(self, runtime: HermesSkillsRuntime, memory: MemoryRecall | None = None):
        self.runtime = runtime
        self.memory = memory

    def skills_list(
        self,
        *,
        category: str | None = None,
        query: str | None = None,
        offset: int = 0,
        limit: int = 50,
    ) -> dict[str, Any]:
        if not isinstance(offset, int) or offset < 0:
            raise BridgeError("invalid_pagination", "offset must be a non-negative integer")
        if not isinstance(limit, int) or not 1 <= limit <= 100:
            raise BridgeError("invalid_pagination", "limit must be between 1 and 100")
        if query is not None and (not isinstance(query, str) or len(query) > 256):
            raise BridgeError("invalid_query", "Skill query is invalid")
        catalog = self.runtime.list(category=category)
        skills = catalog["skills"]
        if query and query.strip():
            needle = query.casefold().strip()
            skills = [
                item
                for item in skills
                if needle in str(item.get("name", "")).casefold()
                or needle in str(item.get("description", "")).casefold()
                or needle in str(item.get("category", "")).casefold()
            ]
        page = skills[offset : offset + limit]
        return {
            "ok": True,
            "skills": page,
            "count": len(skills),
            "offset": offset,
            "limit": limit,
            "next_offset": offset + len(page) if offset + len(page) < len(skills) else None,
            "categories": catalog["categories"],
        }

    def load_skill(self, name: str) -> dict[str, Any]:
        return self.runtime.load(name)

    def load_skill_resource(self, name: str, resource_path: str) -> dict[str, Any]:
        return self.runtime.resource(name, resource_path)

    def route_and_recall(self, task: str, *, include_memory: bool = True) -> dict[str, Any]:
        if not isinstance(task, str) or not task.strip() or len(task) > 8_000:
            raise BridgeError("invalid_task", "task must be a non-empty string up to 8000 characters")
        catalog = self.runtime.list()
        memory: dict[str, Any] = {
            "requested": bool(include_memory),
            "attempted": False,
            "reason": "no_memory_runtime" if self.memory is None else "no_strong_memory_signal",
        }
        if include_memory and self.memory is not None:
            memory = self.memory.recall(task)
        return {
            "ok": True,
            "task": task,
            "routing": "Select a Skill from the live catalog, then load its real SKILL.md before acting.",
            "skills": catalog["skills"],
            "skill_count": catalog["count"],
            "categories": catalog["categories"],
            "memory": memory,
        }
