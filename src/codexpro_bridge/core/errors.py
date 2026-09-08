"""Stable, non-sensitive errors returned by the public Bridge tools."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass(slots=True)
class BridgeError(Exception):
    """An expected Bridge failure with a stable public error code."""

    code: str
    message: str
    details: dict[str, Any] | None = None

    def __str__(self) -> str:
        return self.message

    def as_dict(self) -> dict[str, Any]:
        result: dict[str, Any] = {
            "ok": False,
            "error": {"code": self.code, "message": self.message},
        }
        if self.details:
            result["error"]["details"] = self.details
        return result
