---
title: "Build plugins"
description: "Build a Skill, MCP package, or hybrid through the shared Milestone A authoring contract."
canonicalId: "page:build:index"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Build plugins

> **Milestone A is available.** Install `agentplugins` from
> `universal-agent-plugins@0.1.65` on npm or the `777genius/agentplugins` Homebrew tap.
> GitHub release: `agentplugins-v0.1.65`. Public-channel E2E is verified.
> Runtime/dev/bootstrap, export and publication remain deferred. Legacy YAML
> migration is cancelled; preserved source is not a supported workflow.

| Your job | Journey | Result |
| --- | --- | --- |
| Install someone else's package | [Use plugins](/en/use/) | Managed client installation and activation instructions. |
| Write a new portable package | Continue below | Root `plugin.json`, Skills and/or MCP configuration. |

Install the primary CLI with npm or Homebrew:

```bash
npm install -g universal-agent-plugins@0.1.65
# or: brew install 777genius/agentplugins/agentplugins
```

Then use `agentplugins author` throughout this guide. Milestone A static
authoring is available in 0.1.65. Phase 7 and Phase 8A are implemented in
current source, but that source checkpoint is not a later executable release.
Phases 9–11 remain deferred.

## Choose the smallest useful package

| Need | Template | Inputs beyond destination and identity |
| --- | --- | --- |
| Instructions an agent can follow | [Standalone Skill](./skill) | `--template skill`; optional explicit Skill name. |
| Connect to an existing service | [Remote MCP](./mcp-remote) | `--template mcp-remote --url` with an explicit endpoint. |
| Provide a local MCP process | [Stdio MCP](./mcp-stdio) | `--template mcp-stdio --runtime node`. |
| Instructions plus tools | [Hybrid](./hybrid) | `--template hybrid --mcp-template` plus the selected MCP inputs. |

A standalone Skill here means a standard package whose portable component is a
Skill. It still has root `plugin.json`; it is not an external Skills installer.
Add more instructions later with [extra Skills](./skills).

## Primary authoring commands

| Job | Command |
| --- | --- |
| Create a package | `agentplugins author init` |
| Validate | `agentplugins author validate` |
| Inspect | `agentplugins author inspect` |
| Static test | `agentplugins author test` |
| Static client compatibility | `agentplugins author compat` |
| Project doctor | `agentplugins author doctor` |
| Engine capabilities | `agentplugins author capabilities` |
| Add or validate Skills | `agentplugins author skills` |

Current source builds add these Phase 8A commands; they are not part of the
0.1.65 installation shown above:

| Job | Command |
| --- | --- |
| Plan or write canonical standard JSON | `agentplugins author normalize [package-path] --document plugin.json\|mcp.json [--write]` |
| Plan or write a safe Claude MCP import | `agentplugins author import native <source-file> --from claude --output <absolute-absent-path> --name <plugin-name> --description <text> [--write]` |

Both default to read-only planning. Normalize selects exactly one existing
standard JSON document. Native import reads only the explicit strict-JSON
source, skips unsafe or credential-bearing server entries, and writes only to
an absent package path. Neither command discovers client profiles or introduces
YAML migration.

## Follow the authoring loop

1. Choose a template and an absent destination under an existing parent.
2. Review and edit the resulting [package files](./layout).
3. Run [validate, inspect, and static test](./checks) against the exact root.
4. Evaluate explicit target compatibility and inspect project doctor evidence.
5. Send the package and its limits to the [installer planner](./handoff).

Creating files is local mutation. Validation, inspection, static test, and
compatibility do not install into clients or launch package code. A passing
static report is useful evidence, but it does not establish runtime success.

## What this journey does not expose

The 0.1.65 public release does not include Phase 7 or Phase 8A. Client
generation, export/bundle, and publication remain deferred in current source.
Legacy YAML migration is cancelled. Preserved legacy source is internal reference
material, not a runnable product. Hooks remain client-specific extensions.

Do not add legacy `--platform`, `--strict`, `--typescript`, `--output`, or
`--force` flags to these examples. Select destinations positionally and read
[command boundaries](./checks) before adapting automation.
