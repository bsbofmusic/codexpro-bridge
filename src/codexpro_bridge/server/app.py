"""Authenticated Streamable HTTP MCP server for the standalone CodexPro Bridge."""

from __future__ import annotations

import hmac
import logging
from typing import Any, Awaitable, Callable
from urllib.parse import parse_qs

import uvicorn
try:
    from mcp.server import MCPServer as FastMCP
    _MCP_TRANSPORT_KWARGS_ON_APP = True
except ImportError:  # pragma: no cover
    from mcp.server.fastmcp import FastMCP
    _MCP_TRANSPORT_KWARGS_ON_APP = False
from mcp.server.transport_security import TransportSecuritySettings
from starlette.responses import JSONResponse, PlainTextResponse

from codexpro_bridge import __version__
from codexpro_bridge.capabilities import CapabilityRegistry
from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.core.redaction import bounded_result, redact_value
from codexpro_bridge.mcp.module import SharedMcpModule
from codexpro_bridge.memory.module import SharedMemoryModule
from codexpro_bridge.server.tool_modules import build_builtin_modules, build_instructions
from codexpro_bridge.skills.runtime import SharedSkillsRuntime
from codexpro_bridge.skills.tools import SkillTools
from codexpro_bridge.work import WorkRuntime

LOG = logging.getLogger("codexpro_bridge")


class BridgeCapabilities:
    """Own shared backends while the registry owns their public module surface."""

    def __init__(self, config: BridgeConfig):
        self.config = config
        self.registry = CapabilityRegistry(enabled_modules=config.enabled_modules)
        self.mcp = SharedMcpModule(config)
        self.memory = SharedMemoryModule(config)
        self.skills = SkillTools(SharedSkillsRuntime(config))
        self.work = WorkRuntime(config)

    def register_builtin_modules(self) -> None:
        for descriptor in build_builtin_modules(self, self.config):
            self.registry.register(descriptor)

    def _skills_health(self) -> dict[str, Any]:
        try:
            result = self.skills.skills_list(limit=1)
            return {
                "ok": bool(result.get("ok")),
                "skill_count": result.get("count", 0),
                "source": result.get("source"),
            }
        except BridgeError as exc:
            return exc.as_dict()

    def route_and_recall(
        self,
        task: str,
        *,
        include_memory: bool = True,
        conversation_first_message: str | None = None,
        skill_limit: int = 8,
        memory_limit: int = 6,
    ) -> dict[str, Any]:
        routing = self.skills.route(task, limit=skill_limit)
        memory: dict[str, Any] = {"requested": bool(include_memory), "attempted": False, "reason": "disabled"}
        if include_memory and self.registry.is_enabled("shared_memory"):
            memory = self.memory.recall_if_needed(
                task,
                conversation_first_message=conversation_first_message,
                limit=memory_limit,
            )
        elif include_memory:
            memory = {"requested": True, "attempted": False, "reason": "module_disabled"}
        return bounded_result(
            {"ok": True, "task": task, "routing": routing, "memory": memory},
            self.config.max_output_chars,
        )

    def doctor(self, deep: bool = False) -> dict[str, Any]:
        surface = self.registry.surface_audit()
        result: dict[str, Any] = {
            "ok": bool(surface["consistent"]),
            "configuration": self.config.public_dict(),
            "modules": self.registry.manifest(),
            "surface": surface,
            "health": self.registry.health(),
        }
        if deep:
            if self.registry.is_enabled("shared_skills"):
                try:
                    listed = self.skills.skills_list(limit=1)
                    if listed.get("skills"):
                        name = str(listed["skills"][0]["name"])
                        loaded = self.skills.load_skill(name)
                        result["skill_read"] = {"ok": bool(loaded.get("ok")), "name": name}
                    else:
                        result["skill_read"] = {"ok": False, "message": "No managed Skills are enabled"}
                        result["ok"] = False
                except BridgeError as exc:
                    result["skill_read"] = exc.as_dict()
                    result["ok"] = False
            if self.registry.is_enabled("shared_mcp"):
                mcp_status = self.mcp.mcp_status()
                result["mcp_status"] = mcp_status
                if not mcp_status.get("ok"):
                    result["ok"] = False
            if self.registry.is_enabled("shared_memory"):
                memory_status = self.memory.memory_status()
                result["memory_status"] = memory_status
                if not memory_status.get("ok"):
                    result["ok"] = False
            if self.registry.is_enabled("work_runtime"):
                work_status = self.work.health()
                result["work_runtime"] = work_status
                if not work_status.get("ok"):
                    result["ok"] = False
        if not all(bool(item.get("health", {}).get("ok")) for item in result["health"]):
            result["ok"] = False
        if any(bool(item.get("health", {}).get("degraded")) for item in result["health"]):
            result["degraded"] = True
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
            surface = self.capabilities.registry.surface_audit()
            await JSONResponse(
                {
                    "ok": bool(surface["consistent"]),
                    "service": "codexpro-bridge",
                    "auth_required": not self.config.allow_anonymous,
                    "modules": surface["enabled_modules"],
                    "tool_count": surface["tool_count"],
                    "surface_fingerprint": surface["surface_fingerprint"],
                }
            )(scope, receive, send)
            return
        if scope_type == "http" and scope.get("path") == "/mcp" and not self._valid_token(scope):
            await PlainTextResponse("Unauthorized", status_code=401, headers={"WWW-Authenticate": "Bearer"})(scope, receive, send)
            return
        await self.app(scope, receive, send)


def build_server(config: BridgeConfig | None = None) -> tuple[FastMCP, BridgeCapabilities]:
    config = config or BridgeConfig.from_env()
    capabilities = BridgeCapabilities(config)
    capabilities.register_builtin_modules()
    enabled_modules = capabilities.registry.enabled_module_ids()
    instructions = build_instructions(enabled_modules)

    if _MCP_TRANSPORT_KWARGS_ON_APP:
        server = FastMCP(name="CodexPro Bridge", instructions=instructions, version=__version__)
    else:
        server = FastMCP(
            name="CodexPro Bridge",
            instructions=instructions,
            host=config.host,
            port=config.port,
            streamable_http_path="/mcp",
            stateless_http=True,
            json_response=True,
            transport_security=TransportSecuritySettings(enable_dns_rebinding_protection=False),
        )
        server._mcp_server.version = __version__

    capabilities.registry.mount(server)
    return server, capabilities


def create_asgi_app(config: BridgeConfig | None = None) -> AuthenticatedMcpApp:
    server, capabilities = build_server(config)
    if _MCP_TRANSPORT_KWARGS_ON_APP:
        inner = server.streamable_http_app(
            streamable_http_path="/mcp",
            stateless_http=True,
            json_response=True,
            transport_security=TransportSecuritySettings(enable_dns_rebinding_protection=False),
        )
    else:
        inner = server.streamable_http_app()
    return AuthenticatedMcpApp(inner, capabilities.config, capabilities)


def main() -> None:
    config = BridgeConfig.from_env()
    app = create_asgi_app(config)
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    uvicorn.run(app, host=config.host, port=config.port, log_level="info", access_log=False)


if __name__ == "__main__":
    main()
