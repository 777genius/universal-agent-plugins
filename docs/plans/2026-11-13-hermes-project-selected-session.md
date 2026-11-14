# Hermes Project selected-session integration candidate

The integrated candidate extends the create-only root publication from
`4c096eea961c1758bdc8779d43999355c7cd5ef0` with original ancestor validation
and state-save guards, and uses one public `Adapter`/`New()` composition. The temporary `NewProject()` constructor and selection flag are
removed. Project MCP is `Prepared` only for the selected official standalone
distribution at vendor commit `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`;
source-installed and unqualified executables retain a nonmutating unsupported
Project MCP plan. Existing User support and Project Skills remain separate.

This source integration makes the ordinary public CLI available to the genuine
acceptance lane. It is not a released support claim or completed native
qualification. The new public path, cold restore and all applicable platform,
shared and security gates must pass before merge. No other client, login,
workspace trust or release is added by this candidate. Existing schema versions
are unchanged; the additive private publication-parent field is described below.

## Selected scope and effects

The flat native root is exactly `PROJECT/.hermes/uap-session`. Its parent
`PROJECT/.hermes` remains the Project binding profile authority. Original User
configuration and credentials are neither copied nor modified. Project Skills
continue to use `PROJECT/.hermes/skills` and managed packages/data keep their
existing `.uap` locations and receipts.

| Operation | Candidate behavior and permitted effects |
| --- | --- |
| Prepare / dry run | Existing immutable package planning and ownership reads; refuse a preexisting selected root without a committed standalone receipt, even an empty root or an identical marker. |
| Confirmed install | Existing package/data transaction and native-attempt marker; exclusive directory publication creates the selected root and `.uap-owner`; the existing component mutation installs Skills and exact owned `mcp_servers` entries in `config.yaml`. |
| Repeat / doctor | Read exact root publication and component receipts; repeat is unchanged, and no native executable runs. |
| Update / repair | Reuse the same owned root/data; the existing native component transaction checks exact entry ownership, compensates partial effects, and refuses foreign edits. |
| Remove | Remove only owned YAML entries, Skills and managed package. Retain shared root, marker, standalone receipt and persistent data. No runtime history/cache deletion is authorized. |
| Explicit run | Plan existing managed Project bindings, preserve inherited OS HOME, and override only child `HERMES_HOME`; one bounded verification interval joins the observed version/install directory, `config path` and `config get mcp_servers --json`. `--inspect` selects that exact JSON inventory; ordinary launch adds no invented flags. |

Install, doctor, repair and remove never start Hermes, authenticate, invoke a
model, or infer runtime activation. Successful installation remains
`Prepared`/`Installed`, without an attested activation or authentication result.

## Durable root publication

`ports.NativeProjectRootPublisher` is the narrow client/composition seam.
`transaction.ProjectRootPublisher` contains no Hermes policy or client branch;
the Hermes adapter supplies the exact path, marker object and bounded marker
body. Installer and CLI compose the publisher with their existing kernel and
state store. The marker is not independently sufficient ownership.

`DirectoryMutation.Standalone` is the create-only counterpart of the existing
standalone directory removal receipt. It requires `RequireAbsent`, an exact
unchanged desired state, and a verifier. It rejects lifecycle/native metadata,
replacement and grouping. The ordinary directory mutation behavior is unchanged.
The auxiliary receipt goes in existing `State.TransactionReceipts`; package
TargetLocator, binding receipts, NativeObjects and lifecycle are untouched.
The existing schema versions, state store and transaction framework remain in
use. Fresh standalone swaps additionally record their original parent authority
in the journal and state receipt; existing ordinary receipts keep their nil form.

The publication digest binds the root and immediate parent's held physical
identities, the logical root, and marker digest. `LegacyHandleIdentity` uses the
existing POSIX device/inode or Windows volume/file-index encoding directly from
the held directory handle. It neither opts into another identity schema nor
reopens a replaced name. Reusing the root requires exactly one committed
standalone receipt plus the physical and marker verification. That authority
survives removal of the first installation and can serve another installation.

Fresh staging and marker operations use held `os.Root` authority, not a
path-based temporary-directory or parent-following marker read. Named/held joins
surround the first effect/read and final verification. Bounded marker reads use
the existing rooted file helper. Before kernel handoff, cleanup removes only the
exact newly created marker and known empty directories through held parents;
foreign children, replaced identities or uncertainty are retained. After kernel
entry, cleanup is allowed only if the operation journal is absent and reloaded
state exactly equals the predecessor. An open journal or unknown state retains
kernel ownership for recovery.

Independent review added a concrete moved-staging counterexample. Cleanup now
rejoins the entire named base/staged/selected chain before its first marker read,
before deleting that marker, and before each empty-directory removal. A moved
owned stage and an empty replacement both survive the refusal. The held handle
alone never authorizes cleanup after the recorded pathname changes.

## Session source and native admission boundary

The planner verifies the committed root receipt, exact Project binding objects
and one immutable, bounded `config.yaml` snapshot. The Hermes codec checks all
owned entry digests against that same body. Duplicate YAML keys, aliases, merges,
multiple documents, unrelated settings and unowned servers refuse launch.
JSON returned by the native inventory must have unique keys and match the full
source `mcp_servers` mapping. The native path must identify the exact selected
config, not a substring or a fallback User profile. Failed, truncated, canceled
or incompletely cleaned-up native commands do not establish evidence.

The official snapshot's observed banner is exactly
`Hermes Agent vunknown (2026.9.24)`, followed by the selected install directory.
An unknown, missing or repeated banner stops verification after that first
observation; it does not become an inferred `0.23.0` version. The manifest's
exact vendor ref and contained distribution layout remain separate inputs.

Known alternate sources must be absent before the first native observation and
at every subsequent boundary. The pinned
[backend selector](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/config_backend.py)
must resolve to the default `file` backend. An explicit managed-directory
override, a node at the pinned
[system managed location](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/managed_scope.py),
or the installed `hermes-agent/.env` fallback refuses launch without reading or
disabling policy. On Windows the pinned root-relative `/etc/hermes` uses the
selected command directory's volume; no guessed ProgramData location or vendor
test-only waiver is used. The selected home's `.env` and `.op.env` are already
excluded by the finite member catalogue. A matching `config path` or MCP mapping
alone would not exclude additional managed settings.

The durable physical root and parent receipt, followed by the immutable source,
are rejoined before and after each native observation and before success. Actual
SDK regressions replace either the root with an identical-byte clone or its
parent while preserving the root inode, at both native boundaries. Each refuses
at that boundary without issuing a subsequent command or changing stored state.

The source preparer keeps the original strict source/marker reads and admits a
finite bootstrap catalogue based on the complete original macOS capture below
and the pinned producer. The fourteen possible top-level names are the three
owned config/marker/lock members, `SOUL.md`, and the ten fixed native skeleton
directories. Only `logs/{agent.log,errors.log,curator}` and
`backups/config/config.yaml.good.YYYYMMDD-HHMMSS` may add fixed descendants;
the other skeleton directories and `logs/curator` must be empty. There may be at
most five good backups. Unknown names, redirects, aliases, extra descendants,
unowned nodes, multiple links and oversized leaves refuse before a native child.
This is a bounded catalogue, not a recursive adoption of native history/cache.

The source and marker retain their private 0600/0700 rules. Only the two observed
log leaves use 0644, and only the backup directories use 0755 on POSIX; the
original owner/handle checks remain. Logs and backup leaves have individual
1 MiB bounds. `SOUL.md` must match the pinned 667-byte default and its digest;
the literal has no newline, so no platform-dependent newline variant is added.
These native files are never copied into the binding source or deleted by UAP.

Pinned [backup creation](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/config_backups.py)
reads the newest good backup for byte deduplication even when the primary is
valid. Historical backup bytes may therefore differ from the current config.
Semantic fallback occurs on the primary read/merge exception paths in
[config loading](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/config.py)
and [effective config loading](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/config_effective.py).
UAP requires the existing codec to validate a complete receipt-owned primary
before the first child and rejoins that exact source around every observation.
A missing, malformed, unowned or changed primary refuses; a plausible backup
does not replace it. Native readback must still equal the entire owned mapping.
This does not claim that a vendor OS-read failure can never enter its fallback.

The original capture uses a different fixture path. It supports this finite
policy's review; it does not itself qualify the production
`.hermes/uap-session` path or the other operating systems. Corrected source-bound
public captures remain required, including update with retained historical
backups and repair of a missing owned primary.

POSIX source files require the current UID, private mode 0600 and one link; the
selected root requires current UID and mode 0700. Leaf opens are rooted,
nofollow and nonblocking, and FIFO replacement refuses before reading. Windows
checks ordinary non-reparse held files/directories and single-link files, with
the actual held handle's owner equal to the current token's exact `TokenOwner`.
An elevated token may have a group default owner different from `TokenUser`;
there is no user-or-group fallback and no ACL mutation. This owner check does
not assert that Windows inherited ACLs are private. Actual Windows execution,
including differing default-owner semantics, remains a qualification gate;
cross-compilation is not native evidence.

Token owner semantics: [Microsoft TOKEN_OWNER reference](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-token_owner).

## Verification and recovery limits

Local tests exercise the actual public installer SDK with only the candidate
adapter injected and every native process forbidden: Project A/B install,
repeat, expanded package/data paths, immutable data authority, fresh-engine
update, repair of an absent owned Skill, inspect, remove, retained root/data and
unchanged original User configuration/auth sentinel. Conflicting roots and
edited owned MCP entries refuse before effects. Session planning reads the real
persisted binding and standalone root receipt.

Kernel tests use real subprocess death before rename, after rename and after
the state decision. Recovery rolls back the uncommitted root, or finalizes the
standalone receipt without requiring the original installation to remain.
Publisher tests cover root and parent replacement, marker-only forgery,
substitution before a marker write/read, cleanup of unused staging after kernel
preflight refusal, and retention/recovery after an open-intent failure.

A separate actual SDK subprocess exits after the standalone root receipt is
committed and before the binding acknowledges the native effect. The root and
package authorities remain distinct; MCP has not been written and the native
attempt remains pending. The fresh facade reports the existing generic Project
uncertainty and refuses guessed recovery/repair, preserving the complete tree
and original User files. This is the same declared boundary as existing Codex
Project process-death handling. Automatic recovery of an unrecorded full native
attempt is not implemented or claimed by the auxiliary-root transaction.

Before merge: reconcile the moving PR head, obtain corrected original
three-platform public admission artifacts, execute the real Windows owner/handle
tests, and verify public CLI add/repeat/update/repair/run/remove with genuine
native source/test/restart evidence. These gates must not be reported as
satisfied by inert SDK tests, cross-builds, or marker equality.

## Public cold restore and source compilation

The genuine bundled workflow builds the public UAP, the existing Go native MCP
probe fixture and the cold restore test executable from one clean checkout before any
profile/runtime baseline. It records their bounded SHA-256 values and the exact
source head. Compiling beforehand keeps compiler caches outside the observed
native interval; no fake Hermes executable or detected version is introduced.

`TestProjectHermesBundleColdRestoreAndRepair` reuses the existing TLS Git and
public CLI/store fixtures, production `ForScope` composition and OS detector.
It initializes independent Git projects A/B, exports only `uap.toml`/`uap.lock`,
advances the real remote, and restores the original source/pin in a fresh cache
and Project state. It then checks unchanged locked replay, missing Skill/config
repair, preserved data authority and removal. The whole User/default-profile,
Project B, Project Git metadata and foreign-file sentinels remain unchanged.

Each original, cold and repaired inspection executes the actual public Cobra
command with explicit `--confirm-client-start`, production composition, native
detector and session verification. Its existing final session callback runs the
exact selected native command through the ordinary process runner instead of
replacing the Go test with Unix `syscall.Exec`. All native children therefore
have one live Go owner and cancellation context. This is reported explicitly as
`public-cobra-in-process` with `installer_executed=false`; the separately built
public UAP binary is provenance for this cold scenario, while the warm public
CLI lifecycle exercises that executable separately. The actual native argv,
selected `HERMES_HOME`, original streams and completed cleanup must join the
public Project request and its recorded root. Complete native JSON must match
the exact owned YAML.
Installation receipts remain `Prepared`/`Installed`; an HTTP sentinel must receive
zero requests during config inspection. Native inventory is not a tools/call.

The test explicitly checks the marker/root object and exactly one committed
standalone root receipt. After removal the entire receipt, marker bytes, root
and parent physical identities and modes must be unchanged. Its bounded fresh
report is written only after the complete scenario; the collector also requires
the real test's successful exit, exact execution marker and no skip. Source,
compiled-binary, immutable runtime and process closure remain necessary before
the outer capture can complete. Local compilation or an absent-opt-in skip does
not count as this hosted native test.

One parent-supplied absolute deadline covers the cold scenario without renewal.
The Go context expires thirty seconds before the earlier parent/test limit and
also handles termination signals. Real Git setup, CLI acquisition, detection
and native inspection inherit it. The source TLS server and HTTP sentinel must
finish before a successful report. The standard CGI backend remains in the
outer process group; a blocked server join is a failed, retained scenario and
does not establish cleanup. This does not claim that killing an outer Darwin
process group settles independently created nested groups.

The parent creates and holds the one private cold root before launch. The Go
fixture allocates beneath it without automatic test-directory deletion. Only a
complete validated report and successful test exit authorize parent cleanup;
failure, cancellation, a missing report or unproved process closure preserves
the fixture and withholds raw capture. The original 720-second observation
interval is not extended by cold setup or retry.

## Independently audited original macOS capture

[Run 43505052472](https://github.com/777genius/universal-agent-plugins/actions/runs/43505052472),
artifact `14077169877`, source `95f8648b4ffe03b729c16c3a8b1506f922f33f4e`:
the 39,000-byte original ZIP has SHA-256
`22634998a99e7595d9802e1678179c6b1608797b0b9a51edbf0aa4260afc79db`.
Independent inspection verified its 71 closed members/CRCs, all 70 report stream
digests, 21 command status/stream joins and the chronological 19-command inventory
preimage/postimage chain. The pinned vendor source and complete bundled runtime
inventory remain unchanged. A/B native path and full MCP JSON are exact; tests,
restart, removal, absent listing and expected removed-test exit 3 agree with
the endpoint methods. Every Project command preserves the original User and
sibling root; no unadmitted shared effect appears.

This macOS capture observes twelve directories, the fixed 667-byte `SOUL.md`,
one initial config backup, and initially empty agent/error logs. Only the agent
log grows during the MCP tests/removal, and each post-removal list adds a second
config backup. Native output reports `Hermes Agent vunknown (2026.9.24)`; source
pin attribution must not be relabeled as a measured semantic version. Cold
bootstrap happens on `--version`/the guard, before native config path/get, and
the fixture uses `.uap/hermes-runtime`. Consequently this capture does not yet
qualify arbitrary installed Hermes executables, cold config-path startup,
interactive session writes, the production `.hermes/uap-session` path, or the
other two operating systems. All product/admission flags remain false.

## Ordinary source launcher and bootstrap boundary

The original source [run 43424755622](https://github.com/777genius/universal-agent-plugins/actions/runs/43424755622)
at UAP `aabcd9b3e763d4341884173d37cdd5b1f7d7caf8` records the installed banner
`Hermes Agent v0.23.0 (2026.9.24) · stable · local 2bd378ed1b8d` on Linux
job `131883336442`, macOS job `131883336854` and Windows job `131883336901`.
This is an actual User-install version observation, not Project bootstrap
qualification. The bundled `vunknown` result remains a separate runtime route.

The old source collector booted its observer under a new disposable OS HOME and
selected-profile before its initial inventory. The source had been installed at
`RUNNER_TEMP/hermes-agent` using a different custom selected-profile home. At the
pinned vendor source, `pm.environments.owning_home_root` searches checkout.parent
and the current platform default home for actual committed state. That fixture
can lose recognition of the installation owner; its large observer bootstrap
does not establish ordinary borrowed-Project effects with the installed User
preserved. The next measurement must reuse the existing source fixture/collector
with a recognized real owner and unchanged official installation, preserving
the original OS HOME and installed native User root across A/B. Owner facts must
not be copied or fabricated, and native commands must use installed `HERMES_BIN`.

The immutable input boundary is the original User/source/runtime identity plus
the exact fresh selected root and parent identities and authored configuration.
Only HERMES_HOME changes between Project selections. Native output must join exit
status, bounded streams, exact config path and complete MCP mapping, endpoint
events and a complete bounded member delta; original User, source/runtime and
sibling effects remain separately checked. The source launcher prepares the
selected data root's own dependency generation and may ensure required tools.
Its PM state and bootstrap are client-owned, distinct from UAP's marker/config;
this candidate does not adopt that tree or implement another dependency installer.

Interruption retains the original input and failed-command postimage after
whole-process settlement. A later ordinary native launch must demonstrate its
own recovery under the same launcher/source/root authority; no baseline renewal
or deletion of native update markers is permitted. The standalone root journal
alone does not establish that recovery. Also, source `--version` skips dependency
preparation while the launcher still reaches dependency activation: success in
a fresh Project home cannot be inferred from the measured User banner.

The original 95f Linux bundled job `132163259930` subsequently completed with
the existing 120-second owned-build cleanup timeout, after official payload
verification and a complete metadata census. Native admission was skipped and
no Linux Project artifact was published. The corrected cleanup budget needs its
own genuine result; this failure does not qualify the Linux route.

## Original publication parent amendment (2026-11-14)

The initial candidate admitted a replaced Project ancestor when its original
children, including the selected root, immediate parent, state and journal,
were moved back under the same path. Two actual counterexamples cover both a
committed SDK-created root and an activated journal before the state decision.
A fresh SDK instance accepted the former; recovery could roll back the latter.
An immediate-parent identity and marker digest did not establish the original
Project ancestry.

The corrected writer captures the complete parent authority before staging,
joining that observation to the already held parent directory handle through
`directoryidentity.CaptureHeld`. Ordinary `Capture`, `Revalidate`, the authority
encoding and unsupported-platform refusal stay unchanged. The immutable token
is copied into the first original dirswap journal and copied from the actual
returned journal receipt into `State.TransactionReceipts`. It is not a physical
profile owner grant, and no `ProfileOwners` are fabricated.

`publication_parent_authority` is an additive private JSON field in raw dirswap
and state mutation receipts. It is mandatory for a fresh standalone swap and
invalid on an ordinary binding receipt or standalone removal. The original
canonical parent must match the exact publication roles. Manager validation,
every phase save and terminal journal unlink revalidate that same token before
physical checks or effects. Store loading and both Kernel recovery entrypoints
validate it before journal enumeration, including injected stores and a missing
journal manager. A raw/state token mismatch refuses before deciding rollback;
removing only the raw token does not turn an acknowledged publication into an
unacknowledged rollback.

The same recorded parent authority remains required for subsequent root and
session verification. It is never recaptured from a replacement path. Caller,
raw receipt, returned receipt and saved state cannot share a mutable authority
pointer. Existing full publication-digest checks still govern initial directory
commit or rollback. Later native writes acquire no recursive-deletion authority
from this parent token, and final binding removal retains the root as before.

Compatibility is deliberately bounded. Production before `4c096eea` recorded
standalone directory removals, not standalone swaps; those records and ordinary
binding swaps retain their existing nil-field form. The unregistered `4c096eea`
prototype's standalone swaps without original parent authority are refused,
not migrated or adopted. An older raw schema-4 journal before any state receipt
has no discriminator proving it was this prototype: path suffixes and current
contents cannot manufacture that missing original fact. No automatic safe
recovery claim is made for those historical experimental journals.

Verification covers original parent substitution in the real SDK and original
pending journal, phase-save and terminal-unlink preservation, actual raw-to-state
copying, a canceled commit after durable state save, finalization after journal
removal, stripped-token refusal, concrete and injected stores, and same-held
capture with cancellation or a replaced root. These are filesystem and inert
process contracts. The ancestor regression now uses the single qualified
`Adapter` fixture. These checks do not qualify ordinary source-installed Hermes
bootstrap, authentication or native execution on an unmeasured platform.

## State-save completion amendment (2026-11-14)

A separate review reproduced two remaining write windows in the adopted parent
amendment. An already loaded state could be saved after replacing only the
Project ancestor and moving its original children back. The terminal state save
could also cross that substitution after the kernel check and journal commit.
Both real filesystem regressions failed before this correction: the write
returned success and replaced the original state file. An injected store also
observed unwanted save attempts through `PersistStateDecision`.

Concrete `Store.Save` now revalidates the recorded parent before directory
creation, chmod or atomic writing. The kernel's private `saveState` performs the
same existing authority check before delegating to an injected store. All eight
initial, durability retry, terminal and recovery saves use that helper. The
existing background-context semantics remain unchanged; cancellation does not
invent rollback permission. No new interface, state schema or ownership grant
is introduced. The synthetic migration pending-work fixture now uses the actual
pre-prototype standalone producer, `directory_remove`, instead of fabricating an
unsupported old standalone swap.

These checks refuse replacement already observable at a save boundary. The
existing pathname-based atomic writer does not provide atomic exclusion of an
ancestor rename after its last check; this amendment makes no such claim.

Both new regressions passed after the guard, including zero injected writes and
unchanged original state bytes, identity, mode and modification time. On the
adopted candidate based on `1d64a274`, uncached affected tests passed with no skips:
138 directory-identity/dirswap cases, 472 state/transaction/domain cases and 55
SDK migration/Hermes/lifecycle cases, counting subtests. They establish the
filesystem boundary and existing receipt compatibility, not native qualification.
The combined single-Adapter source still requires its own shared gates and
genuine native runs.
