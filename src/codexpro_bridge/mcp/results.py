"""Normalize MCP SDK results without exposing transport internals."""

from __future__ import annotations

from typing import Any

from codexpro_bridge.core.redaction import redact_value


def normalize_call_result(result: Any) -> dict[str, Any]:
    """Convert the supported MCP result shapes into JSON-safe Bridge data.

    Text and structured content are preserved.  Binary media and files are not
    transferred over this narrow dispatcher, but their presence is reported so
    callers do not mistake a partial result for an empty response.
    """

    content: list[dict[str, Any]] = []
    for block in getattr(result, "content", None) or []:
        block_type = getattr(block, "type", None)
        if block_type == "text" or hasattr(block, "text"):
            content.append({"type": "text", "text": str(getattr(block, "text", ""))})
        elif block_type in {"image", "audio", "resource", "resource_link"}:
            content.append(
                {
                    "type": str(block_type or "unknown"),
                    "supported": False,
                    "message": "This MCP content type is not supported by the Bridge dispatcher",
                }
            )
        else:
            content.append(
                {
                    "type": str(block_type or "unknown"),
                    "supported": False,
                    "message": "Unsupported MCP content block",
                }
            )

    structured = getattr(result, "structuredContent", None)
    if structured is None:
        structured = getattr(result, "structured_content", None)
    payload: dict[str, Any] = {"is_error": bool(getattr(result, "isError", False)), "content": content}
    if structured is not None:
        payload["structured_content"] = structured
    return redact_value(payload)
