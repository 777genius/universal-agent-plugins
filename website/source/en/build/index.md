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
authoring is available; phases 7–11 remain deferred.

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

Runtime execution, dev loops, dependency bootstrap, native import,
normalization, client generation, export/bundle, and publication are deferred.
Legacy YAML migration is cancelled. Preserved legacy source is internal reference
material, not a runnable product. Hooks remain client-specific extensions.

Do not add legacy `--platform`, `--strict`, `--typescript`, `--output`, or
`--force` flags to these examples. Select destinations positionally and read
[command boundaries](./checks) before adapting automation.
