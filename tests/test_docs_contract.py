from __future__ import annotations

import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DOCS = ROOT / "docs"

REQUIRED_DOCS = {
    "architecture.md",
    "configuration.md",
    "capability-contract.md",
    "plugin.md",
    "development.md",
    "security.md",
    "known-limitations.md",
}

EXPECTED_TOOLS = {
    "codexpro_bridge_route_and_recall",
    "codexpro_bridge_skills_list",
    "codexpro_bridge_load_skill",
    "codexpro_bridge_load_skill_resource",
    "codexpro_bridge_hermes_mcp_list",
    "codexpro_bridge_hermes_mcp_status",
    "codexpro_bridge_hermes_mcp_call",
    "codexpro_bridge_doctor",
}

SECRET_VALUE_PATTERNS = (
    re.compile(r"(?i)bearer\s+[A-Za-z0-9._~+/=-]{12,}"),
    re.compile(r"(?i)(?:token|password|api[_-]?key|secret)\s*[=:]\s*[^<\s][^\s]{7,}"),
    re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),
    re.compile(r"(?i)codexpro_token=[^<\s]+"),
)


def _markdown_links(text: str) -> list[str]:
    return re.findall(r"\[[^\]]+\]\(([^)]+)\)", text)


def test_required_public_documents_exist() -> None:
    assert REQUIRED_DOCS <= {path.name for path in DOCS.glob("*.md")}


def test_documentation_index_links_resolve_locally() -> None:
    index = (DOCS / "README.md").read_text(encoding="utf-8")
    for target in _markdown_links(index):
        if "://" in target or target.startswith("#"):
            continue
        assert (DOCS / target).is_file(), target


def test_capability_contract_names_exactly_eight_public_tools() -> None:
    contract = (DOCS / "capability-contract.md").read_text(encoding="utf-8")
    documented = set(re.findall(r"\b(codexpro_bridge_[a-z0-9_]+)\b", contract))
    assert documented == EXPECTED_TOOLS


def test_public_docs_fix_the_core_safety_boundaries() -> None:
    security = (DOCS / "security.md").read_text(encoding="utf-8")
    contract = (DOCS / "capability-contract.md").read_text(encoding="utf-8")

    for allowed_root in ("references", "scripts", "templates", "assets"):
        assert allowed_root in security
    assert "hindsight" in contract.casefold()
    assert "sampling.enabled=false" in security
    assert "elicitation.enabled=false" in security
    assert "delivery_unknown" in contract


def test_human_readable_public_files_contain_no_secret_values() -> None:
    paths = [
        ROOT / "README.md",
        ROOT / "CHANGELOG.md",
        ROOT / "SECURITY.md",
        *(DOCS.glob("*.md")),
    ]
    for path in paths:
        text = path.read_text(encoding="utf-8")
        for pattern in SECRET_VALUE_PATTERNS:
            assert pattern.search(text) is None, f"secret-shaped value in {path}: {pattern.pattern}"
