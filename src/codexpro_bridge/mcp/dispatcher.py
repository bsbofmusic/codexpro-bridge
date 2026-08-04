"""Single-execution MCP dispatcher.  It deliberately contains no retry path."""

from __future__ import annotations

from typing import Any

from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.core.redaction import bounded_result

from .results import normalize_call_result
from .runtime import HermesMcpRuntime


class HermesMcpDispatcher:
    """Execute one validated upstream MCP call on Hermes's own event loop."""

    def __init__(self, runtime: HermesMcpRuntime):
        self.runtime = runtime

    def call(self, *, server: str, tool: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
        if not isinstance(server, str) or not server or len(server) > 128:
            raise BridgeError("invalid_server", "MCP server is invalid")
        if server.casefold() == "hindsight":
            raise BridgeError("mcp_denied", "The Hindsight MCP server is not available through CodexPro Bridge")
        if not isinstance(tool, str) or not tool or len(tool) > 256:
            raise BridgeError("invalid_tool", "MCP tool is invalid")
        if arguments is None:
            arguments = {}
        if not isinstance(arguments, dict):
            raise BridgeError("invalid_arguments", "MCP arguments must be an object")
        if not self.runtime.is_allowed(server, tool):
            raise BridgeError("mcp_denied", "MCP server or tool is not enabled for CodexPro Bridge")

        # Ensure direct calls have the same connection setup as list/status.
        # Registration is idempotent in Hermes and is not a retry of this tool.
        self.runtime.start()
        module = self.runtime._module()
        self.runtime.dispatch_compatibility()
        sent = False

        async def call_once() -> Any:
            nonlocal sent
            with module._lock:
                upstream = module._servers.get(server)
            if upstream is None or getattr(upstream, "session", None) is None:
                raise BridgeError("mcp_unavailable", "MCP server is not connected")
            async with upstream._rpc_lock:
                session = upstream.session
                if session is None:
                    raise BridgeError("mcp_unavailable", "MCP server is not connected")
                sent = True
                return await session.call_tool(tool, arguments=arguments)

        try:
            result = module._run_on_mcp_loop(call_once, timeout=self.runtime.config.mcp_timeout_seconds)
        except BridgeError:
            raise
        except Exception as exc:
            # Once call_tool has been awaited, a timeout/disconnect cannot prove
            # whether the upstream executed.  Reconnect only helps a later call.
            if sent:
                try:
                    module.reconnect_mcp_server(server)
                except Exception:
                    pass
                raise BridgeError("delivery_unknown", "MCP delivery status is unknown; the call was not replayed") from exc
            raise BridgeError("mcp_unavailable", "MCP call could not be delivered") from exc

        return bounded_result(
            {"ok": True, "server": server, "tool": tool, "result": normalize_call_result(result)},
            self.runtime.config.max_output_chars,
        )
