"""Shared MCP capability facade used by the Bridge registry/server layers."""

from __future__ import annotations

from typing import Any

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError

from .runtime import SharedMcpRuntime


class SharedMcpModule:
    module_id = "shared_mcp"
    version = "1.0.0"

    def __init__(self, config: BridgeConfig, runtime: SharedMcpRuntime | None = None):
        self.runtime = runtime or SharedMcpRuntime(config)

    def mcp_list(
        self,
        *,
        query: str | None = None,
        include_schema: bool = False,
        offset: int = 0,
        limit: int = 100,
    ) -> dict[str, Any]:
        return self._result(
            lambda: self.runtime.list_tools(
                query=query,
                include_schema=include_schema,
                offset=offset,
                limit=limit,
            )
        )

    def mcp_status(self) -> dict[str, Any]:
        return self._result(self.runtime.status)

    def mcp_call(self, *, tool: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
        return self._result(lambda: self.runtime.call_tool(tool=tool, arguments=arguments))

    def health(self) -> dict[str, Any]:
        status = self.runtime.status()
        if status.get("ok"):
            return {
                "ok": True,
                "module": self.module_id,
                "version": self.version,
                "compatibility": self.runtime.compatibility(),
                "tool_count": status.get("tool_count", 0),
            }
        return status

    def shutdown(self) -> None:
        self.runtime.shutdown()

    @staticmethod
    def _result(operation: Any) -> dict[str, Any]:
        try:
            return operation()
        except BridgeError as exc:
            return exc.as_dict()
