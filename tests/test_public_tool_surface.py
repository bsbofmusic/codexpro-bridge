from __future__ import annotations

import pytest

from codexpro_bridge.capabilities import CapabilityRegistry, ModuleDescriptor
from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.server.app import build_server


def _manifest_tools(capabilities) -> set[str]:
    return {
        tool
        for module in capabilities.registry.manifest()
        if module["enabled"]
        for tool in module["tools"]
    }


def test_public_tool_surface_is_manifest_driven_and_route_first() -> None:
    server, capabilities = build_server(BridgeConfig(allow_anonymous=True))
    try:
        tools = server._tool_manager._tools
        expected = _manifest_tools(capabilities)
        assert set(tools) == expected
        assert capabilities.registry.surface_audit()["consistent"] is True
        assert capabilities.registry.surface_audit()["tool_count"] == len(expected)
        assert len(server.instructions) <= 512
        assert server.instructions.startswith("For non-trivial work, call codexpro_bridge_route_and_recall first.")

        for name, tool in tools.items():
            annotation = tool.annotations
            if name in {"codexpro_bridge_mcp_call", "codexpro_bridge_memory_call", "codexpro_bridge_work"}:
                assert annotation.read_only_hint is False
                assert annotation.destructive_hint is True
                assert annotation.idempotent_hint is False
            else:
                assert annotation.read_only_hint is True
                assert annotation.destructive_hint is False
                assert annotation.idempotent_hint is True
    finally:
        capabilities.shutdown()


def test_module_allowlist_hot_plugs_surface_without_residue() -> None:
    config = BridgeConfig(
        allow_anonymous=True,
        enabled_modules=("shared_skills", "bridge_doctor"),
    )
    server, capabilities = build_server(config)
    try:
        names = set(server._tool_manager._tools)
        expected = _manifest_tools(capabilities)
        audit = capabilities.registry.surface_audit()
        assert names == expected
        assert audit["consistent"] is True
        assert audit["disabled_modules"] == ["shared_mcp", "shared_memory", "work_runtime"]
        assert "codexpro_bridge_route_and_recall" in names
        assert "codexpro_bridge_doctor" in names
        assert "codexpro_bridge_mcp_call" not in names
        assert "codexpro_bridge_memory_call" not in names
        assert "codexpro_bridge_work" not in names
    finally:
        capabilities.shutdown()


def test_unknown_module_selector_fails_closed() -> None:
    with pytest.raises(BridgeError) as exc:
        build_server(BridgeConfig(allow_anonymous=True, enabled_modules=("shared_skills", "typo_module")))
    assert exc.value.code == "unknown_module"


def test_module_selector_loads_from_environment(monkeypatch) -> None:
    monkeypatch.setenv("CODEXPRO_BRIDGE_ALLOW_ANONYMOUS", "1")
    monkeypatch.setenv("CODEXPRO_BRIDGE_MODULES", "shared_skills,bridge_doctor,shared_skills")
    config = BridgeConfig.from_env()
    assert config.enabled_modules == ("shared_skills", "bridge_doctor")
    server, capabilities = build_server(config)
    try:
        assert set(server._tool_manager._tools) == _manifest_tools(capabilities)
        assert capabilities.registry.enabled_module_ids() == ("shared_skills", "bridge_doctor")
    finally:
        capabilities.shutdown()


def test_registry_rejects_duplicate_module_and_tool_names() -> None:
    noop = lambda: {"ok": True}
    install = lambda server: None
    registry = CapabilityRegistry()
    registry.register(ModuleDescriptor("a", "1", ("tool_a",), noop, install))
    with pytest.raises(BridgeError):
        registry.register(ModuleDescriptor("a", "2", ("tool_b",), noop, install))
    with pytest.raises(BridgeError):
        registry.register(ModuleDescriptor("b", "1", ("tool_a",), noop, install))


def test_registry_rejects_module_installer_residue_and_missing_tools() -> None:
    class DummyToolManager:
        def __init__(self) -> None:
            self._tools: dict[str, object] = {}

    class DummyServer:
        def __init__(self) -> None:
            self._tool_manager = DummyToolManager()

    noop = lambda: {"ok": True}

    def orphan_installer(server) -> None:
        server._tool_manager._tools["declared"] = object()
        server._tool_manager._tools["orphan"] = object()

    registry = CapabilityRegistry()
    registry.register(ModuleDescriptor("bad", "1", ("declared",), noop, orphan_installer))
    with pytest.raises(BridgeError) as exc:
        registry.mount(DummyServer())
    assert exc.value.code == "registry_surface_mismatch"

    def missing_installer(server) -> None:
        return None

    registry = CapabilityRegistry()
    registry.register(ModuleDescriptor("missing", "1", ("declared",), noop, missing_installer))
    with pytest.raises(BridgeError) as exc:
        registry.mount(DummyServer())
    assert exc.value.code == "registry_surface_mismatch"
