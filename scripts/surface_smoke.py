#!/usr/bin/env python3
from __future__ import annotations

import argparse
import asyncio
import json
from pathlib import Path

import httpx2
from mcp import ClientSession
from mcp.client.streamable_http import streamable_http_client


def _structured(result: object | None) -> dict[str, object]:
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


async def run(url: str, token: str | None = None) -> dict[str, object]:
    headers = {"Authorization": f"Bearer {token}"} if token else None
    client = httpx2.AsyncClient(headers=headers, timeout=20.0) if headers else None
    async with streamable_http_client(url, http_client=client, terminate_on_close=False) as (read_stream, write_stream):
        async with ClientSession(read_stream, write_stream, read_timeout_seconds=20) as session:
            await session.initialize()
            listed = await session.list_tools()
            names = sorted(tool.name for tool in listed.tools)
            doctor = None
            if "codexpro_bridge_doctor" in names:
                doctor = await session.call_tool("codexpro_bridge_doctor", arguments={"deep": False})
            doctor_data = _structured(doctor)
            surface = doctor_data.get("surface") if isinstance(doctor_data, dict) else None
            return {
                "tool_count": len(names),
                "tools": names,
                "surface": surface,
                "consistent": bool(surface.get("consistent")) if isinstance(surface, dict) else None,
                "missing_tools": surface.get("missing_tools") if isinstance(surface, dict) else None,
                "orphan_tools": surface.get("orphan_tools") if isinstance(surface, dict) else None,
                "surface_fingerprint": surface.get("surface_fingerprint") if isinstance(surface, dict) else None,
            }


def _read_token(path: str | None) -> str | None:
    if not path:
        return None
    raw = Path(path).read_text(encoding="utf-8").strip()
    if not raw:
        raise SystemExit("token file is empty")
    if "=" not in raw:
        return raw
    values: dict[str, str] = {}
    for line in raw.splitlines():
        if "=" in line and not line.lstrip().startswith("#"):
            key, value = line.split("=", 1)
            values[key.strip()] = value.strip().strip('"').strip("'")
    token = values.get("CODEXPRO_BRIDGE_HTTP_TOKEN") or values.get("CODEXPRO_HTTP_TOKEN")
    if not token:
        raise SystemExit("token file does not contain a Bridge HTTP token")
    return token


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://127.0.0.1:18787/mcp")
    parser.add_argument("--token-file")
    args = parser.parse_args()
    result = asyncio.run(run(args.url, token=_read_token(args.token_file)))
    print(json.dumps(result, ensure_ascii=False, sort_keys=True))
    if result.get("consistent") is False:
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
