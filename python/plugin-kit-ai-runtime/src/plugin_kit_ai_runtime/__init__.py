"""Official Python runtime helpers for plugin-kit-ai executable plugins."""

from __future__ import annotations

import json
import sys
from typing import Any, Callable, Iterable, Optional

__version__ = "0.0.0.dev0"

#: JSON-shaped payload used by the Python runtime helpers.
JSONMap = dict[str, Any]
#: Handler signature for Claude hooks that return a JSON object or ``None``.
ClaudeHandler = Callable[[JSONMap], Optional[JSONMap]]
#: Handler signature for Codex events that return an exit code or ``None``.
CodexHandler = Callable[[JSONMap], Optional[int]]

#: Stable Claude hook names supported by the public Python runtime lane.
CLAUDE_STABLE_HOOKS = (
    "Stop",
    "PreToolUse",
    "UserPromptSubmit",
)

#: Extended Claude hook names exposed by the beta Python runtime lane.
CLAUDE_EXTENDED_HOOKS = (
    "Stop",
    "PreToolUse",
    "UserPromptSubmit",
    "SessionStart",
    "SessionEnd",
    "Notification",
    "PostToolUse",
    "PostToolUseFailure",
    "PermissionRequest",
    "SubagentStart",
    "SubagentStop",
    "PreCompact",
    "Setup",
    "TeammateIdle",
    "TaskCompleted",
    "ConfigChange",
    "WorktreeCreate",
    "WorktreeRemove",
)


def allow() -> JSONMap:
    """Return the empty JSON object expected by Claude for an allow response."""
    return {}


def continue_() -> int:
    """Return exit code ``0`` for Codex handlers that want normal continuation."""
    return 0


class ClaudeApp:
    """Minimal Claude hook app that dispatches supported hook names to handlers."""

    def __init__(self, allowed_hooks: Iterable[str], usage: str):
        """Create a Claude runtime app.

        Args:
            allowed_hooks: Hook names that this binary accepts on argv.
            usage: Usage string printed when the invocation is invalid.
        """
        self._allowed_hooks = tuple(allowed_hooks)
        self._allowed_hook_set = set(self._allowed_hooks)
        self._usage = usage
        self._handlers: dict[str, ClaudeHandler] = {}

    def on(self, hook_name: str) -> Callable[[ClaudeHandler], ClaudeHandler]:
        """Return a decorator that registers a handler for ``hook_name``."""
        def register(handler: ClaudeHandler) -> ClaudeHandler:
            self._handlers[hook_name] = handler
            return handler

        return register

    def on_stop(self, handler: ClaudeHandler) -> ClaudeHandler:
        """Register a handler for the ``Stop`` hook."""
        return self.on("Stop")(handler)

    def on_pre_tool_use(self, handler: ClaudeHandler) -> ClaudeHandler:
        """Register a handler for the ``PreToolUse`` hook."""
        return self.on("PreToolUse")(handler)

    def on_user_prompt_submit(self, handler: ClaudeHandler) -> ClaudeHandler:
        """Register a handler for the ``UserPromptSubmit`` hook."""
        return self.on("UserPromptSubmit")(handler)

    def run(self) -> int:
        """Dispatch the current process invocation and return the exit code."""
        if len(sys.argv) < 2:
            sys.stderr.write(f"usage: {self._usage}\n")
            return 1

        hook_name = sys.argv[1]
        if hook_name not in self._allowed_hook_set:
            sys.stderr.write(f"usage: {self._usage}\n")
            return 1

        handler = self._handlers.get(hook_name)
        if handler is None:
            sys.stderr.write(f"no handler registered for {hook_name}\n")
            return 1

        event = json.load(sys.stdin)
        response = handler(event) or allow()
        if response:
            sys.stdout.write(json.dumps(response))
        else:
            sys.stdout.write("{}")
        return 0


class CodexApp:
    """Minimal Codex app that dispatches the ``notify`` event to a handler."""

    def __init__(self):
        """Create a Codex runtime app with no registered notify handler."""
        self._notify_handler: Optional[CodexHandler] = None

    def on_notify(self, handler: CodexHandler) -> CodexHandler:
        """Register a handler for the Codex ``notify`` event."""
        self._notify_handler = handler
        return handler

    def run(self) -> int:
        """Dispatch the current process invocation and return the exit code."""
        if len(sys.argv) < 2 or sys.argv[1] != "notify":
            sys.stderr.write("usage: main.py notify <json-payload>\n")
            return 1

        if len(sys.argv) < 3:
            sys.stderr.write("missing notify payload\n")
            return 1

        if self._notify_handler is None:
            sys.stderr.write("no handler registered for notify\n")
            return 1

        event = json.loads(sys.argv[2])
        result = self._notify_handler(event)
        if result is None:
            return continue_()
        return int(result)


__all__ = [
    "CLAUDE_EXTENDED_HOOKS",
    "CLAUDE_STABLE_HOOKS",
    "ClaudeApp",
    "CodexApp",
    "allow",
    "continue_",
]
