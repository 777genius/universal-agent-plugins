---
name: lint-repo
description: Check that this skill package is internally consistent and report actionable failures.
execution_mode: command
supported_agents:
  - claude
  - codex
allowed_tools:
  - bash
  - go
command: go run ./cmd/lint-repo
runtime: go
compatibility:
  requires:
    - go >=1.22
  supported_os:
    - darwin
    - linux
  repo_required: true
  notes:
    - Run from the example root so the command can inspect the authored and generated skill files together.
safe_to_retry: true
writes_files: false
produces_json: false
---

# Lint Repository

## What it does

Checks that the example skill package keeps its canonical `SKILL.md`, generated artifacts, and command doc in sync.

## When to use

Use this when you want a small but real Go-backed skill example instead of a placeholder command stub.

## How to run

Run `go run ./cmd/lint-repo` from the example root.

## Constraints

- This is a non-interactive command.
- It assumes the current directory is the example checkout.
- Diagnostics go to stdout and stderr; fix the reported issues before rerunning.
