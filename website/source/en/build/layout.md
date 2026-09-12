---
title: "Understand the package root"
description: "Keep portable identity and components in an explicit standard package root."
canonicalId: "page:build:layout"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Understand the package root

> **Prepared, unreleased authoring contract.** This public page documents a reviewed
> future authoring workflow. The commands shown here are not available in current
> releases; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

The selected directory is the package root. Future authoring reads root
`plugin.json` and its standard components. It does not search parents for a
project, convert YAML, or fall back to a native client's hidden manifest.

## Minimal and combined layouts

A Skill-only package:

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
```

A remote hybrid package:

```text
research-helper/
  plugin.json
  mcp.json
  skills/
    research-guide/
      SKILL.md
```

A stdio package also carries its runtime source and dependency manifests. Those
files are needed for later execution, but their presence does not cause the
authoring reader to execute anything.

## Give each file a clear responsibility

| File or directory | Responsibility |
| --- | --- |
| `plugin.json` | Standard package identity and metadata. |
| `mcp.json` | MCP server configuration when the package supplies tools. |
| `skills/<name>/SKILL.md` | One immediate Skill's identity, description, and instructions. |
| `README.md` | Intended use, prerequisites, validation, and remaining limits. |
| `package.json`, `package-lock.json`, `src/server.mjs` | Node stdio scaffold inputs for separate runtime work. |
| `LICENSE` | Explicit selected license, when supplied during init. |

The scaffold starts the manifest version at `0.1.0`. That is your new package's
metadata, not the authoring executable's release version. Keep the distinction
when reporting versions to users.

## Identity and optional metadata

Use explicit `--name` and `--description` in repeatable init instructions.
The public contract can derive a name from a bare destination name and supplies
a deterministic default description, but paths such as `./review-helper` should
carry an explicit name. These tutorials supply both to avoid accidental identity.

For templates containing a Skill, `--skill-name` defaults to the package name
when omitted. Choose it explicitly if the Skill needs a different identity.
Skill names are never silently normalized.

Optional `--author-name` supplies an explicit author. No author is inferred from
Git or the environment. License generation supports explicit MIT or ISC choices
and requires both `--copyright-holder` and a four-digit `--copyright-year`.
There is no default license selection.

## Select the root consistently

```bash
agentplugins author validate ./research-helper
agentplugins author inspect ./research-helper
plugin-kit-ai test ./research-helper
```

All three select the same root despite the different command prefixes. Passing
`./research-helper/skills` would select the wrong directory. The optional path
on read commands defaults only to the current directory, not a discovered parent.

Standard conformance and filesystem safety are separate concerns. A schema-valid
identity can still be unsuitable for a host path, and a readable manifest alone
can leave required component evidence incomplete. Use the full report.

## Keep legacy material separate

Existing `plugin/plugin.yaml`, launcher source, generated client files, SDK docs,
and runtime examples remain preserved in their existing locations. They are
[historical or supporting material](/en/legacy/v1/), not implicit inputs to this
standard root. Do not copy them wholesale into a new package to satisfy a check.

Continue with [static checks](./checks), then [handoff](./handoff).
