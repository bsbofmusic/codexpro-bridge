"""Built-in Bridge tool modules.

Each module owns its public tool declarations and installer.  The server core
never owns a fixed global tool count; it mounts the enabled module catalog and
audits the resulting surface mechanically.
"""

from __future__ import annotations

import logging
from typing import Any, Callable, Literal

from mcp.types import ToolAnnotations

from codexpro_bridge.capabilities import ModuleDescriptor
from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.core.redaction import bounded_result, redact_value

LOG = logging.getLogger("codexpro_bridge")

READ_ONLY = ToolAnnotations(
    readOnlyHint=True,
    destructiveHint=False,
    idempotentHint=True,
    openWorldHint=False,
)
UPSTREAM_DISPATCH = ToolAnnotations(
    readOnlyHint=False,
    destructiveHint=True,
    idempotentHint=False,
    openWorldHint=True,
)
STATE_DISPATCH = ToolAnnotations(
    readOnlyHint=False,
    destructiveHint=True,
    idempotentHint=False,
    openWorldHint=False,
)

WorkOperation = Literal[
    "task.create", "task.get", "task.list", "task.update", "task.transition",
    "task.step_add", "task.step_update", "task.summary", "task.resume",
    "checkpoint.create", "checkpoint.get", "checkpoint.list",
    "project.create", "project.get", "project.list", "project.context", "project.refresh",
    "audit.create", "audit.record", "audit.evaluate", "audit.get", "audit.list",
    "artifact.register", "artifact.get", "artifact.list", "artifact.finalize", "artifact.archive",
]


def _safe(config: BridgeConfig, operation: Callable[[], dict[str, Any]], *, tool_name: str) -> dict[str, Any]:
    LOG.info("tool_call name=%s", tool_name)
    try:
        return bounded_result(redact_value(operation()), config.max_output_chars)
    except BridgeError as exc:
        return exc.as_dict()
    except Exception:
        LOG.exception("Bridge tool failed")
        return {"ok": False, "error": {"code": "internal_error", "message": "Bridge operation failed"}}


def _skills_module(capabilities: Any, config: BridgeConfig) -> ModuleDescriptor:
    tools = (
        "codexpro_bridge_route_and_recall",
        "codexpro_bridge_route",
        "codexpro_bridge_skills_list",
        "codexpro_bridge_load_skill",
        "codexpro_bridge_load_skill_resource",
    )

    def install(server: Any) -> None:
        @server.tool(
            name="codexpro_bridge_route_and_recall",
            description="Canonical first tool for non-trivial work: route to shared Skills and independently recall Obsidian/MemOS when prior context is requested.",
            annotations=READ_ONLY,
        )
        def route_and_recall(
            task: str,
            include_memory: bool = True,
            conversation_first_message: str | None = None,
            skill_limit: int = 8,
            memory_limit: int = 6,
        ) -> dict[str, Any]:
            return _safe(
                config,
                lambda: capabilities.route_and_recall(
                    task,
                    include_memory=include_memory,
                    conversation_first_message=conversation_first_message,
                    skill_limit=skill_limit,
                    memory_limit=memory_limit,
                ),
                tool_name="codexpro_bridge_route_and_recall",
            )

        @server.tool(
            name="codexpro_bridge_route",
            description="Route a non-trivial task to the most relevant managed Skills. Load the selected SKILL.md before acting.",
            annotations=READ_ONLY,
        )
        def route(task: str, limit: int = 8) -> dict[str, Any]:
            return _safe(config, lambda: capabilities.skills.route(task, limit=limit), tool_name="codexpro_bridge_route")

        @server.tool(
            name="codexpro_bridge_skills_list",
            description="List enabled Skills from the Skills Manager central library.",
            annotations=READ_ONLY,
        )
        def skills_list(category: str | None = None, query: str | None = None, offset: int = 0, limit: int = 50) -> dict[str, Any]:
            return _safe(
                config,
                lambda: capabilities.skills.skills_list(category=category, query=query, offset=offset, limit=limit),
                tool_name="codexpro_bridge_skills_list",
            )

        @server.tool(
            name="codexpro_bridge_load_skill",
            description="Load the canonical SKILL.md for one enabled Skills Manager Skill.",
            annotations=READ_ONLY,
        )
        def load_skill(name: str) -> dict[str, Any]:
            return _safe(config, lambda: capabilities.skills.load_skill(name), tool_name="codexpro_bridge_load_skill")

        @server.tool(
            name="codexpro_bridge_load_skill_resource",
            description="Load an allowed reference, template, script, asset, agent, prompt, or eval file from one managed Skill.",
            annotations=READ_ONLY,
        )
        def load_skill_resource(name: str, resource_path: str) -> dict[str, Any]:
            return _safe(
                config,
                lambda: capabilities.skills.load_skill_resource(name, resource_path),
                tool_name="codexpro_bridge_load_skill_resource",
            )

    return ModuleDescriptor(
        module_id="shared_skills",
        version="1.0.0",
        tools=tools,
        health=capabilities._skills_health,
        install=install,
    )


def _mcp_module(capabilities: Any, config: BridgeConfig) -> ModuleDescriptor:
    tools = ("codexpro_bridge_mcp_list", "codexpro_bridge_mcp_status", "codexpro_bridge_mcp_call")

    def install(server: Any) -> None:
        @server.tool(
            name="codexpro_bridge_mcp_list",
            description="List canonical MCP tools exposed by the local AgentGateway. Tool names are already namespaced by the gateway.",
            annotations=READ_ONLY,
        )
        def mcp_list(query: str | None = None, include_schema: bool = False, offset: int = 0, limit: int = 100) -> dict[str, Any]:
            return _safe(
                config,
                lambda: capabilities.mcp.mcp_list(query=query, include_schema=include_schema, offset=offset, limit=limit),
                tool_name="codexpro_bridge_mcp_list",
            )

        @server.tool(
            name="codexpro_bridge_mcp_status",
            description="Return sanitized health for the shared local AgentGateway MCP endpoint.",
            annotations=READ_ONLY,
        )
        def mcp_status() -> dict[str, Any]:
            return _safe(config, capabilities.mcp.mcp_status, tool_name="codexpro_bridge_mcp_status")

        @server.tool(
            name="codexpro_bridge_mcp_call",
            description="Invoke exactly one namespaced tool exposed by AgentGateway. Calls are never replayed after uncertain delivery.",
            annotations=UPSTREAM_DISPATCH,
        )
        def mcp_call(tool: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
            return _safe(
                config,
                lambda: capabilities.mcp.mcp_call(tool=tool, arguments=arguments),
                tool_name="codexpro_bridge_mcp_call",
            )

    return ModuleDescriptor(
        module_id="shared_mcp",
        version=capabilities.mcp.version,
        tools=tools,
        health=capabilities.mcp.health,
        install=install,
        shutdown=capabilities.mcp.shutdown,
    )


def _memory_module(capabilities: Any, config: BridgeConfig) -> ModuleDescriptor:
    tools = ("codexpro_bridge_memory_search", "codexpro_bridge_memory_list", "codexpro_bridge_memory_call")

    def install(server: Any) -> None:
        @server.tool(
            name="codexpro_bridge_memory_search",
            description="Search shared memory directly through Obsidian and/or MemOS without going through the Skill or AgentGateway layers.",
            annotations=READ_ONLY,
        )
        def memory_search(
            query: str,
            source: str = "auto",
            limit: int = 6,
            conversation_first_message: str | None = None,
        ) -> dict[str, Any]:
            return _safe(
                config,
                lambda: capabilities.memory.memory_search(
                    query=query,
                    source=source,
                    limit=limit,
                    conversation_first_message=conversation_first_message,
                ),
                tool_name="codexpro_bridge_memory_search",
            )

        @server.tool(
            name="codexpro_bridge_memory_list",
            description="List the complete upstream Obsidian/MemOS MCP tool surfaces. No Bridge-side permission filtering is applied.",
            annotations=READ_ONLY,
        )
        def memory_list(
            source: str | None = None,
            query: str | None = None,
            include_schema: bool = False,
            offset: int = 0,
            limit: int = 100,
        ) -> dict[str, Any]:
            return _safe(
                config,
                lambda: capabilities.memory.memory_list(
                    source=source,
                    query=query,
                    include_schema=include_schema,
                    offset=offset,
                    limit=limit,
                ),
                tool_name="codexpro_bridge_memory_list",
            )

        @server.tool(
            name="codexpro_bridge_memory_call",
            description="Invoke any tool currently exposed by the selected Obsidian or MemOS MCP source with its original arguments. Calls are never replayed after uncertain delivery.",
            annotations=UPSTREAM_DISPATCH,
        )
        def memory_call(source: str, tool: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
            return _safe(
                config,
                lambda: capabilities.memory.memory_call(source=source, tool=tool, arguments=arguments),
                tool_name="codexpro_bridge_memory_call",
            )

    return ModuleDescriptor(
        module_id="shared_memory",
        version=capabilities.memory.version,
        tools=tools,
        health=capabilities.memory.health,
        install=install,
        shutdown=capabilities.memory.shutdown,
    )


def _work_module(capabilities: Any, config: BridgeConfig) -> ModuleDescriptor:
    tools = ("codexpro_bridge_work",)

    def install(server: Any) -> None:
        @server.tool(
            name="codexpro_bridge_work",
            description="Bridge 2.1 durable work-state dispatcher for tasks, checkpoints, project context, mechanical audits, and artifact references. It records state only and never replays external mutations.",
            annotations=STATE_DISPATCH,
        )
        def work(operation: WorkOperation, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
            return _safe(
                config,
                lambda: capabilities.work.dispatch(operation=operation, arguments=arguments),
                tool_name="codexpro_bridge_work",
            )

    return ModuleDescriptor(
        module_id="work_runtime",
        version=capabilities.work.version,
        tools=tools,
        health=capabilities.work.health,
        install=install,
    )


def _doctor_module(capabilities: Any, config: BridgeConfig) -> ModuleDescriptor:
    tools = ("codexpro_bridge_doctor",)

    def install(server: Any) -> None:
        @server.tool(
            name="codexpro_bridge_doctor",
            description="Check the Bridge, shared Skill library, AgentGateway, Obsidian, MemOS and Work Runtime connectivity without exposing secrets.",
            annotations=READ_ONLY,
        )
        def doctor(deep: bool = False) -> dict[str, Any]:
            return _safe(config, lambda: capabilities.doctor(deep=deep), tool_name="codexpro_bridge_doctor")

    return ModuleDescriptor(
        module_id="bridge_doctor",
        version="1.0.0",
        tools=tools,
        health=lambda: {"ok": True},
        install=install,
    )


BUILTIN_MODULE_FACTORIES = (
    _skills_module,
    _mcp_module,
    _memory_module,
    _work_module,
    _doctor_module,
)


def build_builtin_modules(capabilities: Any, config: BridgeConfig) -> tuple[ModuleDescriptor, ...]:
    """Build the current module catalog.  Tool totals are derived, never encoded."""

    return tuple(factory(capabilities, config) for factory in BUILTIN_MODULE_FACTORIES)


def build_instructions(enabled_modules: tuple[str, ...]) -> str:
    enabled = set(enabled_modules)
    if "shared_skills" in enabled:
        memory = " Shared memory recall is available when the memory module is enabled." if "shared_memory" in enabled else ""
        return (
            "For non-trivial work, call codexpro_bridge_route_and_recall first. Choose a matching managed Skill and "
            "load its SKILL.md before acting." + memory +
            " Use the separate CodexPro application for VPS files, Bash, Git and edits."
        )
    return "Use the enabled Bridge modules for shared capabilities. Use the separate CodexPro application for VPS files, Bash, Git and edits."
