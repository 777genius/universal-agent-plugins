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

The saved dependency workflow supports the 11 qualified local clients: Codex, Claude,
Cursor, Copilot, Kiro, Gemini, OpenCode, Cline, Windsurf, Grok Build and
Kimi Code. `deps status` prints the scope, exact manifest/lock paths, supported
targets and `excluded_targets`, including before `deps init`. ChatGPT is listed
as `manual_preparation_only`. VS Code is frozen by owner decision; its new
dependency qualification is excluded from this release
and is reported as `dependency_management_deferred`; its existing imperative
adapter and explicitly registered SDK APIs remain available. Initialization creates a private root when necessary
and never overwrites an existing manifest.

Each mutating target must be detected with an explicit local profile. CLI
registries (Codex, Claude, Cursor, Copilot, Kiro, Grok) also need an
explicit absolute executable, as does OpenCode's host-version probe. Native
configuration clients (Gemini, Cline, Windsurf, Kimi) do not require an executable.
Any supplied executable must be an absolute clean path. A detected client with
a discovery error cannot authorize installation, update or repair. Removal can
use an existing recorded profile even when the client is no longer installed.

Support is per component, not a promise that every package feature works in every
client. Preflight rejects packages with no supported components. A mixed package
installs its supported components and reports `component_support` for each binding
in sync/dry-run/saved-add JSON; human output identifies unsupported components.
For example, Kimi supports skills but cannot install an SSE MCP server. Partial
component support alone does not trigger repeated lifecycle updates.

| Status fact | Meaning and follow-up |
| --- | --- |
| `activation_manual` | Prepared materialized artifact; verify activation in the client, then use `agentplugins add <same-source> --target <agent> --activation-complete`. |
| `activation_prepared` | Validated prepared artifact; complete the client's manual steps and attest activation with the same command. |
| `auth_unverified`, `auth_pending`, `auth_failed` | Authentication remains separate from activation; complete authentication and use `--auth-complete` when required. |
| `partial` | Failed/unverified activation or incomplete materialization; inspect and retry the failed lifecycle after addressing the cause. |

Manual readiness remains visible and stops automatic repeated installation.
It does not claim observed native activation. For Cursor, explicit `--activation-complete` uses the existing composed provider's
operator-attestation contract. Its active/installation-verified state is attested,
not observed runtime discovery. Authentication can still remain `auth_unverified`;
this does not require repeating activation. Codex with an unknown
plugin-list contract also retains manual readiness rather than repeating update
on every sync. Failed/not-run verification and unmaterialized packages never
qualify as ready. Persisted lifecycle convergence semantics are unchanged.

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

Saved targets must be registered local clients supported by the typed installer.
ChatGPT mixed with local targets is rejected before source acquisition: prepare
ChatGPT separately, or explicitly choose the imperative `--no-save` flow. Explicit local paths, discovery sources,
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


ChatGPT uses the existing manual preparation path, separate from reproducible
local profile installation: `agentplugins add <source> --target chatgpt` prepares
an eligible package and marketplace without saving a dependency. App bindings
must come from signed Directory evidence or the verified personal mapping path.
The user installs the package in ChatGPT Plugins, verifies the remote connection
in a new chat, and can attest with `--activation-complete`. External uninstall
precedes `remove --external-uninstalled`. This does not install a local stdio
server into ChatGPT or claim automatic remote connector registration.

These flows share one installer lifecycle, ownership receipts, immutable package
store and locks. They do not enable project scope, expand the separately qualified
local-file facade route, or publish a CLI/npm/site release. Adapter contract tests
use disposable profiles, fixture native processes and a small trusted helper
fixture whose real digest/copy/metadata contracts are exercised; they are not qualification
of running native clients.
