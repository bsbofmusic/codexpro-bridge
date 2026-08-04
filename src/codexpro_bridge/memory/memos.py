"""Conditional Memos recall using the already-enabled Hermes MCP server."""

from __future__ import annotations

import re
from typing import Any

from codexpro_bridge.core.errors import BridgeError

from codexpro_bridge.mcp.dispatcher import HermesMcpDispatcher
from codexpro_bridge.mcp.runtime import HermesMcpRuntime

_MEMORY_SIGNAL = re.compile(
    r"(?:\bremember\b|\bmemory\b|\brecall\b|\bprevious(?:ly)?\b|"
    r"\bknowledge\s*base\b|\bsecond\s*brain\b|\bpreferences?\b|"
    r"\bprior\s+decisions?\b|记忆|回忆|之前|过去|第二大脑|知识库|用户偏好|历史决策)",
    re.IGNORECASE,
)


class MemosMemory:
    """Recall Memos only when a task explicitly asks for prior context."""

    def __init__(self, runtime: HermesMcpRuntime, dispatcher: HermesMcpDispatcher | None = None):
        self.runtime = runtime
        self.dispatcher = dispatcher or HermesMcpDispatcher(runtime)

    @staticmethod
    def should_recall(task: str) -> bool:
        return bool(_MEMORY_SIGNAL.search(task))

    def recall(self, task: str) -> dict[str, Any]:
        if not self.should_recall(task):
            return {"requested": True, "attempted": False, "reason": "no_strong_memory_signal"}
        try:
            response = self.dispatcher.call(
                server="memos-api-mcp",
                tool="search_memory",
                arguments={"query": task, "conversation_first_message": task},
            )
        except BridgeError as exc:
            return {"requested": True, "attempted": True, "ok": False, "reason": exc.code}
        return {"requested": True, "attempted": True, "ok": bool(response.get("ok")), "result": response.get("result")}
