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

**Source-build authoring:** the commands below require the
[authoring-enabled source build](./#build-from-source). They are not included in
the current installer release.

Choose stdio when your package supplies a local process that communicates over
standard input and output. The scaffold supports Node.js 22 or newer and uses
the official MCP SDK. Creating it does not install dependencies or start a server.

## Create the scaffold

Choose an absent destination under an existing parent directory:

```bash
agentplugins author init ./local-tools \
  --template mcp-stdio \
  --name local-tools \
  --description 'Provide local tools through a Node MCP server' \
  --runtime node
```

`node` is the supported runtime input for this portable template. A Python or Go
runtime flag is not part of this command's current contract.

## Review the files

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

`mcp.json` declares the server `local-tools`, with command `node` and argument
`${PLUGIN_ROOT}/src/server.mjs`. The package token resolves within the selected
package; it does not mean your shell's current directory. The source starts with
a small `hello` tool. Adapt it for your task and send application logs to stderr
so that stdout remains available for the MCP protocol.

Review the dependency manifest and lockfile. Init writes them but does not run
npm or prove that the host can import the SDK.

## Gather static evidence

```bash
agentplugins author validate ./local-tools
agentplugins author inspect ./local-tools
agentplugins author test ./local-tools
agentplugins author compat ./local-tools --target codex,claude
agentplugins author doctor ./local-tools
```

These are static checks. Doctor inspects bounded project and executable metadata;
it does not establish installed dependencies, a successful handshake or a working
tool. Interpret the [evidence layers](./checks) separately.

## Bootstrap the recognized template

For the generated Node layout with its matching lockfile, current source can
preview and install dependencies:

```bash
agentplugins author bootstrap ./local-tools --dry-run
agentplugins author bootstrap ./local-tools
```

Applying bootstrap currently requires Linux and its handle-bound filesystem
support; other platforms report `bootstrap_platform_unsupported`. The dry run
can still inspect a recognized layout without installing dependencies.

Review the preview before applying. Bootstrap uses locked npm installation with
lifecycle scripts disabled. It accepts the recognized generated layout, matching
package files and an absent `node_modules`; it is not a general package-manager
replacement. Modified dependency files, competing lockfiles or existing outputs
can cause refusal. Resolve the reported layout requirement rather than adding
an overwrite flag. Dependency acquisition may use the network.

## Run one explicit MCP check

Local stdio execution currently requires Linux, system `bwrap` (bubblewrap),
working process containment (including delegated cgroup v2), namespace support,
and Node resolved inside the system runtime
(`/usr`, `/bin`, `/lib` or `/lib64`). The command requires successful process-containment and bubblewrap preflights
before launching package code and refuses hosts where it cannot establish it. A Node
installation in a user version-manager directory does not meet this runtime
contract. Static checks remain available separately.

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

This executes the selected local process and checks MCP initialization and tool
discovery. Calling `hello` requires an explicit JSON argument fixture; see
[the runtime check example](./checks#evaluate-mcp-runtime-explicitly).
For ongoing iteration:

```bash
agentplugins author dev ./local-tools --server local-tools
```

Keep the runtime result's exact engine, package and platform identity. A direct
authoring check does not prove the same server works through every native agent,
or that a client has granted trust or completed authentication.

## Prepare the handoff

Explain the tools, Node.js prerequisite, dependency files and runtime evidence
in your README. State the selected server and any untested tools or environments.
Then share the package through the [installer handoff](./handoff). Add instructions
with [extra Skills](./skills) when users need guidance alongside tools.
