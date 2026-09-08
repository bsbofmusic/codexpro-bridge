"""Shared memory capability facade used by Bridge registry/server layers."""

from __future__ import annotations

from typing import Any

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError

from .runtime import SharedMemoryRuntime


class SharedMemoryModule:
    module_id = "shared_memory"
    version = "1.0.0"

    def __init__(self, config: BridgeConfig, runtime: SharedMemoryRuntime | None = None):
        self.runtime = runtime or SharedMemoryRuntime(config)

    def memory_list(
        self,
        *,
        source: str | None = None,
        query: str | None = None,
        include_schema: bool = False,
        offset: int = 0,
        limit: int = 100,
    ) -> dict[str, Any]:
        return self._result(
            lambda: self.runtime.list_tools(
                source=source,
                query=query,
                include_schema=include_schema,
                offset=offset,
                limit=limit,
            )
        )

    def memory_call(
        self,
        *,
        source: str,
        tool: str,
        arguments: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        return self._result(lambda: self.runtime.call_tool(source=source, tool=tool, arguments=arguments))

    def memory_search(
        self,
        *,
        query: str,
        source: str = "auto",
        limit: int = 6,
        conversation_first_message: str | None = None,
    ) -> dict[str, Any]:
        return self._result(
            lambda: self.runtime.search(
                query=query,
                source=source,
                limit=limit,
                conversation_first_message=conversation_first_message,
            )
        )

    def recall_if_needed(
        self,
        task: str,
        *,
        conversation_first_message: str | None = None,
        limit: int = 6,
    ) -> dict[str, Any]:
        return self._result(
            lambda: self.runtime.recall_if_needed(
                task,
                conversation_first_message=conversation_first_message,
                limit=limit,
            )
        )

    def memory_status(self) -> dict[str, Any]:
        return self._result(self.runtime.status)

    def health(self) -> dict[str, Any]:
        status = self.runtime.status()
        return {"module": self.module_id, "version": self.version, **status}

    def shutdown(self) -> None:
        self.runtime.shutdown()

    @staticmethod
    def _result(operation: Any) -> dict[str, Any]:
        try:
            return operation()
        except BridgeError as exc:
            return exc.as_dict()
