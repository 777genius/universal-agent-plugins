---
title: "Build a stdio MCP package"
description: "Prepare a local Node MCP server while keeping runtime setup and execution separate."
canonicalId: "page:build:mcp-stdio"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Build a stdio MCP package


Choose stdio MCP when the package should supply a local process that communicates
with the agent over standard input and output. The prepared template supports
Node.js 22 or newer. This is a package scaffold; it does not establish that the
host can start the server or that the tools behave correctly.

## Create the scaffold

Use an existing disposable parent directory with no `./local-tools` child.
The runtime is an explicit template input.

Run the released authoring command:

```bash
agentplugins author init ./local-tools \
  --template mcp-stdio \
  --name local-tools \
  --description 'Provide local tools through a Node MCP server' \
  --runtime node
```

The released portable stdio template accepts the documented Node runtime
choice. Other runtime flags are not part of this authoring command.

## Review the runtime boundary

```text
local-tools/
  plugin.json
  mcp.json
  package.json
  package-lock.json
  src/
    server.mjs
  README.md
  .gitignore
```

`mcp.json` records a stdio server with command `node` and argument
`${PLUGIN_ROOT}/src/server.mjs`. The package root token lets the consumer resolve
the server file within the selected package; it is not your shell's current
working directory.

The template includes the official MCP SDK dependency and its lockfile.
`src/server.mjs` starts from a small `hello` tool. Review and adapt that source
for your intended behavior, keeping application logs away from protocol output.
No dependency install occurs during init, and no server process starts.

Version 0.1.65 performs static checks only. It does not start the package
process or infer an executable.

## Gather static evidence

```bash
agentplugins author validate ./local-tools
agentplugins author inspect ./local-tools
agentplugins author test ./local-tools
agentplugins author compat ./local-tools --target codex,claude
agentplugins author doctor ./local-tools
```

Static test inspects configuration, hygiene, Skills, and MCP; it is
not an MCP handshake or a test invocation of `hello`.

Doctor inspects captured native files and executable metadata without processes
or network. Its evidence is bounded. A toolchain finding cannot prove dependencies
are installed, that imports resolve, or that the server starts successfully.

## Prepare an honest README

Record Node.js 22 or newer as a runtime prerequisite and explain what tools the
server provides. Identify the pinned dependency files and describe the remaining
runtime verification work as separate work. Do not present bootstrap, dev, or
runtime test commands as available v2 authoring operations.

A receiver needs both a conforming package and an explicit understanding of its
execution requirements. Installation planning can evaluate target support, but
it cannot stand in for independent runtime testing or user authorization to
execute the server.

Continue with [checks and evidence](./checks). Add instructions through
[extra Skills](./skills), or prepare the [installer handoff](./handoff).
