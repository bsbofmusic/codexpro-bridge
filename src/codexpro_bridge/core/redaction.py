"""Deterministic redaction and output bounding for every public result."""

from __future__ import annotations

import json
import re
from collections.abc import Mapping, Sequence
from typing import Any

REDACTED = "[REDACTED]"

_SENSITIVE_KEY = re.compile(
    r"(?:^|[_-])(api[_-]?key|token|password|passwd|secret|credential|cookie|"
    r"authorization|private[_-]?key)(?:$|[_-])",
    re.IGNORECASE,
)
_PRIVATE_KEY = re.compile(
    r"-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----",
    re.IGNORECASE | re.DOTALL,
)
_BEARER = re.compile(r"(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{8,}")
_KEY_LIKE = re.compile(
    r"\b(?:sk|xai|ghp|github_pat|cpk|hf|AIza)[_-][A-Za-z0-9._-]{12,}\b",
    re.IGNORECASE,
)
_ASSIGNMENT = re.compile(
    r"(?i)(\b(?:api[_-]?key|token|password|passwd|secret|credential|"
    r"authorization)\b\s*[:=]\s*)([^\s,;\]}]{4,}|\"[^\"]*\"|'[^']*')"
)
_QUERY_SECRET = re.compile(
    r"(?i)([?&](?:[^=&]*(?:token|key|secret|password)[^=&]*)=)[^&#\s]+"
)


def redact_text(value: str) -> str:
    """Redact credential-shaped text without logging the original value."""

    text = _PRIVATE_KEY.sub(REDACTED, str(value))
    text = _BEARER.sub(f"Bearer {REDACTED}", text)
    text = _KEY_LIKE.sub(REDACTED, text)
    text = _ASSIGNMENT.sub(lambda match: match.group(1) + REDACTED, text)
    return _QUERY_SECRET.sub(lambda match: match.group(1) + REDACTED, text)


def redact_value(value: Any, *, _key: str | None = None) -> Any:
    """Recursively redact secret-bearing keys and credential-shaped strings."""

    if _key and _SENSITIVE_KEY.search(_key):
        return REDACTED
    if isinstance(value, str):
        return redact_text(value)
    if isinstance(value, Mapping):
        return {
            str(key): redact_value(item, _key=str(key))
            for key, item in value.items()
        }
    if isinstance(value, Sequence) and not isinstance(value, (bytes, bytearray)):
        return [redact_value(item) for item in value]
    if isinstance(value, (bytes, bytearray)):
        return f"[binary data: {len(value)} bytes]"
    return value


def bounded_result(value: Any, max_chars: int) -> Any:
    """Return a JSON-safe, redacted value whose serialized size is bounded."""

    safe = redact_value(value)
    serialized = json.dumps(safe, ensure_ascii=False, default=str)
    if len(serialized) <= max_chars:
        return safe
    preview_budget = max(256, max_chars - 256)
    return {
        "ok": False,
        "error": {
            "code": "output_truncated",
            "message": "Result exceeded the Bridge output limit",
        },
        "original_chars": len(serialized),
        "preview": redact_text(serialized[:preview_budget]),
    }


def safe_exception_message(exc: BaseException, limit: int = 500) -> str:
    text = redact_text(str(exc)).replace("\n", " ").strip()
    if not text:
        text = type(exc).__name__
    return text[:limit]
