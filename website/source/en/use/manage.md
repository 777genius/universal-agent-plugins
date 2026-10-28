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


These are **existing installer commands**. They operate on managed installation
state, not on authored package scaffolds. Keep the package name, selected agents,
and any reported installation identity available when diagnosing a problem.

## Inspect the situation first

```bash
npx -y universal-agent-plugins@latest list
npx -y universal-agent-plugins@latest doctor
npx -y universal-agent-plugins@latest outdated --all
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
npx -y universal-agent-plugins@latest update context7 --target codex,cursor --dry-run
```

Review the planned source and destinations before applying the update:

```bash
npx -y universal-agent-plugins@latest update context7 --target codex,cursor
```

The existing installer also supports `npx -y universal-agent-plugins@latest update --all`. Use it only
when the intended scope really is all managed update candidates. A direct
full-SHA source stays immutable; do not expect an update to follow a branch.

## Repair recorded state

```bash
npx -y universal-agent-plugins@latest repair context7 --target codex,cursor --dry-run
```

Repair reapplies the recorded source. It is useful for managed-file drift,
not for changing the authored package or selecting a different remote revision.
After reviewing the plan, the corresponding apply command is:

```bash
npx -y universal-agent-plugins@latest repair context7 --target codex,cursor
```

If an operation reports a partial rollback or recovery instruction, retain that
report and follow it. Avoid deleting user-owned files or treating every client
configuration file as installer-owned.

## Remove the selected installation

```bash
npx -y universal-agent-plugins@latest remove context7 --target codex,cursor --dry-run
```

Review what the installer owns before applying removal:

```bash
npx -y universal-agent-plugins@latest remove context7 --target codex,cursor
```

Removal changes files owned by the CLI. It does not revoke a service account,
uninstall an unrelated runtime, or delete an author's source directory.

## Restore on another machine

Carry your plugin set to another computer with `deps export` and `deps import`.
The transfer contains only `uap.toml` and `uap.lock`. Installation on the
destination then fetches the exact pinned packages.

On the source machine, run this in the folder you want to carry:

```bash
npx -y universal-agent-plugins@latest deps export
```

On the destination, install the target clients, then run:

```bash
npx -y universal-agent-plugins@latest deps import
npx -y universal-agent-plugins@latest install --locked --scope user
```

Or import and restore in one step:

```bash
npx -y universal-agent-plugins@latest deps import --install
```

- The optional `[directory]` argument overrides the exchange folder, which
  defaults to the current folder. Export reads the global user environment;
  import writes it. `AGENTPLUGINS_HOME` selects that environment. Explicit
  `--scope user` also keeps the following install in that user environment when
  the exchange folder contains a project-style `uap.toml`.
- Every copy shows a preview and needs a confirmation or `--yes`. `--dry-run`
  previews without writing.
- Import validates the pair strictly and offline. It refuses missing or
  inconsistent files, stale locks, unsafe sources and excluded targets.
- Store content, receipts, client profiles, settings, secrets and `PLUGIN_DATA`
  are not transferred. Configure client profiles and authentication on the
  destination, and back up `PLUGIN_DATA` separately.
- Installation with an empty store fetches each exact pin and verifies its
  package and manifest digests. It never selects newer versions. Missing source
  bytes or mismatched digests fail the install.
- If installation fails after import saved the agreement, fix the error and run
  `npx -y universal-agent-plugins@latest install --locked --scope user` again. If recovery is pending, run
  `npx -y universal-agent-plugins@latest deps recover` first.
- Do not run `npx -y universal-agent-plugins@latest deps lock` to recreate transferred pins. Moving
  selectors could resolve differently.

## Report a problem clearly

Record the operation, package source, selected clients, result, and remaining
activation instruction. Share diagnostics with credentials removed. State
whether the problem occurred during planning, managed-file application, client
activation, authentication, or runtime use; these are different failure stages.

Authoring `doctor ./my-plugin` supplies project evidence only. It cannot
replace installer doctor, repair managed state, or complete OAuth. Authors and
installers can compare evidence through the [handoff boundary](/en/build/handoff).

## Restore project dependencies

Explicit project installation is available in
[CLI 0.1.93 and later](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.93).
Commit the project's `uap.toml` and `uap.lock` to version control. A teammate can
check out the project, install the required clients, and run this from that project folder:

```bash
npx -y universal-agent-plugins@latest install --locked --scope project
```

When invoking from another directory, also select the existing project directory
with `--project-root /path/to/project`. Recorded sources and private-source access
must remain available. Client-side trust, authentication or activation may still
be required; see the [project client boundaries](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md).
Credentials, local receipts and plugin data stay local rather than travelling
with the agreement.

## Maintain a project in the current source

**Source only / unreleased.** Argumentless `agentplugins install` selects project
scope when a regular `uap.toml` is present in the exact current directory; it
never searches parents. Explicit `--scope` wins. An existing `uap.lock` is
validated strictly before changes; only a missing lock allows first-install
resolution. `--locked` requires that lock to exist. Use `update` to advance pins.
The install preview shows the selected scope and agreement paths. Other
maintenance commands require explicit project selection:

```sh
agentplugins list --scope project --project-root /path/to/project
agentplugins install --locked --scope project --project-root /path/to/project
agentplugins repair review --target cursor --scope project --project-root /path/to/project
agentplugins deps recover --scope project --project-root /path/to/project
```

Commit the portable `uap.toml` and `uap.lock` when you want to share dependency
intent and pins. Keep `.uap/` receipts, recovery state and retained data local;
back up data separately. Removal retains data and the shared content cache.
Repair restores missing owned components and damaged managed packages; changed
native files require review. Recovery must use the original selected project.

[Project installation guide](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md) · [Current project support and limitations](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md)
