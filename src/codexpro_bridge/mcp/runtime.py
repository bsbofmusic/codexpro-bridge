"""A minimal, fail-closed adapter around Hermes's Native MCP runtime."""

from __future__ import annotations

import copy
import hashlib
import inspect
from pathlib import Path
from types import ModuleType
from typing import Any

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from codexpro_bridge.core.redaction import bounded_result, redact_value


_SCHEMA_KEYS = frozenset(
    {
        "type",
        "properties",
        "required",
        "items",
        "enum",
        "default",
        "const",
        "format",
        "minimum",
        "maximum",
        "exclusiveMinimum",
        "exclusiveMaximum",
        "minLength",
        "maxLength",
        "minItems",
        "maxItems",
        "additionalProperties",
        "oneOf",
        "anyOf",
        "allOf",
    }
)


def _project_input_schema(value: Any, key: str | None = None) -> Any:
    """Keep argument structure while dropping upstream prose/instructions."""

    if isinstance(value, dict):
        if key == "properties":
            return {str(name): _project_input_schema(item) for name, item in value.items()}
        return {
            str(name): _project_input_schema(item, str(name))
            for name, item in value.items()
            if str(name) in _SCHEMA_KEYS
        }
    if isinstance(value, list):
        return [_project_input_schema(item) for item in value]
    return value


def _enabled(value: Any) -> bool:
    if isinstance(value, str):
        return value.strip().casefold() not in {"0", "false", "no", "off"}
    return value is not False


def _tool_allowed(config: dict[str, Any], tool_name: str) -> bool:
    tools = config.get("tools") or {}
    if not isinstance(tools, dict):
        return True
    include = tools.get("include") or []
    exclude = tools.get("exclude") or []
    if isinstance(include, str):
        include = [include]
    if isinstance(exclude, str):
        exclude = [exclude]
    include_names = {str(item) for item in include}
    exclude_names = {str(item) for item in exclude}
    return tool_name in include_names if include_names else tool_name not in exclude_names


class HermesMcpRuntime:
    """Connect, list, and shut down Hermes MCP servers from in-memory config."""

    _REQUIRED = ("_load_mcp_config", "register_mcp_servers", "get_mcp_status", "shutdown_mcp_servers")

    def __init__(self, config: BridgeConfig, mcp_module: ModuleType | Any | None = None):
        self.config = config
        self._mcp_module = mcp_module
        self._configs: dict[str, dict[str, Any]] = {}
        self._compatibility: dict[str, Any] | None = None
        self._config_generation = 0

    def _module(self) -> Any:
        if self._mcp_module is not None:
            return self._mcp_module
        try:
            from tools import mcp_tool
        except Exception as exc:
            raise BridgeError("mcp_unavailable", "Hermes Native MCP runtime is unavailable") from exc
        self._mcp_module = mcp_tool
        return mcp_tool

    def compatibility(self) -> dict[str, Any]:
        """Validate exactly the Hermes API surface the bridge depends on."""

        if self._compatibility is not None:
            return dict(self._compatibility)
        module = self._module()
        missing = [name for name in self._REQUIRED if not callable(getattr(module, name, None))]
        if missing:
            raise BridgeError("mcp_incompatible", "Hermes Native MCP compatibility gate failed")
        try:
            source_file = inspect.getsourcefile(module if inspect.ismodule(module) else type(module))
            source = Path(source_file or "")
            source_sha256 = hashlib.sha256(source.read_bytes()).hexdigest() if source.is_file() else None
            signatures = {name: str(inspect.signature(getattr(module, name))) for name in self._REQUIRED}
        except (OSError, TypeError, ValueError) as exc:
            raise BridgeError("mcp_incompatible", "Hermes Native MCP compatibility gate failed") from exc
        self._compatibility = {
            "ok": True,
            "source_sha256": source_sha256,
            "symbols": sorted(self._REQUIRED),
            "signatures": signatures,
        }
        return dict(self._compatibility)

    def dispatch_compatibility(self) -> None:
        """Gate the private, single-call adapter before it reaches a session."""

        module = self._module()
        self.compatibility()
        runner = getattr(module, "_run_on_mcp_loop", None)
        if (
            not callable(runner)
            or not hasattr(module, "_lock")
            or not hasattr(module, "_servers")
            or not callable(getattr(module, "reconnect_mcp_server", None))
        ):
            raise BridgeError("mcp_incompatible", "Hermes MCP single-call adapter is unavailable")
        try:
            parameters = inspect.signature(runner).parameters
        except (TypeError, ValueError) as exc:
            raise BridgeError("mcp_incompatible", "Hermes MCP single-call adapter is unavailable") from exc
        if "coro_or_factory" not in parameters or "timeout" not in parameters:
            raise BridgeError("mcp_incompatible", "Hermes MCP single-call adapter is incompatible")

    def _overlay(self) -> dict[str, dict[str, Any]]:
        module = self._module()
        self.compatibility()
        try:
            configured = module._load_mcp_config()
        except Exception as exc:
            raise BridgeError("mcp_unavailable", "Hermes MCP configuration is unavailable") from exc
        if not isinstance(configured, dict):
            raise BridgeError("mcp_incompatible", "Hermes MCP configuration is invalid")

        overlay: dict[str, dict[str, Any]] = {}
        for name, raw in configured.items():
            if not isinstance(name, str) or not isinstance(raw, dict):
                continue
            if name.casefold() == "hindsight":
                continue
            item = copy.deepcopy(raw)
            sampling = item.get("sampling") if isinstance(item.get("sampling"), dict) else {}
            elicitation = item.get("elicitation") if isinstance(item.get("elicitation"), dict) else {}
            sampling["enabled"] = False
            elicitation["enabled"] = False
            item["sampling"] = sampling
            item["elicitation"] = elicitation
            if name.casefold() == "memos-api-mcp":
                # Keep Hermes's outer wrapper so its protected secret loader and
                # default-recall proxy remain authoritative.  Only replace the
                # proxy's npx/@latest child with the already-installed binary.
                environment = item.get("env") if isinstance(item.get("env"), dict) else {}
                environment["MEMOS_UPSTREAM_MCP_COMMAND"] = str(self.config.memos_command)
                environment["MEMOS_UPSTREAM_MCP_ARGS_JSON"] = "[]"
                item["env"] = environment
            overlay[name] = item
        self._configs = overlay
        self._config_generation += 1
        return overlay

    def start(self) -> list[str]:
        overlay = self._overlay()
        try:
            registered = self._module().register_mcp_servers(overlay)
        except Exception as exc:
            raise BridgeError("mcp_unavailable", "Hermes MCP registration failed") from exc
        return [str(name) for name in registered] if isinstance(registered, list) else []

    def server_config(self, server: str) -> dict[str, Any] | None:
        if not self._configs:
            self._overlay()
        item = self._configs.get(server)
        return copy.deepcopy(item) if item is not None else None

    def is_allowed(self, server: str, tool: str) -> bool:
        if server.casefold() == "hindsight":
            return False
        config = self.server_config(server)
        return bool(config and _enabled(config.get("enabled", True)) and _tool_allowed(config, tool))

    def list_tools(
        self,
        *,
        server: str | None = None,
        query: str | None = None,
        include_schema: bool = False,
        offset: int = 0,
        limit: int = 100,
    ) -> dict[str, Any]:
        if server is not None and (not isinstance(server, str) or not server or len(server) > 128):
            raise BridgeError("invalid_server", "MCP server filter is invalid")
        if query is not None and (not isinstance(query, str) or len(query) > 256):
            raise BridgeError("invalid_query", "MCP tool query is invalid")
        if not isinstance(offset, int) or offset < 0:
            raise BridgeError("invalid_pagination", "offset must be a non-negative integer")
        if not isinstance(limit, int) or not 1 <= limit <= 100:
            raise BridgeError("invalid_pagination", "limit must be between 1 and 100")
        self.start()
        module = self._module()
        try:
            from tools.registry import registry
        except Exception as exc:
            raise BridgeError("mcp_unavailable", "Hermes MCP registry is unavailable") from exc

        tools: list[dict[str, Any]] = []
        prefix = getattr(module, "mcp_prefixed_tool_name", None)
        if not callable(prefix):
            raise BridgeError("mcp_incompatible", "Hermes MCP tool naming helper is unavailable")
        for server_name, config in self._configs.items():
            if server is not None and server_name != server:
                continue
            if not _enabled(config.get("enabled", True)):
                continue
            try:
                # Hermes owns this exact toolset naming contract; querying it
                # prevents a same-looking tool from another server leaking in.
                tool_names = registry.get_tool_names_for_toolset(f"mcp-{server_name}")
            except Exception as exc:
                raise BridgeError("mcp_unavailable", "Hermes MCP registry is unavailable") from exc
            marker = prefix(server_name, "")
            for full_name in tool_names:
                schema = registry.get_schema(full_name)
                if not isinstance(schema, dict):
                    continue
                raw_name = full_name[len(marker) :] if full_name.startswith(marker) else None
                if raw_name and _tool_allowed(config, raw_name):
                    if query and query.casefold() not in raw_name.casefold() and query.casefold() not in full_name.casefold():
                        continue
                    item: dict[str, Any] = {"server": server_name, "tool": raw_name, "name": full_name}
                    if include_schema:
                        parameters = schema.get("parameters", schema.get("inputSchema", schema.get("input_schema", {})))
                        item["input_schema"] = _project_input_schema(parameters)
                    tools.append(item)
        tools.sort(key=lambda item: (item["server"], item["tool"]))
        total = len(tools)
        page = tools[offset : offset + limit]
        return bounded_result(
            {
                "ok": True,
                "tools": redact_value(page),
                "count": total,
                "offset": offset,
                "limit": limit,
                "next_offset": offset + len(page) if offset + len(page) < total else None,
                "schemas_included": bool(include_schema),
            },
            self.config.max_output_chars,
        )

    def status(self) -> dict[str, Any]:
        self._overlay()
        try:
            statuses = self._module().get_mcp_status()
        except Exception as exc:
            raise BridgeError("mcp_unavailable", "Hermes MCP status is unavailable") from exc
        visible = [entry for entry in statuses if isinstance(entry, dict) and str(entry.get("name", "")).casefold() != "hindsight"]
        return bounded_result(
            {"ok": True, "servers": redact_value(visible), "config_generation": self._config_generation},
            self.config.max_output_chars,
        )

    def shutdown(self) -> None:
        try:
            self._module().shutdown_mcp_servers()
        except Exception:
            # Shutdown is best-effort; never reflect an internal transport error.
            return
