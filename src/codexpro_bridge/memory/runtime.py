"""Direct full-permission MCP access to the shared Obsidian and MemOS memory sources."""

from __future__ import annotations

import asyncio
import re
from pathlib import Path
from typing import Any

from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.core.redaction import bounded_result
from codexpro_bridge.mcp.results import normalize_call_result
from codexpro_bridge.mcp.runtime import _AsyncRunner

_MEMORY_SIGNAL = re.compile(
    r"(?:\bremember\b|\bmemory\b|\brecall\b|\bprevious(?:ly)?\b|\bprior\b|\bhistory\b|"
    r"\bknowledge\s*base\b|\bsecond\s*brain\b|\bpreferences?\b|"
    r"记忆|回忆|之前|以前|过去|历史|第二大脑|知识库|偏好|上次|此前|先前|说过|聊过)",
    re.IGNORECASE,
)


def _schema(tool: Any) -> dict[str, Any]:
    value = getattr(tool, "inputSchema", None)
    if value is None:
        value = getattr(tool, "input_schema", None)
    return value if isinstance(value, dict) else {"type": "object"}


def _annotations(tool: Any) -> dict[str, Any] | None:
    value = getattr(tool, "annotations", None)
    if value is None:
        return None
    if hasattr(value, "model_dump"):
        dumped = value.model_dump(by_alias=True, exclude_none=True)
        return dumped if isinstance(dumped, dict) and dumped else None
    return None


class StdioMcpTransport:
    """Open short-lived stdio MCP sessions against one trusted local wrapper."""

    def __init__(self, command: Path | str, timeout_seconds: int):
        self.command = str(command)
        self.timeout_seconds = timeout_seconds

    async def list_tools(self) -> list[Any]:
        try:
            params = StdioServerParameters(command=self.command, args=[])
            async with stdio_client(params) as (read_stream, write_stream):
                async with ClientSession(
                    read_stream,
                    write_stream,
                    read_timeout_seconds=float(self.timeout_seconds),
                ) as session:
                    await session.initialize()
                    result = await session.list_tools()
                    return list(result.tools or [])
        except BridgeError:
            raise
        except Exception as exc:
            raise BridgeError("memory_unavailable", "Shared memory MCP source is unavailable") from exc

    async def call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        sent = False
        try:
            params = StdioServerParameters(command=self.command, args=[])
            async with stdio_client(params) as (read_stream, write_stream):
                async with ClientSession(
                    read_stream,
                    write_stream,
                    read_timeout_seconds=float(self.timeout_seconds),
                ) as session:
                    await session.initialize()
                    sent = True
                    return await session.call_tool(
                        name,
                        arguments=arguments,
                        read_timeout_seconds=float(self.timeout_seconds),
                    )
        except BridgeError:
            raise
        except Exception as exc:
            if sent:
                raise BridgeError(
                    "delivery_unknown",
                    "Memory MCP delivery status is unknown; the call was not replayed",
                ) from exc
            raise BridgeError("memory_unavailable", "Shared memory MCP call could not be delivered") from exc


class SharedMemoryRuntime:
    """Full upstream tool parity plus a bounded convenience search over Obsidian/MemOS."""

    VALID_SOURCES = ("obsidian", "memos")

    def __init__(
        self,
        config: BridgeConfig,
        transports: dict[str, Any] | None = None,
    ):
        self.config = config
        if transports is None:
            self.transports: dict[str, Any] = {
                "obsidian": StdioMcpTransport(config.obsidian_mcp_command, config.memory_timeout_seconds),
                "memos": StdioMcpTransport(config.memos_mcp_command, config.memory_timeout_seconds),
            }
            self._runner: _AsyncRunner | None = _AsyncRunner()
        else:
            self.transports = dict(transports)
            self._runner = None

    def _run(self, coroutine: Any) -> Any:
        timeout = float(self.config.memory_timeout_seconds) + 10.0
        if self._runner is not None:
            return self._runner.run(coroutine, timeout)
        return asyncio.run(coroutine)

    @staticmethod
    def should_recall(task: str) -> bool:
        return bool(_MEMORY_SIGNAL.search(task))

    def _source(self, source: str) -> Any:
        if not isinstance(source, str) or source not in self.VALID_SOURCES:
            raise BridgeError("invalid_memory_source", "Memory source must be obsidian or memos")
        transport = self.transports.get(source)
        if transport is None:
            raise BridgeError("memory_unavailable", "Requested memory source is unavailable", {"source": source})
        return transport

    def _tools_for_source(self, source: str) -> list[Any]:
        transport = self._source(source)
        return self._run(transport.list_tools())

    def list_tools(
        self,
        *,
        source: str | None = None,
        query: str | None = None,
        include_schema: bool = False,
        offset: int = 0,
        limit: int = 100,
    ) -> dict[str, Any]:
        if source is not None and source not in self.VALID_SOURCES:
            raise BridgeError("invalid_memory_source", "Memory source must be obsidian or memos")
        if query is not None and (not isinstance(query, str) or len(query) > 256):
            raise BridgeError("invalid_query", "Memory tool query is invalid")
        if not isinstance(offset, int) or offset < 0:
            raise BridgeError("invalid_pagination", "offset must be a non-negative integer")
        if not isinstance(limit, int) or not 1 <= limit <= 100:
            raise BridgeError("invalid_pagination", "limit must be between 1 and 100")

        wanted = (source,) if source else self.VALID_SOURCES
        needle = query.casefold().strip() if query else None
        items: list[dict[str, Any]] = []
        degraded: list[str] = []
        for current in wanted:
            try:
                tools = self._tools_for_source(current)
            except BridgeError:
                if source:
                    raise
                degraded.append(current)
                continue
            for tool in tools:
                name = str(getattr(tool, "name", ""))
                description = str(getattr(tool, "description", "") or "")
                if not name:
                    continue
                if needle and needle not in name.casefold() and needle not in description.casefold():
                    continue
                item: dict[str, Any] = {"source": current, "name": name, "description": description}
                if include_schema:
                    item["input_schema"] = _schema(tool)
                    annotations = _annotations(tool)
                    if annotations:
                        item["annotations"] = annotations
                items.append(item)

        items.sort(key=lambda item: (item["source"], item["name"]))
        total = len(items)
        page = items[offset : offset + limit]
        return bounded_result(
            {
                "ok": bool(items) or not degraded,
                "source": "shared-memory",
                "tools": page,
                "count": total,
                "offset": offset,
                "limit": limit,
                "next_offset": offset + len(page) if offset + len(page) < total else None,
                "degraded_sources": degraded,
            },
            self.config.max_output_chars,
        )

    def call_tool(
        self,
        *,
        source: str,
        tool: str,
        arguments: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        if not isinstance(tool, str) or not tool or len(tool) > 256:
            raise BridgeError("invalid_tool", "Memory MCP tool is invalid")
        if arguments is None:
            arguments = {}
        if not isinstance(arguments, dict):
            raise BridgeError("invalid_arguments", "Memory MCP arguments must be an object")

        transport = self._source(source)
        # Dispatch directly after source validation. Callers can use memory_list
        # for live schema discovery; an extra tools/list here would double-spawn
        # short-lived stdio servers and would add latency without adding a real
        # permission boundary. The upstream MCP server remains authoritative.
        result = self._run(transport.call_tool(tool, arguments))
        return bounded_result(
            {
                "ok": True,
                "source": source,
                "tool": tool,
                "result": normalize_call_result(result),
            },
            self.config.max_output_chars,
        )

    def search(
        self,
        *,
        query: str,
        source: str = "auto",
        limit: int = 6,
        conversation_first_message: str | None = None,
    ) -> dict[str, Any]:
        if not isinstance(query, str) or not query.strip() or len(query) > 8_000:
            raise BridgeError("invalid_query", "Memory search query is invalid")
        if source not in {"auto", *self.VALID_SOURCES}:
            raise BridgeError("invalid_memory_source", "Memory source must be auto, obsidian or memos")
        if not isinstance(limit, int) or not 1 <= limit <= 25:
            raise BridgeError("invalid_limit", "Memory search limit must be between 1 and 25")
        first = conversation_first_message if isinstance(conversation_first_message, str) and conversation_first_message else query

        wanted = self.VALID_SOURCES if source == "auto" else (source,)
        results: list[dict[str, Any]] = []
        degraded: list[str] = []
        for current in wanted:
            try:
                if current == "obsidian":
                    item = self.call_tool(
                        source="obsidian",
                        tool="search_content",
                        arguments={"query": query, "max_results": limit},
                    )
                else:
                    item = self.call_tool(
                        source="memos",
                        tool="search_memory",
                        arguments={
                            "query": query,
                            "conversation_first_message": first,
                            "memory_limit_number": limit,
                        },
                    )
                results.append(item)
            except BridgeError as exc:
                degraded.append(current)
                results.append({"ok": False, "source": current, "error": exc.as_dict()["error"]})

        return bounded_result(
            {
                "ok": any(bool(item.get("ok")) for item in results),
                "source": "shared-memory",
                "query": query,
                "results": results,
                "degraded_sources": degraded,
            },
            self.config.max_output_chars,
        )

    def recall_if_needed(
        self,
        task: str,
        *,
        conversation_first_message: str | None = None,
        limit: int = 6,
    ) -> dict[str, Any]:
        if not self.should_recall(task):
            return {"requested": True, "attempted": False, "reason": "no_strong_memory_signal"}
        result = self.search(
            query=task,
            source="auto",
            limit=limit,
            conversation_first_message=conversation_first_message,
        )
        return {"requested": True, "attempted": True, **result}

    def status(self) -> dict[str, Any]:
        sources: list[dict[str, Any]] = []
        for source in self.VALID_SOURCES:
            try:
                tools = self._tools_for_source(source)
                sources.append({"source": source, "ok": True, "tool_count": len(tools)})
            except BridgeError as exc:
                sources.append({"source": source, "ok": False, "error": exc.as_dict()["error"]})
        healthy = [item for item in sources if item.get("ok")]
        return {
            "ok": bool(healthy),
            "degraded": len(healthy) != len(sources),
            "source": "shared-memory",
            "sources": sources,
        }

    def shutdown(self) -> None:
        if self._runner is not None:
            self._runner.close()
