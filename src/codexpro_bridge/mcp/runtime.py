"""Direct Streamable-HTTP client for the shared AgentGateway MCP endpoint."""

from __future__ import annotations

import asyncio
import threading
from concurrent.futures import TimeoutError as FutureTimeoutError
from typing import Any

from mcp import ClientSession
from mcp.client.streamable_http import streamable_http_client

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.core.redaction import bounded_result

from .results import normalize_call_result


def _schema(tool: Any) -> dict[str, Any]:
    value = getattr(tool, "inputSchema", None)
    if value is None:
        value = getattr(tool, "input_schema", None)
    return value if isinstance(value, dict) else {"type": "object"}


class _AsyncRunner:
    """Own one private asyncio loop so sync MCP tools never nest event loops."""

    def __init__(self) -> None:
        self._loop = asyncio.new_event_loop()
        self._thread = threading.Thread(target=self._serve, name="codexpro-bridge-mcp", daemon=True)
        self._thread.start()

    def _serve(self) -> None:
        asyncio.set_event_loop(self._loop)
        self._loop.run_forever()

    def run(self, coroutine: Any, timeout: float) -> Any:
        future = asyncio.run_coroutine_threadsafe(coroutine, self._loop)
        try:
            return future.result(timeout=timeout)
        except FutureTimeoutError as exc:
            future.cancel()
            raise BridgeError("mcp_timeout", "Shared MCP request timed out") from exc

    def close(self) -> None:
        if not self._loop.is_running():
            return
        self._loop.call_soon_threadsafe(self._loop.stop)
        self._thread.join(timeout=2)


class AgentGatewayTransport:
    """Open short-lived MCP sessions against one local AgentGateway endpoint."""

    def __init__(self, url: str, timeout_seconds: int):
        self.url = url
        self.timeout_seconds = timeout_seconds

    async def list_tools(self) -> list[Any]:
        try:
            async with streamable_http_client(self.url, terminate_on_close=True) as (read_stream, write_stream):
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
            raise BridgeError("mcp_unavailable", "Shared MCP gateway is unavailable") from exc

    async def call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        sent = False
        try:
            async with streamable_http_client(self.url, terminate_on_close=True) as (read_stream, write_stream):
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
                    "MCP delivery status is unknown; the call was not replayed",
                ) from exc
            raise BridgeError("mcp_unavailable", "Shared MCP call could not be delivered") from exc


class SharedMcpRuntime:
    """List and call the canonical tools exposed by AgentGateway."""

    def __init__(self, config: BridgeConfig, transport: Any | None = None):
        self.config = config
        if transport is not None:
            self.transports: list[tuple[str, Any, bool]] = [(config.mcp_url, transport, False)]
            self.transport = transport
            self._runner = None
        else:
            primary = AgentGatewayTransport(config.mcp_url, config.mcp_timeout_seconds)
            self.transports = [(config.mcp_url, primary, False)] + [
                (url, AgentGatewayTransport(url, config.mcp_timeout_seconds), True)
                for url in config.mcp_optional_urls
            ]
            self.transport = primary
            self._runner = _AsyncRunner()

    def _run(self, coroutine: Any) -> Any:
        timeout = float(self.config.mcp_timeout_seconds) + 10.0
        if self._runner is not None:
            return self._runner.run(coroutine, timeout)
        return asyncio.run(coroutine)

    def _catalog(self) -> list[tuple[Any, Any, str]]:
        catalog: list[tuple[Any, Any, str]] = []
        seen: set[str] = set()
        for endpoint, transport, optional in self.transports:
            try:
                tools = self._run(transport.list_tools())
            except BridgeError:
                if optional:
                    continue
                raise
            for tool in tools:
                name = str(getattr(tool, "name", ""))
                if not name or name in seen:
                    continue
                seen.add(name)
                catalog.append((tool, transport, endpoint))
        return catalog

    def _tools(self) -> list[Any]:
        return [tool for tool, _transport, _endpoint in self._catalog()]

    def list_tools(
        self,
        *,
        query: str | None = None,
        include_schema: bool = False,
        offset: int = 0,
        limit: int = 100,
    ) -> dict[str, Any]:
        if query is not None and (not isinstance(query, str) or len(query) > 256):
            raise BridgeError("invalid_query", "MCP tool query is invalid")
        if not isinstance(offset, int) or offset < 0:
            raise BridgeError("invalid_pagination", "offset must be a non-negative integer")
        if not isinstance(limit, int) or not 1 <= limit <= 100:
            raise BridgeError("invalid_pagination", "limit must be between 1 and 100")

        items: list[dict[str, Any]] = []
        needle = query.casefold().strip() if query else None
        for tool in self._tools():
            name = str(getattr(tool, "name", ""))
            if not name:
                continue
            description = str(getattr(tool, "description", "") or "")
            if needle and needle not in name.casefold() and needle not in description.casefold():
                continue
            item: dict[str, Any] = {"name": name, "description": description}
            if include_schema:
                item["input_schema"] = _schema(tool)
            items.append(item)
        items.sort(key=lambda item: item["name"])
        total = len(items)
        page = items[offset : offset + limit]
        return bounded_result(
            {
                "ok": True,
                "tools": page,
                "count": total,
                "offset": offset,
                "limit": limit,
                "next_offset": offset + len(page) if offset + len(page) < total else None,
                "source": "agentgateway",
            },
            self.config.max_output_chars,
        )

    def status(self) -> dict[str, Any]:
        try:
            count = len(self._tools())
            result: dict[str, Any] = {
                "ok": True,
                "source": "agentgateway",
                "endpoint": self.config.mcp_url,
                "tool_count": count,
            }
            if self.config.mcp_optional_urls:
                result["optional_endpoints"] = list(self.config.mcp_optional_urls)
            return result
        except BridgeError as exc:
            return exc.as_dict()

    def call_tool(self, *, tool: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
        if not isinstance(tool, str) or not tool or len(tool) > 256:
            raise BridgeError("invalid_tool", "MCP tool is invalid")
        if arguments is None:
            arguments = {}
        if not isinstance(arguments, dict):
            raise BridgeError("invalid_arguments", "MCP arguments must be an object")

        selected_transport = None
        for item, transport, _endpoint in self._catalog():
            if str(getattr(item, "name", "")) == tool:
                selected_transport = transport
                break
        if selected_transport is None:
            raise BridgeError("mcp_denied", "MCP tool is not exposed by the shared gateway")
        result = self._run(selected_transport.call_tool(tool, arguments))
        return bounded_result(
            {"ok": True, "tool": tool, "result": normalize_call_result(result)},
            self.config.max_output_chars,
        )

    def compatibility(self) -> dict[str, Any]:
        return {
            "ok": True,
            "transport": "streamable-http",
            "source": "agentgateway",
            "endpoint": self.config.mcp_url,
        }

    def shutdown(self) -> None:
        if self._runner is not None:
            self._runner.close()
