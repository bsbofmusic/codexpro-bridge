"""Small registry that keeps Bridge modules independent and observable."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Callable

from codexpro_bridge.core.errors import BridgeError


Health = Callable[[], dict[str, Any]]
Shutdown = Callable[[], None]


@dataclass(frozen=True, slots=True)
class ModuleDescriptor:
    """Stable metadata for one independently health-checked capability."""

    module_id: str
    version: str
    tools: tuple[str, ...]
    health: Health
    shutdown: Shutdown | None = None


class CapabilityRegistry:
    """Register fixed modules and reject tool/module collisions at startup."""

    def __init__(self) -> None:
        self._modules: dict[str, ModuleDescriptor] = {}
        self._tool_owner: dict[str, str] = {}

    def register(self, descriptor: ModuleDescriptor) -> None:
        if not descriptor.module_id or descriptor.module_id in self._modules:
            raise BridgeError("registry_collision", "Bridge capability module is duplicated")
        for tool in descriptor.tools:
            if tool in self._tool_owner:
                raise BridgeError("registry_collision", "Bridge public tool name is duplicated")
        self._modules[descriptor.module_id] = descriptor
        self._tool_owner.update({tool: descriptor.module_id for tool in descriptor.tools})

    def manifest(self) -> list[dict[str, Any]]:
        return [
            {
                "module": item.module_id,
                "version": item.version,
                "tools": list(item.tools),
            }
            for item in self._modules.values()
        ]

    def health(self) -> list[dict[str, Any]]:
        results: list[dict[str, Any]] = []
        for descriptor in self._modules.values():
            try:
                health = descriptor.health()
            except Exception:
                health = {"ok": False, "error": {"code": "health_unavailable", "message": "Capability health check failed"}}
            results.append(
                {
                    "module": descriptor.module_id,
                    "version": descriptor.version,
                    "health": health,
                }
            )
        return results

    def shutdown(self) -> None:
        # Reverse registration order keeps dependent modules alive while their
        # dependencies complete their own cleanup.
        for descriptor in reversed(tuple(self._modules.values())):
            if descriptor.shutdown is None:
                continue
            try:
                descriptor.shutdown()
            except Exception:
                continue
