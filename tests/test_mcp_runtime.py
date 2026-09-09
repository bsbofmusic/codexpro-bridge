from __future__ import annotations

import asyncio
from dataclasses import dataclass
from types import SimpleNamespace
from typing import Any

import pytest

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.mcp.runtime import AgentGatewayTransport, SharedMcpRuntime


@dataclass
class FakeTool:
    name: str
    description: str = ""
    inputSchema: dict[str, Any] | None = None


@dataclass
class TextBlock:
    type: str = "text"
    text: str = "ok"


class FakeTransport:
    def __init__(self, tools: list[FakeTool], *, error: BridgeError | None = None):
        self.tools = tools
        self.error = error
        self.calls = 0
        self.last_call: tuple[str, dict[str, Any]] | None = None

    async def list_tools(self) -> list[FakeTool]:
        return self.tools

    async def call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        self.calls += 1
        self.last_call = (name, arguments)
        if self.error is not None:
            raise self.error
        return SimpleNamespace(
            isError=False,
            content=[TextBlock()],
            structuredContent={"name": name, "arguments": arguments},
        )


def _config() -> BridgeConfig:
    return BridgeConfig(allow_anonymous=True, max_output_chars=20_000)


def test_list_uses_agentgateway_names_as_canonical_tool_names() -> None:
    fake = FakeTransport(
        [
            FakeTool("searxng_search", "Search the web", {"type": "object"}),
            FakeTool("patent_get_patent", "Read a patent", {"type": "object", "properties": {"id": {"type": "string"}}}),
        ]
    )
    runtime = SharedMcpRuntime(_config(), fake)

    result = runtime.list_tools(query="patent", include_schema=True)

    assert result["ok"] is True
    assert result["source"] == "agentgateway"
    assert result["count"] == 1
    assert result["tools"][0]["name"] == "patent_get_patent"
    assert result["tools"][0]["input_schema"]["type"] == "object"


def test_call_executes_exactly_once_and_preserves_namespaced_tool() -> None:
    fake = FakeTransport([FakeTool("web-search_search")])
    runtime = SharedMcpRuntime(_config(), fake)

    result = runtime.call_tool(tool="web-search_search", arguments={"query": "agent gateway"})

    assert result["ok"] is True
    assert fake.calls == 1
    assert fake.last_call == ("web-search_search", {"query": "agent gateway"})
    assert result["result"]["is_error"] is False


def test_unknown_tool_fails_before_dispatch() -> None:
    fake = FakeTransport([FakeTool("searxng_search")])
    runtime = SharedMcpRuntime(_config(), fake)

    with pytest.raises(BridgeError) as caught:
        runtime.call_tool(tool="missing_tool")

    assert caught.value.code == "mcp_denied"
    assert fake.calls == 0


def test_delivery_unknown_is_never_replayed() -> None:
    fake = FakeTransport(
        [FakeTool("patent_get_patent")],
        error=BridgeError("delivery_unknown", "MCP delivery status is unknown; the call was not replayed"),
    )
    runtime = SharedMcpRuntime(_config(), fake)

    with pytest.raises(BridgeError) as caught:
        runtime.call_tool(tool="patent_get_patent", arguments={"id": "US1"})

    assert caught.value.code == "delivery_unknown"
    assert fake.calls == 1


def test_status_reports_shared_gateway_tool_count() -> None:
    fake = FakeTransport([FakeTool("a"), FakeTool("b")])
    runtime = SharedMcpRuntime(_config(), fake)

    status = runtime.status()

    assert status == {
        "ok": True,
        "source": "agentgateway",
        "endpoint": "http://127.0.0.1:19090/mcp",
        "tool_count": 2,
    }


def test_optional_gateway_failure_does_not_break_primary_catalog() -> None:
    primary = FakeTransport([FakeTool("core_search")])

    class BrokenOptional:
        async def list_tools(self) -> list[FakeTool]:
            raise BridgeError("mcp_unavailable", "optional gateway unavailable")

    runtime = SharedMcpRuntime(_config(), primary)
    runtime.config.mcp_optional_urls = ("http://127.0.0.1:19094/mcp",)
    runtime.transports.append(("http://127.0.0.1:19094/mcp", BrokenOptional(), True))

    result = runtime.list_tools()

    assert result["count"] == 1
    assert result["tools"][0]["name"] == "core_search"


def test_optional_gateway_tool_dispatches_to_its_own_transport() -> None:
    primary = FakeTransport([FakeTool("core_search")])
    optional = FakeTransport([FakeTool("xydc_lookup")])
    runtime = SharedMcpRuntime(_config(), primary)
    runtime.config.mcp_optional_urls = ("http://127.0.0.1:19094/mcp",)
    runtime.transports.append(("http://127.0.0.1:19094/mcp", optional, True))

    result = runtime.call_tool(tool="xydc_lookup", arguments={"asin": "B000TEST"})

    assert result["ok"] is True
    assert primary.calls == 0
    assert optional.calls == 1


def test_agentgateway_transport_terminates_short_lived_sessions(monkeypatch: pytest.MonkeyPatch) -> None:
    observed: list[bool | None] = []

    class FakeHttpTransport:
        async def __aenter__(self):
            return object(), object()

        async def __aexit__(self, exc_type, exc, tb):
            return False

    class FakeSession:
        def __init__(self, *args: Any, **kwargs: Any):
            pass

        async def __aenter__(self):
            return self

        async def __aexit__(self, exc_type, exc, tb):
            return False

        async def initialize(self) -> None:
            return None

        async def list_tools(self) -> Any:
            return SimpleNamespace(tools=[])

        async def call_tool(self, name: str, *, arguments: dict[str, Any], read_timeout_seconds: float) -> Any:
            return SimpleNamespace(isError=False, content=[], structuredContent={"name": name})

    def fake_streamable_http_client(
        url: str,
        *,
        terminate_on_close: bool | None = None,
        **kwargs: Any,
    ) -> FakeHttpTransport:
        observed.append(terminate_on_close)
        return FakeHttpTransport()

    monkeypatch.setattr("codexpro_bridge.mcp.runtime.streamable_http_client", fake_streamable_http_client)
    monkeypatch.setattr("codexpro_bridge.mcp.runtime.ClientSession", FakeSession)

    transport = AgentGatewayTransport("http://127.0.0.1:19090/mcp", 30)

    async def exercise() -> None:
        assert await transport.list_tools() == []
        await transport.call_tool("example", {})

    asyncio.run(exercise())
    assert observed == [True, True]
