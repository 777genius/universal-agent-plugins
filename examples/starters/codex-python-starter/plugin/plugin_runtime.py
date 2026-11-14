#!/usr/bin/env python3
from __future__ import annotations

import json
import sys
from typing import Any, Callable, Optional


JSONMap = dict[str, Any]
CodexHandler = Callable[[JSONMap], Optional[int]]


def continue_() -> int:
    return 0


class CodexApp:
    def __init__(self):
        self._notify_handler: Optional[CodexHandler] = None

    def on_notify(self, handler: CodexHandler) -> CodexHandler:
        self._notify_handler = handler
        return handler

    def run(self) -> int:
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
