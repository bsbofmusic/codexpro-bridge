"""Direct shared-memory capability for Obsidian and MemOS MCP servers."""

from .module import SharedMemoryModule
from .runtime import SharedMemoryRuntime

__all__ = ["SharedMemoryModule", "SharedMemoryRuntime"]
