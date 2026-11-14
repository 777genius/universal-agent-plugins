---
title: "Hand a package to the installer planner"
description: "Pass source and static evidence across an explicit installation boundary."
canonicalId: "page:build:handoff"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Hand a package to the installer planner

The useful handoff is a standard package directory plus a clear account of what
was checked. The receiving installer makes its own plan for selected agents and
scope. Source-build authoring tools can supply evidence, but are not required on
the receiving machine.

## Prepare the package and evidence

Select the directory containing root `plugin.json`, rather than an ambiguous
parent repository. Include:

- Package identity, version and exact source revision or reviewed snapshot.
- The relative package root and the purpose of its Skills and MCP servers.
- Validation and inspection results, with the tool version or source revision.
- Intended client IDs and component-specific compatibility findings.
- Runtime prerequisites, authentication needs and work still not evaluated.
- Any explicit runtime test, including the selected server/tool and platform.

Package version and CLI version describe different things. Keep them separate.
Avoid credentials and unnecessary absolute paths in shared diagnostics.

## Finish source authoring checks when available

These optional commands require the [authoring-enabled source build](./#build-from-source),
which is separate from the current installer release:

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
agentplugins author compat ./review-helper --target codex
agentplugins author doctor ./review-helper
```

This sequence is static and does not launch package code or inspect a real
user's client profile. Record explicit [MCP runtime checks](./checks) separately
when performed. A successful direct runtime check still does not establish
native client discovery or OAuth.

## Ask the released installer to plan

```bash
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

The installer can fetch security metadata or its pinned scanner even for a
local source. Its validation and dry run are not the offline authoring checks.

The first command supplies installer package-reading evidence. The second
selects Codex and previews installation without applying managed changes. For
another client, choose a supported target from the compatibility findings. For a
project plan, add explicit `--scope project --project-root /path/to/project`.

Check package identity, source, destinations, security findings and remaining
activation requirements. Resolve differences from the author's intended handoff
before applying. A plan is not an installed-state receipt.

## Share a reproducible source

For remote distribution, use a source accepted by the receiving CLI: a signed
Directory package, an exact GitHub source, or a supported direct Git/archive
source. See [installation sources](../use/install) for syntax and availability.
Use an immutable revision or durable versioned archive where you need exact
restoration. A local directory needs a separate deliberate transfer.

The source authoring CLI does not expose a package publication, bundle export
or Directory-submission command. Hosting your source is a separate step from
validating it. Dependency declarations in `uap.toml` are a UAP manager contract;
root `plugin.json` remains the portable package's identity authority.

## Explain what remains

| Evidence supplied | Remaining work |
| --- | --- |
| Standard conformance | Useful instructions and correct tool behavior. |
| Static authoring checks | Release approval and public-channel qualification. |
| Static client compatibility | Effective native discovery and activation. |
| Direct MCP runtime check, if performed | Other tools, clients, authentication and untested environments. |
| Installer dry run | Applying the reviewed changes and completing client setup. |

The receiving user can continue with [Use plugins](../use/install) when ready to
install. Retain client-specific trust and activation instructions with the package.
