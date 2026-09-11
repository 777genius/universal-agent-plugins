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

> **Prepared, unreleased authoring contract.** This page is a review draft on a
> non-deploying preparation branch. Future authoring examples require the accepted
> release; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

The author's output is a standard package directory plus evidence. The installer
makes a separate plan for selected agents. This page deliberately ends at a dry
run; no install invocation is part of the authoring handoff.

## Prepare the package and evidence

Use the same exact directory for authoring checks and installation planning.
For example, `./review-helper` should contain root `plugin.json` and its Skills
and/or MCP configuration, not a parent repository with an ambiguous nested root.

Before handing it off, record:

- The package identity, version, and exact source revision or reviewed snapshot.
- The relative package root within that snapshot.
- Which components are present and their purpose.
- Static validate, inspect, and test findings from the accepted authoring engine.
- Compatibility findings for the intended client IDs.
- Runtime prerequisites, authentication needs, and work not evaluated.

Keep package version and CLI version separate. Include the authoring engine
revision with reports so the receiver can tell which contract produced them.
Avoid secrets and unnecessary absolute paths in shared evidence.

## Finish the authoring side

Future authoring commands for the selected root:

```bash
agentplugins author validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
agentplugins author compat ./review-helper --target codex
agentplugins author doctor ./review-helper
```

The equivalent future prefix is `plugin-kit-ai`. These commands do not inspect
or modify a real user's client profile. Runtime execution and activation remain
separate from this static package review.

## Ask the existing installer to plan

The following are **existing installer commands**, not authoring aliases:

```bash
agentplugins validate ./review-helper
agentplugins add ./review-helper --target codex --dry-run
```

Installer validation supplies its own package-reading evidence. The dry run
selects Codex and prints the proposed installation without writing managed
installation changes. It does not execute the package or finish client setup.
For another agent, use a supported client ID reviewed in compatibility findings.

Read the package identity, selected source, target destinations, security
findings, and remaining activation requirements in the plan. If these differ
from the author's intended handoff, resolve the discrepancy before applying.
Do not treat the plan as proof that installation has already occurred.

## Describe the remaining boundary

| Evidence provided | Work still separate |
| --- | --- |
| Standard conformance | Useful behavior and runtime correctness. |
| Static authoring readiness | Release approval, channel publication, and platform acceptance. |
| Static client support | Actual client availability and native activation. |
| Installer dry run | Intentional application of managed changes. |
| MCP configuration | Connectivity, authentication, and tool execution. |

The receiving user can continue through [Use plugins](/en/use/install) when they
intend to install. Keep that decision explicit. A dry run in this documentation
is not permission to run against a real user's profile during preparation.

## Keep unavailable jobs out of the handoff

Do not ask the receiver to run v2 import, migrate, export, bundle, publish,
bootstrap, or dev commands. Do not label a YAML example as a migrated standard
package. Historical workflows remain linked from [v1 context](/en/legacy/v1/).

For remote source sharing, the existing installer requires a full 40-character
commit SHA and an explicit package subpath when discovery is ambiguous. That is
source selection, not a v2 publication operation. Registry submission and public
release evidence belong to their own owners.

At release time, recheck examples against the final accepted engine and published
channels. This prepared source contract alone cannot make that acceptance true.
