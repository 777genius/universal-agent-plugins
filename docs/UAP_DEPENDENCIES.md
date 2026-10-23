# UAP dependency environments

**Unreleased:** this workflow is available in current source and has not been
qualified as a published CLI release.

`agentplugins deps` manages the selected dependency environment. By default it
uses the current-user environment in `AGENTPLUGINS_HOME` (default:
`os.UserConfigDir()/agentplugins`). With `--scope project`, it uses the selected
existing project's `uap.toml` and `uap.lock`, with local mutable state under
`.uap/`. See [Project installation](PROJECT_INSTALLATION.md) for root selection,
client boundaries and restore commands. `uap.toml` records intent and `uap.lock` records exact
sources, package versions, package digests and dependency edges in one lock. The files are portable; local receipts, the
verified package store, profile paths and `PLUGIN_DATA` are not.

```toml
schema_version = 1

[dependencies]
context7 = "^1.2"

[dependencies.review]
git = "https://github.com/acme/agent-tools.git"
path = "plugins/review"
ref = "main"

[dependencies.docs]
registry = "uap"
distribution = "acme/docs"
release = 42

[targets]
codex = ["review", "docs", "context7"]
claude = ["review"]
```

Git sources may use a canonical GitHub repository, any public/private HTTPS clone
endpoint, an `ssh://` endpoint or native SCP syntax. Direct Git uses existing
native credential helpers/SSH trust and pins an immutable commit. Ready HTTPS
archives use a separate source table:

```toml
[dependencies.release]
archive = "https://downloads.example.com/plugin.tar.gz"
path = "plugins/demo"
# Optional exact original-byte pin: sha256 = "<64 lowercase hex characters>"
```

ZIP, TAR, TAR.GZ/TGZ and extensionless HTTPS downloads use bounded format
recognition and strict extraction. The lock records original-byte SHA256 in
addition to selected package tree/manifest digests; moving endpoints cannot
replace those bytes during locked restore or repair. Local archives are usable
through imperative add, and are not portable dependency requirements. See
[Direct complete package sources](DIRECT_PACKAGE_SOURCES.md) for source grammar,
provider browser URLs, native credentials, selection and exact archive limits.
The standard `plugin.json` format is unchanged; raw skill files are not imported.

Git dependencies use exactly one of `commit` and `ref`; omitting both records
the repository default branch intent. Registry tables retain exact directory
`release` selection. A string under `[dependencies]` selects a Directory package
by its key, using an exact stable version (`"1.2.3"`) or caret range (`"^1.2"`
or `"^1.2.3"`). Short names resolve through the signed Directory. Prefix `v`,
prereleases, build suffixes and other range operators are unsupported.

Caret lower bounds are inclusive and upper bounds exclusive: `^1.2` ends at
`2.0.0`, `^0.2` / `^0.2.3` at `0.3.0`, `^0.0` at `0.1.0`, and `^0.0.3` at
`0.0.4`. Package versions differ from Directory release sequences; the lock
records both. Existing exact Git/release pins may retain non-semver versions.

Packages declare their mandatory dependencies in `uap.toml` at the package root,
beside the standard `plugin.json`:

```toml
schema_version = 1

[dependencies]
"acme/docs" = "^1.2"
```

The same `[dependencies]` section always describes the manifest owner's direct
requirements. Installing a plugin through the managed workflow installs its full
dependency closure. Published package shortcuts must use canonical `owner/name`
IDs; short Directory names are a convenience for the user's root manifest only.
Git, archive and exact registry-release tables use the same syntax in either context.
An absent package TOML, or one containing only `schema_version = 1`, declares no
dependencies. An invalid or unreadable TOML is an error.

Only the selected root assigns `[targets]`. Each dependency inherits the union
of its consumers' targets, including any direct assignments of its own. One
canonical source has one version across the environment. A fresh lock chooses
the lowest compatible stable version, then the lowest release sequence for equal
versions, subject to signed Directory policy for every required target. It tries
later candidates when an earlier candidate has incompatible dependencies;
exhausted conflicts report the package, consumers and selectors without applying
a partial graph. Existing compatible pins take priority over fresh minimums.

Package-local targets are structurally checked and then ignored by consumers;
they are neither a compatibility allowlist nor permission to install into the
author's selected agents. Package-local locks do not select consumer pins.
Both files still contribute to the immutable package tree digest when present.
`plugin.json` remains authoritative for standard identity and components;
`uap.toml` is a UAP manager contract, not an Agent Plugins 1.0 component.
Project-scoped installation uses this same manifest and lock contract through
the qualified project adapters. `[dev-dependencies]` remains unsupported and is
rejected as an unknown field.

`uap.lock` remains version 1 and is the sole dependency lock. Each node records
its exact `version` and immediate `required_by` consumers/selectors, with no
synthetic root edge. Old flat locks remain usable when their pinned manifests
have no dependencies. Pins declaring dependencies in the retired
`extensions.io.github.777genius.agentplugins.dependencies` field are rejected
with `dependency_declaration_moved`, even when TOML is also present. Move the
declaration in the source package, publish a new revision, then explicitly change
the corresponding requirement and rebuild the lock. Immutable old pins are never
rewritten or silently treated as leaves. Removal of unneeded owned packages and
recovery of already pending transactions retain their existing authority.

```sh
agentplugins deps init
agentplugins deps lock
agentplugins deps sync
agentplugins deps sync --locked
agentplugins deps update review
agentplugins deps status
agentplugins deps tree
agentplugins deps tree --format json
agentplugins deps recover
```

`lock` resolves changed selectors and preserves compatible pins. `update
<alias>` rechecks a moving Git ref, unpinned archive endpoint or registry range, updating the selected
root and related dependency pins while preserving independent pins. Registry
ranges stay unchanged in `uap.toml`; apply the new lock with install/sync. Exact
commit, archive checksum, release or semver selectors require an intent change instead.

`tree` reads the saved graph offline, showing roots, versions and consumers;
shared nodes and cycles are references. JSON contains sorted `roots` and
`packages` arrays. It neither fetches sources nor changes environment files.
Manifest declaration keys may differ from stable package aliases. CLI selectors
accept the root key; ownership stays bound to the same canonical source when a
transitive package becomes a direct root or remains needed after its direct
declaration is removed. Ambiguous short names require explicit canonical intent.

`sync --locked` and `install --locked` reject a missing or inconsistent lock
before applying changes. Replay verifies pinned manifest declarations against
the saved edges before no-op or repair, so a deleted edge cannot hide behind an
unchanged root-intent digest. Intact replay uses store bytes without enumerating
new versions or downloading sources. With an empty store, each missing immutable
pin is fetched once per command and its verified bytes are staged for validation
and preflight. Store publication follows confirmation. Dry runs, declined
confirmation and cancellation remove temporary acquired bytes. `status`
is offline and reports manifest, owner/receipt, store and projection facts
independently. `recover` covers one complete owned pending scope and requires
confirmation; run `sync` separately after it succeeds.

`agentplugins install` is the everyday converge verb over the same machinery:
a first run installs everything declared in `uap.toml`, a repeat run leaves
matching state alone, and missing or drifted environment-owned projections are
repaired from their recorded `uap.lock` pin before the machine converges to the
locked state. It accepts `--locked`, `--yes` and `--external-uninstalled` with
the same fail-closed meaning as `deps sync`, and with a positional source it
behaves as `agentplugins add` for that one package.

The default user-scope saved dependency workflow supports the 11 qualified local clients: Codex, Claude,
Cursor, Copilot, Kiro, Gemini, OpenCode, Cline, Windsurf, Grok Build and
Kimi Code. `deps status` prints the scope, exact manifest/lock paths, supported
targets and `excluded_targets`, including before `deps init`. ChatGPT is listed
as `manual_preparation_only`. VS Code is frozen by owner decision; its new
dependency qualification is excluded from this release
and is reported as `dependency_management_deferred`; its existing imperative
adapter and explicitly registered SDK APIs remain available. Initialization creates a private root when necessary
and never overwrites an existing manifest.

Each mutating target must be detected with an explicit local profile. In user scope, CLI
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
| `activation_manual` | Prepared materialized artifact; follow the client's manual steps and use `--activation-complete` only where that adapter supports attestation. |
| `activation_prepared` | Validated prepared artifact; follow the reported next action. Project configuration can remain prepared until native discovery is observed. |
| `auth_unverified`, `auth_pending`, `auth_failed` | Authentication remains separate from activation; complete authentication and use `--auth-complete` when required. |
| `partial` | Failed/unverified activation or incomplete materialization; inspect and retry the failed lifecycle after addressing the cause. |

Manual readiness remains visible and stops automatic repeated installation.
It does not claim observed native activation. For user-scope Cursor, explicit `--activation-complete` uses the existing composed provider's
operator-attestation contract. Its active/installation-verified state is attested,
not observed runtime discovery. Authentication can still remain `auth_unverified`;
this does not require repeating activation. Codex with an unknown
plugin-list contract also retains manual readiness rather than repeating update
on every sync. Failed/not-run verification and unmaterialized packages never
qualify as ready. Persisted lifecycle convergence semantics are unchanged.

Dependency commands always use manifest targets. They reject an inherited
`--target` override and accept explicit `--scope project` for the selected
project environment. Use `--dry-run` for a temporary preview;
it does not write the lock, store, state, profiles or persistent scanner cache.
Native-only/YAML packages and implicit compatibility fallbacks are not part of
this dependency flow.
Imperative paths such as `--no-save` reject packages with mandatory dependencies
rather than installing only their parent. Use the managed manifest workflow for
such packages. Dependency edges describe co-installation and version constraints;
they do not define service startup order or runtime readiness.

Remove a dependency or target assignment from `uap.toml`, then run `deps lock`
and `deps sync --yes`. A shared package stays while a reachable consumer or
direct root requires it; the output names consumers retaining it. Removing a
target drops only bindings no longer required on that target. Unreachable
cycles do not retain themselves. Store originals and `PLUGIN_DATA` are retained. If the
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

A copied registry lock records exact publication identity and does not claim
installed history. Exact locked restore on a fresh machine permits a superseded,
non-revoked release when its signed publication still matches every locked
source and digest field. Current target, host, compatibility and suspension
checks still apply. Revoked releases cannot be restored. Ordinary fresh registry
selection continues to require an eligible active release; installed-state
repair keeps its existing signed-policy rules.

### Restore on another machine

Use a compatible CLI built from current source; the transfer commands below
remain unreleased. In the folder you want to carry to another machine, run:

```sh
agentplugins deps export
```

Copy that folder to the destination machine and open it there. Install the
required target clients, then run:

```sh
agentplugins deps import
agentplugins install --locked
```

Or import and restore together:

```sh
agentplugins deps import --install
```

The optional `[directory]` argument overrides the exchange folder, which defaults
to the current working directory. Without scope flags, export reads the global
user environment and import writes it; `AGENTPLUGINS_HOME` selects that
environment as usual. A project manifest in the current folder does not change
the default scope. With `--scope project`, both commands use the selected
project environment. Choose a separate exchange directory because the portable
pair already lives in the project root; see [project restore](PROJECT_INSTALLATION.md#restore-in-another-checkout).

Plain export/import transfers only `uap.toml` and `uap.lock`, preserving their
exact bytes, comments and formatting. It validates the complete source pair
strictly and offline: missing or inconsistent files, stale locks, unknown schema
fields, unsafe sources and excluded targets are refused. It does not fetch
packages, detect clients or install anything. Store content, receipts, profiles,
settings, secrets and plugin data are not transferred, and a valid pair is not
proof that its sources remain available.

Every actual copy shows a preview and needs a default-no terminal confirmation
or `--yes`, including first creation and replacement of an incomplete or invalid
destination pair. Non-interactive and JSON use require `--yes` for a copy.
Use `--dry-run` to preview without writing; `--install --dry-run` also previews
the locked installation. An identical pair is a successful no-op without a
prompt; `--install` still checks and applies the locked environment.

`import --install` prepares the transfer and completes strict locked-install
preflight before one action confirmation. Current security prompts and target, host, ownership
and recovery policy still apply; `--yes` does not bypass them. A preflight refusal
leaves the destination agreement unchanged. If existing projection drift blocks
the combined preflight, import the files alone, then run `install --locked` to
review repair through its normal flow.

An empty store is supported: installation fetches each exact recorded pin once
and verifies its package and manifest digests. It does not select newer versions,
including when the recorded release has been superseded. Missing source bytes,
changed publication identity, mismatched digests, revocations or other ineligible
registry policy cause an error. Offline file transfer does not promise offline
installation; that requires the package bytes already in the destination store.

File-write errors trigger best-effort restoration of the previous pair. The two
files are not a crash-atomic transaction: an interruption can leave a mixed pair.
To repair it, rerun the original export or import from the intact source and
confirm replacement. Do not run `deps lock` to recreate transferred pins; moving
selectors could resolve differently.

After import successfully saves the agreement, an installation failure keeps it
as the new desired state and reports any partial installation. Address the error
and retry `agentplugins install --locked`; if pending recovery is reported, run
`agentplugins deps recover` first. The installer never falls back to unlocked
resolution.

Configure client profiles, executable discovery and any required authentication
on the destination. Do not copy local ownership receipts as installation proof.
User settings, secrets and `PLUGIN_DATA` contents require their own backup.

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
In user scope, ChatGPT mixed with local targets is rejected before source acquisition: prepare
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


In user scope, ChatGPT uses the existing manual preparation path, separate from reproducible
local profile installation: `agentplugins add <source> --target chatgpt` prepares
an eligible package and marketplace without saving a dependency. App bindings
must come from signed Directory evidence or the verified personal mapping path.
The user installs the package in ChatGPT Plugins, verifies the remote connection
in a new chat, and can attest with `--activation-complete`. External uninstall
precedes `remove --external-uninstalled`. This does not install a local stdio
server into ChatGPT or claim automatic remote connector registration.

All ChatGPT project requests are explicitly unsupported; `--no-save` does not
override that boundary. See [ChatGPT and project scope](CHATGPT_PROJECT_SCOPE.md).

These flows share one installer lifecycle, ownership receipts, immutable package
store and locks. Project scope selects separate mutable ownership through that
same lifecycle. These changes do not expand the separately qualified
local-file facade route, or publish a CLI/npm/site release. Adapter contract tests
use disposable profiles, fixture native processes and a small trusted helper
fixture whose real digest/copy/metadata contracts are exercised; they are not qualification
of running native clients.
