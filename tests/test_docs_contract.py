from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DOCS = ROOT / "docs"

REQUIRED_DOCS = {
    "README.md",
    "architecture.md",
    "capability-contract.md",
    "module-surface-governance.md",
    "security-and-secrets.md",
    "operations.md",
    "deployment-and-rollback.md",
    "testing-and-evals.md",
    "known-limitations.md",
    "bridge-v2-blueprint.md",
}


def test_required_docs_exist() -> None:
    assert REQUIRED_DOCS <= {path.name for path in DOCS.glob("*.md")}


def test_docs_index_links_resolve() -> None:
    index = (DOCS / "README.md").read_text(encoding="utf-8")
    for target in re.findall(r"\[[^\]]+\]\(([^)]+)\)", index):
        if "://" in target or target.startswith("#"):
            continue
        assert (DOCS / target).is_file(), target


def test_capability_contract_is_module_derived_not_count_locked() -> None:
    contract = (DOCS / "capability-contract.md").read_text(encoding="utf-8")
    governance = (DOCS / "module-surface-governance.md").read_text(encoding="utf-8")
    assert "module-derived" in contract
    assert "CODEXPRO_BRIDGE_MODULES" in contract
    assert "union(tools declared by enabled modules)" in contract
    assert "Tool count is telemetry, not a contract." in governance
    assert "missing tools = 0" in governance
    assert "orphan tools  = 0" in governance
    assert re.search(r"fixed\s+\d+[- ]tool", contract, flags=re.IGNORECASE) is None


def test_security_contract_fixes_shared_boundaries() -> None:
    security = (DOCS / "security-and-secrets.md").read_text(encoding="utf-8")
    for allowed_root in ("references", "templates", "scripts", "assets", "agents", "prompts", "evals"):
        assert allowed_root in security
    assert "0600" in security
    assert "loopback" in security.casefold()


def test_human_readable_new_project_has_no_retired_runtime_identifiers() -> None:
    retired = re.compile("(?i)" + "her" + "mes[_ -](?:runtime|home|data)|/her" + "mes/")
    candidates = [ROOT / "README.md", ROOT / "TASK.md", ROOT / "CHANGELOG.md", *DOCS.glob("*.md")]
    for path in candidates:
        assert retired.search(path.read_text(encoding="utf-8")) is None, path
