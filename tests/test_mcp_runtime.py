from __future__ import annotations

import asyncio
import threading
from dataclasses import dataclass
from pathlib import Path
from types import SimpleNamespace
from typing import Any

import pytest

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.mcp.dispatcher import HermesMcpDispatcher
from codexpro_bridge.mcp.runtime import HermesMcpRuntime
from codexpro_bridge.memory.memos import MemosMemory


@dataclass
class TextBlock:
    type: str = "text"
    text: str = "ok"


class FakeSession:
    def __init__(self, error: Exception | None = None):
        self.calls = 0
        self.error = error
        self.last_call: tuple[str, dict[str, Any]] | None = None

    async def call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        self.calls += 1
        self.last_call = (name, arguments)
        if self.error is not None:
            raise self.error
        return SimpleNamespace(isError=False, content=[TextBlock()], structuredContent={"name": name, "arguments": arguments})


class FakeMcp:
    def __init__(self, config: dict[str, dict[str, Any]], session: FakeSession | None = None):
        self.config = config
        self.session = session or FakeSession()
        self._lock = threading.RLock()
        self._servers = {"memos-api-mcp": SimpleNamespace(session=self.session, _rpc_lock=asyncio.Lock())}
        self.registered: list[dict[str, dict[str, Any]]] = []
        self.reconnects: list[str] = []
        self.shutdowns = 0

    def _load_mcp_config(self) -> dict[str, dict[str, Any]]:
        return self.config

    def register_mcp_servers(self, servers: dict[str, dict[str, Any]]) -> list[str]:
        self.registered.append(servers)
        return ["mcp__memos_api_mcp__search_memory"]

    def get_mcp_status(self) -> list[dict[str, Any]]:
        return [{"name": "memos-api-mcp", "status": "connected"}, {"name": "hindsight", "status": "disabled"}]

    def shutdown_mcp_servers(self) -> None:
        self.shutdowns += 1

    def reconnect_mcp_server(self, name: str) -> bool:
        self.reconnects.append(name)
        return True

    @staticmethod
    def _run_on_mcp_loop(coro_or_factory: Any, timeout: int) -> Any:
        return asyncio.run(coro_or_factory())


def _config() -> BridgeConfig:
    return BridgeConfig(
        allow_anonymous=True,
        max_output_chars=20_000,
        memos_command=Path("/opt/memos/bin/memos-api-mcp"),
    )


def test_overlay_disables_interaction_rejects_hindsight_and_preserves_source() -> None:
    source = {
        "memos-api-mcp": {
            "command": "/srv/hermes/scripts/memos-api-mcp.sh",
            "args": [],
            "env": {
                "MEMOS_DEFAULT_KB_IDS": '["kb-live"]',
                "MEMOS_UPSTREAM_MCP_COMMAND": "npx",
                "MEMOS_UPSTREAM_MCP_ARGS_JSON": '["-y","@memtensor/memos-api-mcp@latest"]',
            },
            "sampling": {"enabled": True},
            "elicitation": {"enabled": True},
        },
        "hindsight": {"enabled": True, "command": "never-used"},
    }
    fake = FakeMcp(source)
    runtime = HermesMcpRuntime(_config(), fake)

    runtime.start()

    registered = fake.registered[-1]
    assert set(registered) == {"memos-api-mcp"}
    memos = registered["memos-api-mcp"]
    assert memos["command"] == "/srv/hermes/scripts/memos-api-mcp.sh"
    assert memos["args"] == []
    assert memos["env"]["MEMOS_DEFAULT_KB_IDS"] == '["kb-live"]'
    assert memos["env"]["MEMOS_UPSTREAM_MCP_COMMAND"] == "/opt/memos/bin/memos-api-mcp"
    assert memos["env"]["MEMOS_UPSTREAM_MCP_ARGS_JSON"] == "[]"
    assert registered["memos-api-mcp"]["sampling"]["enabled"] is False
    assert registered["memos-api-mcp"]["elicitation"]["enabled"] is False
    assert source["memos-api-mcp"]["command"] == "/srv/hermes/scripts/memos-api-mcp.sh"
    assert source["memos-api-mcp"]["env"]["MEMOS_UPSTREAM_MCP_COMMAND"] == "npx"
    assert source["memos-api-mcp"]["env"]["MEMOS_UPSTREAM_MCP_ARGS_JSON"] == '["-y","@memtensor/memos-api-mcp@latest"]'
    assert source["memos-api-mcp"]["sampling"]["enabled"] is True
    assert runtime.is_allowed("hindsight", "anything") is False


def test_dispatcher_calls_transport_once_and_never_replays_uncertain_delivery() -> None:
    secret = "cpk_abcdefghijklmnopqrstuvwxyz0123456789"
    fake = FakeMcp({"memos-api-mcp": {"enabled": True}}, FakeSession(RuntimeError("token=" + secret)))
    runtime = HermesMcpRuntime(_config(), fake)
    dispatcher = HermesMcpDispatcher(runtime)

    with pytest.raises(BridgeError) as caught:
        dispatcher.call(server="memos-api-mcp", tool="search_memory", arguments={"query": "find context"})

    assert caught.value.code == "delivery_unknown"
    assert fake.session.calls == 1
    assert fake.reconnects == ["memos-api-mcp"]
    assert secret not in str(caught.value)


def test_dispatcher_rejects_hindsight_before_connecting() -> None:
    fake = FakeMcp({"hindsight": {"enabled": True}})
    dispatcher = HermesMcpDispatcher(HermesMcpRuntime(_config(), fake))

    with pytest.raises(BridgeError) as caught:
        dispatcher.call(server="hindsight", tool="search")

    assert caught.value.code == "mcp_denied"
    assert fake.registered == []


def test_dispatcher_fails_closed_when_single_call_adapter_is_missing() -> None:
    fake = FakeMcp({"memos-api-mcp": {"enabled": True}})
    fake._run_on_mcp_loop = None
    dispatcher = HermesMcpDispatcher(HermesMcpRuntime(_config(), fake))

    with pytest.raises(BridgeError) as caught:
        dispatcher.call(server="memos-api-mcp", tool="search_memory")

    assert caught.value.code == "mcp_incompatible"
    assert fake.session.calls == 0


def test_memos_recall_uses_native_mcp_for_explicit_memory_request() -> None:
    fake = FakeMcp({"memos-api-mcp": {"enabled": True}})
    memory = MemosMemory(HermesMcpRuntime(_config(), fake))

    recalled = memory.recall("Recall previous deployment decision")
    skipped = memory.recall("Summarize the current task")

    assert recalled["attempted"] is True
    assert recalled["ok"] is True
    assert fake.session.calls == 1
    assert fake.session.last_call == (
        "search_memory",
        {
            "query": "Recall previous deployment decision",
            "conversation_first_message": "Recall previous deployment decision",
        },
    )
    assert skipped == {"requested": True, "attempted": False, "reason": "no_strong_memory_signal"}
