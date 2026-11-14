# go-command-lint

This example demonstrates the strongest happy path for `plugin-kit-ai skills`:

- authored `SKILL.md`
- a Go command entrypoint
- generated artifacts for Claude and Codex

The command is intentionally simple but real: it checks that the example skill package is internally consistent and reports missing authored or generated files.

Use it when you want typed, testable execution rather than shell glue.
