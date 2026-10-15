# UAP dependency environments

`agentplugins deps` manages one current-user environment in
`AGENTPLUGINS_HOME`. `uap.toml` records intent and `uap.lock` records exact
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
