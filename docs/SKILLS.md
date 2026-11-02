# plugin-kit-ai Skills

This page preserves the experimental Skills commands of the retired
`plugin-kit-ai` CLI. For the current public installer workflow, read
[Installing Agent Skills](AGENT_SKILLS_INSTALLER.md). The two command families
have different source formats and release boundaries.

`plugin-kit-ai skills` is an experimental, compatibility-first authoring layer for repository-local skills.

It also includes a small external skills wrapper:

```bash
plugin-kit-ai skills install flutter/skills --global --all
plugin-kit-ai skills list --global --json
plugin-kit-ai skills update --global --yes
plugin-kit-ai skills remove flutter-use-http-package --global --yes
```

The external `install`, `list`, `update`, and `remove` commands are pass-through wrappers around `npx -y skills@1.5.5 ...`. They are for installing or managing third-party Agent Skills in project or global agent directories. They are separate from the canonical authoring workflow below.

The goal is not to replace the broader `SKILL.md` ecosystem. The goal is to make the common workflow easier to maintain when you want:

- one canonical authored `SKILL.md`
- validation before you publish or share a skill
- generated artifacts for both Claude and Codex from the same source
- a strong executable path for Go without forcing Go on everyone

## What It Is

`plugin-kit-ai skills` treats a skill as:

- authored instructions in `skills/<name>/SKILL.md`
- YAML frontmatter for light validation and generating metadata
- optional executable behavior through a non-interactive command contract
- optional `scripts/`, `references/`, and `assets/`

The canonical authored file stays:

- `skills/<name>/SKILL.md`

Generated files are derived outputs. Edit the canonical skill, then re-generate.

Handwritten `SKILL.md` is a first-class input. `plugin-kit-ai skills init` is a convenience scaffold, not a required entrypoint.

## Supported Authored Contract

Minimal supported frontmatter fields:

- always supported:
  - `name`
  - `description`
  - `execution_mode`
  - `supported_agents`
  - `allowed_tools`
  - `compatibility`
  - `inputs`
  - `outputs`
  - `agent_hints`
- only for `execution_mode: command`:
  - `command`
  - `args`
  - `working_dir`
  - `runtime`
  - `timeout`
  - `safe_to_retry`
  - `writes_files`
  - `produces_json`

Required body sections:

- `What it does`
- `When to use`
- `How to run`
- `Constraints`

Compatibility guarantees in this beta layer:

- authored source of truth stays `skills/<name>/SKILL.md`
- handwritten skills are supported even without `plugin-kit-ai skills init`
- generated files under `generated/skills/...` and `commands/...` are derived and disposable
- extra ecosystem files like `scripts/`, `references/`, `assets/`, or agent-specific helper files are tolerated unless they directly break validation

Out of scope for this experimental contract:

- auto-installing generated local authored skills into agent configs
- registry/publish flows
- MCP/DXT packaging
- command execution during validation
- support for every possible external `SKILL.md` convention field

## How It Differs From Hooks

Skills and hooks solve different problems.

- Hooks are deterministic runtime integrations triggered by Claude or Codex lifecycle events.
- Skills are authored instruction packages that may optionally point to an executable command.

Use hooks when you need lifecycle automation.
Use skills when you need reusable guidance, workflows, or command-backed procedures.

## Why Use This Instead Of Writing SKILL.md By Hand

If all you need is a tiny `SKILL.md` and a trivial one-off command, writing it by hand is fine.

`plugin-kit-ai skills` becomes useful when you want:

- a canonical authoring workflow: `init -> edit -> validate -> generate`
- clearer validation than "hope the frontmatter is right"
- one authored source for both Claude and Codex generate targets
- an executable path that is not biased toward shell scripts

## Canonical Workflow

```bash
plugin-kit-ai skills init lint-repo --template go-command
# edit skills/lint-repo/SKILL.md
plugin-kit-ai skills validate .
plugin-kit-ai skills generate . --target all
```

Templates:

- `go-command`: best default for typed, testable executable skills
- `cli-wrapper`: for an existing Python, Node, shell, or external CLI workflow
- `docs-only`: for instructional skills with no executable step

The same workflow also works for a handwritten skill package:

```bash
# start with a handwritten skills/<name>/SKILL.md
plugin-kit-ai skills validate .
plugin-kit-ai skills generate . --target all
```

## Execution Model

Supported execution modes:

- `docs_only`
- `command`

`command` is language-neutral. It can describe:

- a Go binary or Go command
- a shell script
- Python, Node, or Deno commands
- `npx`, `uvx`, `go run`, Docker wrappers, or another external CLI

`plugin-kit-ai` does not execute commands during validation. It only validates the authored contract statically.

## When Not To Use plugin-kit-ai Skills

You probably do not need this subsystem when:

- your skill is a tiny standalone `SKILL.md`
- you do not need validation
- you do not care about generating for multiple agents
- you are already happy with handwritten artifacts for one agent only

## Examples

See:

- [examples/skills/README.md](../examples/skills/README.md)
- [examples/skills/go-command-lint](../examples/skills/go-command-lint/README.md)
- [examples/skills/cli-wrapper-formatter](../examples/skills/cli-wrapper-formatter/README.md)
- [examples/skills/docs-only-review](../examples/skills/docs-only-review/README.md)

## Stability

This subsystem is `public-experimental`.

That means:

- `SKILL.md` remains the canonical authored contract
- handwritten `SKILL.md` remains a supported input path
- the current workflow is intended for real use
- validation and renderer output may still evolve
- this subsystem is not part of the stable `v1.0` runtime compatibility promise
