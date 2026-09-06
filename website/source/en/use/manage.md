---
title: "Maintain installed plugins"
description: "Separate installed-state diagnostics, updates, repair, and authoring checks."
canonicalId: "page:use:manage"
section: "use"
locale: "en"
generated: false
translationRequired: true
---

# Maintain installed plugins

> **Prepared, unreleased authoring contract.** This page is a review draft on a
> non-deploying preparation branch. Future authoring examples require the accepted
> release; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

These are **existing installer commands**. They operate on managed installation
state, not on future authoring scaffolds. Keep the package name, selected agents,
and any reported installation identity available when diagnosing a problem.

## Inspect the situation first

```bash
agentplugins list
agentplugins doctor
agentplugins outdated --all
```

`list` reports installations. Installer `doctor` diagnoses managed state.
`outdated --all` identifies update candidates; it does not perform the update.
These jobs cannot prove a remote MCP endpoint is healthy or that the client has
completed authentication.

Before changing anything, identify the symptom:

| Symptom | Next step |
| --- | --- |
| Package was never installed | Return to [find and install](./install). |
| Managed files drifted | Review a repair plan for the affected package and clients. |
| A registry package has a newer version | Review an update plan. |
| Client still needs activation or sign-in | Follow that client's printed instructions. |
| Local authored package fails validation | Give the report to the author; use [Build checks](/en/build/checks). |

## Update deliberately

```bash
agentplugins update context7 --target codex,cursor --dry-run
```

Review the planned source and destinations before applying the update:

```bash
agentplugins update context7 --target codex,cursor
```

The existing installer also supports `agentplugins update --all`. Use it only
when the intended scope really is all managed update candidates. A direct
full-SHA source stays immutable; do not expect an update to follow a branch.

## Repair recorded state

```bash
agentplugins repair context7 --target codex,cursor --dry-run
```

Repair reapplies the recorded source. It is useful for managed-file drift,
not for changing the authored package or selecting a different remote revision.
After reviewing the plan, the corresponding apply command is:

```bash
agentplugins repair context7 --target codex,cursor
```

If an operation reports a partial rollback or recovery instruction, retain that
report and follow it. Avoid deleting user-owned files or treating every client
configuration file as installer-owned.

## Remove the selected installation

```bash
agentplugins remove context7 --target codex,cursor --dry-run
```

Review what the installer owns before applying removal:

```bash
agentplugins remove context7 --target codex,cursor
```

Removal changes files owned by the CLI. It does not revoke a service account,
uninstall an unrelated runtime, or delete an author's source directory.

## Report a problem clearly

Record the operation, package source, selected clients, result, and remaining
activation instruction. Share diagnostics with credentials removed. State
whether the problem occurred during planning, managed-file application, client
activation, authentication, or runtime use; these are different failure stages.

Future authoring `doctor ./my-plugin` supplies project evidence only. It cannot
replace installer doctor, repair managed state, or complete OAuth. Authors and
installers can compare evidence through the [handoff boundary](/en/build/handoff).
