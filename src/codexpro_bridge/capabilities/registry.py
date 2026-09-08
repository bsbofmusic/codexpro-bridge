"""Capability-module registry and public tool-surface governance."""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from typing import Any, Callable

from codexpro_bridge.core.errors import BridgeError

Health = Callable[[], dict[str, Any]]
Shutdown = Callable[[], None]
Install = Callable[[Any], None]


@dataclass(frozen=True, slots=True)
class ModuleDescriptor:
    """Metadata and installer for one independently controlled capability module."""

    module_id: str
    version: str
    tools: tuple[str, ...]
    health: Health
    install: Install
    shutdown: Shutdown | None = None


class CapabilityRegistry:
    """Own module selection, mounting, collision checks and residue detection."""

    def __init__(self, enabled_modules: tuple[str, ...] | None = None) -> None:
        self._modules: dict[str, ModuleDescriptor] = {}
        self._tool_owner: dict[str, str] = {}
        self._enabled_selector = None if enabled_modules is None else tuple(dict.fromkeys(enabled_modules))
        self._tool_source: Callable[[], set[str]] | None = None

    def register(self, descriptor: ModuleDescriptor) -> None:
        if not descriptor.module_id or descriptor.module_id in self._modules:
            raise BridgeError("registry_collision", "Bridge capability module is duplicated")
        if not descriptor.tools:
            raise BridgeError("registry_invalid", "Bridge capability module must declare at least one public tool")
        if len(set(descriptor.tools)) != len(descriptor.tools):
            raise BridgeError("registry_collision", "Bridge capability module declares a duplicate tool name")
        for tool in descriptor.tools:
            if not tool:
                raise BridgeError("registry_invalid", "Bridge public tool name cannot be empty")
            if tool in self._tool_owner:
                owner = self._tool_owner[tool]
                raise BridgeError(
                    "registry_collision",
                    f"Bridge public tool name is duplicated by modules {owner!r} and {descriptor.module_id!r}",
                )
        self._modules[descriptor.module_id] = descriptor
        self._tool_owner.update({tool: descriptor.module_id for tool in descriptor.tools})

    def _selected_module_ids(self) -> tuple[str, ...]:
        if self._enabled_selector is None:
            return tuple(self._modules)
        unknown = [module_id for module_id in self._enabled_selector if module_id not in self._modules]
        if unknown:
            raise BridgeError("unknown_module", f"Unknown Bridge module selector: {', '.join(unknown)}")
        selected = set(self._enabled_selector)
        return tuple(module_id for module_id in self._modules if module_id in selected)

    def is_enabled(self, module_id: str) -> bool:
        return module_id in set(self._selected_module_ids())

    def enabled_module_ids(self) -> tuple[str, ...]:
        return self._selected_module_ids()

    def disabled_module_ids(self) -> tuple[str, ...]:
        enabled = set(self._selected_module_ids())
        return tuple(module_id for module_id in self._modules if module_id not in enabled)

    def expected_tools(self) -> tuple[str, ...]:
        enabled = set(self._selected_module_ids())
        return tuple(
            tool
            for module_id, descriptor in self._modules.items()
            if module_id in enabled
            for tool in descriptor.tools
        )

    def mount(self, server: Any) -> None:
        """Install enabled modules and fail closed on missing/orphaned tool registrations."""

        selected = set(self._selected_module_ids())
        for module_id, descriptor in self._modules.items():
            if module_id not in selected:
                continue
            before = set(server._tool_manager._tools)
            descriptor.install(server)
            after = set(server._tool_manager._tools)
            added = after - before
            declared = set(descriptor.tools)
            if added != declared:
                missing = sorted(declared - added)
                orphan = sorted(added - declared)
                raise BridgeError(
                    "registry_surface_mismatch",
                    f"Module {module_id!r} tool registration mismatch; missing={missing}, orphan={orphan}",
                )

        self._tool_source = lambda: set(server._tool_manager._tools)
        audit = self.surface_audit()
        if not audit["consistent"]:
            raise BridgeError(
                "registry_surface_mismatch",
                f"Bridge tool surface has residue; missing={audit['missing_tools']}, orphan={audit['orphan_tools']}",
            )

    def manifest(self) -> list[dict[str, Any]]:
        enabled = set(self._selected_module_ids())
        return [
            {
                "module": item.module_id,
                "version": item.version,
                "enabled": item.module_id in enabled,
                "tools": list(item.tools),
            }
            for item in self._modules.values()
        ]

    def health(self) -> list[dict[str, Any]]:
        enabled = set(self._selected_module_ids())
        results: list[dict[str, Any]] = []
        for descriptor in self._modules.values():
            if descriptor.module_id not in enabled:
                continue
            try:
                health = descriptor.health()
            except Exception:
                health = {"ok": False, "error": {"code": "health_unavailable", "message": "Capability health check failed"}}
            results.append({"module": descriptor.module_id, "version": descriptor.version, "health": health})
        return results

    def surface_audit(self) -> dict[str, Any]:
        expected = set(self.expected_tools())
        actual = expected if self._tool_source is None else set(self._tool_source())
        missing = sorted(expected - actual)
        orphan = sorted(actual - expected)
        fingerprint_input = "\n".join(sorted(actual)).encode("utf-8")
        return {
            "enabled_modules": list(self.enabled_module_ids()),
            "disabled_modules": list(self.disabled_module_ids()),
            "module_count": len(self.enabled_module_ids()),
            "tool_count": len(actual),
            "expected_tool_count": len(expected),
            "missing_tools": missing,
            "orphan_tools": orphan,
            "consistent": not missing and not orphan,
            "surface_fingerprint": hashlib.sha256(fingerprint_input).hexdigest(),
        }

    def shutdown(self) -> None:
        enabled = set(self._selected_module_ids())
        for descriptor in reversed(tuple(self._modules.values())):
            if descriptor.module_id not in enabled or descriptor.shutdown is None:
                continue
            try:
                descriptor.shutdown()
            except Exception:
                continue
