---
title: "Check a package and its evidence"
description: "Separate static checks, explicit source-build MCP execution and client activation evidence."
canonicalId: "page:build:checks"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Check a package and its evidence

**Source-build authoring:** these `agentplugins author` commands require the
[authoring-enabled source build](./#build-from-source). The current installer
release does not include them. Run relative examples from the package's parent
folder and use the same exact root throughout.

## Run the offline authoring loop

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

Validation checks the captured standard configuration and authoring readiness.
Inspection shows the components and unresolved requirements. `test` without
runtime flags checks configuration, hygiene, Skills and MCP statically. These
commands do not start servers or execute a Skill's instructions.

## Read the evidence layers

| Layer | What it establishes | What needs another check |
| --- | --- | --- |
| Conformance | Captured standard data satisfies the embedded schemas/profile. | Useful behavior in a particular agent. |
| Host safety | Input meets bounded filesystem and host checks. | User authorization for installation or execution. |
| Readiness | Available evidence meets the checked policy. | Public release approval or platform acceptance. |
| Compatibility | Static support for selected targets and components. | Effective discovery and activation in a real client. |
| Toolchain | Bounded project, native-file and executable metadata. | Dependency resolution and server startup. |
| Runtime | An explicitly executed MCP check reports observed stages. | Other tools, clients, OAuth flows or untested environments. |

Read conformance and overall readiness separately. A valid manifest can still
have incomplete component or host evidence. Keep “not evaluated” visible rather
than converting it into a pass.

## Select clients explicitly

```bash
agentplugins author compat ./review-helper --target codex,claude
agentplugins author inspect ./review-helper --target codex,claude
agentplugins author doctor ./review-helper
agentplugins author capabilities
```

`compat` needs distinct comma-separated target IDs; it does not accept `all`.
These static checks do not require installed clients. Doctor inspects captured
native files and executable metadata without processes or network. Capabilities
reports this engine's schemas, client metadata and implemented commands; it does
not certify a package or establish public release qualification.

## Evaluate MCP runtime explicitly

Local stdio runtime currently requires Linux, system bubblewrap and Node within
the system runtime directories. Process containment (including delegated cgroup
v2) and bubblewrap namespace preflights must pass before launch.
See the [stdio prerequisites](./mcp-stdio). Streamable HTTP runtime does not use
that local process sandbox and requires its own explicit network opt-in.

For a package created by the [stdio tutorial](./mcp-stdio), install its recognized
locked dependencies separately, then select the exact server name from `mcp.json`:

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools
```

This source command executes the selected MCP process and checks initialization
and tool discovery. To call a tool, supply both `--tool` and `--fixture`. The
fixture is a package-relative JSON object containing that tool's arguments.
For the template's `hello` tool, create `fixtures/hello.json` containing `{}`:

```bash
agentplugins author test ./local-tools --runtime mcp --server local-tools \
  --tool hello --fixture fixtures/hello.json
```

For Streamable HTTP, explicitly permit network access:

```bash
agentplugins author test ./docs-service --runtime mcp --server docs-service \
  --allow-network
```

Use the real server identity from your package. The deadline defaults to ten
seconds and may be set with `--deadline` to a positive duration of at most one
minute. Inspect initialization, tools, optional tool-call and cleanup evidence
individually. A runtime pass here is not execution through an installed agent,
and does not prove OAuth, other tools or a public deployment.

## Iterate with the source development loop

```bash
agentplugins author dev ./local-tools --server local-tools
```

`dev` runs bounded MCP cycles as the package changes; stop it with Ctrl+C. Use
`--once` for one cycle and when requesting JSON output. Remote servers require
`--allow-network`; tool calls still require the explicit tool/fixture pair.
The command does not install into a native client.

## Capture a reviewable report

```bash
agentplugins author validate ./review-helper --format json
agentplugins author inspect ./review-helper --format json
```

Use `--format human` for human output and `--no-color` where needed.
`--include-root` discloses the selected root; avoid unnecessary private paths in
shared diagnostics. `--release-policy` adds bounded release hygiene to the
package checks and is not publication approval. Skills subcommands have a
smaller flag surface; check their own help.

## Maintain JSON deliberately

Current source can plan deterministic normalization of one selected standard
file, then write it only with explicit `--write`:

```bash
agentplugins author normalize ./review-helper --document plugin.json
agentplugins author normalize ./review-helper --document plugin.json --write
```

Select `mcp.json` instead when that is the document you intend to normalize.
Native import reads one explicit strict-JSON Claude MCP file; it does not search
profiles. The current importer accepts only safe stdio entries containing
`command` and optional `args`; HTTP/URL entries, `env` and extra server fields
are skipped, even when the source client accepts them. Plan a new absent absolute destination before applying:

```bash
agentplugins author import native ./claude.json --from claude \
  --output /absolute/path/to/new-plugin --name new-plugin \
  --description 'Imported Claude MCP package'
```

Review safe/skipped servers and unsupported fields. Add `--write` to publish the
validated package to that absent destination. Unsafe or credential-bearing
servers are skipped; inspect the result rather than assuming every original
server travelled. No other native client import is exposed here.

## Choose the right command boundary

Installer `agentplugins validate ./review-helper` reads a package for installation;
`agentplugins doctor` diagnoses managed state. They are separate from the
source-build authoring operations. Installer scope and security flags do not
become authoring flags. Init creates real files; normalization and import require
`--write` to apply their plans. Continue with the [package handoff](./handoff).
