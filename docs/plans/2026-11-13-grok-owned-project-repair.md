# Grok exact Project enablement through repair

## Current safety boundary

The `2b557b30e57206485aa35b79986da2ecff75f691` prototype is not admitted
for native Project enablement. The adapter advertises no explicit enablement
support and refuses confirmed enablement before recording an attempt or starting
ACP. Direct adapter calls have the same refusal. Removal of a prototype enabled
member refuses before ACP; ordinary Prepared add, repair and directory removal
remain available. Legacy User behavior is unchanged.

Original warmed-profile evidence already shows ACP log and SQLite writes outside
the owned config member. The private membership journal cannot restore those
writes and carries no original physical User-parent authority. Three causal
inert regressions on unchanged `2b557b30` reproduced the consequences:

- First ACP failure after an auxiliary write reported `NativeEffectUnchanged`
  before any recovery record existed.
- A failed second ACP left an auxiliary postimage after recovery reported
  success and cleared both the attempt and member journal.
- Cold recovery accepted a substituted User parent with the original config
  and lock leaves moved back, then rewrote config/state and deleted the journal.

Public recovery now observes and retains/refuses prototype work. It does not
reconstruct authority from the current path, clean unpublished stages or claim
that undoing a config span recovered native startup effects. The regressions
check original bytes, inode, mode and mtime, including the observed-reconciliation
entrypoint. For new enablement, public SDK and Cobra tests check zero native
calls, no new attempt and unchanged files, alongside the plain Prepared lifecycle.
Historical owned removal invokes no native client and preserves exact predecessor
state bytes and native/package files. The existing generic core may persist and
acknowledge its own unchanged removal attempt, changing state inode and mtime.
Historical owned-state fixtures are explicitly authored test data, not native
enablement proof.

The design below records the superseded prototype and its intended future
consumer. Its private codec, protocol and finite journal component tests remain;
public enablement and recovery acceptance are deliberately unavailable. No
native workflow or lifecycle predicate was weakened to turn this refusal into
acceptance. Genuine native lifecycle gates therefore remain incomplete.

Reopening requires a source-qualified native route with bounded startup effects
and exact recovery **before the first ACP call**, including original physical
parent authority retained through interruption and cold retry. Config membership
undo alone is insufficient. A documented auxiliary-state redirect would also
need native proof that original selected config, registry, auth and trust remain
unchanged; the pinned route examined so far supplies no such admitted contract.
Final-source metadata/foreign-MCP, held-lock and actual full-ID native lifecycle
proof remain separate prerequisites. No generic flag, copied profile, whole-file
foreign-state restoration or broader version admission supplies that proof.

## Superseded prototype scope and publication boundary

This candidate replaces the unpublished standalone enable-command proposal.
The sole public consent surface is `repair <installation-or-alias>
--target grok --scope project --confirm-plugin-enable`, with the ordinary repair
plan confirmation. `--yes` alone grants no enable consent. One explicit target,
one selected installation and Project scope are required. SDK callers supply
the same transient `Request.ConfirmPluginEnable` before `Prepare`; changing the
original request or diagnostic JSON cannot alter a prepared operation.
For this consented repair only, the mandatory target selector cannot skip the
existing interactive repair confirmation. Decline and EOF leave native state
unchanged; `--yes` and existing JSON automation still require the explicit flag.

This is Linux-only prospective product work for Grok 1.0.70. It does not promote
the existing User integration, trust a project, start a conversation, authenticate
an account, invoke a model, or qualify native MCP. Activation remains Prepared.
Publication of native effects still requires the accepted original pinned
metadata/foreign-MCP witness and a genuine public lifecycle on the final source.
The old generic native API failure remains a failure. Source and inert contracts
are not substituted for those original native results.

The latest audited generic gate is job `132191360314` on
`0dd77f85ca682989dadbde430fc371b6574f61c3`, artifact `14084069531` (SHA-256
`edd6aed2edf0d38297fd609ddba063dd560e6b6b45eddc473e1ee09ed87424e2`).
All 20 original embedded stream hash/length joins match. Native `plugin list
--json` returns `[]` in both projects while `inspect --json` reports each exact
prepared Project path. The gate refuses before enablement because it has no
exact admitted plugin ID. This candidate has not repaired or superseded that
negative result, and Project evidence does not establish a User lifecycle.

## Existing lifecycle and explicit authority

The CLI forwards consent through its existing saved dependency repair, typed
EnvironmentAuthority and public installer facade. The core's existing repair
path persists NativeActivationAttempt before native effects. Preview and
unconfirmed Apply do not invoke ACP. A repeated confirmed repair reaches the
same owned-member verification rather than an intact-package no-change shortcut.

Composition captures the already resolved UserHome once. Config.UserHome and
clients.Env.UserHome are transient; no persisted schema or ambient home fallback
is introduced. Empty or different home refuses native effects and native recovery.
The Grok recorder is constructed once per lifecycle composition and supplied to
activation/removal and the same existing transaction recovery dispatch.

Removal carries the original frozen NativeAttemptIdentity, including a complete
attempted-state digest derived by transaction.ComparableStateDigest from the
kernel's existing comparable serialization. This is transient request data.
The adapter joins operation, installation, binding, profile root, full state,
artifact, package digest and exact NativeObjects before ACP or config mutation.
A replacement attempt, same-ID foreign-state edit, or missing proof refuses.

## Native route and effect boundary

The metadata route is the pinned sessionless initialize followed by
`_x.ai/plugins/list` using a fresh nonexistent session ID. No session is created
or resumed. Initialization version/project and the complete full-ID/path/name/
scope/version inventory must match. Duplicates, aliases, conflicts, unknown
notifications and incomplete output refuse. The only admitted notification is
the measured empty MCP metadata event.

The consumer retains a CLOEXEC duplicate of the original nonblocking stdout
pipe without consuming it or changing file-status flags. Only after successful
owned-family cleanup does it drain prefetched and remaining bytes to bounded
EOF under a mandatory read deadline. Extra frames, partial tails and cleanup
failure refuse. Native foreign-MCP marker absence is bounded observed absence
of witness execution, not a universal claim that no process was ever scheduled.

The native permanent `.config-init.lock` is held through original config read,
the native before/after inventory, exact membership edit, state decision and
cleanup. No lock file is created or replaced. The existing ExactFile kernel
performs the config CAS and rollback. Only a full project ID is inserted into
the enabled array; prior disabled name/full-ID preferences and ambiguous aliases
refuse. No bare-name or implicit trust fallback is used.

## Owned member, staging and recovery

The ownership object is `grok_enabled_plugin`, bound to the independently owned
artifact, native full ID and explicit profile hash. Existing update/repair
preflight holds and checks the exact current config member and original receipt.
Missing, disabled, changed-ID or foreign-home ownership refuses before package
mutation. Unrelated native members remain intact. Staging carries only the exact
acknowledged receipt alongside the new managed package receipt; it does not infer
loading or enablement from a package version change. The durable package commit
boundary retains this receipt even if the later activation callback stops.

The client uses one private extensionless record in the existing operations
namespace. It is staged privately, fsynced and atomically published with a
held-parent no-replace rename. The record contains operation/package/state
digests and one reversible owned insertion/removal span, never a raw config,
credential, complete previous state, or copied provider configuration.

Enablement recognizes exact before, pending and target whole states. Removal
uses a finite bounded footer for the exact desired state already constructed by
the existing core; no arbitrary timestamp/status normalization is admitted.
Incomplete footer bytes allow only exact-pending rollback. A complete footer
and cleared marker retain the same operation/environment recovery authority.
Third-version config, full-state drift, identity substitution or unknown stages
retain/refuse instead of overwriting foreign state.

The existing facade observation/recovery fence delegates to registry recorders.
It revalidates unique operation/installation/binding observations and derives
environment ownership from current typed receipts. Pending/Inspect only read.
OpenCode's existing observation shape is moved to a ports contract with a source
alias; its persisted records and native reconciliation are unchanged. No second
controller, public state schema, separate recovery store or client-ID branch is
added to generic dispatch.

## Historical prototype contracts and remaining native gates

The original `2b557b30` contracts used real statev2, package projection, directory/kernel
transactions and inert native protocol pipes. Its public SDK cases covered frozen
consent, preview/cancel with zero ACP, exact enable/readback/repeat/removal, foreign
config preservation, explicit-home refusals, prior disablement, saved dependency
ownership, cold rollback, full removal-decision recovery after marker clearing,
partial-publication read-only inspection, and changed original removal authority.
The existing OpenCode public facade durable/crash/foreign-fact tests also pass.
The original single/group removal decision/no-effect/attempt tests remain gates.

The original public Cobra saved lifecycle used the supported local TLS ZIP source:
add, ordinary `--yes` repair, consented dry-run, invalid scope/target refusals,
explicit installation repair, alias repeat and remove. Only the vendor transport
was inert. Current public tests instead require the safety refusal described
above, while retaining the ordinary Prepared lifecycle. A separate attempted local TLS Git journey failed before Grok at
production process containment (`ordinary_unsupported`); that failure is retained
and is not a native success or a reason to relax process containment.

Required before promotion: final-source original metadata/witness admission,
combined held-lock native readback, successful exact public Project lifecycle
including MCP where qualified, original User lifecycle audit, independent code
review, final branch CI, merge and post-merge checks. This document records the
reviewable candidate, not completion of those gates.

## 2026-11-14 private cold-parent correction

The public safety refusal introduced in `ef8968d` remains in force:
`SupportsPluginEnablement` is false, direct activation/removal refuse native
effects, and public recovery does not acknowledge incomplete auxiliary-effect
recovery. The first ACP exchange still lacks a durable record for all startup
writes. No native qualification follows from this correction.

Private member records now use version 2 and require the existing immutable
`directoryidentity.Authority` for the original User parent. The prototype
captures it from a no-follow held directory before the first inventory exchange;
the value includes the complete canonical ancestry. Completion and rollback
revalidate that same value before config writes, state decisions and committed
journal mutations. Journal publication, footer append/truncate and record unlink
retain the existing held-operation-directory and held-leaf joins. Moving the
original config and native lock leaves into a replacement parent does not grant
that replacement recovery authority.

The existing 32 KiB whole-record bound, strict unique JSON decoding, record hash,
finite footer, whole-state digests and exact config CAS remain. Version 1,
missing/null parent authority and a different canonical root fail closed, with
no recapture or migration. An empty unpublished stage still has no native-effect
authority; only its existing held-namespace cleanup is retained. A complete
unpublished record must pass the new parent join before cleanup. No public state
schema, ownership object, configuration body or credential is added.

The portable ExactFile CAS and directoryidentity algorithms do not claim atomic
exclusion of an external rename after their final observation. The public guard
therefore remains unchanged; the separate first-exchange and readback auxiliary
write scenarios remain unresolved. New low-level tests exercise cold recovery
through a newly constructed state store, all original enable/remove durable
boundaries, same-leaf parent substitution, late replacement before writes, record
publication/footer/unlink boundaries, and pre-inventory capture. The three
published public-guard attack regressions are retained.

### Prepared removal on non-Linux hosts (2026-11-14)

The original Terminal UI run `43559930375` on head `5ff83587` failed
its existing composed Grok removal tests on macOS (job `132341201150`)
and Windows (job `132341201030`). Both original logs name the unsupported
enablement journal as the first removal refusal. These were prepared packages
without owned enabled-membership records; the ordinary removal core had created
a fresh native-attempt identifier. Linux distinguishes a missing journal from
an existing record, whereas the non-Linux stub refused even that absence check.

The non-Linux read entrypoint now validates the operation ID and uses the
existing anchored, non-following, nonblocking path opener solely to establish
actual absence. It returns the original error unchanged. Any existing ordinary
record is closed and remains unsupported; no record body is read. Directories,
leaf or ancestor aliases, and unknown errors do not become absence. The caller
continues to require no enabled ownership before ordinary removal may proceed.
An owned enabled membership with a missing record still fails. Journal writes,
recovery, explicit enablement and enabled-member removal remain unqualified.

Focused non-Linux regressions cover prepared versus owned absence, preservation
of existing file/directory records, dangling leaf and missing-leaf ancestor
aliases, and invalid operation IDs. The original cross-platform CLI journeys
remain required; Linux compilation or source review does not qualify their
macOS or Windows execution.

## Ordinary prepared removal on platforms without a native journal

The original macOS Terminal run `43559930375`, job `132341201150`, tested
merge `6ebf655d` with the exact `5ff83587` tree. Its retained artifact
`14100755315` passed the outer SHA256 and all 2017 member CRC checks. All 29
resilience cases passed, including the corrected corrupt-journal fixture. Four
ordinary saved Project removal cases instead reached the unsupported native
journal reader after the generic core recorded its ordinary removal attempt.

Without an enabled-member receipt, the recorder now validates that attempt's
existing leaf/containment contract and uses the existing pending-name reader.
Only an empty native inventory plus the anchored record reader's actual absence
returns to ordinary state persistence. A native
stage or inventory error refuses; existing native records require the exact
current-attempt record, so another operation cannot take a missing-record
fallback. Enabled ownership and native journal mutations are unchanged; no native operation, state schema or consent rule
is added. The cross-platform regression preserves original state and journal
bytes, identity, mode, size and mtime. Its real Linux stage case fails before the
change and passes afterward; the ordinary macOS failure is original hosted
evidence, and the corrected macOS/Windows execution still requires normal CI.

The composed path retains the non-Linux absence reader described above. An
empty directory listing alone cannot authorize ordinary removal: the exact
reader must also verify the complete path ancestry. Regression cases preserve
an aliased parent above both existing and absent operation namespaces and
require refusal before any ordinary-removal acknowledgement.

## Original parent checks before config reads and repeat acknowledgement

The unchanged source through `f6adab5a` could lose its original config parent
after package verification or native readback, then read the preserved config
leaf through the replacement parent. The correction reuses the held record's
existing parent revalidation before predecessor config inspection, enable
readback verification, remove readback verification and repeated enablement
acknowledgement. It adds no new authority, journal or public admission.

Repeated enablement can return a state-validation error after recognizing its
previous owned record. That error must retain the original Prepared guidance;
the enabled guidance is now selected only when the repeat has no error.

Actual Linux causal replay used the exact five-file packet with SHA256
`707eb40a298351082d92bca6e874c23a94ff1e3c4a0f8964ecde7a5c8dc7709a`.
The old runtime failed all four top-level tests: three late-parent boundaries
performed one forbidden config read each, the repeated-parent boundary returned
enabled guidance, and the repeated-state boundary returned enabled guidance
alongside its error. With the correction, the same tests passed in 0.098 seconds.
The filesystem cases preserve the original config and lock objects inside a
replacement parent and require zero subsequent config reads, writes or removals.
The state case changes the actual saved attempt after a genuine first commit.

The existing focused Grok and installer security/SDK matrix also passed, with
51 top-level tests and 163 pass records including subtests. These results belong
to that isolated packet; the final composed commit still requires its ordinary
lint/core and native gates. Public native enablement remains closed until the
separate auxiliary-write and exact recovery admission is established.

## Private v3 observed recovery preparation (2026-11-14)

This change prepares the existing private member-journal codec for a future
qualified producer. It does not enable public recovery, public Project
membership, or a v3 producer. Activation and removal still publish their
existing v2 records, and public `Recover` and `ReconcileObserved` still refuse.
Bare attempts and legacy v2 observations do not enter the new private observed
entrypoint. The separate readonly native transport and its actual input-root
qualification remain prerequisites for admitting a public v3 producer.

The new private entrypoint retains the exact supplied observation: root, raw
record digest, header digest, binding, installation, target and phase. It passes
the admitted record through recovery instead of replacing it with a later
operation-ID lookup. Every complete-state boundary is classified before native
config IO. Reopening uses the recorded original User parent, then retains the
original native flock and existing `ExactFile` transaction through completion
or rollback. Committed completion requires the exact C1 bytes, existence and
mode; predecessor completion requires C0 and no footer. A pending attempt can
restore C0 only from its exact C0 or reversible owned C1 image.

Before and after later effects, the same original parent, raw journal/header,
whole state, package and config image are checked again. Removal's exact footer
is written under the same parent/native-lock/config lease. Existing journal
checkpoints carry the original caller context through a value copy of the
recovery service. No journal mutation primitive, persisted public schema,
generic transition port or directory-identity algorithm changes. The repeated
observations do not exclude arbitrary hostile writers atomically after the
last check.

The existing v2 fixtures keep their default version and assertions. New fixtures
select v3 before their first publication; no retained legacy record is upgraded.
The new cases cover enablement S0/SA/ST, removal footer absence/partial/complete
against those states and C0/C1, foreign config/mode refusal, exact observation
and raw-journal drift, parent replacement preserving the original config and
lock leaves, busy native lock, state-save failure and cold retry, and public
refusal for both versions. Additional cases change an unrelated actual saved
state field between observation admission and recovery, cancel after footer
truncation or immediately before/after unlink, and distinguish rollback from
completion when a user already disabled the ID so C0 equals C1. Late failures
report the actual retained or already-removed journal; they do not recreate a
stage or claim an effect that did not occur.

These are private codec and local contract checks. The reviewed implementation
and exact test execution receipt must accompany publication; they do not stand
in for original native EOF/Wait/process-tree cleanup, a complete readonly view,
actual Grok input-root physical qualification, or the final public lifecycle.

## Readonly native inventory qualification

The optional process port now retains the original HOME, selected ordinary Git
root, project and native executable handles. Its Linux implementation uses the
existing contained process owner with a fixed Bubblewrap view, an unchanged
caller deadline, and checks before and after native execution. It has no
ordinary-process fallback. Runtime symlinks retain their original metadata and
link text; their outside targets are not mounted or followed by the census.
This is a bounded runtime view, not a claim that every host input is available.

Both inventory reads share one client input lease and native config lock. The
protocol workflow compiles a tagged test that runs the actual production port
twice against the pinned Grok executable and selected installed package, then
requires unchanged original input snapshots and no MCP witness invocation.
The parent joins that exact held test binary, original output and source to the
result. A successful inventory result alone does not qualify the public
lifecycle: its existing refusal remains until native qualification is complete.

## Readonly observation limit identified in source review (2026-11-14)

Review of `ce47d1895dac145028c8dbc9707290fcf477ccd1` corrects the
preceding phrase "no MCP witness invocation": the current check establishes
only an unchanged protocol-event file. That file is outside the mounted readonly
HOME and Project view, so a probe in that view cannot append its event there.
The exact probe also emits initialize/list/call events, with no unconditional
startup event. An unchanged file therefore cannot prove absence of an MCP
process, including one that starts silently or fails before its first request.

The tagged test performs two reads of the same untrusted, disabled C0 state; it
does not test enabled C1 readback. Its partial result contains only two calls and
an input-snapshot digest. These observations must not be promoted to a complete
public lifecycle or no-startup verdict. Public enablement and recovery remain
closed, and production records remain v2.

Any additional invocation observer needs its own source-bound positive control
in the same readonly view and complete bounded process-family evidence before
that verdict can be admitted. Keep the readonly mounts and existing process,
source, lock, deadline and cleanup guards; neither a writable witness directory
nor an assumed private-stderr forwarding path establishes the missing proof.
