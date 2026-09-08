#!/usr/bin/env python3
from __future__ import annotations

import argparse
import asyncio
import json
from pathlib import Path

import httpx2
from mcp import ClientSession
from mcp.client.streamable_http import streamable_http_client


async def run(url: str, token: str | None = None, *, memory_semantics: bool = False) -> dict[str, object]:
    headers = {"Authorization": f"Bearer {token}"} if token else None
    client = httpx2.AsyncClient(headers=headers, timeout=60.0) if headers else None
    async with streamable_http_client(url, http_client=client, terminate_on_close=False) as (read_stream, write_stream):
        async with ClientSession(read_stream, write_stream, read_timeout_seconds=60) as session:
            await session.initialize()
            listed = await session.list_tools()
            names = sorted(tool.name for tool in listed.tools)
            available = set(names)

            async def call_if_present(name: str, arguments: dict[str, object]) -> object | None:
                if name not in available:
                    return None
                return await session.call_tool(name, arguments=arguments)

            doctor = await call_if_present("codexpro_bridge_doctor", {"deep": False})
            route = await call_if_present(
                "codexpro_bridge_route_and_recall",
                {"task": "Audit VPS deployment transition safety", "include_memory": False, "skill_limit": 3},
            )
            mcp_status = await call_if_present("codexpro_bridge_mcp_status", {})
            memory_list = await call_if_present(
                "codexpro_bridge_memory_list",
                {"include_schema": False, "limit": 100},
            )
            work_list = await call_if_present(
                "codexpro_bridge_work",
                {"operation": "task.list", "arguments": {"limit": 1}},
            )

            history = None
            simple = None
            if memory_semantics and "codexpro_bridge_route_and_recall" in available and "codexpro_bridge_memory_list" in available:
                history = await session.call_tool(
                    "codexpro_bridge_route_and_recall",
                    arguments={"task": "Leah之前怎么说的？", "memory_limit": 2, "conversation_first_message": "Bridge v2 acceptance"},
                )
                simple = await session.call_tool(
                    "codexpro_bridge_route_and_recall",
                    arguments={"task": "2+2", "memory_limit": 2, "conversation_first_message": "Bridge v2 acceptance"},
                )

            def structured(result: object | None) -> dict[str, object]:
                if result is None:
                    return {}
                value = getattr(result, "structuredContent", None) or getattr(result, "structured_content", None)
                if isinstance(value, dict):
                    return value
                for block in getattr(result, "content", None) or []:
                    text = getattr(block, "text", None)
                    if text:
                        try:
                            parsed = json.loads(text)
                        except Exception:
                            continue
                        if isinstance(parsed, dict):
                            return parsed
                return {}

            def is_error(result: object | None) -> bool | None:
                if result is None:
                    return None
                return bool(getattr(result, "isError", False))

            history_data = structured(history)
            simple_data = structured(simple)
            doctor_data = structured(doctor)
            return {
                "tool_count": len(names),
                "tools": names,
                "surface": doctor_data.get("surface") if isinstance(doctor_data, dict) else None,
                "doctor_is_error": is_error(doctor),
                "route_is_error": is_error(route),
                "mcp_status_is_error": is_error(mcp_status),
                "memory_list_is_error": is_error(memory_list),
                "work_list_is_error": is_error(work_list),
                "history_memory_attempted": bool(history_data.get("memory", {}).get("attempted")) if isinstance(history_data.get("memory"), dict) else None,
                "history_memory_ok": bool(history_data.get("memory", {}).get("ok")) if isinstance(history_data.get("memory"), dict) else None,
                "simple_memory_attempted": bool(simple_data.get("memory", {}).get("attempted")) if isinstance(simple_data.get("memory"), dict) else None,
            }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://127.0.0.1:18788/mcp")
    parser.add_argument("--token-file")
    parser.add_argument("--memory-semantics", action="store_true")
    args = parser.parse_args()
    token = None
    if args.token_file:
        raw = Path(args.token_file).read_text(encoding="utf-8").strip()
        if not raw:
            raise SystemExit("token file is empty")
        if "=" in raw:
            values = {}
            for line in raw.splitlines():
                if "=" in line and not line.lstrip().startswith("#"):
                    key, value = line.split("=", 1)
                    values[key.strip()] = value.strip().strip('"').strip("'")
            token = values.get("CODEXPRO_BRIDGE_HTTP_TOKEN") or values.get("CODEXPRO_HTTP_TOKEN")
        else:
            token = raw
        if not token:
            raise SystemExit("token file does not contain a Bridge HTTP token")
    result = asyncio.run(run(args.url, token=token, memory_semantics=args.memory_semantics))
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
