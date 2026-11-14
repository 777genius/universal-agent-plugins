---
name: format-changed
description: Format changed files through an existing external formatter command.
execution_mode: command
supported_agents:
  - claude
  - codex
allowed_tools:
  - bash
  - node
command: npx prettier@3.4.2 --write .
runtime: node
compatibility:
  requires:
    - node >=20
  supported_os:
    - darwin
    - linux
  repo_required: true
  network_required: true
  notes:
    - The first run may download the pinned package through npm.
safe_to_retry: true
writes_files: true
produces_json: false
---

# Format Changed Files

## What it does

Runs a repository formatter through an existing external CLI instead of custom Go code.

## When to use

Use this when the repository already standardizes on a formatter and you want a reusable skill wrapper around it.

## How to run

Run `npx prettier@3.4.2 --write .` from the repository root or adapt the command to the subset of files you want to format.

## Constraints

- This is a non-interactive command.
- It may download dependencies on the first run.
- It writes files in place, so review the diff after execution.
