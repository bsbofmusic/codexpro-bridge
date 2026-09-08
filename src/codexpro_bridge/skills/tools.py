"""Public Skill-tool behavior over the Skills Manager central library."""

from __future__ import annotations

import re
from typing import Any

from codexpro_bridge.core.errors import BridgeError

from .runtime import SharedSkillsRuntime

_TOKEN = re.compile(r"[a-z0-9][a-z0-9_-]{1,}|[\u4e00-\u9fff]{2,}", re.IGNORECASE)


class SkillTools:
    """Expose one managed Skill catalog with progressive disclosure."""

    def __init__(self, runtime: SharedSkillsRuntime):
        self.runtime = runtime

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
                or any(needle in str(tag).casefold() for tag in item.get("tags", []) if isinstance(tag, str))
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
            "source": catalog["source"],
        }

    def load_skill(self, name: str) -> dict[str, Any]:
        return self.runtime.load(name)

    def load_skill_resource(self, name: str, resource_path: str) -> dict[str, Any]:
        return self.runtime.resource(name, resource_path)

    def route(self, task: str, *, limit: int = 8) -> dict[str, Any]:
        if not isinstance(task, str) or not task.strip() or len(task) > 8_000:
            raise BridgeError("invalid_task", "task must be a non-empty string up to 8000 characters")
        if not isinstance(limit, int) or not 1 <= limit <= 20:
            raise BridgeError("invalid_limit", "route limit must be between 1 and 20")

        catalog = self.runtime.list()
        terms = {token.casefold() for token in _TOKEN.findall(task) if len(token) >= 2}
        ranked: list[tuple[int, str, dict[str, Any]]] = []
        task_folded = task.casefold()
        for item in catalog["skills"]:
            name = str(item.get("name", ""))
            description = str(item.get("description", ""))
            tags = " ".join(str(tag) for tag in item.get("tags", []) if isinstance(tag, str))
            haystack = f"{name} {description} {tags}".casefold()
            score = 0
            if name and name.casefold() in task_folded:
                score += 100
            for term in terms:
                if term in name.casefold():
                    score += 12
                elif term in haystack:
                    score += 2
            if score:
                ranked.append((score, name.casefold(), item))

        ranked.sort(key=lambda row: (-row[0], row[1]))
        matches = [row[2] for row in ranked[:limit]]
        return {
            "ok": True,
            "task": task,
            "routing": "Choose a matching managed Skill, then load its SKILL.md before acting.",
            "skills": matches,
            "match_count": len(matches),
            "catalog_count": catalog["count"],
            "source": catalog["source"],
        }
