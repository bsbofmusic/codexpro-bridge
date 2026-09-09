#!/usr/bin/env python3
from __future__ import annotations

import argparse
import asyncio
import json
import re
import time
from pathlib import Path

import httpx2
from mcp import ClientSession
from mcp.client.streamable_http import streamable_http_client


def read_token(path: Path) -> str:
    raw = path.read_text(encoding="utf-8").strip()
    values: dict[str, str] = {}
    for line in raw.splitlines():
        if "=" in line and not line.lstrip().startswith("#"):
            key, value = line.split("=", 1)
            values[key.strip()] = value.strip().strip('"').strip("'")
    token = values.get("CODEXPRO_BRIDGE_HTTP_TOKEN") or values.get("CODEXPRO_HTTP_TOKEN")
    if not token and "=" not in raw:
        token = raw
    if not token:
        raise RuntimeError("Bridge token unavailable")
    return token


def structured(result: object) -> dict[str, object]:
    value = getattr(result, "structuredContent", None) or getattr(result, "structured_content", None)
    if isinstance(value, dict):
        return value
    for block in getattr(result, "content", None) or []:
        text = getattr(block, "text", None)
        if not text:
            continue
        try:
            parsed = json.loads(text)
        except Exception:
            continue
        if isinstance(parsed, dict):
            return parsed
    return {}


async def run(url: str, token: str) -> dict[str, object]:
    headers = {"Authorization": f"Bearer {token}"}
    client = httpx2.AsyncClient(headers=headers, timeout=120.0)
    note = f"__codexpro_bridge_v2_e2e_{int(time.time())}.md"
    note_path = Path("/home/agent/obsidian-vault") / note
    kb_removed = False
    try:
        async with streamable_http_client(url, http_client=client, terminate_on_close=True) as (read_stream, write_stream):
            async with ClientSession(read_stream, write_stream, read_timeout_seconds=120) as session:
                await session.initialize()

                async def call(name: str, arguments: dict[str, object]) -> dict[str, object]:
                    result = await session.call_tool(name, arguments=arguments)
                    if getattr(result, "isError", False):
                        raise RuntimeError(f"{name} returned MCP error")
                    data = structured(result)
                    if data.get("ok") is False:
                        raise RuntimeError(f"{name} returned Bridge error")
                    return data

                await call(
                    "codexpro_bridge_memory_call",
                    {"source": "obsidian", "tool": "write_note", "arguments": {"path": note, "content": "bridge-v2 e2e", "overwrite": False}},
                )
                await call(
                    "codexpro_bridge_memory_call",
                    {"source": "obsidian", "tool": "read_note", "arguments": {"path": note}},
                )
                await call(
                    "codexpro_bridge_memory_call",
                    {"source": "obsidian", "tool": "write_note", "arguments": {"path": note, "content": "bridge-v2 e2e updated", "overwrite": True}},
                )
                await call(
                    "codexpro_bridge_memory_call",
                    {"source": "obsidian", "tool": "delete_note", "arguments": {"path": note}},
                )
                if note_path.exists():
                    raise RuntimeError("Obsidian audit note residue remains")

                created = await call(
                    "codexpro_bridge_memory_call",
                    {
                        "source": "memos",
                        "tool": "create_knowledge_base",
                        "arguments": {
                            "knowledgebase_name": f"__bridge_v2_e2e_{int(time.time())}__",
                            "knowledgebase_description": "temporary reversible Bridge 2.x end-to-end permission audit",
                        },
                    },
                )
                blob = json.dumps(created, ensure_ascii=False)
                candidates = re.findall(r"[0-9a-fA-F]{8}-[0-9a-fA-F-]{20,}|\b\d{6,}\b", blob)
                if not candidates:
                    raise RuntimeError("Could not safely extract disposable MemOS KB id")
                await call(
                    "codexpro_bridge_memory_call",
                    {"source": "memos", "tool": "remove_knowledge_base", "arguments": {"knowledgebase_id": candidates[0]}},
                )
                kb_removed = True

        return {
            "obsidian_bridge_crud": "PASS",
            "obsidian_residue": note_path.exists(),
            "memos_bridge_create_remove": "PASS" if kb_removed else "FAIL",
            "full_permission_end_to_end": True,
        }
    finally:
        if note_path.exists():
            note_path.unlink()
        await client.aclose()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://127.0.0.1:18787/mcp")
    parser.add_argument("--token-file", required=True)
    args = parser.parse_args()
    result = asyncio.run(run(args.url, read_token(Path(args.token_file))))
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
