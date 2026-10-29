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

Use the installer to inspect and change managed installation state. Keep the
package source, selected clients, scope and installation identity with the report
when investigating a problem. The examples below use user scope unless they
explicitly select a project.

## Inspect the situation first

```bash
agentplugins list
agentplugins doctor
agentplugins outdated --all
```

`list` reports installations, `doctor` diagnoses managed state, and `outdated`
finds update candidates without applying them. None of these proves that a
remote service is healthy or that a client has completed authentication.

| Symptom | Next step |
| --- | --- |
| Package is absent | Return to [find and install](./install). |
| Owned files are missing or drifted | Review a repair plan for that package and scope. |
| A tracked source has a newer version | Review an update plan. |
| Client needs trust, activation or sign-in | Follow the target's reported client instructions. |
| Local package fails validation | Give the author the package report and [check guidance](../build/checks). |

## Local management API (CLI 0.1.95) {#local-management-api}

[CLI 0.1.95](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.95) adds
`agentplugins manage serve` for applications embedding the installer and
`agentplugins manage discover` for read-only local client/profile discovery.
Discovery does not execute clients, probe their versions, fetch sources or
change files.

`manage serve` is an owned process with a captured startup context and a private
authenticated IPv4 loopback API. The integrating host supplies JSON on stdin and
keeps the bootstrap URL and bearer capability private. Check the command help
before integration:

```bash
agentplugins version
agentplugins manage serve --help
agentplugins manage discover --help
```

See the [management wire contract](https://github.com/777genius/universal-agent-plugins/blob/main/contracts/management/README.md)
for startup and lifecycle rules. The
[TypeScript Management SDK](https://github.com/777genius/universal-agent-plugins/blob/main/sdks/typescript/README.md)
remains a separate, unreleased development package.

## Update deliberately

```bash
agentplugins update context7 --target codex,cursor --dry-run
```

Review the selected source, version and destinations. Omit `--dry-run` to apply:

```bash
agentplugins update context7 --target codex,cursor
```

`update --all` covers all managed update candidates. Use it when that whole set
is your intended scope. An exact commit source stays immutable; an update does
not make it follow a branch. Saved dependency ranges and moving refs advance
only through the appropriate explicit update operation.

## Repair recorded state

```bash
agentplugins repair context7 --target codex,cursor --dry-run
agentplugins repair context7 --target codex,cursor
```

The first command previews; the second applies. Repair replays the recorded
source to restore owned content. It does not select a new remote revision or
silently overwrite externally changed native entries. Review conflicts and any
pending recovery instructions before continuing.

## Remove the selected installation

```bash
agentplugins remove context7 --target codex,cursor --dry-run
agentplugins remove context7 --target codex,cursor
```

Review the first plan before running the second command. Removal consumes the
CLI's recorded ownership. It does not revoke a service account, remove unrelated
runtimes or delete an author's source directory. Project removal retains plugin
data and the shared verified content store by default.

## Use a saved dependency environment

`uap.toml` records the packages and targets you want; `uap.lock` records their
exact resolved sources and digests. Inspect the selected environment with:

```bash
agentplugins deps status
agentplugins deps tree
```

For a new environment, use `agentplugins deps init`, edit the manifest deliberately,
then `agentplugins deps lock` to resolve it and `agentplugins install --locked --scope user`
to apply that lock. Review confirmations and use `--dry-run` for a preview.
The [dependency guide](https://github.com/777genius/universal-agent-plugins/blob/main/docs/UAP_DEPENDENCIES.md)
explains manifest syntax, mandatory package dependencies, version ranges and
update rules.

Eligible remote `add` operations save portable dependency intent by default in
CLI 0.1.93. Explicit local paths, discovery sources, ChatGPT-only preparation and
`--no-save` use the imperative flow instead. Do not assume an older imperative
installation is in the agreement: check `deps status` and the actual files
before transferring them.

## Restore on another machine

Transfer a saved user environment through an exchange folder. From that folder
on the source machine:

```bash
agentplugins deps export
```

Copy the folder to the destination, install the target clients, then run there:

```bash
agentplugins deps import
agentplugins install --locked --scope user
```

Or combine import and restoration:

```bash
agentplugins deps import --install
```

The optional directory argument selects another exchange folder. Export reads
the user environment and import writes it; `AGENTPLUGINS_HOME` can select a
specific environment. The transferred folder is not itself a replacement client
profile. Explicit `--scope user` keeps restoration in the user environment.

Only the unchanged, strictly validated `uap.toml` and `uap.lock` are transferred.
An actual copy shows a preview and requires confirmation or `--yes`;
`--dry-run` previews it. Import validation is offline, while installation needs
the recorded sources or verified cached packages. Cold restoration fetches exact
pins and checks their digests; it does not choose newer versions.

Client profiles, settings, credentials, receipts and `PLUGIN_DATA` are not part
of the transfer. Configure authentication on the new machine and back up data
separately. If installation fails after import saved the pair, fix the reported
cause and repeat `install --locked --scope user`. Run `deps recover --scope user`
first when pending recovery is reported. Do not recreate transferred pins with
`deps lock`, because a moving selector can resolve differently.

<a id="restore-project-dependencies"></a>

## Restore and maintain a project

Automatic project installation is available in
[CLI 0.1.94 and later](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94).
Commit the project's `uap.toml` and `uap.lock`. In another existing checkout,
install the required clients and run this from the exact project directory:

```bash
agentplugins install
```

An existing lock is validated strictly. `--locked` additionally requires it to
exist, which is useful in CI; without a lock, the first install may create one.
When invoking from another directory, select the existing project explicitly:

```bash
agentplugins install --locked --scope project --project-root /path/to/project
```

Select the same scope and root for maintenance:

```bash
agentplugins list --scope project --project-root /path/to/project
agentplugins deps status --scope project --project-root /path/to/project
agentplugins repair review --target cursor --scope project --project-root /path/to/project
agentplugins deps recover --scope project --project-root /path/to/project
```

Use the installation identity from `list` if a name is ambiguous. Maintenance
commands default to user scope, so select project scope explicitly. An `install`
without a source argument has the 0.1.94 selection rule below. Explicit
`--scope project` without `--project-root` uses only the invocation directory.

Keep `.uap/` receipts, recovery journals and retained data local. Restore native
components from the portable agreement and follow the client's trust or
activation instructions. Sources and private-source access must remain available.
See [project installation](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_INSTALLATION.md)
and [per-client limits](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PROJECT_CLIENT_SUPPORT.md).

## Default install selection in CLI 0.1.94

Check `agentplugins version` first. This behavior requires 0.1.94; if your
package-manager channel is older, use the native installer. The
explicit project-scope commands above remain compatible with 0.1.93.

**Released in [CLI 0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94):**
`agentplugins install` without a source argument or explicit scope selects
project scope when a regular `uap.toml` exists in the exact current directory.
It does not search parents. A directory or symlink with that name is refused
during automatic selection; choose scope explicitly to resolve the ambiguity.
Without that local manifest, installation selects the user environment.
Explicit `--scope user` or `--scope project` always wins. Other commands continue
to default to user scope.

An existing `uap.lock` is validated strictly before changes, even without
`--locked`. A mismatch stops the operation; use `update` to advance pins rather
than expecting install to refresh them. Only a missing lock permits first-install
resolution and lock creation. `--locked` additionally requires an existing lock.
For predictable automation and project restoration, the explicit scope and
`--locked` examples above remain appropriate.

## Report a problem clearly

Record the CLI version, operation, scope, package source, target clients and
remaining activation instruction. State whether failure occurred during source
acquisition, planning, file application, recovery, client discovery, authentication
or runtime use. Remove credentials before sharing diagnostics. Authoring
`doctor ./my-plugin` inspects a source package; installer `doctor` diagnoses
managed installation state.
