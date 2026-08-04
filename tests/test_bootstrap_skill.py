from __future__ import annotations

from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
SKILL_ROOT = ROOT / "plugin" / "codexpro-bridge" / "skills" / "codexpro-bridge-router"


def _frontmatter(text: str) -> dict[str, object]:
    _, raw, _ = text.split("---", 2)
    loaded = yaml.safe_load(raw)
    assert isinstance(loaded, dict)
    return loaded


def test_bootstrap_skill_is_the_only_bundled_skill_and_front_loads_routing() -> None:
    skills = [path for path in SKILL_ROOT.parent.iterdir() if path.is_dir()]
    text = (SKILL_ROOT / "SKILL.md").read_text(encoding="utf-8")
    normalized = " ".join(text.split())
    metadata = _frontmatter(text)

    assert [path.name for path in skills] == ["codexpro-bridge-router"]
    assert metadata["name"] == "codexpro-bridge-router"
    description = str(metadata["description"])
    assert description.startswith("Use for every non-trivial task")
    assert "ordinary chat" in description
    assert "codexpro_bridge_route_and_recall" in text
    assert "codexpro_bridge_load_skill" in text
    assert (
        "existing CodexPro connection for VPS files, Bash, Git, and writes"
        in normalized
    )
    assert "do not route or recall memory" in normalized.casefold()
    assert "do not use codexpro bridge to edit files" in normalized.casefold()


def test_bootstrap_skill_keeps_implicit_invocation_enabled() -> None:
    metadata = yaml.safe_load((SKILL_ROOT / "agents" / "openai.yaml").read_text())

    assert metadata["policy"].get("allow_implicit_invocation") is True
