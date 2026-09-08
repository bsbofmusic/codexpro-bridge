"""Facade for Bridge 2.1 Work Runtime operations."""

from __future__ import annotations

from typing import Any

from codexpro_bridge.core.config import BridgeConfig
from codexpro_bridge.core.errors import BridgeError
from .artifacts import ArtifactOps
from .audits import AuditOps
from .checkpoints import CheckpointOps
from .common import WORK_OPERATIONS
from .db import WorkStore
from .projects import ProjectOps
from .tasks import TaskOps


class WorkRuntime(ProjectOps, TaskOps, CheckpointOps, AuditOps, ArtifactOps):
    version = "1.0.0"

    def __init__(self, config: BridgeConfig):
        self.config = config
        self.store = WorkStore(config.work_db_path)

    def health(self) -> dict[str, Any]:
        return self.store.health()

    def dispatch(self, operation: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
        if operation not in WORK_OPERATIONS:
            raise BridgeError("unsupported_work_operation", "Unsupported Work Runtime operation")
        handler = getattr(self, operation.replace(".", "_"), None)
        if handler is None:
            raise BridgeError("unsupported_work_operation", "Unsupported Work Runtime operation")
        return handler(**dict(arguments or {}))
