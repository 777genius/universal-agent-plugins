---
title: "Build plugins"
description: "Build a Skill, MCP package, or hybrid with Agent Plugins authoring."
canonicalId: "page:build:index"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Build plugins

An Agent Plugins 1.0 package is a directory with root `plugin.json` and Skills,
MCP configuration, or both. Start with the smallest component that solves your
users' task. A Skill supplies instructions; MCP supplies a connection to tools.

## Availability

The current native installer release,
[CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94),
can validate and install standard packages. Package-manager channels can lag;
check `agentplugins version` for the installed version. It does **not** include
`agentplugins author`. The authoring commands in this Build journey describe
**current source builds** with authoring enabled. Historical 0.1.65 authoring
release evidence remains version-specific and does not describe today's installer.

You can write the files by hand and use the released installer, or build the
source tools below for scaffolding, static checks and an explicit MCP runtime
loop. A passing static check does not prove native client activation or a working
service connection.

## Make a small package by hand

Create this structure:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

Put package identity in `plugin.json`:

```json
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "review-helper",
  "version": "0.1.0",
  "description": "Review documentation changes for clarity and evidence"
}
```

Start `skills/review-docs/SKILL.md` with:

```markdown
---
name: review-docs
description: Use when reviewing documentation for clarity and evidence.
---

# Review documentation

Identify the intended reader and task. Check each command's input and effect.
Separate verified behavior from prerequisites and untested claims. Return
findings with file locations and a suggested correction.
```

Install the released manager, then validate and preview the package:

```bash
npm install -g universal-agent-plugins
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

Review the plan and use [Use plugins](../use/install) when ready to apply it.
See [package layout](./layout) for the responsibility of each file.

## Build from source {#build-from-source}

The following commands build the current CLI from source with authoring enabled.
They require Git and the Go toolchain declared by the repository
(currently Go 1.26.10). Record the checked-out commit when sharing results.

```bash
git clone https://github.com/777genius/universal-agent-plugins.git
cd universal-agent-plugins/cli
mkdir -p ../bin
authoring_module=$(go list -f '{{.Module.Path}}' ./cmd/agentplugins)
authoring_revision=$(git rev-parse HEAD)
go build -ldflags "-X ${authoring_module}/internal/authoring/commands.Enabled=release-cli-contract-v1 -X ${authoring_module}/internal/authoring/commands.Revision=${authoring_revision}" \
  -o ../bin/agentplugins ./cmd/agentplugins
export PATH="$(cd ../bin && pwd):$PATH"
agentplugins author --help
```

Use a separate working directory for the tutorial packages. This source binary
is a development build. Its existence is not a qualified public authoring release.
Check your executable path if the installed manager still reports an unknown
`author` command.

## Choose a package to scaffold

| Need | Journey | Required template inputs |
| --- | --- | --- |
| Instructions an agent can follow | [Standalone Skill](./skill) | `--template skill` |
| Connect to an existing service | [Remote MCP](./mcp-remote) | `--template mcp-remote --url <endpoint>` |
| Provide a local Node MCP process | [Stdio MCP](./mcp-stdio) | `--template mcp-stdio --runtime node` |
| Instructions together with tools | [Hybrid](./hybrid) | `--template hybrid --mcp-template <choice>` and that MCP template's inputs |

A standalone Skill package still has root `plugin.json`. Installing standalone
Skills from another repository is a separate [installer operation](../use/install#install-agent-skills).

## Follow the authoring loop

1. Create a package in an absent destination, then edit the generated files.
2. Run [validate, inspect and static test](./checks) on the exact package root.
3. Check the intended clients with `compat` and inspect `doctor` evidence.
4. For MCP, evaluate runtime separately with explicit source-build execution.
5. Share the package and its evidence through the [installer handoff](./handoff).

Current source also provides lockfile-based `bootstrap`, continuous `dev`, JSON
`normalize` and explicit Claude MCP `import native`. See [checks](./checks) and
[stdio MCP](./mcp-stdio). Client projection, bundle export and package publication
are outside this current authoring command surface. Client-specific hooks remain
separate from the portable Skill and MCP components.
