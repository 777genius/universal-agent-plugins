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

> **Milestone A is available.** Standard-first static authoring is published as
> `universal-agent-plugins@0.1.65` (`agentplugins author`) and `plugin-kit-ai@2.0.5`.
> GitHub tags: `agentplugins-v0.1.65` and `plugin-kit-ai-v2.0.5`.
> npm, PyPI, Homebrew, GitHub Releases and the public-channel E2E are verified.
> Runtime/dev/bootstrap, migration, export and publication remain deferred.
> Historical YAML v1 workflows remain separate and preserved.

| Your job | Journey | Result |
| --- | --- | --- |
| Install someone else's package | [Use plugins](/en/use/) | Managed client installation and activation instructions. |
| Write a new portable package | Continue below | Root `plugin.json`, Skills and/or MCP configuration. |
| Keep an existing YAML/runtime project working | [Historical v1](/en/legacy/v1/) | Version-pinned 1.2.4 context; no v2 migration. |

Use `npm install -g universal-agent-plugins@0.1.65` for `agentplugins author`,
or `npm install -g plugin-kit-ai@2.0.5` for the standalone entrypoint (Node.js 22+).
Milestone A static authoring is available; phases 7–11 remain deferred.

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

## Two equivalent authoring entrypoints

Choose one spelling and use it consistently:

| Shared authoring job | Installer executable | Authoring executable |
| --- | --- | --- |
| Create a package | `agentplugins author init` | `plugin-kit-ai init` |
| Validate | `agentplugins author validate` | `plugin-kit-ai validate` |
| Inspect | `agentplugins author inspect` | `plugin-kit-ai inspect` |
| Static test | `agentplugins author test` | `plugin-kit-ai test` |
| Static client compatibility | `agentplugins author compat` | `plugin-kit-ai compat` |
| Project doctor | `agentplugins author doctor` | `plugin-kit-ai doctor` |
| Engine capabilities | `agentplugins author capabilities` | `plugin-kit-ai capabilities` |
| Add or validate Skills | `agentplugins author skills` | `plugin-kit-ai skills` |

The table names jobs, not complete invocations. The tutorials supply explicit
paths and template inputs. Both entrypoints use the same shared authoring
engine; neither is a second YAML implementation.

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

Runtime execution, dev loops, dependency bootstrap, import, normalization,
project migration, client generation, export/bundle, and publication are deferred
from the v2 journey. Their retained v1 source and documentation remain useful;
they are not runnable v2 features. Hooks remain client-specific extensions.

Do not add legacy `--platform`, `--strict`, `--typescript`, `--output`, or
`--force` flags to these examples. Select destinations positionally and read
[command boundaries](./checks) before adapting automation.
