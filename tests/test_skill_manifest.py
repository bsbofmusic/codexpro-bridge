from __future__ import annotations

import json
from pathlib import Path

import pytest

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.skills.runtime import SharedSkillsRuntime
from codexpro_bridge.skills.tools import SkillTools


def _managed_skill(
    root: Path,
    *,
    skill_id: str,
    path: str,
    name: str,
    description: str,
    enabled: bool = True,
    category: str | None = None,
) -> Path:
    skill_dir = root / path
    skill_dir.mkdir(parents=True, exist_ok=True)
    category_line = f"category: {category}\n" if category else ""
    (skill_dir / "SKILL.md").write_text(
        "---\n"
        f"name: {name}\n"
        f"description: {description!r}\n"
        f"{category_line}"
        "tags: [shared, test]\n"
        "---\n\n"
        f"# {name}\n",
        encoding="utf-8",
    )
    metadata = root / ".skills-manager" / "skills"
    metadata.mkdir(parents=True, exist_ok=True)
    (metadata / f"{skill_id}.json").write_text(
        json.dumps(
            {
                "schema_version": 1,
                "skill_id": skill_id,
                "path": path,
                "path_key": path,
                "enabled": enabled,
                "tags": [],
                "source": {"type": "local", "ref": None, "subpath": None, "branch": None},
            }
        ),
        encoding="utf-8",
    )
    return skill_dir


def _runtime(root: Path) -> SharedSkillsRuntime:
    return SharedSkillsRuntime(BridgeConfig(allow_anonymous=True, skills_root=root))


def test_manager_metadata_is_membership_authority(tmp_path: Path) -> None:
    root = tmp_path / "skills"
    _managed_skill(
        root,
        skill_id="one",
        path="alpha",
        name="alpha",
        description="Use for alpha deployment work.",
        category="ops",
    )
    _managed_skill(
        root,
        skill_id="two",
        path="beta",
        name="beta",
        description="Disabled beta skill.",
        enabled=False,
    )

    result = _runtime(root).list()

    assert result["source"] == "skills-manager"
    assert result["count"] == 1
    assert [item["name"] for item in result["skills"]] == ["alpha"]
    assert result["skills"][0]["category"] == "ops"
    assert result["skills"][0]["tags"] == ["shared", "test"]
    assert result["categories"] == ["ops"]


def test_load_returns_canonical_skill_file_and_hash(tmp_path: Path) -> None:
    root = tmp_path / "skills"
    skill_dir = _managed_skill(
        root,
        skill_id="one",
        path="alpha",
        name="alpha",
        description="Use for alpha deployment work.",
    )

    result = _runtime(root).load("alpha")

    assert result["ok"] is True
    assert result["skill"]["name"] == "alpha"
    assert result["skill"]["path"] == "alpha"
    assert result["skill"]["text"] == (skill_dir / "SKILL.md").read_text(encoding="utf-8")
    assert len(result["skill"]["sha256"]) == 64


def test_resource_is_bounded_to_allowed_skill_directories(tmp_path: Path) -> None:
    root = tmp_path / "skills"
    skill_dir = _managed_skill(
        root,
        skill_id="one",
        path="alpha",
        name="alpha",
        description="Use for alpha deployment work.",
    )
    (skill_dir / "references").mkdir()
    (skill_dir / "references" / "guide.md").write_text("guide", encoding="utf-8")

    result = _runtime(root).resource("alpha", "references/guide.md")
    assert result["resource"]["text"] == "guide"

    for invalid in ("../escape", "/etc/passwd", "notes/guide.md", "references/.env", "references/private-key.pem"):
        with pytest.raises(BridgeError):
            _runtime(root).resource("alpha", invalid)


def test_manager_path_escape_is_rejected(tmp_path: Path) -> None:
    root = tmp_path / "skills"
    metadata = root / ".skills-manager" / "skills"
    metadata.mkdir(parents=True)
    (metadata / "bad.json").write_text(
        json.dumps({"skill_id": "bad", "path": "../escape", "enabled": True}),
        encoding="utf-8",
    )

    with pytest.raises(BridgeError) as caught:
        _runtime(root).list()
    assert caught.value.code == "skills_unavailable"


def test_route_returns_bounded_relevant_matches(tmp_path: Path) -> None:
    root = tmp_path / "skills"
    _managed_skill(
        root,
        skill_id="one",
        path="patent-search",
        name="patent-search",
        description="Search and audit patent claims.",
    )
    _managed_skill(
        root,
        skill_id="two",
        path="copywriting",
        name="copywriting",
        description="Write marketing copy.",
    )

    routed = SkillTools(_runtime(root)).route("Please search a patent claim", limit=1)

    assert routed["source"] == "skills-manager"
    assert routed["catalog_count"] == 2
    assert routed["skills"][0]["name"] == "patent-search"
