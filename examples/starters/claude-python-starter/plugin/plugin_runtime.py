#!/usr/bin/env python3
from __future__ import annotations

import json
import sys
from typing import Any, Callable, Iterable, Optional


JSONMap = dict[str, Any]
ClaudeHandler = Callable[[JSONMap], Optional[JSONMap]]


def allow() -> JSONMap:
    return {}


class ClaudeApp:
    def __init__(self, allowed_hooks: Iterable[str], usage: str):
        self._allowed_hooks = tuple(allowed_hooks)
        self._allowed_hook_set = set(self._allowed_hooks)
        self._usage = usage
        self._handlers: dict[str, ClaudeHandler] = {}

    def on(self, hook_name: str) -> Callable[[ClaudeHandler], ClaudeHandler]:
        def register(handler: ClaudeHandler) -> ClaudeHandler:
            self._handlers[hook_name] = handler
            return handler

        return register

    def on_stop(self, handler: ClaudeHandler) -> ClaudeHandler:
        return self.on("Stop")(handler)

    def on_pre_tool_use(self, handler: ClaudeHandler) -> ClaudeHandler:
        return self.on("PreToolUse")(handler)

    def on_user_prompt_submit(self, handler: ClaudeHandler) -> ClaudeHandler:
        return self.on("UserPromptSubmit")(handler)

    def run(self) -> int:
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
