from __future__ import annotations

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.server.app import build_server


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


def test_public_tool_surface_is_fixed_and_route_first_without_starting_workers() -> None:
    server, capabilities = build_server(BridgeConfig(allow_anonymous=True))
    try:
        tools = server._tool_manager._tools

        assert set(tools) == EXPECTED_TOOLS
        assert len(server.instructions) <= 512
        assert server.instructions.startswith(
            "For every non-trivial task, first call codexpro_bridge_route_and_recall."
        )

        for name, tool in tools.items():
            annotation = tool.annotations
            if name == "codexpro_bridge_hermes_mcp_call":
                assert annotation.readOnlyHint is False
                assert annotation.destructiveHint is True
                assert annotation.idempotentHint is False
            else:
                assert annotation.readOnlyHint is True
                assert annotation.destructiveHint is False
                assert annotation.idempotentHint is True

        registered = {
            tool
            for module in capabilities.registry.manifest()
            for tool in module["tools"]
        }
        assert registered == EXPECTED_TOOLS
    finally:
        capabilities.shutdown()
