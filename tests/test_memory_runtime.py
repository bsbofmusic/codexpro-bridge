from __future__ import annotations

from dataclasses import dataclass
from types import SimpleNamespace
from typing import Any

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.memory.runtime import SharedMemoryRuntime


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
    def __init__(self, tools: list[FakeTool], *, fail_list: bool = False):
        self.tools = tools
        self.fail_list = fail_list
        self.calls: list[tuple[str, dict[str, Any]]] = []

    async def list_tools(self) -> list[FakeTool]:
        if self.fail_list:
            raise BridgeError("memory_unavailable", "test unavailable")
        return self.tools

    async def call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        self.calls.append((name, arguments))
        return SimpleNamespace(isError=False, content=[TextBlock()], structuredContent={"name": name, "arguments": arguments})


def _runtime(obsidian: FakeTransport, memos: FakeTransport) -> SharedMemoryRuntime:
    return SharedMemoryRuntime(
        BridgeConfig(allow_anonymous=True, max_output_chars=50_000),
        {"obsidian": obsidian, "memos": memos},
    )


def test_memory_list_preserves_mutation_tools_and_schemas_without_allowlist() -> None:
    obsidian = FakeTransport([
        FakeTool("read_note"),
        FakeTool("write_note", inputSchema={"type": "object", "required": ["path", "content"]}),
        FakeTool("delete_note"),
    ])
    memos = FakeTransport([FakeTool("search_memory"), FakeTool("delete_memory"), FakeTool("create_knowledge_base")])
    result = _runtime(obsidian, memos).list_tools(include_schema=True)
    names = {(item["source"], item["name"]) for item in result["tools"]}
    assert ("obsidian", "write_note") in names
    assert ("obsidian", "delete_note") in names
    assert ("memos", "delete_memory") in names
    assert ("memos", "create_knowledge_base") in names
    write = next(item for item in result["tools"] if item["name"] == "write_note")
    assert write["input_schema"]["required"] == ["path", "content"]


def test_memory_call_dispatches_mutation_exactly_once_with_original_arguments() -> None:
    obsidian = FakeTransport([FakeTool("write_note")])
    memos = FakeTransport([])
    runtime = _runtime(obsidian, memos)
    arguments = {"path": "audit.md", "content": "probe", "overwrite": True}
    result = runtime.call_tool(source="obsidian", tool="write_note", arguments=arguments)
    assert result["ok"] is True
    assert obsidian.calls == [("write_note", arguments)]


def test_one_memory_source_failure_does_not_break_the_other_catalog() -> None:
    obsidian = FakeTransport([FakeTool("search_content")])
    memos = FakeTransport([], fail_list=True)
    result = _runtime(obsidian, memos).list_tools()
    assert result["ok"] is True
    assert result["degraded_sources"] == ["memos"]
    assert result["tools"][0]["name"] == "search_content"


def test_memory_intent_is_independent_and_simple_math_skips_recall() -> None:
    runtime = _runtime(FakeTransport([FakeTool("search_content")]), FakeTransport([FakeTool("search_memory")]))
    assert runtime.should_recall("Leah之前怎么说的？") is True
    skipped = runtime.recall_if_needed("2+2")
    assert skipped["attempted"] is False
    assert skipped["reason"] == "no_strong_memory_signal"


def test_convenience_search_calls_both_canonical_search_tools() -> None:
    obsidian = FakeTransport([FakeTool("search_content")])
    memos = FakeTransport([FakeTool("search_memory")])
    result = _runtime(obsidian, memos).search(query="shared bridge", conversation_first_message="first", limit=4)
    assert result["ok"] is True
    assert obsidian.calls == [("search_content", {"query": "shared bridge", "max_results": 4})]
    assert memos.calls == [(
        "search_memory",
        {"query": "shared bridge", "conversation_first_message": "first", "memory_limit_number": 4},
    )]
