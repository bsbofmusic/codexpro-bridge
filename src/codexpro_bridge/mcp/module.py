"""Capability-module facade used by the Bridge registry/server layers."""

from __future__ import annotations

from typing import Any

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError

from .dispatcher import HermesMcpDispatcher
from .runtime import HermesMcpRuntime


class HermesMcpModule:
    module_id = "hermes_mcp"
    version = "0.1.0"

    def __init__(self, config: BridgeConfig, runtime: HermesMcpRuntime | None = None):
        self.runtime = runtime or HermesMcpRuntime(config)
        self.dispatcher = HermesMcpDispatcher(self.runtime)

    def hermes_mcp_list(
        self,
        *,
        server: str | None = None,
        query: str | None = None,
        include_schema: bool = False,
        offset: int = 0,
        limit: int = 100,
    ) -> dict[str, Any]:
        return self._result(
            lambda: self.runtime.list_tools(
                server=server,
                query=query,
                include_schema=include_schema,
                offset=offset,
                limit=limit,
            )
        )

    def hermes_mcp_status(self) -> dict[str, Any]:
        return self._result(self.runtime.status)

    def hermes_mcp_call(self, *, server: str, tool: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
        return self._result(lambda: self.dispatcher.call(server=server, tool=tool, arguments=arguments))

    def health(self) -> dict[str, Any]:
        try:
            return {"ok": True, "module": self.module_id, "version": self.version, "compatibility": self.runtime.compatibility()}
        except BridgeError as exc:
            return exc.as_dict()

    def shutdown(self) -> None:
        self.runtime.shutdown()

    @staticmethod
    def _result(operation: Any) -> dict[str, Any]:
        try:
            return operation()
        except BridgeError as exc:
            return exc.as_dict()
