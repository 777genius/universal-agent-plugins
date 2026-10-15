# UAP dependency environments

**Unreleased:** this workflow is available in current source and has not been
qualified as a published CLI release.

`agentplugins deps` manages one current-user environment in
`AGENTPLUGINS_HOME` (default: `os.UserConfigDir()/agentplugins`). `uap.toml` records intent and `uap.lock` records exact
sources and package digests. The files are portable; local receipts, the
verified package store, profile paths and `PLUGIN_DATA` are not.

```toml
schema_version = 1

[dependencies.review]
git = "https://github.com/acme/agent-tools.git"
path = "plugins/review"
ref = "main"

[dependencies.docs]
registry = "uap"
distribution = "acme/docs"
release = 42

[targets]
codex = ["review", "docs"]
claude = ["review"]
```

Git dependencies use exactly one of `commit` and `ref`; omitting both records
the repository default branch intent. Registry dependencies require an exact
directory `release`. A dependency is assigned only through `[targets]`.

```sh
agentplugins deps init
agentplugins deps lock
agentplugins deps sync
agentplugins deps sync --locked
agentplugins deps update review
agentplugins deps status
agentplugins deps recover
```

`lock` resolves changed selectors and preserves compatible pins. `update
<alias>` is the only command that rechecks a moving ref. `sync --locked` rejects
manifest/lock disagreement and only fetches immutable locked bytes. `status`
is offline and reports manifest, owner/receipt, store and projection facts
independently. `recover` covers one complete owned pending scope and requires
confirmation; run `sync` separately after it succeeds.

Only targets registered by the composed SDK installer are supported: currently
Claude and Codex. `deps status` prints the scope, exact manifest/lock paths and
supported targets, including before `deps init`. Initialization creates a private
root when necessary and never overwrites an existing manifest.

Dependency commands always use manifest targets. They reject inherited
`--target` and `--scope project`. Use `--dry-run` for an in-memory preview;
it does not write the lock, store, state, profiles or persistent scanner cache.
Native-only/YAML packages and implicit compatibility fallbacks are not part of
this dependency flow.

Remove a dependency or target assignment from `uap.toml`, then run `deps lock`
and `deps sync --yes`. Store originals and `PLUGIN_DATA` are retained. If the
client requires manual uninstallation, sync reports an incomplete outcome;
after uninstalling it (or if it was never imported), explicitly attest with
`deps sync --locked --yes --external-uninstalled`.

On drift, preserve any edits you need, then explicitly run
`agentplugins repair <installation-id> --target <agent> --format json` to restore
the recorded exact pin. Repair does not resolve a moving ref or adopt another
installation. Foreign pending recovery blocks both repair and environment
recovery. Ordinary imperative update/remove cannot mutate environment-owned
bindings; use the manifest workflow for those changes.

Schema 2/3/4 state requires the existing explicit `migrate-state --dry-run`
followed by `migrate-state`. Pending old-schema transactions must first be
recovered with their corresponding previous binary. Migration retains a backup
and leaves all historical installations unmanaged.

A copied registry lock is identity, not proof of an installed release. On a fresh
machine the signed Directory must still permit a new installation of that exact
release. A superseded release requires a newer pin; a revoked release cannot be
reproduced. Existing installed releases retain only their explicit signed-policy
exceptions, using live state rather than the copied lock.

Authentication is reported separately from retryable lifecycle work. An active,
materialized, installation-verified binding with unchecked authentication shows
`auth_unverified`; pending or failed authentication shows `auth_pending` or
`auth_failed`. These facts do not repeatedly reinstall the package. Activation
or installation-verification failures still retry. Sync prints planned changes
before confirmation and reports actual per-binding outcomes, manual actions and
SDK next actions after execution. Run `deps recover` only when inspection reports
pending recovery; an activation failure alone does not imply pending recovery.

An installation with any environment receipt requires environment authority for
all its bindings, including new targets. Mixing an imperative binding into it
is refused. Conversely an environment cannot adopt an installation containing
unmanaged bindings, even when the unmanaged target is absent from the manifest.

## Ordinary commands (unreleased)

Eligible remote `add` commands now save exact dependency intent by default in the
current user's `uap.toml` and `uap.lock`. For example:

```sh
agentplugins add github:acme/demo@0123456789012345678901234567890123456789 --target codex
agentplugins update demo
agentplugins remove demo --target codex
```

The optional `--alias` selects the saved name. Repeating the same pin and targets
keeps the files and installed state unchanged. Updating a moving Git ref changes
only that alias; exact Git pins change through a new `add` command. Owned removal
removes target assignments and then the dependency when its final target is
removed. If manual external uninstall is still pending, repeat `remove` with
`--external-uninstalled`; the live owner receipt remains sufficient after the
manifest table has gone. Package data and the content store remain retained.

Saved targets must be supported by the typed installer (currently Codex and
Claude). Other or mixed remote target selections fail before acquisition and
provide a `--no-save` command. Explicit local paths, discovery sources,
ChatGPT-only preparation and `--no-save` keep the imperative flow; JSON reports
`environment.saved=false` and its reason. `--no-save` never overrides an existing
environment owner receipt.

Saving publishes the manifest first, followed by the derived lock, after
preflight and confirmation. Each file is replaced atomically; this is not a
transaction across both files. If lock publication fails, no installation is
applied: run `agentplugins deps lock`, then retry, or review `agentplugins deps
sync`. Ordinary commands refuse mismatched intent and lock files until this
reconciliation. An external editor can still race a rename; the session rechecks
published bytes before applying each binding and refuses a changed plan.
