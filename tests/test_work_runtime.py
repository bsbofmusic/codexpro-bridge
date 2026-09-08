from __future__ import annotations

from pathlib import Path

import pytest

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.server.app import build_server
from codexpro_bridge.work import WORK_OPERATIONS, WorkRuntime


@pytest.fixture()
def runtime(tmp_path: Path) -> WorkRuntime:
    return WorkRuntime(BridgeConfig(allow_anonymous=True, work_db_path=tmp_path / "work.sqlite3"))


def test_sqlite_schema_is_local_simple_and_foreign_keys_enabled(runtime: WorkRuntime) -> None:
    with runtime.store.connect() as conn:
        assert conn.execute("PRAGMA user_version").fetchone()[0] == 1
        assert conn.execute("PRAGMA foreign_keys").fetchone()[0] == 1
        assert str(conn.execute("PRAGMA journal_mode").fetchone()[0]).lower() == "delete"
        tables = {
            row[0]
            for row in conn.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()
            if not str(row[0]).startswith("sqlite_")
        }
        assert tables == {"projects", "tasks", "task_steps", "checkpoints", "audits", "audit_checks", "artifacts"}


def test_project_context_is_reference_only(runtime: WorkRuntime) -> None:
    project = runtime.project_create(
        name="Bridge 2.1",
        workspace_ref="/opt/gpt-workspace/codexpro-bridge",
        relevant_skills=["codexpro-bridge-operations"],
        relevant_mcp=["agentgateway"],
        memory_queries=["Bridge 2.1 decisions"],
        project_rules=["Do not copy Skill bodies"],
        known_risks=["connector schema cache"],
    )["project"]
    context = runtime.project_context(project["project_id"])
    assert context["project"]["workspace_ref"] == "/opt/gpt-workspace/codexpro-bridge"
    assert context["project"]["relevant_skills"] == ["codexpro-bridge-operations"]
    assert "workspace_action" in context
    assert "SKILL.md" not in str(context)


def test_task_state_machine_steps_and_revision(runtime: WorkRuntime) -> None:
    task = runtime.task_create(title="Upgrade", audit_required=True)["task"]
    assert task["status"] == "pending"
    assert task["revision"] == 1

    step = runtime.task_step_add(task_id=task["task_id"], title="Implement")["step"]
    task_after_add = runtime.task_get(task["task_id"])["task"]
    assert task_after_add["revision"] == 2

    runtime.task_transition(task["task_id"], "running")
    with pytest.raises(BridgeError, match="cannot transition"):
        runtime.task_transition(task["task_id"], "done")

    runtime.task_step_update(step["step_id"], status="doing")
    runtime.task_step_update(step["step_id"], status="done")
    task_after_steps = runtime.task_get(task["task_id"])["task"]
    assert task_after_steps["revision"] == 4
    runtime.task_transition(task["task_id"], "audit")


def test_checkpoint_history_is_immutable_and_resume_is_state_only(runtime: WorkRuntime) -> None:
    project = runtime.project_create(name="P", workspace_ref="/x")["project"]
    task = runtime.task_create(title="Resume me", project_id=project["project_id"])["task"]
    runtime.task_transition(task["task_id"], "running")
    step = runtime.task_step_add(task["task_id"], "S1")["step"]
    runtime.task_step_update(step["step_id"], status="done")

    cp1 = runtime.checkpoint_create(task["task_id"], decisions=["A"], next_action="Do S2")["checkpoint"]
    cp2 = runtime.checkpoint_create(task["task_id"], decisions=["B"], next_action="Do S3")["checkpoint"]
    assert cp1["sequence"] == 1
    assert cp2["sequence"] == 2
    assert cp2["parent_checkpoint_id"] == cp1["checkpoint_id"]

    history = runtime.checkpoint_list(task["task_id"])["checkpoints"]
    assert [item["sequence"] for item in history] == [2, 1]
    assert history[-1]["decisions"] == ["A"]

    resumed = runtime.task_resume(task_id=task["task_id"])
    assert resumed["latest_checkpoint"]["checkpoint_id"] == cp2["checkpoint_id"]
    assert resumed["next_action"] == "Do S3"
    assert resumed["mutation_replayed"] is False
    assert resumed["resume_semantics"] == "state_only"


def test_ambiguous_resume_fails_closed(runtime: WorkRuntime) -> None:
    runtime.task_create(title="Same project A")
    runtime.task_create(title="Same project B")
    result = runtime.task_resume()
    assert result["ok"] is False
    assert result["error"]["code"] == "ambiguous_task"
    assert len(result["candidates"]) == 2
    assert result["mutation_replayed"] is False


def test_audit_is_deterministic_manual_evidence_is_not_mechanical(runtime: WorkRuntime) -> None:
    task = runtime.task_create(title="Audit", audit_required=True)["task"]
    runtime.task_transition(task["task_id"], "running")
    runtime.task_transition(task["task_id"], "audit")
    audit = runtime.audit_create(task["task_id"], checks=[{"name": "tests", "expected": "PASS"}])
    check_id = audit["checks"][0]["check_id"]
    runtime.audit_record(audit["audit"]["audit_id"], check_id, actual="PASS", evidence_type="manual", evidence_ref="user-said-so")
    evaluated = runtime.audit_evaluate(audit["audit"]["audit_id"])
    assert evaluated["audit"]["status"] == "UNCERTAIN"
    assert evaluated["checks"][0]["status"] == "UNCERTAIN"
    with pytest.raises(BridgeError, match="PASS audit"):
        runtime.task_transition(task["task_id"], "done")


def test_verified_audit_pass_unlocks_done_but_stale_pass_does_not(runtime: WorkRuntime) -> None:
    task = runtime.task_create(title="Audit gate", audit_required=True)["task"]
    runtime.task_transition(task["task_id"], "running")
    runtime.task_transition(task["task_id"], "audit")
    audit = runtime.audit_create(task["task_id"], checks=[{"name": "pytest", "expected": {"exit": 0}}])
    check_id = audit["checks"][0]["check_id"]
    runtime.audit_record(
        audit["audit"]["audit_id"],
        check_id,
        actual={"exit": 0},
        evidence_type="verified_structured",
        evidence_ref="codexpro:test:123",
        command_or_tool="pytest",
        exit_code=0,
    )
    evaluated = runtime.audit_evaluate(audit["audit"]["audit_id"])
    assert evaluated["audit"]["status"] == "PASS"
    assert evaluated["stale"] is False

    runtime.task_update(task["task_id"], description="material change after audit")
    with pytest.raises(BridgeError, match="current task revision"):
        runtime.task_transition(task["task_id"], "done")

    audit2 = runtime.audit_create(task["task_id"], checks=[{"name": "pytest", "expected": 0}])
    check2 = audit2["checks"][0]["check_id"]
    runtime.audit_record(audit2["audit"]["audit_id"], check2, actual=0, evidence_type="verified_structured", evidence_ref="codexpro:test:124")
    assert runtime.audit_evaluate(audit2["audit"]["audit_id"])["audit"]["status"] == "PASS"
    assert runtime.task_transition(task["task_id"], "done")["task"]["status"] == "done"


def test_artifact_registry_tracks_verification_without_claiming_file_access(runtime: WorkRuntime) -> None:
    task = runtime.task_create(title="Artifact task")["task"]
    with pytest.raises(BridgeError, match="requires verification evidence"):
        runtime.artifact_register(
            name="report",
            path_or_ref="/tmp/report.pdf",
            task_id=task["task_id"],
            verification_status="verified",
        )

    artifact = runtime.artifact_register(
        name="report",
        path_or_ref="/tmp/report.pdf",
        task_id=task["task_id"],
        sha256="abc",
        size=123,
        verification_status="verified",
        verification_evidence_ref="codexpro:file-hash:1",
    )["artifact"]
    assert artifact["status"] == "draft"
    assert artifact["verification_status"] == "verified"
    assert runtime.artifact_finalize(artifact["artifact_id"])["artifact"]["status"] == "final"
    assert runtime.artifact_archive(artifact["artifact_id"])["artifact"]["status"] == "archived"


def test_task_summary_aggregates_progress_audit_checkpoint_and_artifacts(runtime: WorkRuntime) -> None:
    task = runtime.task_create(title="Summary")["task"]
    step = runtime.task_step_add(task["task_id"], "One")["step"]
    runtime.task_step_update(step["step_id"], status="done")
    runtime.checkpoint_create(task["task_id"], next_action="Ship")
    runtime.artifact_register(name="doc", path_or_ref="ref:1", task_id=task["task_id"])
    summary = runtime.task_summary(task["task_id"])
    assert summary["progress"]["total_steps"] == 1
    assert summary["progress"]["by_status"]["done"] == 1
    assert summary["latest_checkpoint"] is not None
    assert len(summary["artifacts"]) == 1
    assert summary["next_action"] == "Ship"


def test_work_db_failure_is_isolated_from_original_bridge_surface() -> None:
    config = BridgeConfig(allow_anonymous=True, work_db_path=Path("/proc/codexpro-bridge/work.sqlite3"))
    server, capabilities = build_server(config)
    try:
        names = set(server._tool_manager._tools)
        manifest_tools = {
            tool
            for module in capabilities.registry.manifest()
            if module["enabled"]
            for tool in module["tools"]
        }
        assert names == manifest_tools
        assert "codexpro_bridge_work" in names
        assert capabilities.registry.surface_audit()["consistent"] is True
        assert capabilities.work.health()["ok"] is False
        original = names - {"codexpro_bridge_work"}
        assert "codexpro_bridge_route_and_recall" in original
        assert "codexpro_bridge_mcp_call" in original
        assert "codexpro_bridge_memory_call" in original
    finally:
        capabilities.shutdown()


def test_work_public_schema_uses_closed_operation_enum(tmp_path: Path) -> None:
    server, capabilities = build_server(BridgeConfig(allow_anonymous=True, work_db_path=tmp_path / "schema.sqlite3"))
    try:
        schema = server._tool_manager._tools["codexpro_bridge_work"].parameters
        enum = schema["properties"]["operation"]["enum"]
        assert tuple(enum) == WORK_OPERATIONS
        assert schema["required"] == ["operation"]
    finally:
        capabilities.shutdown()
