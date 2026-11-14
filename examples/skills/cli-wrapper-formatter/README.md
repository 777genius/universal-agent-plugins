# cli-wrapper-formatter

This example demonstrates a canonical `SKILL.md` that wraps an existing external formatter command.

Use it when you already have a Node, Python, shell, or vendor CLI and want:

- validation
- one authored source
- generated Claude/Codex artifacts

without rewriting the formatter in Go.

It is also a good model for handwritten skills that already have an existing CLI invocation and only need validation plus agent-specific generated artifacts.
