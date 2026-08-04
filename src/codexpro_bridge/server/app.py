"""Authenticated Streamable HTTP MCP server for CodexPro Bridge."""

from __future__ import annotations

import asyncio
import hmac
import logging
from typing import Any, Awaitable, Callable
from urllib.parse import parse_qs

import uvicorn
from mcp.server.fastmcp import FastMCP
from mcp.server.transport_security import TransportSecuritySettings
from mcp.types import ToolAnnotations
from starlette.responses import JSONResponse, PlainTextResponse

from codexpro_bridge.capabilities import CapabilityRegistry, ModuleDescriptor
from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.core.redaction import bounded_result, redact_value
from codexpro_bridge.mcp.module import HermesMcpModule
from codexpro_bridge.memory.memos import MemosMemory
from codexpro_bridge.skills.runtime import HermesSkillsRuntime
from codexpro_bridge.skills.tools import SkillTools


LOG = logging.getLogger("codexpro_bridge")
ROUTE_FIRST = (
    "For every non-trivial task, first call codexpro_bridge_route_and_recall. "
    "Choose a matching live Hermes Skill and load its real SKILL.md before acting. "
    "Use Memos only for prior decisions, preferences, second-brain or knowledge queries. "
    "Use the separate CodexPro application for VPS files, Bash, Git and edits."
)


READ_ONLY = ToolAnnotations(
    readOnlyHint=True,
    destructiveHint=False,
    idempotentHint=True,
    openWorldHint=False,
)
RECALL = ToolAnnotations(
    readOnlyHint=True,
    destructiveHint=False,
    idempotentHint=True,
    openWorldHint=True,
)
UPSTREAM_DISPATCH = ToolAnnotations(
    readOnlyHint=False,
    destructiveHint=True,
    idempotentHint=False,
    openWorldHint=True,
)


class BridgeCapabilities:
    """Wire the independent live-Skill, MCP and conditional-memory modules."""

    def __init__(self, config: BridgeConfig):
        self.config = config
        self.registry = CapabilityRegistry()
        self.mcp = HermesMcpModule(config)
        self.memory = MemosMemory(self.mcp.runtime, self.mcp.dispatcher)
        self.skills = SkillTools(HermesSkillsRuntime(config), self.memory)
        self.registry.register(
            ModuleDescriptor(
                module_id="hermes_skills",
                version="0.1.0",
                tools=(
                    "codexpro_bridge_route_and_recall",
                    "codexpro_bridge_skills_list",
                    "codexpro_bridge_load_skill",
                    "codexpro_bridge_load_skill_resource",
                ),
                health=self._skills_health,
            )
        )
        self.registry.register(
            ModuleDescriptor(
                module_id="hermes_native_mcp",
                version=self.mcp.version,
                tools=(
                    "codexpro_bridge_hermes_mcp_list",
                    "codexpro_bridge_hermes_mcp_status",
                    "codexpro_bridge_hermes_mcp_call",
                ),
                health=self.mcp.health,
                shutdown=self.mcp.shutdown,
            )
        )
        self.registry.register(
            ModuleDescriptor(
                module_id="bridge_doctor",
                version="0.1.0",
                tools=("codexpro_bridge_doctor",),
                health=lambda: {"ok": True},
            )
        )

    def _skills_health(self) -> dict[str, Any]:
        try:
            result = self.skills.skills_list(limit=1)
            return {"ok": bool(result.get("ok")), "skill_count": result.get("count", 0)}
        except BridgeError as exc:
            return exc.as_dict()

    def doctor(self, deep: bool = False) -> dict[str, Any]:
        result: dict[str, Any] = {
            "ok": True,
            "configuration": self.config.public_dict(),
            "modules": self.registry.manifest(),
            "health": self.registry.health(),
        }
        if deep:
            try:
                obsidian = self.skills.load_skill("obsidian")
                result["skill_read"] = {"ok": bool(obsidian.get("ok")), "name": "obsidian"}
            except BridgeError as exc:
                result["skill_read"] = exc.as_dict()
                result["ok"] = False
            mcp_status = self.mcp.hermes_mcp_status()
            result["mcp_status"] = mcp_status
            if not mcp_status.get("ok"):
                result["ok"] = False
        if not all(bool(item.get("health", {}).get("ok")) for item in result["health"]):
            result["ok"] = False
        return bounded_result(redact_value(result), self.config.max_output_chars)

    def shutdown(self) -> None:
        self.registry.shutdown()


class AuthenticatedMcpApp:
    """Apply token verification without logging the URL query secret."""

    def __init__(self, app: Callable[..., Awaitable[None]], config: BridgeConfig, capabilities: BridgeCapabilities):
        self.app = app
        self.config = config
        self.capabilities = capabilities

    def _valid_token(self, scope: dict[str, Any]) -> bool:
        if self.config.allow_anonymous:
            return True
        expected = self.config.http_token
        if not expected:
            return False
        query = parse_qs(scope.get("query_string", b"").decode("latin-1"), keep_blank_values=True)
        supplied = query.get("codexpro_token", [""])[0]
        if not supplied:
            header_map = {key.lower(): value for key, value in scope.get("headers", [])}
            authorization = header_map.get(b"authorization", b"").decode("latin-1")
            if authorization.lower().startswith("bearer "):
                supplied = authorization[7:]
        return bool(supplied) and hmac.compare_digest(expected, supplied)

    async def __call__(self, scope: dict[str, Any], receive: Callable[..., Awaitable[Any]], send: Callable[..., Awaitable[None]]) -> None:
        scope_type = scope.get("type")
        if scope_type == "lifespan":
            async def receiving() -> Any:
                message = await receive()
                if message.get("type") == "lifespan.shutdown":
                    self.capabilities.shutdown()
                return message

            await self.app(scope, receiving, send)
            return
        if scope_type == "http" and scope.get("path") == "/health":
            await JSONResponse({"ok": True, "service": "codexpro-bridge", "auth_required": not self.config.allow_anonymous})(scope, receive, send)
            return
        if scope_type == "http" and scope.get("path") == "/mcp" and not self._valid_token(scope):
            await PlainTextResponse("Unauthorized", status_code=401, headers={"WWW-Authenticate": "Bearer"})(scope, receive, send)
            return
        await self.app(scope, receive, send)


def _safe(
    config: BridgeConfig,
    operation: Callable[[], dict[str, Any]],
    *,
    tool_name: str,
) -> dict[str, Any]:
    LOG.info("tool_call name=%s", tool_name)
    try:
        return bounded_result(redact_value(operation()), config.max_output_chars)
    except BridgeError as exc:
        return exc.as_dict()
    except Exception:
        LOG.exception("Bridge tool failed")
        return {"ok": False, "error": {"code": "internal_error", "message": "Bridge operation failed"}}


def build_server(config: BridgeConfig | None = None) -> tuple[FastMCP, BridgeCapabilities]:
    config = config or BridgeConfig.from_env()
    capabilities = BridgeCapabilities(config)
    server = FastMCP(
        name="CodexPro Bridge",
        instructions=ROUTE_FIRST,
        host=config.host,
        port=config.port,
        streamable_http_path="/mcp",
        stateless_http=True,
        json_response=True,
        transport_security=TransportSecuritySettings(enable_dns_rebinding_protection=False),
    )
    server._mcp_server.version = "0.1.0"

    @server.tool(
        name="codexpro_bridge_route_and_recall",
        description="First tool for non-trivial work. Return live Hermes Skill metadata and conditionally recall Memos for prior-context questions.",
        annotations=RECALL,
    )
    def route_and_recall(task: str, include_memory: bool = True) -> dict[str, Any]:
        return _safe(
            config,
            lambda: capabilities.skills.route_and_recall(task, include_memory=include_memory),
            tool_name="codexpro_bridge_route_and_recall",
        )

    @server.tool(
        name="codexpro_bridge_skills_list",
        description="List the live enabled Hermes Skills. Use before selecting a Skill; this never reads a copied index.",
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
        description="Load the real SKILL.md for one active Hermes Skill with preprocessing disabled.",
        annotations=READ_ONLY,
    )
    def load_skill(name: str) -> dict[str, Any]:
        return _safe(config, lambda: capabilities.skills.load_skill(name), tool_name="codexpro_bridge_load_skill")

    @server.tool(
        name="codexpro_bridge_load_skill_resource",
        description="Load an allowed real Hermes Skill reference, template, script, or asset after its SKILL.md has been loaded.",
        annotations=READ_ONLY,
    )
    def load_skill_resource(name: str, resource_path: str) -> dict[str, Any]:
        return _safe(
            config,
            lambda: capabilities.skills.load_skill_resource(name, resource_path),
            tool_name="codexpro_bridge_load_skill_resource",
        )

    @server.tool(
        name="codexpro_bridge_hermes_mcp_list",
        description="List enabled Hermes Native MCP tools after their live include/exclude filters. Hindsight is never exposed.",
        annotations=READ_ONLY,
    )
    def hermes_mcp_list(
        server: str | None = None,
        query: str | None = None,
        include_schema: bool = False,
        offset: int = 0,
        limit: int = 100,
    ) -> dict[str, Any]:
        return _safe(
            config,
            lambda: capabilities.mcp.hermes_mcp_list(
                server=server,
                query=query,
                include_schema=include_schema,
                offset=offset,
                limit=limit,
            ),
            tool_name="codexpro_bridge_hermes_mcp_list",
        )

    @server.tool(
        name="codexpro_bridge_hermes_mcp_status",
        description="Return sanitized health and configuration-generation status for enabled Hermes Native MCP servers.",
        annotations=READ_ONLY,
    )
    def hermes_mcp_status() -> dict[str, Any]:
        return _safe(config, capabilities.mcp.hermes_mcp_status, tool_name="codexpro_bridge_hermes_mcp_status")

    @server.tool(
        name="codexpro_bridge_hermes_mcp_call",
        description="Invoke exactly one enabled Hermes MCP tool after listing its schema. This can reach the network or make state changes; never retry a delivery-unknown result.",
        annotations=UPSTREAM_DISPATCH,
    )
    def hermes_mcp_call(server: str, tool: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
        return _safe(
            config,
            lambda: capabilities.mcp.hermes_mcp_call(server=server, tool=tool, arguments=arguments),
            tool_name="codexpro_bridge_hermes_mcp_call",
        )

    @server.tool(
        name="codexpro_bridge_doctor",
        description="Check the Bridge module manifest, live Skill access, MCP compatibility and optional deeper runtime health without exposing secrets.",
        annotations=READ_ONLY,
    )
    def doctor(deep: bool = False) -> dict[str, Any]:
        return _safe(config, lambda: capabilities.doctor(deep=deep), tool_name="codexpro_bridge_doctor")

    return server, capabilities


def create_asgi_app(config: BridgeConfig | None = None) -> AuthenticatedMcpApp:
    server, capabilities = build_server(config)
    return AuthenticatedMcpApp(server.streamable_http_app(), capabilities.config, capabilities)


def main() -> None:
    config = BridgeConfig.from_env()
    app = create_asgi_app(config)
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    uvicorn.run(app, host=config.host, port=config.port, log_level="info", access_log=False)


if __name__ == "__main__":
    main()
