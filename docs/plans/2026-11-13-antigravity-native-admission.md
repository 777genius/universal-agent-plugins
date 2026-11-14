# Antigravity native admission after prepared delivery

Baseline: main `c94c8c752ea23a6dcb89dec9a952bd7520ee841c`.

## Immutable UAP scanner admission correction

On 2026-11-13 the genuine digest-verified Linux agy 1.5.3 and UAP built from
`ee561399d01484dcdd04fce3a504b81839882b7b` completed the earlier native lifecycle.
The new direct User fixture refused before its first native action: the ordinary
UAP add had installed its pinned LintAI scanner at
`config/agentplugins/security/lintai/0.1.3/linux-amd64/lintai`, an ordinary owned
single-link file of 9,989,584 bytes. This is executable runtime, not mutable client
configuration. The refusal's `member_sha256` identifies the relative pathname,
not the executable content; the independently verified binary SHA-256 is below.

Amend only this plan and `scripts/e2e/antigravity-native-cross.py`: explicitly
select the task-owned `AGENTPLUGINS_HOME=<fixture>/uap-state` for ordinary UAP
delivery, preserving the original native HOME/profile. The ledger's keyword-only
scanner mode chooses exactly one fixed current-platform/version leaf internally.
That leaf must already exist from ordinary add; there is no new downloader,
runtime provisioning, arbitrary path allowance or native write authority.

The four official release archives were downloaded and their SHA-256 values
verified against `securityscan/release.go` before deriving these member pins:

| Platform | Archive SHA-256 | Binary bytes | Binary SHA-256 |
| --- | --- | ---: | --- |
| Linux amd64 | `2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da` | 9989584 | `b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba` |
| macOS arm64 | `be8b263e2323074080d928ea7c2129458299a6d03f7a9f178dfc1aa8e6bc17ff` | 7949216 | `37342414aa7feadc09b8473d1bfafe95fd79ab99f284d37518c930fcb7986c57` |
| macOS amd64 | `abc170612a847bf1a896ef85a4ee93977baa8275f303dac3ed27bf34050b7513` | 8417744 | `6899e59bab426565fa2433b798a0b6e8accb114386b5d0def30a8a071be8de13` |
| Windows amd64 | `2f61f6a83a160afa3feed9ea1722b82d0d938ebff865a4e20d39b5f55270c911` | 7620608 | `9d486e5b66875384c12a147b8a19007beebe2691bc6d23a7867bf019b70be453` |

Stream this one leaf only up to its exact published member length, retaining its
SHA-256 and original metadata without a large body copy. Reuse every held parent,
ordinary-file, owner, volume, single-link, pre/open/post identity and immutable
before/after comparison. A missing or changed scanner, wrong path/version/hash,
alias, unknown large file or changed foreign configuration still refuses before
the next command. All other files retain the 1 MiB bound and 16 MiB aggregate;
the selected config retains its existing 16 KiB/schema/boolean-only rule.

Independent plan R2 approved the two-path change and revised estimate of 55–75
runtime and 40–60 contract lines. Inert coverage must demonstrate the exact
large pinned member, wrong/missing/changed bytes, same-hash foreign copy,
mid-read mutation and unchanged ordinary configuration aggregate bound.
Source review and fresh three-platform native execution remain required; neither
archive inspection nor a local candidate execution is final exact-head acceptance.

The actual local candidate passed the corrected scanner admission, then refused
the first supervised receipt; it is not a native sequence PASS. All eight inert
contracts and independent code/archive R2 passed. The local ordinary process
environment also fails existing containment prerequisites in the unchanged core
suite; hosted execution remains decisive. Do not infer a completed native action
or recovery from the opaque receipt refusal.

Scheduling amendment: include `.github/workflows/antigravity-native-e2e.yml` only
to set `cancel-in-progress: false`, as for the related Cline and Hermes lanes.
The ee561399 macOS capture acquired a runner at 13:09 UTC while the corrected
patch was ready; retain that original observation when publishing new commits.
Queued replacement runs still require their own exact-head acceptance. This
does not rerun, cancel, extend deadlines or change release/native commands.

## Actual pinned admission

The digest-verified Linux agy 1.5.3 was executed locally without account login,
provider credentials or a model request. Its archive SHA-256 is
`1b4850599cce73a17f94ffb422df3bf9531531b77dbddc40c1a435e0c2c16807`.
All HOME, USERPROFILE, APPDATA, LOCALAPPDATA and XDG configuration/cache paths
were private disposable directories.

- Strict local validation succeeds.
- A directly prepared User plugin appears in `plugin list --json`, including
  its exact native path, version, component names and boolean enabled state.
  No vendor import is needed. Its source field can be absent.
- Inventory names are physical directory basenames, not manifest names. A
  renamed native directory is listed and enabled by its new basename.
  Enable/disable stores the physical basename in config.json, independently
  from the manifest name.
- Direct Project `.agents/plugins` packages are absent from this CLI inventory;
  enable/disable of the Project identity fails without effects.
- `plugin install PROJECT_PATH` copies the package into the global User plugin
  root and writes import_manifest.json. It is not a Project activation route.
- Enable/disable writes the shared config.json `plugins[FOLDER].enabled` member.
  This requires separate consent and ownership-aware recovery before production
  mutation; ordinary installation confirmation does not authorize it.

## Bounded change and acceptance

Strengthen the existing genuine native lifecycle oracle to verify direct UAP
User inventory by exact physical path/basename, manifest version and selected
Skill/MCP names. Verify that inventory reads preserve profile/package bytes.
Exercise vendor enable/disable only in the already disposable imported-plugin
profile, verifying readback and source preservation. Keep product Prepared and
do not add a global registration fallback for Project scope.

The standalone IDE remains a separate required observation: open the selected
workspace and inspect its agent-side-panel Customizations dropdown. A CLI
inventory response does not establish IDE loading or a successful MCP call.

## Critique rounds

1. **Scope:** CLI install has actual global-copy effects. Correction: no Project
   registration or enable fallback; assert Project stays absent from CLI User
   inventory before the deliberately separate vendor import.
2. **Identity:** a name-only inventory decoder loses UAP's physical artifact
   identity. Correction: require exact path/basename and declared manifest
   version/components; allow an absent source on direct directory discovery.
3. **Consent and recovery:** enable changes shared config. Correction: retain
   production behavior and exercise toggles only in an isolated vendor fixture.
   Enabled inventory is an observation, not runtime/IDE activation attestation.

References: https://antigravity.google/docs/plugins/ and
https://antigravity.google/docs/customization-discovery/ . Current documentation
is supplementary; pinned executable observations determine admitted behavior.

## Local execution

On 2026-11-13, the ordinary UAP CLI built from the baseline plus this E2E-only
change completed the entire cross-platform harness on Linux against genuine
agy 1.5.3: Project add/update/partial-drift refusal/absent-bundle repair,
strict Skills/stdio/HTTP validation, separate vendor import/disable/enable/
uninstall with exact inventory readback and foreign disablement preservation,
direct UAP User exact-path/component discovery (`enabled=true` observed), and
both scopes' removal/source preservation. Python syntax and diff whitespace
checks passed. macOS/Windows and final-commit hosted execution remain pending;
no IDE loading, MCP call or authenticated model turn was observed.

## Follow-up plan: direct ordinary UAP User enable admission

Plan-only base: merged main `54d211042fcab4845b96f975187cdd4da9de530d`.
No production change is authorized by this admission plan. Integrate current
main before implementation; do not cancel useful earlier native captures.

### Existing evidence and remaining question

The original pinned observations above already identify shared config storage
as `config.json` → `plugins[physical-folder].enabled` and demonstrate a renamed
native folder. The merged harness toggles a separately imported vendor fixture;
its ordinary UAP User fixture receives only read-only inventory. The remaining
question is whether this exact ordinarily delivered directory can be selected
and toggled without altering any other profile authority. Reproduce storage and
selection facts on this fixture; do not infer them from the imported fixture.
A fixture inverse does not establish production ownership/recovery or IDE state.

### Exact file allowance and budget

- `scripts/e2e/antigravity-native-cross.py`: reuse existing profiles, command
  runner, exact inventory oracle, package snapshots and lifecycle fixture.
- `.github/workflows/antigravity-native-e2e.yml`: only if necessary to select
  the same source harness on all three existing OS lanes; preserve pins/release
  logic and original source/release separation.
- This plan file only. No adapter, public capability, receipt schema, generic
  process runner, dependency or production configuration changes.

Expected runtime/inert-contract delta: 100–160 lines. If held authority, bounded
storage admission or meaningful negative contracts require more, report the
concrete revised budget before expanding; do not omit checks to meet a number.

### Finite fixture sequence and consent

Use only the existing disposable direct-UAP User profile, pinned agy 1.5.3 and
verified ordinary `uap add --scope user` delivery. Explicit consent is limited
to this generated profile and exact selected physical directory identity.
No real profile, account, model, IDE, Project enablement, import, second copy,
marketplace install, trust bypass or native server startup is part of the test.

1. Record the immutable initial selected-profile ledger and source/package
   snapshots. Read native JSON inventory; admit one exact selected path and
   physical basename, manifest identity/version and selected component names.
   Absence, duplicate identity, unexpected same-name resolution or wrong path
   refuses before mutation. Record initial boolean enablement without forcing it.
2. In that profile include a separately named foreign directory with the same
   declared manifest name and a separately disabled foreign sentinel. Their
   bytes, selected path and disabled state must remain unchanged throughout.
3. Immediately before each mutation rejoin the full original selected-profile
   authority, source identity, exact owned directory/manifest/components and
   unique native row. Issue only `agy plugin enable PHYSICAL_BASENAME`; read
   inventory and profile effects. Then, only if fully admitted, issue the fixed
   disable command and repeat the same checks.
4. Restore the originally observed boolean only if all previous operations and
   effects remain admitted. Re-read inventory and original foreign/source state.
   This is an isolated inverse observation, not automatic production recovery.

The maximum sequence is initial list, enable, list, disable, list, and, if the
initial state was enabled, enable and final list. No retries or alternate name,
path, import or scope selectors. Each command retains existing finite runner
bounds; the whole sequence remains inside existing parent supervision.

### Immutable ledger, bytes and refusal policy

Hold original profile/root/parent identities, admit ordinary nofollow files
and bounded membership/bytes, and compare before/after metadata and hashes for
every command. The initial ledger is never renewed. Inventory must remain
read-only. Only the expected selected `plugins[physical-folder].enabled`
boolean may differ from the original profile state during the toggle sequence;
all other original keys, files, foreign disabled state and package bytes remain
identical. Revalidate the full source/profile binding immediately before every
mutation and after command settlement. Unknown files or fields, rewritten
foreign values, aliases, replacement parents or unexplained metadata changes
remain captured negative observations and stop further native mutation.

Retain bounded command output and a closed effect receipt. Expose only reviewed
schema keys and boolean/null state, task-authored fixture literals and fixed
identity facts; unfamiliar native strings/keys/numbers receive type/size/hash
projections. No generic raw profile/config/cache dump or credentials are read.
Before/after originals remain separately identified; do not restore over unknown
writes or silently adopt native bootstrap state. Cleanup removes only the owned
disposable fixture after existing child settlement proof; unknown settlement
withholds successful capture and any recovery claim.

### Acceptance and negative contracts

Require positive exact-path readback, both initial boolean cases, same-declared-
name foreign preservation, foreign-disabled preservation, package/source bytes,
and final equality to the initial selected state. Inert contracts must reject
wrong/duplicate paths or identities, declared-name fallback, unknown profile
writes, changed foreign keys, replaced/aliased parents/files and incomplete
command/cleanup. An unknown first effect must prevent every later mutation.

All original three-OS logs must show exact source and pinned executable identity;
queued, skipped or earlier-head results are not acceptance. Keep production
Prepared and every IDE/MCP/auth/model qualification unchanged regardless of the
fixture outcome. A subsequent production design still needs specific user
consent, exact selected profile authority and ownership-aware conditional
recovery over concurrent foreign edits; no such design is inferred here.

### Review correction: two authored initial states and exact leaf authority

Run the sequence in TWO independently authored ordinary UAP User fixtures,
with distinct disposable HOME/USERPROFILE and complete original baselines.
Before either immutable baseline, seed only the known task-literal schema
`{"plugins":{"SELECTED_PHYSICAL_BASENAME":{"enabled":true|false},
"FOREIGN_PHYSICAL_BASENAME":{"enabled":false}}}` in that fixture's exact
configuration leaf. Select true in one fixture and false in the other; author
this input once rather than using vendor toggles to manufacture the initial
state. Validate initial native inventory against the authored boolean before
any mutation. A pre-disabled fixture is interface measurement only, never
permission to override a real user's prior disablement.

The exact leaf is `<HOME>/.gemini/config/config.json` on Linux/macOS and
`<USERPROFILE>\\.gemini\\config\\config.json` on Windows. Existing cross-harness
`profile()` sets HOME and USERPROFILE to the SAME `base/home`; its
XDG_CONFIG_HOME (`base/config`) is a separate directory, not this config leaf.
The adapter's DetectProfile derives `HomeDir()/.gemini/config`; the pinned
observations above identify config.json under that root. Official path reference:
https://antigravity.google/docs/remote-control/ (settings-file path on each OS).
Do not substitute XDG, APPDATA, another profile, or project `.gemini/config.json`.

Finite admission limits: at most 4096 profile ledger entries/depth24, each
ordinary file <=1MiB and aggregate <=16MiB; known config <=16KiB, strict unique
JSON object/depth8/at most128 members per object. Directory traversal and file
opens are nofollow, with original held root/parent identities and platform-native
no-reparse/no-alias guards. Files require single links, task UID/GID and original
device (or the existing equivalent Windows ownership/volume identity); metadata,
file length and hash must remain stable throughout each read. Unknown ownership
or unavailable platform identity proof refuses, rather than pretending POSIX
UID/GID proves Windows ownership. These are admission bounds, not widened
production or package limits.

ONLY the exact already-authored config leaf may be independently atomically
replaced by a vendor toggle. Keep separate before and after identities, and
require each opened version independently ordinary/single-link/owned/bounded
and stable; keep original parent/root authority continuously held. Admit only
the selected plugins[physical-basename].enabled boolean transition. All other
schema keys, nested values and foreign configuration must remain identical to
the ORIGINAL baseline. Config serialization length/hash and leaf inode/mtime/
ctime may change for that exact admitted transition; mode, ownership, device,
link count and all parent identities remain fixed. No other metadata change,
new path, temporary sibling left behind, foreign leaf replacement or unknown
field is admitted. First unknown effect stops the sequence without an inverse,
restore, retry or adoption. Final inverse must restore original semantic config
and selected enabled state; byte changes confined to vendor serialization stay
explicitly recorded, never relabeled original byte equality.

The same-manifest foreign fixture has a physical basename DIFFERENT from the
selected basename. Validate BOTH physical paths, basenames, manifest identities,
component inventories and enablement before EVERY mutation; refuse any missing,
duplicate, wrong-path or declared-name-fallback row. The unrelated disabled
foreign fixture additionally retains its original disabled state and bytes.

### Implementation review: portable command bridge and revised budget

Implementation base is prepared main integration34aa17966, which includes54d;
this is not an acceptance result. Revised allowance is the three paths above
plus exactly `install/integrationctl/research/antigravityinventory/main.go` and
`main_test.go`, in the parent integrationctl module. Candidate budget420–500
code/inert-contract lines. No generic runner edits, privileged bootstrap, new
framework or production adapter. Report a concrete amendment if actual held
Windows authority cannot fit; never remove a check to satisfy the estimate.

The dedicated unprivileged Go bridge receives ONE bounded strict JSON request
(<=16KiB, duplicate/unknown fields and wrong types refused), containing only the
pinned agy executable path/hash, generated fixture profile root, selected safe
physical basename, action enum `list|enable|disable`, immutable source identity,
and original finite deadline. No arbitrary argv, environment, cwd or shell API.
Derive literal `plugin list --json` or `plugin enable|disable BASENAME`, fixed
fixture cwd and fresh allowlisted HOME/USERPROFILE/APPDATA/LOCALAPPDATA/XDG/TEMP
paths. Only fixed platform essentials such as PATH/SystemRoot/LANG may survive;
no provider credentials, profile files or caller-defined native flags.

Reuse process.OS.RunWithDescendantExitGrace with a30s command context and5s
natural descendant grace, within the original bounded whole sequence. Signal
and parent cancellation preserve cleanup. Set finite StdoutLimitBytes=1MiB;
existing stderr cap32KiB is not a complete transcript guarantee. Refuse stdout
at the ceiling and any stderr, so silently truncated diagnostics cannot become
successful evidence. Emit one bounded closed receipt only after runErr=nil,
context unexpired, exit0 and literal CleanupComplete=true. NoChildStarted may
classify a negative, never authorize another mutation. Any capture/containment
failure yields no successful receipt, no subsequent snapshot admission/mutation.

Hold and verify the selected executable's pinned bytes/identity and original
fixture-root authority before and after each command. Python validates closed
receipt identity, action and source against its request before touching the
next ledger/readback. Per-command supervision is necessary: wrapping the entire
Python harness alone would not prove descendants settled between mutations.
Both stdout receipt framing and output lengths/hash joins are finite; no raw
native error or caller-controlled diagnostic field is printed by the bridge.

Windows positive file/volume/birth/reparse identity alone is not SID ownership.
Retain real task-created root/parent authority and Windows no-delete-held
handles; require actual owner-security identity where the plan claims ownership.
Existing Python synthetic UID/GID cannot satisfy that check. Use existing
platform APIs without a new generic abstraction, or refuse that platform and
report the exact missing authority. A portable fixture result must not disguise
unavailable Windows ownership or process cleanup as an accepted observation.

Inert contracts additionally exercise closed request type/field/action/path
refusals, wrong executable/source/profile identity, incomplete/truncated output,
cleanup false/NoChildStarted, and prevention of every later mutation after an
unproved command. Verify both initial booleans and immutable-ledger/foreign
negative contracts independently before any hosted native acceptance claim.

Parent cancellation contract: stdin starts with exactly ONE bounded JSON+LF
request frame (not read-until-EOF). Python keeps the same parent-control pipe
open while the helper runs. A finite existing-pattern watcher cancels the
ORIGINAL command context on parent EOF or any extra input byte; signal.NotifyContext
also cancels it. Neither event resets the deadline or skips process cleanup.
The helper exits only after the existing runner has returned and containment
cleanup is classified. Parent death is never inferred solely from a signal or
successful native leader exit. Request prefetch/trailing bytes remain visible
to the control watcher; duplicate frames are not silently discarded.


Implementation budget measured during authoring: root approved780 code/contracts
lines across these same five paths. The added actual portable process bridge,
parent EOF/extra-byte negatives and held Windows HANDLE/SID identity required
more than the initial420–500 estimate. No checks were compressed away and no
shared supervisor, foreign policy, production receipt or legacy lint exemption
changed. Source/inert results are not native acceptance; source workflow lanes
exercise the new bridge while the existing release-proof path remains separate.


Final safety correction: the TWO NEW independent disposable-host fixture roots
are retained on ALL outcomes. Release held descriptors and emit only the fixed
fixtures_retained=true fact; do not recursively delete by pathname after closing
original handles. Existing outer-harness cleanup remains unchanged. This avoids
a new deletion engine and grants no profile-filesystem cleanup or recovery
success. Native-family CleanupComplete is still mandatory before every ledger
or next mutation. Root approved820 code/contracts only for the concrete atime,
strict boolean/number and retained-unknown-effect negative contracts. Metadata
stability excludes atime only; config semantic equality preserves literal types.

## fc8 Antigravity platform-test correction and refusal metadata

Original Windows job131862540827 refused the inert root rename because the
held directory handle denied sharing. External follow-up4994e4e preserves the
denied-rename case and verifies the original root remains while the destination
is absent. Keep that correction and additionally require the exact observed
Windows sharing violation, rather than accepting an unrelated rename error.
A successful replacement still requires refusal. This Go bridge case is not
skipped on either platform and the runtime authority is unchanged.

Original macOS job131862541242 passed the existing ordinary User inventory,
then the new disposable fixture stopped at the combined file size/link/device
gate. Its log does not identify the entry or establish which predicate failed.
Add only bounded metadata at that existing refusal: <=128 UTF-8 bytes of the
fixture-relative name, full relative-name SHA, fixed reason, mode/size/links,
same-device boolean and saturation flag. No file is opened for this diagnostic;
no contents, absolute path, link target or SID is recorded. Keep single-link,
1MiB/member and16MiB aggregate bounds, original refusal and retained fixtures.

Actual oversized-file/no-read/privacy and saturated-integer contracts are added.
These focused follow-ups revise the earlier source-size estimate because of
observed hosted failures; they add no native action or production capability.
Post-outage source edits remain unexecuted; current-head hosted tests and native
originals must pass before acceptance. No production enablement is added.

## Exact initial POSIX scanner permissions

Follow-up to external cab330c8, preserving its four independently verified
archive-derived pins and explicit fully-ledgered AGENTPLUGINS_HOME: require
POSIX0700 for the known scanner before any scanner byte read, matching
securityscan/release.go's actual temporary.Chmod(0700). Windows retains its
existing complete native metadata domain; do not infer POSIX bits there.
Correct inert fixtures now author0700 and a real ledger negative checks0600,
0755 and0777 are refused before os.read can reach that inode. Other files,
1MiB/16MiB limits, baseline identities, scanner content pins and qualification
flags are unchanged. Nine inert contracts pass; no native execution occurred.

## Bound both bridge streams and cancellation before receipt admission

Review PRR_kwDORxw17c8AAAABXyzTmA identified a real parent deadlock: reading
stdout to EOF before stderr can block on a stderr-first flood, before the old
wait timeout is reached. The old finally block also waited without terminating
a stuck helper, and closing buffered stdin could replace the original failure.

The narrowly local transport now uses unbuffered pipes, concurrent stdout and
stderr drainers with unchanged separate 2MiB/64KiB bounds, and a bounded16KiB
writer that handles partial writes. Overflow is retained as a refusal while
excess bytes are discarded so the existing supervisor can finish cleanup.
One monotonic35s transport deadline and absolute40s total deadline include all
joins and shutdown; the original29s native request deadline is unchanged.

On any exception, timeout or overflow, close only the control pipe first.
Its EOF cancels the Go bridge's original context and existing ordinary native
process supervisor. Continue draining, then reap the helper. If it cannot exit
within the cleanup reserve, kill and reap that exact helper and refuse capture.
This fallback is not proof that native descendants settled: retain the fixture,
make no next native action, and never emit cleanup or qualification success.
No new tree supervisor, runner exemption, native action or product label is added.

Success still needs exit0, all three transport threads finished, no stream or
writer errors, empty stderr and the original closed identity/byte/hash receipt
with literal CleanupComplete=true. A child exit alone cannot admit unread output.
Existing original-source/executable and profile-ledger guards remain in charge.

Independent R1 accepted this two-existing-file scope and explicitly retained
the distinction between helper reaping and native cleanup authority. Estimated
runtime/test correction180–220 changed lines. Real inert Python children cover
stderr-first and stdout floods, nonempty stderr, early exit before request
consumption, full16KiB/partial writes, control-EOF cancellation, a helper ignoring
control, KeyboardInterrupt and closed streams/reaped helpers/no surviving threads.
The successful receipt plus wrong source/cleanup/hash-count cases use the full
supervised_action boundary. Source review and new exact-source native execution
remain required; prior b378 Linux/Windows native successes do not prove this fix.

A follow-up review reproduced an actual false acceptance on 061f512e: a real
inert child returned the complete valid receipt, but its stdout wrapper closed
the real pipe and then raised OSError outside the drainer's exception guard.
The thread ended without setting failure. Close errors now mark transport
failure too; the full receipt test injects this error independently for stdout
and stderr and requires refusal. Native commands and cleanup authority are unchanged.

## Held scanner mode before hashing

The original path metadata already requires the pinned POSIX scanner to be mode 0700. The scanner now checks the held descriptor's mode before its first byte read as well. The inert same-inode race test changes 0700 to 0600, 0755 or 0777 between path observation and opening, and requires refusal with zero scanner reads. Windows keeps its separate permission domain.

The exact guard and race test passed the independent 13-contract Python run on the 40692b0f union before execution became unavailable. This reconciliation preserves the later concurrent pipe-close checks and portable byte-output fixture through 64cf9251. Current-source native Linux, Windows and macOS verification is still required.

## Selected-profile list and version nonmutation measurement

This follow-up measures the pinned disposable fixture contract before any product
ReadOnlyVerifier integration. Add only literal `version` to the existing closed
bridge action enum; its argv is exactly the pinned executable plus `--version`.
The same selected environment, original deadline, finite captures, binary pins
and positive process-family settlement remain mandatory. Exact 1.5.3 readback is
required before inventory or toggles.

Every direct-fixture version/list now brackets the settled command with complete
bounded profile snapshots. Compare original directory identities and full stable
metadata, file identities/links/modes/sizes/mtime/ctime/platform metadata plus
bytes/digests, and exact membership. Atime alone is excluded. Windows pathname
and handle metadata remain in separate comparison domains. Preserve the original
immutable ledger; only enable/disable retain the previous exact selected-boolean
replacement allowance. A list/version rewrite, even identical bytes via a new
inode, stops the sequence without inverse action, baseline renewal or restoration.
The config-absent/bootstrap case is not measured by these preauthored fixtures.

Local inert contracts cover no-effect version, metadata-only file/parent changes,
same-byte replacement, new members, content changes and unproved settlement.
These checks do not qualify production inventory, IDE activation or arbitrary
profiles. Genuine new three-platform native execution is required to establish
whether the finite 1.5.3 fixture contract actually holds.

## Source-native supervised receipt retention

Source jobs retain the existing fourteen ordered direct-fixture exchanges:
false version/list/enable/list/disable/list, then true version/list/enable/list/
disable/list/enable/list. There are no additional native actions or modified
release-proof paths. Exact request and receipt bytes enter bounded memory only
after the existing source, pins, stream and positive family-cleanup validator
accepts them. Filesystem effects are acknowledged separately by the original
ledger; an unknown later effect retains settled bytes with capture_complete=false.
Any unproved command family suppresses the entire raw archive and upload output.

The publisher uses a fresh owner-checked private sibling outside the profiles,
retains its original handle and separate path/handle identities, admits only
fixed request/receipt names plus one closed manifest, and bounds the ZIP to
29 members/1 MiB. It verifies exact bytes, member closure, CRC and SHA256 before
exclusive hardlink publication, verifies the original archive again afterwards,
and closes all held descriptors before exposing an explicit workflow output.
Unexpected entries, aliases, replaced roots or byte changes refuse; fixture and
evidence directories are retained rather than path-deleted. No unknown profile
or stderr content is read for publication. Existing native cleanup, deadlines,
scanner pins and exact no-write/toggle rules remain unchanged.

The manifest binds source commit/tree and native/helper hashes, every retained
slot's byte counts/hashes, separate effects flags and config digests. Partial
captures are explicit and all IDE, Project, native MCP, authentication and model
qualification remains false. Existing source jobs upload only the explicit
sealed ZIP output, including admitted negative captures; release upload and
aggregate logic remain unchanged. New publisher contracts are UNEXECUTED while
the local environment is unavailable; independent source review and genuine
three-platform originals, outer/inner CRC and stream joins remain required.

## Original canonical detector generation admitted

Original [run 43460106819, job 132004864154](https://github.com/777genius/universal-agent-plugins/actions/runs/43460106819/job/132004864154)
checked out reviewed source `5f9a43a4ebea0ba5358ff2ffd2d975f3c9e595f5`,
tree `3173fc7db908bb7e32785dfc686182288f3f4da4`. Go 1.26.11 executed
`UPDATE_GOLDEN=1` with the existing `TestDetectGoldenAcrossPlatforms`;
the original job reports PASS in 0.012s. This Ubuntu-hosted test models three
platforms; it does not represent three native OS executions.

Root and independent review parsed the three actual generated records,
recomputed their complete UTF-8 byte counts and SHA-256, and matched the full
committed blobs. Only Antigravity's NativeConfigRoot changes in these fixtures:

| Fixture | Bytes | SHA-256 |
| --- | ---: | --- |
| detect_darwin.json | 12417 | c73e3dc2c3259731d0de5ba08b34f131501cb631cff5bc9dbada486768be62a2 |
| detect_linux.json | 12146 | 6abc0fe7a07bae5c57d4e775d1e77936bc2aa4022c5493512bd99322dbc39b6d |
| detect_windows.json | 12123 | 9ed51e592536bf04e47be967b56756ab5ae41c8ddfa2a4c38472d019bb809602 |

Integrate the exact reviewed adapter/test pair and genuine generated fixtures,
and remove the now-completed temporary fixed-candidate generator from future
workflows. Existing running jobs are preserved. Default native provenance is
qualified only when the resolved path exactly equals the original host HOME
spelling; an aliased or explicitly selected delivery root grants no native HOME
or executable authority. No native process, verifier trait or activation is
added by this change.

The historical original generator result is not final-source acceptance.
The new provenance tests and complete affected CI remain required. Retained
native inventory row schema, original archive CRC/stream joins, exact product
environment and ordinary CLI lifecycle acceptance remain separate gates.

## Conditional production User observation source candidate

This seven-path candidate is not publication-ready capability evidence. The
existing pure14 source passed original Linux, Windows and macOS jobs on b526;
retained full native row schemas remain pending. Provenance must first be
canonically generated/tested on the reviewed two-file alias-qualified detector;
no hand-edited golden or decorative DetectedClient.Version is an authority.
The runtime draft is 380 physical lines, with one new file below
500 effective lines; the size reflects fixed config/backend leases, strict
schema/component checks and initial/resume distinctions, not a generic runner.

Use only Env.Runner.Run and its real CleanupComplete/NoChildStarted result.
The native commands are --version (exact1.5.3) then plugin list --json under one
original ten-second context. The child receives only the selected original
HOME/USERPROFILE; genuine product Windows execution must confirm necessary
platform infrastructure bindings rather than importing provider credentials.
The final accepted row schema must be joined to retained originals before
publication: the draft six-field closed reader is conservative and intentionally
refuses unknown fields. No unsupported output is an authoritative absence.

Eligibility is User-only, original unaliased Host-default profile, exact selected
backend/plan locator and an existing bounded qualified plugin-boolean config with the selected override.
Absent selected overrides remain unmeasured and cannot silently become a new
native bootstrap predicate.
Absent, malformed or unsupported/bootstrap config starts no child. Initial file
delivery can remain Prepared; VerifyOnly or PackageUnchanged unavailability is
an error so historical Active is never treated as fresh native observation.
Project retains its existing file activation and unavailable native-classifier
behavior; the inherited routing trait never grants Project CLI verification.
Recognized disabled/absent observations retain Prepared with a remaining action;
there is no enable, disable, import, global copy, session, auth or model action.

Exactly the selected config and backend retain rooted original handles across
both native commands. Rejoin namespace, named/held portable identities, mode,
size, mtime and streaming SHA256 before/after; close errors and final context
expiry refuse observation. This is not new UID/nlink/SID/ACL ownership or atomic
exclusion of concurrent namespace writers. The existing core owns any persisted
ProfileAuthority checks; legacy bindings are never captured or upgraded here.
Configuration effects are not restored or adopted; an attempted failed native
observation reports uncertainty. No new NativeObjects or persisted schema.

Inert contracts cover the real provider/resume path, historical Active, repeat,
update, exact absent-bundle repair, authentication and package-receipt continuity,
Project isolation, unknown/dirty/unsettled/cancelled results and same-byte inode
replacement. A real cold locked-restore acceptance case belongs to the existing
CLI agreement/transport layer and is NOT simulated by a Service reload or an
ordinary reinstall. Its concrete later CLI/native test remains an acceptance
gap. All newly authored source/contracts are UNEXECUTED while the executor is
unavailable; formatting, compilation, lint, full caller tests, canonical goldens
and genuine exact-environment native product proof are mandatory before merge.

Source-review correction: reject casefold-selected name/path aliases and duplicate
casefold inventory identifiers before presence/absence classification. Error
sentences follow lowercase Go conventions. Backend/config digest reads now check
the same original context between fixed32KiB chunks, including immediately after
each read; no deadline is renewed. A first-chunk cancellation contract requires
one read and zero subsequent native commands. Alias-only and alias-plus-exact
rows both refuse. These added contracts are UNEXECUTED; all prior publication
gates remain unchanged. The API-only draft runtime was404 source lines before restored lint factoring.

Follow-up source review preserves the original limit-plus-one read bound by
clamping each digest chunk to the remaining admitted bytes plus one. The config
recording-reader contract refuses oversize after requesting exactly16385bytes;
it remains UNEXECUTED. No publication qualification changes.

## Restored execution and conditional public User acceptance

The original811 run43460106819 archives14057172623/14057495173/14057860650
were downloaded after executor restoration. All three outer SHA256/size records
match original uploads; both ZIP CRCs pass and each inner archive has exactly29
members. Each has28 manifest request/receipt byte+SHA joins and14 stdout joins,
source811bec555b0515bcbeb6143228a4afc583d2b090/treee1299d6652523905349c4995234e8101b4e6216b.
All21 actual list arrays have exactly enabled,mcpServers,name,path,skills,version;
all typed states, physical selectors, component sets and versions match. Pure
version/list config digests match and all effects are acknowledged. IDE, Project,
native MCP, auth and model qualifications remain false.

Actual restored local checks now include gofmt, full Antigravity/domain suites,
focused real provider history/repeat/update/repair contracts, and21 inert Python
contracts. These are process-inert local tests, not native product acceptance.
One initially wrong test interface was corrected to the actual transaction port;
three lint findings were corrected without exemptions. The module's unchanged
legacy lint findings require the standard changed-line/architecture gate.

A separate draft acceptance amendment, EXCLUDED from this RO-only freeze,
adds genuine PUBLIC UAP User repeat,
changed2.0 update, completely absent owned-bundle repair and exact remove. A
separate disabled fixture must retain its initial false state. Four independently
authored negative fixtures require repeat refusal for missing config, malformed
config, foreign same-manifest collision and actual config symlink; no baseline is
restored after a refusal. An absent backend or absent canonical detector provenance
must instead retain file-only Prepared, without a native inventory success claim.
An unavailable symlink
prerequisite fails the test rather than skipping or spoofing an OS. Original
ProjectA/B, source and three independent User/vendor profiles remain protected.

A separate cold PUBLIC fixture uses the immutable current source commit's
existing repotests/testdata/agentplugins_native_probe. It transfers ONLY saved
uap.toml/uap.lock into an independent empty native/User state/cache world, runs
install--locked, joins actual source/tree/manifest and committed environment
receipt identity, then authors one known enabled override and exercises real
read-only repeat. This is the existing saved-install route, not a reinstallation
masquerading as restore. Public JSON intentionally hides paths and declared
versions, so tests use the existing bounded state binding for those identities.

Only source runs with the existing receipt collector execute the extension;
released-lane argv and evidence remain unchanged. Exact new RO action text plus
Prepared/Installed outcome establishes that the real compiled adapter reached
native observation. In particular, genuine Windows execution must prove its
closed HOME/USERPROFILE child environment works; no ambient provider variables
are added. These added public/native journeys have NOT run locally and remain
mandatory hosted acceptance. The original14 retained direct exchanges retain
exactly their original scope and are not relabeled as product-command streams.

## Corrective lifecycle routing boundary

Identical inert reproductions on source920 and4ad found three routing regressions.
The public SDK Prepare/Apply/VerifyPreparedBinding path proved both User and
Project ownership on920. On4ad, User repair preparation refused because the SDK
does not supply canonical native HOME provenance; Project materialization
verified but the generic Active-only coherence gate could never accept the
adapter's truthful Prepared result. Separately, an eligible User dry-run repair
started both metadata commands, and repeated add started them despite a foreign
same-manifest collision already recognized by the existing filesystem observer.

The correction restores Antigravity's previous VerifiesNativeDirectory trait.
It preserves the generic Active-only contract and the existing filesystem
ownership route used by SDK/management recovery. Unqualified canonical
provenance, backend or runner returns the adapter's file-only Prepared outcome;
an eligible canonical-profile repeat still refuses missing/malformed config and
every attempted failed observation. The native lifecycle rechecks the existing
filesystem namespace and exact declared/physical identity before and after each
command. A foreign same-manifest entry appearing during the final list refuses
the observation and preserves that entry for inspection; this is not atomic
exclusion of concurrent writers.
Live version output must equal the captured bytes `1.5.3\n`; decorative
DetectedClient.Version never replaces the held-binary live check.

Native readback remains in Activate: ordinary Prepared repeat, a changed-package
update and absent-bundle repair still reach it. Direct Service.Repair of an intact
bundle and a previously fully converged historical Active binding retain their earlier file-only path;
they are not newly qualified native observations. Project and targeted info
receive no new native route. No enablement, IDE loading, authentication or model
readiness is inferred, and no generic Prepared-to-Active acceptance is added.

The corrective contracts exercise public SDK recovery without state mutation,
real-provider dry-run and collision refusal before native execution, and exact
version framing. The existing repeat/update/absent-repair cases remain required.
These inert cases do not qualify the ordinary process environment, descendant
cleanup or arbitrary profile effects. Genuine three-OS public-path acceptance
and the remaining direct-repair/historical-Active observation design are still
separate work.

## Confirmed public lifecycle resumption

The scoped Prepared capability now sends a fully converged historical binding
through the existing resume path when a dynamic verifier is available and the
static native-directory trait is false. Resume verifies the owned package, returns
RequiresConfirmation without starting a child during preview, and invokes the
adapter only after confirmation. It retains existing receipts and authentication;
new native observation still reports Prepared. Static ownership/coherence and
the generic read-only verifier stay unchanged. Pi, Grok and frozen VSCode keep
their existing trait/classifier decisions.

For an already complete lifecycle with a pending scoped Prepared plan, the public
TTY add route reaches its existing confirmation/apply stage rather than returning
through the completed-attestation shortcut. Existing consent rules and manual
activation prompts are preserved. Public repair already uses a dry-run group
preview followed by confirmed group staging and activation, including intact
packages; that route needs no production change. Direct Service.Repair retains
its previous file-only boundary.

The public Cobra contracts cover JSON/--yes/TTY historical repeat, inert dry-run
and unconfirmed preview, receipt/authentication continuity, and actual grouped
intact repair. The runner returns bounded native-shaped fixture data and starts
no vendor process; genuine three-OS public native acceptance remains pending.


## Supervised public User consumption acceptance amendment

Five existing paths only: this plan, the Antigravity native harness/workflow and
existing antigravityinventory Go main/test. No production, release input, pin,
shared runner, public schema or independent workflow changes. The released lane
retains its old command sequence; only source runs with the existing capture
object execute the new public journey.

The same ordinary process.OS supervisor now admits nine fixed public-UAP
operations. The original eight request fields and four native actions/ten-field
receipts remain intact. Public selectors/argv are literal local-fixture or
current-commit public-fixture operations; no caller argv/env/cwd is accepted.
A separate compiled installerSHA binds the exact built UAP binary. Its native
shim also has the original official1.5.3 pin held/rejoined/hashed across command
execution. Root authority is held from creation. Public roots are independently
authored, never adopted or path-deleted, and retained on every outcome.

Public commands keep the original240s bound:234s caller wall deadline, maximum
235s helper acceptance and5s descendant grace, with the caller's original
monotonic deadline checked before launch and final admission. Native30/5s
budgets are unchanged. Parent EOF/extra bytes/signals cancel the same context.
Only actual CleanupComplete, nil execution/context, closed bounded streams and
final original pins clear capture.pending. A negative CLI exit is not cleanup
proof. Any unproved public family suppresses the entire original raw archive;
a later known effects failure retains only the previously settled direct bytes.

The real public UAP journey covers User add/repeat, changed2.0 update, exact
absent-bundle repair/remove, an independently initially-disabled fixture and
four separate missing-config/malformed-config/foreign-collision/config-symlink
refusal fixtures plus a successful file-only Prepared missing-backend case,
matching the published b778 correction. Unqualified backend absence is never
labeled a native observation or an unavailable-error proof. Existing ProjectA/B, three User/vendor worlds
and original source remain protected. No config is restored after refusal.
Each public action has only fixed enum/exit/cleanup readout; raw public stderr
is bounded and retained only in the validated public receipt, never printed. Read-only result requires the actual
installation_verified domain value and exact adapter observation action.

User cold restore uses the actual User dependency root (uap-state), not Project
cwd agreements. Only original uap.toml/uap.lock are copied into a fresh root;
install--locked joins source commit/tree/manifest, committed environment receipt,
owned User data receipt, actual native manifest, exact public Go source bytes
and complete MCP command/args/cwd/env projection. Cold repeat is real locked
install; it is not falsely labeled a native callback. No standalone Git init,
manual state transfer, fake backend, trust, enable, authentication or model call.
The genuine closed HOME/USERPROFILE production child environment, particularly
Windows, still requires the ensuing native jobs; local test mocks do not prove it.

Actual inert Go suite/scoped lint pass; Python25 contracts pass, including
cleanup/stream/pin/frame/deadline refusal with no archive output. Windowsamd64
and Darwinarm64 helper tests cross-compile; those are not native execution.
Standard final make lint passes all existing gates with the138-rule baseline
unchanged. Independent source review and actual three-platform native acceptance
remain publication requirements.
The existing fourteen direct request/receipt labels and semantics remain separate.
Source capture schema2 adds twenty fixed public pairs in explicit world/action
order, with source/installer/native/helper pins and per-member byte/SHA joins.
The same held-root atomic publisher admits at most69 members/32MiB; original
direct-frame16KiB and public-frame2MiB limits are distinct. Partial or late effects
keep public_capture_complete/capture_complete false; an unproved family suppresses
all raw output. A full20-pair effects-negative regression verifies that fence.

Only the two exact uap-state mutation.lock/environment.lock control files may
refresh their literal positive integer PID and UTC RFC3339Nano acquisition time.
Closed schema, bounded owner-checked reads, same inode/device/owner/mode, single
link and all other state/native/foreign bytes remain mandatory. POSIX0600 is
required; Windows uses the existing owner/nonreparse authority and unchanged
actual mode, rather than pretending OpenFile0600 maps to POSIX bits. An actual
same-inode write regression distinguishes valid metadata refresh from unknown
fields, malformed values, new/missing/replaced locks and unrelated state changes.
No store directory or persistent state is blanket-excluded.

Control lock metadata/size/owner/link/mode authority is checked before any body
read; only those exact leaves bypass the generic snapshot body walk. Full
same-domain path/FD metadata is rejoined after reading. Windows uses the existing
artifact cross-open projection excluding only path-versus-FD ctime semantics,
retaining owner-checked handles and positive birth/reparse identity. New oversized,
owner-refusal and changed-open metadata tripwires confirm no body is read; all25
Python contracts pass locally. Hosted native acceptance remains pending.

## Prospective public receipt and bounded snapshot follow-up

This four-path source proposal completes the existing public journey; it adds
no workflow, process framework, product behavior, state schema, native action,
authentication, enablement or release-lane change. The legacy fourteen direct
exchanges retain their original request/receipt shapes, order and semantics.

The fixed public catalog grows from twenty to twenty-three pairs. The enabled
world has add, observed repeat, an observed repeat of a deliberately historical
Active binding, changed update, observed intact repair, exact absent-bundle
repair and remove. The disabled and five refusal worlds keep their original
pairs. The saved world has one add; the cold world has locked install, one
ordinary public add of the exact current-commit source URL with --no-save to
observe the restored selected User bundle, then a separate locked no-op repeat.
The fixed Go recipes still accept no arbitrary argv, environment or source URL.
There are ten public action names,23 public pairs,14 legacy pairs and75 total
ZIP members including the manifest. No legacy research toggle is relabeled as
a production public observation.

Source capture schema2 remains bounded to32MiB. Before accumulating another
public pair it reserves64KiB for a manifest capped at32KiB plus bounded ZIP
headers; excess stops admission with capture.pending still set, so no raw
archive is published. The legacy-only29-member/1MiB archive is unchanged.
One original360s monotonic public-journey deadline spans all worlds and fixture
work after its first call. Each existing234s request/240s parent allowance is
clamped to remaining time, with5s reserved for existing settlement; the helper's
235s maximum, EOF cancellation, native30/5s path and strict cleanup receipts
are unchanged. Final public effects must also finish within that same deadline.

New public snapshots reuse ProfileLedger, including held ancestor descriptors,
same-domain path/handle fences, ownership/no-alias checks and streaming reads.
They admit at most4096 entries/depth24,1MiB per mutable file and16MiB aggregate,
including a pre-read aggregate check and a final named-file/directory metadata
fence after all body reads. Both exact control-lock leaves have a
256-byte cap before any content read through their held parent. Reopened fstat
must still match the captured owner/mode/link/size metadata before reading.
Only the known platform scanner and exact bin/agy shim get immutable size/SHA
exceptions; those hashes come from the original pinned inputs, never a changed
snapshot file. Their bytes are streamed without retaining content. No general
large-file or hardlink allowance is added, and legacy tree_snapshot is unchanged.
Closed state JSON uses the same parser with finite depth16 for nested existing
ownership receipts; ordinary config/request parsing keeps depth8.

Warm and cold proofs join the exact installation/binding/source/package revision
to canonical physical User path, native manifest and the real single owned
data receipt/owner-marker bytes. A harmless exclusive task marker is retained
through repeat, update, repair and removal. Read-only repeat, historical
refinement and cold observation preserve full owned bundle metadata and receipts.
Confirmed grouped repair replaces the selected directory even when intact;
that replacement compares original contents/modes while retaining binding,
revision and data authority. Foreign User settings and members remain exact. Historical authoring changes only
activation in the captured state, through original held state parent/file
authority, and explicitly leaves authentication unsynthesized. The absent
bundle is renamed to an owned retained quarantine, never recursively deleted.
Removal must retain installation source/package, empty clients, data_retained,
data receipts and exact owner/task-marker files. Cold observation does not turn
the later install--locked no-op into an inventory callback claim.

Read-only public callbacks require the actual adapter guidance and an unchanged
complete private HOME snapshot. During real UAP package transactions only the
selected bundle and temporal size/link/time fields of its two exact transaction
parent directories may change; their original identities/birth/attributes and
all foreign/config bytes remain checked. Original User/ProjectA/ProjectB/vendor
sentinels and portable source stay protected. The authored symlink refusal uses
a separate bounded lstat/readlink fingerprint for that one known negative
fixture, retaining its ordinary target independently; it cannot enter the
no-alias snapshot allowance or be mistaken for an ordinary file.

The prospective contracts cover named and opened hardlinks/oversize/mode races
before reads, original immutable pins, exact data/revision joins, read-only
versus repaired inode semantics, the authored alias without target reads,
aggregate refusal before accumulation and expiry without deadline renewal.
This follow-up was prepared during executor unavailability: its added/changed
contracts and native journey have not been executed. Earlier25 Python contracts
and the original helper's focused tests/lint are separate historical evidence.
Source review, ordinary CI contracts/lint and genuine three-OS public execution
of this exact source remain required before acceptance.

The independent review corrections keep every public control/runtime role an
ordinary file before directory recursion, with a no-read directory-at-role
negative for both controls and pins. The persisted binding must carry exactly
one private managed_package_directory object, joined to the declared/physical
identity, canonical path and independently recomputed projected bundle digest.
The same digest must match the latest committed directory-swap receipt for that
binding; the fresh fixture has consecutive binding-level sequences, and repeated
historical receipts with the same AfterDigest are valid. Pending native intent
or an unacknowledged activation attempt refuses. ActivationOutcome keeps its
NativeObjects/NativeEffect fields private in the existing public JSON DTO.

Digest measurement reuses the packagesnapshot NUL-delimited kind/path/execute/
size headers and file bytes, sorted by slash-relative path with the root omitted.
This narrow fixture permits ASCII names and refuses snapshot-excluded .git or
workspace-lock members rather than silently omitting observed files. Windows
ordinary-file execute bits follow the pinned Go1.26.11 os mode rule. A separate
literal-header vector checks root omission, ordering and executable semantics;
realistic ownership/receipt fixtures check the original digest, a changed latest
receipt, equal historical digests, pending effects and the private DTO boundary.
These added corrections have also not been executed during the current outage.

### Missing selected executable: public detection refusal (2026-11-13)

R1/R2 scope is the existing cross-platform admission script, its inline
contracts and this plan. Production detection and lifecycle policy do not
change. Antigravity's `DetectSurfaces.SelectionSurfaceIDs` selects only
`antigravity_cli`; an existing `.gemini/config` directory or owned `config.json`
does not select an absent `agy` executable. The adapter's file-only behavior
for an already supplied client is not a public CLI discovery fallback.

The real public UAP binary built from clean
`f8025bfabdfbe40ea2a0f6e5b2a09f6273d97308` reproduced this boundary in an isolated
task-owned profile, after an initial successful add and preservation of the
original pinned agy outside PATH. With the authored owned native config still
present, repeated ordinary `add --target antigravity --scope user --no-save
--yes --format json` returned exit1, empty stdout and exactly:

```text
agentplugins: target "antigravity" was not detected; no target was changed
```

The existing missing-backend world now expects that refusal. Its original
fixed helper action, pinned retained executable, source/runtime identity,
closed streams, cleanup receipt, deadlines and all state/bundle/config/foreign
before-after comparisons remain required. Only exit1 with empty stdout and
that complete stderr line qualifies this specific refusal; another nonzero
exit or diagnostic is not interchangeable. The final summary reports all five
refusal worlds and `missing_backend_refused`, removing the false file-only
fallback assertion.

The new inert contract exercises the existing validated receipt path with the
exact result and rejects success, a different exit, stdout bytes, absent or
different stderr and appended diagnostics. Before the correction, five of
these negatives were incorrectly accepted. Local production RCA and inert
contracts do not qualify the genuine agy lifecycle; the complete current
public journey still requires original three-OS CI receipts.

### Windows path-stat execute bits at the public file boundary (2026-11-13)

Original source 6569cac0454b83ceb5e32d91f3a9e00efd91a19a,
[run 43506679761, job 132170838747](https://github.com/777genius/universal-agent-plugins/actions/runs/43506679761/job/132170838747),
failed the first real-file public binding contract before native agy execution.
The failure was the cross-domain opened-file comparison; this fixture includes
bin/agy.exe. The old diagnostic does not record the mismatched field values.

The official CPython 3.12.10
[path-stat implementation](https://github.com/python/cpython/blob/v3.12.10/Modules/posixmodule.c)
adds 0111 for the case-insensitive exe, bat, cmd and com extensions. Its
[handle-stat implementation](https://github.com/python/cpython/blob/v3.12.10/Python/fileutils.c)
performs attribute conversion without that filename adjustment. Exact mode
comparison therefore rejects this documented API-domain difference.

Use one comparator for the two existing public opened-file checks. On Windows
only, differing modes require zero execute bits in the held mode and exactly
all three filename-derived bits in the named mode, with one of those four
extensions. Equal modes retain their prior behavior. Read-only versus writable
bits, file type, owner, device, inode, link count, size, mtime, positive creation
identity, attributes and reparse metadata still compare exactly. The preexisting
cross-domain ctime distinction remains; both complete temporal snapshots and
all before/after checks remain unchanged. No directory rule changes.

The new inert contract covers exact extension/read-only positives, partial and
reversed mode differences, unrelated suffixes, metadata drift and unavailable
creation identity. The original hosted failure is the real red evidence.
These new contracts are UNEXECUTED locally during the environment outage;
normal CI must run them before the unchanged genuine Windows journey.

### Intact repair reads the projected native MCP document (2026-11-13)

Original source `6569cac0454b83ceb5e32d91f3a9e00efd91a19a`,
[run 43506679761, macOS job 132170838371](https://github.com/777genius/universal-agent-plugins/actions/runs/43506679761/job/132170838371),
completed the fourteen direct native actions, public add, two repeats and
the changed update to version 2.0.0. The following ordinary public repair
failed with exit1 and `preparation_failed`. Its two MCP components were
`unsupported/not_in_recorded_delivery`; `mutated=false`. The complete stderr
ended with `agentplugins: antigravity refuses unqualified required components`.

The original artifact `14078543124` is 10045 bytes, SHA256
`53ad8f213cf58271ca32d0f7a320e7fef1839d78b3c4c4e8255d1f0c1ec38b75`.
Both ZIP CRC checks and the exact 39-member inner inventory passed, with
nineteen source/request/receipt joins and twenty-four stream hash/length
joins. All nineteen cleanup receipts completed. The harness and public
capture/effects completion flags remained false; this is genuine failed
repair evidence, not acceptance of the whole lifecycle.

The projector emits nested `mcp_config.json` and removes portable `mcp.json`
from the staged native bundle. Without an adapter `SelectionReader`, the
existing Stager read the default portable filename, proved an empty selection
and correctly constrained repair to that mistaken selection. The adapter now
declares exactly `SelectionLayout{File: "mcp_config.json", Nested: true}`.
Both User and Project use that same native document. No generic fallback,
digest-verification exception, ownership rule or activation policy changes.

Existing provider tests add both-scope native/portable divergence, known
missing and empty native documents, malformed syntax/member/null/array
refusals and changed-artifact rejection. The existing public Cobra regression
now starts with MCP, changes version through ordinary update, verifies actual
native output and absence of portable output, then repairs the intact bundle.
It uses the real Stager and explicitly inert native inventory callbacks;
preserved profile/native bytes and the existing Prepared/authentication
boundary remain asserted. It does not constitute a native client E2E pass.

These new Go regressions are authored but UNEXECUTED locally during the
environment outage. The original hosted repair is the real red evidence;
normal Core/provider/CLI tests and fresh genuine three-OS admission are still
required for the correction. The separately reviewed support-table edit
retains main's historical evidence and Kimi text without promoting runtime
or authentication capability.

### Preserve the actual cause of a negative-contract failure (2026-11-13)

Source `0dd77f85ca682989dadbde430fc371b6574f61c3`,
[run 43512382040, Windows job 132193845411](https://github.com/777genius/universal-agent-plugins/actions/runs/43512382040/job/132193845411),
reported the known opened-file mode failure and a second failure in the
missing-backend contract's nonempty-stdout case. That case reached its final
assertion with the raw-capture family still pending. Its broad `assertRaises`
had swallowed an earlier assertion, whose message the original log does not
retain. This does not establish that the earlier assertion was the intended
semantic refusal, and it does not justify clearing the cleanup fence.

The negative contract now requires the exact missing-backend semantic assertion.
An earlier runtime, identity or stream refusal is consequently reported with its
actual cause. Production admission, pending-state transitions, stream checks and
publication rules are unchanged. The second Windows failure remains unresolved
until an actual corrected-source run; this diagnostic correction is not a native
PASS and does not reinterpret the original capture.

After execution was restored, exact `b5512a38` passed all 35 portable Python
contracts, the affected Antigravity/provider selection tests and public Cobra
tests, plus all eight `make lint` passes with the unchanged 138-rule baseline.
The stronger diagnostic candidate also passed all 35 contracts on Linux.
No new Windows/native result or full Core success is inferred from these checks.

### Deterministic clocks in inert public-refusal contracts (2026-11-13)

At source `4c096eea961c1758bdc8779d43999355c7cd5ef0`,
[run 43523128070, Windows job 132222204181](https://github.com/777genius/universal-agent-plugins/actions/runs/43523128070/job/132222204181)
failed before native execution: the nonempty-stdout and extra-stderr cases
hit the callback's `0 < timeout <= 240` assertion before their required
missing-backend semantic assertion. The original log contains no numeric timeout.

The unchanged contract at `b923b4e6b4b80b063c298a2416b20c9118ee547b`
reproduced that failure with an inert fixed monotonic clock of `1000.003`.
Actual `public_uap` passed `(1000.003 + 240) - 1000.003`, or
`240.0000000000001`, to the callback, so all seven cases failed prematurely.
This proves a real floating-point oracle defect, not the sole cause of the
historical Windows failure whose numeric observation is unavailable.

Both existing inert public-bridge contracts now use exactly representable
`100.0` clocks. Their strict timeout bounds, semantic refusal message,
original-journey expiry, pending-family and stream/cleanup assertions remain.
Production code outside `admission_contracts` is byte-for-byte and AST-identical,
including `public_uap`, `bridge_exchange` and every native deadline.

All 35 contracts passed locally in 5.681 seconds. The two affected contracts
also passed under an externally supplied fractional clock in 0.030 seconds.
No native client or hosted flag was substituted for these inert checks; a fresh
corrected-source Windows/native run remains required.

### Confirmed grouped repair retains content, not directory identity (2026-11-13)

At source `4c096eea961c1758bdc8779d43999355c7cd5ef0`,
[run 43523128070, Linux job 132222204410](https://github.com/777genius/universal-agent-plugins/actions/runs/43523128070/job/132222204410)
completed direct native exchanges and public add, repeat, historical repeat,
changed update and intact repair. The last public receipt has exit 0,
cleanup_complete=true, mutated=true and group_phase=external_completed.
The harness then failed its whole-User-profile read-only comparison.

Artifact 14085982872 (10,053 bytes; SHA-256
`7d30eb1c402ef1b3449dea38b7612c5f177b20757ba1126aa7b6818c86d5c0bc`)
and its nested receipts archive passed CRC checks. All 19 request/receipt
identity joins and all 24 captured stream size/base64/SHA joins passed.
The archive contains no before/after User inventory, so the exact original
changed member or metadata field is unavailable and is not inferred.

The existing public repair dispatch calls RepairGroup. Its Repair flag excludes
the no-change path, stages the exact recorded revision and commits the selected
directory through the existing transaction kernel. The existing public Cobra
contract explicitly tests grouped activation and retained bytes. Therefore a
whole-directory physical-identity assertion is inappropriate for this operation;
this correction does not change product repair behavior.

Only intact repair stops using unchanged_home. Its replacement must pass the
full public_binding_proof and existing replacement-content comparison, exact
binding/revision/data joins and unchanged foreign User/config/source/Project-B
guards. Only then does the accepted repaired binding become the quarantine
baseline. Read-only repeat/historical and cold observation checks stay exact.
The existing inert contract now replaces an actual directory at the same
revision, records a new valid committed receipt, and retains rejection of
read-only replacement, changed source, data, foreign User files and config.

An attempted local public-Cobra metadata replay did not execute: the compiler
was killed. The cause was not established. The subsequent executor outage
prevented running these new Python edits locally. No new local red/green or native PASS
is claimed; ordinary contracts and fresh genuine native acceptance must run
on the corrected source.

After the executor recovered, the complete corrected directory-replacement
contracts passed: 35 tests in 5.309 seconds, with no skips. Independent review
then required preserving the exact previous receipt prefix, exactly one new
committed directory-swap receipt and unchanged standalone receipts for each
repair. This matches the existing producer, which copies the previous binding
receipts and appends one directory mutation.

The three new missing-commit, changed-history and changed-standalone cases fail
when only that new receipt guard is removed. With the guard present, all 35
contracts passed in 5.221 seconds with no skips. Independent source review
accepted the final receipt guard and regressions. The original read-only
checks, foreign contents and all revision/data joins remain strict. These are
local contract results; genuine native acceptance on the corrected source
remains required.

### Disabled repeat preserves state while allowing exact state-parent metadata (2026-11-14)

At source `ea7af4b185116ac57cf74064ae90e448793109aa`,
[run 43536058947, macOS job 132270481666](https://github.com/777genius/universal-agent-plugins/actions/runs/43536058947/job/132270481666)
and [Windows job 132270481431](https://github.com/777genius/universal-agent-plugins/actions/runs/43536058947/job/132270481431)
reached the separate disabled User fixture. Public add and repeat completed with
exit 0 and cleanup_complete=true. Repeat retained the installation ID, reported
mutated=false and Prepared/installation_verified, and said the plugin remained
disabled. The harness then refused its whole-fixture snapshot comparison.

Artifacts 14091746474 and 14091198551 have SHA-256
`457ae66776488e8f81e7b9dbd18b0bb2385348fae851d61545fc38675b9369fc` and
`b2eeb492dd516f63940aaede736b8c7b6fcc724d21952901a6944a24ecfe9d21`.
Both original outer/nested ZIP CRCs, 47-member closure, 46 manifest member joins
and 32 stream joins passed. All 14 native and 9 public cleanup receipts joined;
the capture, public-effects and harness-complete flags remained false. These
archives contain no before/after file snapshots or state-v2.json bodies. The
exact original changed member/metadata field is therefore unavailable.

The actual local acquisition path creates and removes sealed package snapshots
directly beneath AGENTPLUGINS_HOME. The collector compares complete directory size/mtime/ctime,
so successful scratch cleanup can fail the previous whole-fixture equality.
A controlled public Cobra repeat through the real SourceAcquirer and the existing
inert inventory runner confirmed state-parent mtime/ctime changes while its
device, inode, mode, UID/GID, link count, size and final members remained equal.
Raw state bytes, parsed state/receipts and native fixture file bytes also stayed
equal. Antigravity does not carry QualifiedLocalInstallerFacade, so this public
operation does not enter the SDK resealing route. No installer-tmp or descendant
metadata allowance is justified. This is a source-backed oracle defect, not
proof of the sole historical cause.

Only the disabled-repeat comparison now permits size/mtime/ctime changes for the
single exact existing directory row uap-state. Each
named/held API domain retains exact device, inode, mode, owner, link count,
birth, attributes and reparse facts. Complete membership and every other row
remain exact, including state bytes, receipts, native config, foreign files,
source and retained data. The existing bounded lock refresh normalization is
unchanged. Repeat must additionally report mutated=false and the original
installation ID, then pass full binding proof and read-only receipt/content
continuity. No production acquisition, native command, deadline or cleanup
contract changes.

The real state-parent create/remove case failed the extracted original equality
predicate; all other 35 contracts passed in that replay. With the correction,
all 36 contracts passed in 5.635 seconds with no skips. The corrected contract also requires rejection of
installer-tmp metadata changes, changed state/config/source/data, extra members,
replaced child directory identity and each protected named/held state-parent
metadata field. The controlled Cobra probe passed with an inert native runner;
it is not native qualification. Fresh genuine acceptance of the corrected source
is still required.
### Cold locked repeat has a separate scratch parent (2026-11-14)

The disabled-repeat correction above remains unchanged, including its exact
installation ID, `mutated=false` and complete binding-preservation joins. The
earlier held `e1438bd0` draft's proposal to allow only UAP-root ctime for both
repeat paths is superseded: the ordinary source add and the locked no-op use
different snapshot parents. This reconciliation preserves the draft's public
disabled-repeat Go regression without retaining its proposed cold UAP-root exception.

For the exact locked path, `resolveDependencySyncCandidate` verifies declarations
through `inspectPinnedDependency` and `environmentstore.Store.Snapshot`.
`dependencySyncUnchanged` also calls `Store.Get`, which creates and removes a
verified snapshot. `App.dependencyStore` supplies no `TempRoot`, so the existing
package builder uses the process temporary directory. The sealed public helper
sets `TMP`, `TEMP` and `TMPDIR` to the original fixture's `root/tmp`. Antigravity's
unchanged preflight does not call `Prepare`, and the intact sync returns before
the mutation/lock path. There is no basis to exempt UAP-root ctime in this case.

Only this cold comparison permits size/mtime/ctime changes on the exact original
`root/tmp` directory. Both pathname and held-handle metadata retain device,
inode, mode, owner, links, birthtime, attributes and reparse facts. Final membership
and every other row remain exact, including `uap-state`, `installer-tmp`, all
scratch descendants and all state, source, native, package and data bytes.
Directory size can retain allocation changes after creating and removing a
child; it is not a file-content allowance. Refusals emit bounded field names,
counts and one relative-name digest, with no paths or contents.

A real Linux create/write/remove replay under that original temporary parent
failed the prior exact cold comparison: only its pathname/held mtime and ctime
changed. The same replay passes the narrow comparison. All 38 Python contracts
pass, including isolated protected-field/identity refusals, exact UAP-root ctime
refusal, descendant scratch refusal, byte mutation and same-byte inode replacement.
The existing disabled-repeat public Go test is preserved byte-for-byte from the
reviewed draft; it was not rerun for this Python-only reconciliation. No native
command was executed, and none of the failed hosted runs becomes qualified.

## 2026-11-14: confirmed saved-owner cold inventory correction

The original `5ff83587` native runs failed on all three platforms at cold
`add --no-save`: the restored installation already had a committed saved-owner
receipt, so an imperative add without that typed owner correctly refused. The
original first-refusal audit is
`native-5ff-owned-originals/antigravity/independent-first-refusal-audit.json`,
SHA256 `3c90526c931cd60797b0212d3db0413bb48b1bf78ace5ed358e6ba4c08766588`.
Its native failure remains a failure.

Peer commit `4cd5b613ae5948742231ed0e7babe67b9c3a1c81` replaced that command
with `doctor`. Independent public-Cobra controls, with and without the exact
installation selector, both returned success and preserved the original files
while making **zero** native runner calls. The peer cold-doctor parser accepted
both original outputs and five missing/unknown-observation counterexamples.
The independent rejection receipt is
`antigravity-peer-4cd-review/independent-r2.json`, SHA256
`3f05951a8061df92a3414a99c949a615e856bace45a186663bb88689a6eb3617`.
The actual doctor-control log is 5,526 bytes, SHA256
`9bd6435a1e186f72f3b3fc999ac892dd8b37769258f483c9518cc04be340f55c`.
A truthful recorded-state doctor result is not native inventory evidence.

The same source also left the fixed-argv contract expecting the former add
vector. Original `4cd5b6` Antigravity Linux/Windows job logs, Required, and
Coverage show `TestPublicActionsHaveFixedSelectorsArgvPinsAndEnvironment`
failing before the native collector. These are source-contract failures, not
new native qualification. No run was rerun or cancelled for this correction.

### Accepted R1 and bounded implementation

The root reviewer accepted exactly nine existing paths. Antigravity now
implements the existing `UserNativeRegistryLayout` capability and returns its
unchanged selected ConfigRoot. This lets the existing CLI/SDK forwarding retain
the detector's separately observed NativeConfigRoot. An empty or foreign native
root still grants no inventory authority; no HOME or profile is fabricated.

The existing read-only verification path also admits a confirmed, non-preview,
materialized Prepared User binding when the existing scoped
`canResumePreparedObservation` capability is available. It still sends
`VerifyOnly` and uses the original saved-owner repair composition. No generic
client-ID branch, lifecycle trait, public DTO, trust grant, native enablement or
authentication path is added. SDK NoChange planning only omits staging resources;
Apply still dispatches the existing repair use case.

The cold helper now selects only
`repair uap-cline-public-probe --target antigravity --scope user --yes --format json`.
All other command vectors, Cline code, public/native deadlines, profile setup,
source acquisition, child cleanup and artifact admission remain unchanged.
The exact fixed-argv test is updated with that same repair selector.

The cold collector admits the existing top-level dependency repair DTO from
`deps_render.go`: exact command/scope, complete action and result sets, original
installation and binding IDs, alias, before/after source revision and digest,
the fixture's native Skill `check` and MCP `uap_public_probe`, unchanged result and the single
enabled-inventory guidance produced by the Antigravity verifier. Prepared,
installation-verified and authentication-not-checked recorded state is joined
to the held cold identity. A doctor envelope, missing or extra field, wrong
owner/revision, disabled/absent/unknown inventory, extra result or promoted
recorded lifecycle refuses. All pre-existing cold source, receipt, DATA,
agreement-only transfer, full User preservation and locked-repeat assertions
remain intact.

### Evidence identity and outstanding execution

The immutable original saved-owner Cobra repair RED is separate from the new
tests in this patch. Original source SHA256:
`0d05c4aa9caae106864e51b2bd1f3674a22cd38e12386388073ba5b4b19a322c`.
Its 496-byte original log, SHA256
`b12ce15500019e35db678af466a94d6a8d7045950ea6f8f81c1dad74c25424b6`,
observed repair success with zero native calls, complete preservation, and no
inventory guidance. That result established the missing observation path.

The two runtime candidate bodies were recovered exactly from the held original
source and complete patch, matching their previously recorded local SHA256s:
`adapter.go` `9e47aef87ad12a4061ab291150e1ba4374ec7ce8e853857623bd70f7995f8fb5`;
`lifecycle_observation.go`
`3d06bac86b0d72f2212d0b79ddb256b7476a756d123c5a4d1b1a0f2977bd66a1`.
The first local candidate build was killed in compilation, before test execution.
Its log SHA256 is
`1213c32b3c28753c0ae38e7269064c16d5eb5859274882d716b7eea19aedb9fc`.
The authorized unchanged overlay attempt, session 76044, lost the executor;
its result is **unknown**, and it has not been restarted.

During that outage a new, explicitly unexecuted saved-owner fixture was authored
from the current public test seams. It uses real source acquisition, typed
EnvironmentAuthority, SDK, kernel and Cobra, with the existing inert scanner and
runner. It does not fabricate persisted state or grant an allow assessment.
Its identity is new and is not the original `0d05c4aa` RED. It checks ownerless
add refusal, doctor zero-call controls, two exact repair observations, original
source/receipt/binding/DATA/User bytes and file identities, and negative consent,
profile, cleanup, disabled, native-absent and absent-config cases. Added use-case and adapter
tests exercise the same scoped capability and preserve Project file-only rules.
Synthetic collector mutations test the real DTO shape; they are not native
runtime evidence.

These newly authored Go/Python cases, formatting/lint, and the complete native
candidate have **not** been executed locally while the executor is unavailable.
Immutable source objects and a review commit are prepared for independent R2;
no branch update is authorized by that preparation alone. Existing hosted
contracts and genuine native runs must validate the eventual reviewed head.
No new native, IDE-loading, MCP execution, trust, model or authentication
qualification is claimed by this source correction.

## 2026-11-14: independent saved-repair oracle and fixture corrections

The follow-up started at `440f0666dad9255c1667c86f95654f7d960ffeb6` and is composed
onto `9326475ff499be6f836727d268dbaec87baf7835`. It preserves the two reviewed
production changes from `ff5ed890`. The incoming doctor JSON
decoder correction is retained: the original `86a` hosted output was compact
JSON (`"read_only":true`), so the former space-sensitive substring assertion
refused a valid doctor result. Doctor remains a zero-native-call control.

Independent source review found that the fixed remote package also includes
`skills/check/SKILL.md`. The standard loader includes it, the planner emits
Skill decisions before MCP decisions, and the public dependency renderer keeps
both. The cold repair oracle now requires exactly those two ordered native
components. It still refuses missing, duplicate, foreign and extra components,
unknown fields, changed identities, missing inventory evidence and doctor output.
The original doctor parser and its complete independent control test remain
available; a doctor result does not satisfy the repair observation slot.

A real temporary-file counterexample also showed that replacing `state-v2.json`
with the same bytes, mode and mtime on another inode passed the former selected
field comparison. The cold observation now joins the complete original state row
and decoded state before any new baseline. This includes all environment receipts
and immutable binding fields. Existing package, DATA receipts, original DATA
directory/file identities and full User-profile comparisons remain in force.
The inert contracts exercise same-byte state and DATA replacements and changes
to an environment owner and an otherwise unselected binding field.

The public test fixture now obtains saved ownership through the real Cobra
`add` path, without constructing an `EnvironmentAuthority`. A separate cold
fixture starts without state, store, delivery or DATA, receives only the two
original `uap.toml` and `uap.lock` files, and performs public `install --locked`.
Repeated confirmed repair must invoke the existing exact version/inventory pair
while retaining the original source, state, complete receipts, portable agreement,
environment store, selected profile and DATA. The pre-command file observer is
the existing handle-based dependency-check observer; it captures Windows file
identity before a later replacement can occur. No native executable is launched
by these Go fixtures: the existing runner and scanner remain explicit inert ports.

The initial local public fixture failed before acquisition because its original
temporary path traversed the restored execution cache's `tmp` symlink. A
diagnostic overlay identified that exact component in the shared recovery
preflight. The fixture now resolves its fresh directory before constructing any
state or authority. A small extraction of the existing CLI fixture accepts that
original root; its default wrapper and all existing dependencies are unchanged.
No production path or environment guard is weakened.

The earlier three-platform `f6` missing-config refusal retained a compound
preservation error, without the changed field. A new Linux replay exercised the
actual public local `add --no-save` refusal with the ordinary User acquirer's
scratch root set to `AGENTPLUGINS_HOME`. It refused missing native config with
zero native calls. Original named and held state-root identity, mode, owner,
link count, size and final membership remained equal; root mtime/ctime advanced
after the sealed snapshot was created and removed. Every descendant, source
and profile remained unchanged except the already admitted advisory lock refresh.
The strict root predicate consequently failed. This is new causal evidence;
the historical `f6` logs do not themselves establish their metadata delta.

The refusal comparator now uses the same closed state-parent scratch rule as
the disabled-repeat control. Its only two spellings are `uap-state` inside the
full fixture and `.` when the selected state root itself is captured. Only that
directory's size/mtime/ctime may differ. Both named and held device, inode, mode,
owner, link count, birth, attribute and reparse facts, final membership, and every
descendant remain exact. State, receipts, DATA, store entries and nested scratch
directories receive no exception. Real temporary-directory and same-byte inode
replacement cases, every retained metadata field in both API domains, unknown
members and changed descendant content are covered by the contracts.

The four comparisons retain their short-circuit order. The first failing domain
(`state`, `bundle`, `config-members`, or `config-parent`) is appended to the
existing bounded error. The first predicate now uses the specifically reviewed
root rule; the other three comparisons and all snapshot helpers are unchanged.
There are no additional reads, paths, bytes, JSON fields or commands. A four-case
inert contract trips if any later comparison runs after the first failure.

Actual local validation on the preserved incoming production gate:

- Seven public AGY CLI tests and eleven subcases passed, including saved add,
  agreement-only cold restore, repeated observation, both doctor controls,
  original state/receipt/DATA preservation, disabled and absent inventory,
  missing config, cleanup failure, dry run, and a declined interactive repair.
  Existing JSON and explicit-target confirmation semantics are unchanged.
- Replaying only the original `f6` lifecycle-observation source made both real
  public saved/cold repair cases fail with zero inventory calls. The current
  source passed both without an overlay. The RED log SHA256 is
  `58c7cebd55efafa540960a996dd712dce55a2a26ce29d0fa15ebf0b3d6f153cf`;
  the GREEN log SHA256 is
  `425f009a9d3b4c8482ee99c92f282ee1558afa15fa41d5ec826ad79a12577b3f`.
- The five AGY use-case tests, complete AGY adapter suite and research helper
  suite passed. The original frozen Python run passed 43 contracts in 5.513 seconds;
  its log SHA256 is
  `88afe846b500d4c822222718b70ff0c5529f468b2ef1456137ad12b3ee469010`.
- The final composition retains all seven incoming `9326475` component negatives,
  their exact mutation bodies, and the incoming two-component repair DTO. The
  additional second-component unknown-field control also remains explicit. All
  43 Python contracts passed again in 5.323 seconds; the final log SHA256 is
  `fbccba43eeabfb4c3e852ea983ae6067b6a28ab3ba46e76384a18c40bb3fbca3`.
  The Go source files and production gate are byte-identical to the tested freeze;
  no new Go or native execution is claimed by this composition.

The local root-metadata diagnostic source, overlay and exact failing log are
retained; the log SHA256 is
`a31c7dbc0fe9734deb88b78c466c622d7407b1d16ea6bc6d5707ad870597b033`.
These are inert Linux contracts, not macOS/Windows native runtime evidence.
Native admission, enablement, IDE loading, model and authentication qualifications
remain unchanged; genuine current-source captures and the final combined gates
remain necessary.

## Prospective first-preservation-refusal diagnostic (2026-11-14)

The original f6adab5a0c660909d892961285f2d8af050eae72 native run
43569444834 failed on Linux (job 132373147150), macOS (132373147120),
and Windows (132373146907). The original logs show the first
missing-config negative command returned exit 1 with cleanup_complete=true,
then the collector refused: "Refused public repeat changed original
membership/bytes". The four-way predicate did not identify its first changed
field. The original receipt archive contains command exchanges, not those
filesystem before/after snapshots. This record does not infer which
preservation predicate failed or claim a native pass.

Root R1 accepted a prospective correction in this collector and this plan
only. It preserves the same ordered state, bundle, native-membership, and
native-parent comparisons. Each getter runs once when reached; no later
getter runs after the first mismatch. The helper adds no filesystem reads
and does not change admission, control-lock normalization, native actions,
budgets, cleanup, source identity, or publication authority.

An existing refusal may append one closed ASCII JSON diagnostic; the
complete error remains at most 1 KiB. The record names a fixed group, view,
and field, plus SHA-256 of the relative member identifier and the two
changed primitive values. It never prints member names, paths, raw state or
config bytes, or arbitrary object/exception representations. Only exact
bounded integer, boolean, known-kind/digest string, optional null, or
already-held bounded byte values are hashed. Unknown shapes/values retain
a fixed representation diagnostic. The explicit tuple mapping includes
all original named/held fields and the existing nine-field normalized
control-lock shape; it grants no new normalization exception.

Four new inert contract methods cover lazy getter call counts and
unchanged success, every named/held metadata field, the first lexical
changed member and content/membership changes, and poisoned/unsupported
values without rendering them. Existing contracts and other collector
functions remain byte-identical. These source changes and contracts are
UNEXECUTED locally while the execution environment is unavailable; source
review is not a test or native result. The prior overlay session 76044
remains UNKNOWN and was not restarted.

The initial candidate was held on
86a619bb31d33014236e4564fb1aa7a77de16583. The current
9326475ff499be6f836727d268dbaec87baf7835 run 43577051075 now supplies
fresh evidence of the same preservation refusal on Linux (job
132393456858), Windows (132393457146), and macOS (132393457172).
All three original logs first complete cold observation and locked repeat,
then record the missing-config public repeat at index 14 with exit 1 and
cleanup_complete=true, followed by the same uninformative preservation
refusal. The preceding cold two-component oracle and its seven new
counterexamples remain byte-identical to the incoming 932 commit.

The Linux original log is 54881 bytes with SHA-256
1838845819da276b4961d501860ebb991595897f49dde1ea16036e7163d64d92.
Its official artifact is 14107568423 (16485 bytes, SHA-256
d99c07b33d21e68a1181cf4277ab8f2aea315e0840d29fa1d33d721ad695d866).
The raw log and official metadata are retained; the binary ZIP has not been
replayed in the unavailable local executor. Neither the old nor current
log identifies which of the four snapshots changed.

This revision carries the exact previously held helper and four inert
contract methods onto 932, with the same ordered lazy comparisons.
Removing only these additions and restoring the original predicate
reconstructs the entire incoming collector byte-for-byte. Root R1 permits
this diagnostic after the new actual refusal; independent source R2 and
genuine hosted execution remain required. No failing field is normalized,
ignored, or assumed repaired.

## 2026-11-14: original fe6 field evidence and combined refusal oracle

The independent replay of the exact `fe6f7d31e08b560395a19d33422ddd45f17a02cb`
Windows and Linux originals joins both official ZIP digests, physical headers,
catalogs and CRCs, 59 inner members, 58 member hashes, 29 request/receipt pairs
and 44 stream hashes per platform. The completed cold saved repair carries the
native inventory witness and both authored components; the later missing-config
public repeat correctly refuses with `mutated=false` and completed child cleanup.

The new parent diagnostic identifies the same first changed field in both runs:
`state`, `metadata`, `named`, `mtime_ns`. Its member hash is exactly SHA-256 of
`relative-member-v1\0.`, identifying the original state-root directory. The
Windows original artifact is `14108680071` (16,382 bytes, SHA-256
`a66dc19abd26794c0f95b5c6e53525096527410342604869924801d72e9ede97`);
Linux is `14108151835` (16,475 bytes, SHA-256
`533bb54f1781a6440144db707422904d784561d852d024f0fdfd2dbaeebf1e4a`).
These records establish the first delta only. They do not retain the complete
filesystem postimages or prove that no subsequent difference exists.

The combined collector preserves the reviewed original-root scratch comparator
and the complete incoming bounded JSON diagnostic. It accepts only the original
state root's size/mtime/ctime differences under the existing identity and final
membership checks. A refused state comparison describes the first remaining
field after those already permitted values; it cannot mistake the root timestamp
for a changed descendant or hide an original root identity mismatch. Diagnostic
projection operates only on copies of the captured rows, adds no reads and does
not change the admission comparison or any stored snapshot. The four getters
retain their original order and stop at the first refused domain. The complete
ASCII message remains at most 1,024 bytes and retains the closed, path-free JSON.

All four incoming diagnostic test methods and their negative bodies remain
unchanged. The earlier end-to-end four-domain test keeps its original effects
and later-getter tripwire; only its expected group labels, JSON format and bound
are updated. A real temporary-directory composition contract checks accepted
root scratch, a held root inode mismatch and a changed descendant in the same
snapshot. Full state, saved/cold fixture, DATA, doctor and all prior component
negatives remain in place. This composition does not claim a completed native
lifecycle, IDE loading, MCP execution, authentication or model qualification.

## 2026-11-14: proposed missing-config discovery measurement

Status: concrete plan for source review; no implementation or native execution.
Plan base is `751f87114212bee56324ef4f58a991328da7b43f`, tree
`28d15a70572cd230a6643a90d2424389c52ac179`. Its four relevant files are unchanged
from `3cab58db0cd950f664c778815990ee0a662b8072`. This plan does not admit missing
configuration in the product's `inventoryConfig`, change activation status, or
qualify enablement recovery. The question is whether the exact pinned CLI can
discover the already prepared User directory when its default config is absent.

### Scope and existing authorities

The implementation, after a separate source approval, is limited to:

| Existing path | Exact proposed responsibility |
| --- | --- |
| `install/integrationctl/research/antigravityinventory/main.go` | Two closed research action roles using the existing process supervisor, pins, control frame and receipt. |
| `install/integrationctl/research/antigravityinventory/main_test.go` | Real helper request/runner/receipt negatives and preservation of existing actions. |
| `scripts/e2e/antigravity-native-cross.py` | One existing-fixture call site, the metadata-first ledger check, the fixed pair and sidecar, and embedded inert contracts. |
| `docs/plans/2026-11-13-antigravity-native-admission.md` | Reviewed scope, limits and later evidence disposition. |

No workflow, dependency, client adapter, public CLI/API, state format, feature
flag, acquisition, copy, or separate fixture is added. Existing 14 native slots,
23 public slots, their ordering, the five refusal cases and their conditions
remain unchanged. The proposed additional measurement has exactly two roles;
neither role enters those existing slot catalogs. Target additional runtime is
220–280 lines and meaningful contracts 120–160 lines across the existing files.
Any excess needed for a concrete authority check requires another source review;
the 500-effective-line, 60-line/40-statement limits and existing exemptions are
unchanged.

`public_user_refusals` already owns one fixture with `condition == "missing-config"`.
It performs ordinary `uap-add`, records the real binding, authors then removes
that fixture's config, and receives the expected UAP repeat refusal. Product
detection resolves the binary without running `--version`; this initial add is
Prepared and does not constitute a native activation attempt. The new call is
placed only after the existing state, bundle, membership, parent and applicable
config checks all succeed, while the original `public_fixture` context is open.
An earlier refusal or preservation failure cannot launch either new command.

Retain the complete result of the existing `public_binding_proof` call instead
of retaining only its `binding` field. Pass that already verified source/package
revision, IID, binding ID, physical selector, committed directory receipts,
DATA receipt/owner marker and selected path into the new fixed function. Rejoin
them to bytes in its strict baseline. Do not obtain a replacement identity from
new native output. The request selector and declared name remain the fixture's
literal `NAME` (`uap-agy-native-smoke`). Its physical basename/path come from the
verified binding and must match the unique native row; name equality alone is
insufficient.

The raw baseline is the whole existing fixture, including source, package, DATA,
state, control lock, executable and all ordinary siblings. Use a single
`ProfileLedger(root, public=...)` with exactly the current public runtime/scanner
pins and caps, joined to the still-held root `(fd, named, held)` authority. It
does not reuse `public_state_snapshot` normalization or the state/tmp scratch
exceptions. No file is created or normalized to make this baseline pass. The
selected `home/.gemini/config/config.json` must be absent in the complete
metadata membership, with its original existing parents held. This measures
default discovery with an absent file; it does not claim an absent selected
override, environment-policy equivalence, or Project discovery.

### Two fixed helper actions and one deadline

Add only `missing-config-version` and `missing-config-list` to a private
`missingConfigAction` predicate. The existing eight request fields remain exact;
unknown, duplicate, missing, malformed and trailing input stays refused.
`validateRequest` additionally requires the literal selector and the exact
`<original root>/bin/agy` (`agy.exe` on Windows) for these two roles. Both use the
existing native SHA pin, original source commit/tree, held root and binary checks.

| Role | Exact native argv | Working directory | Native command environment |
| --- | --- | --- | --- |
| `missing-config-version` | `<pinned bin/agy> --version` | `<root>/home/.gemini/config` | Exactly `HOME=<root>/home`, `USERPROFILE=<root>/home` |
| `missing-config-list` | `<pinned bin/agy> plugin list --json` | Same original directory | The same exact two entries |

The helper derives these fields; request input supplies no argv, cwd or env.
Only the two new roles take this branch in `fixedArgv`, `commandEnvironment`,
the fixed directory selection, `commandBudget`, `admittedExit` and
`commandReceipt`. All old action predicates, commands, environments and limits
remain unchanged. The parent invokes the same pinned helper through
`bridge_exchange`, preserving the existing public-fixture parent environment;
it does not add PATH, TMP, credentials or XDG overrides to the native command.
Use the same actual `process.OS.RunWithDescendantExitGrace`, not a new runner.

Freeze one monotonic pair end and its wall-clock counterpart on entry:
`pair_end = min(start + 10 seconds, capture.public_deadline - 5 seconds)`.
The last five seconds of that pair are reserved for the existing helper/family
cleanup, so the one native deadline is `pair_end - 5 seconds`. Both requests
contain that identical original wall-clock deadline. The helper's new-role
maximum is ten seconds; the parent supplies only the remaining native interval.
The existing bridge's additional five-second cleanup window therefore ends no
later than `pair_end`. Baseline, reservation, transport and validation consume
this original budget. Do not start if the remaining native interval is exhausted.
Check the original clock around and during bounded ledger work; expiry makes
the measurement incomplete and prevents the next child. The outer 360-second
public deadline and its final five-second reserve are never renewed.

A valid settled version receipt must have exit 0, empty stderr and the exact
already pinned version body `1.5.3\n`, followed by a complete unchanged ledger.
Only then may list run. Nonzero exit, dirty stderr, wrong version, malformed or
truncated receipt, changed authority, profile effect or expiry means zero list
calls. The list JSON is decoded from the captured body without another command;
reuse the current physical path/name, version and component predicates as pure
checks. Require the one selected row and explicitly typed boolean enablement,
the exact original selected path, Skill `native-smoke`, and both authored MCP
names. An absent/duplicate/wrong row is a failed measurement. Record the observed
boolean without forcing it or treating it as activation or recovery authority.

For these roles only, reuse the existing 14-field public receipt shape, including
typed exit 0..255 and exact base64/length/SHA joins for stdout and stderr. This
allows a settled negative native result to remain evidence; the parent still
refuses its success claim. Retain the current strict stdout `<1 MiB`, stderr
`<32 KiB`, single receipt `<=2 MiB`, request `<=16 KiB`, and bridge bounds.
No transport error, deadline cancellation, missing child, overflow, unsettled
family or failed held-runtime check can manufacture a receipt. Such a failure
leaves `capture.pending` true and withholds raw publication under the existing
rule. Successful child cleanup alone never proves unchanged profile effects.

### Whole-tree metadata check before any post-command body read

The existing `ProfileLedger.scan` interleaves enumeration and body reads; its
ordinary `observe` method therefore cannot enforce this new condition unchanged.
Add narrow optional keywords `expected=None` and `deadline=None` to `scan`.
They are used only by this fixed pair. Default callers retain the current path.
The expected value is the original raw `ledger.base`, never a callback, adopted
postimage, normalized view, or new set of created identities.

With `expected` present, first enumerate the entire bounded tree using the
original held directories and no content reads. Admit the exact set of relative
members and roles, all named/held directory metadata and all named regular-file
metadata. Reject an unknown directory before opening/adopting it. Original file
dev/inode, owner, mode, nlink, size, mtime/ctime, birthtime, attributes/reparse
fields and each platform's original named/held domains remain exact. Check
known-file/opened metadata against the original row before reading it. Existing
atime handling is unchanged. The original 4,096-member/depth-24, file-size,
volume, single-link, owner, per-file and 16 MiB ordinary aggregate bounds apply.

Only after the complete metadata pass succeeds may the existing streaming hash
pass start. Check original expected metadata again immediately before each open
and on the returned handle, then preserve all current post-read and final held
parent/file fences. Compare the complete raw result with the original baseline.
Use the optional absolute deadline for checks during bounded traversal/chunks;
there is no renewed timeout or broad callback seam. Rehash external runtime pins
in the parent only after this whole-fixture metadata pass: a late unknown
profile member refuses before any parent runtime/config body read.

One existing boundary needs explicit review: before returning its receipt the
Go supervisor already rehashes its original held executable descriptor. That
operation reads no newly selected profile member and must not be removed to
make this plan appear stricter. The metadata-first claim above covers every
new parent profile/pin capture; it does not assert that this existing held-FD
rehash happens after the parent metadata pass. If the required order includes
that original descriptor too, this four-path design needs a separate sequencing
decision before implementation, rather than silently weakening its pin proof.

Perform this check before the first child, after its settled receipt, and before
and after the second child against the same baseline and held directory set.
Parsing an already captured receipt in memory to establish family settlement
does not read the filesystem. A metadata refusal records only bounded captured
metadata and its reason; it must not read a changed/unknown member to construct
an evidence report. It has no accepted post-hash ledger and no next child.
No state lock, PID, timestamp, original root size or tmp entry is normalized.

### One bounded sidecar and publication order

Use exactly one optional member, `missing-config-discovery.json`, in the existing
`SourceReceipts` archive. It is enabled only by the validated missing-config
fixture call, not by arbitrary request flags. Its independent measurement status
does not change the meanings of the existing native/public slot-completion flags.
Default archive closure stays 75 members (29 without public slots); only this
one explicit member permits 76. The total physical ZIP stays at most 32 MiB,
with the existing 32 KiB manifest bound and publication identity/CRC fences.

Proposed sidecar cap for review: 8 MiB. Before either child, reserve that capacity
plus the existing 64 KiB manifest/ZIP allowance from the same 32 MiB budget.
The already retained members must fit the remainder. Compute and bound the
actual immutable context and baseline metadata/hash projection before launch;
require that three complete projections, two maximum base64-encoded 2 MiB
receipt frames, two maximum base64-encoded 16 KiB request frames, fixed fields
and a bounded negative diagnostic fit the sidecar cap. Refuse before launching
if they cannot. The later unchanged projections cannot grow: their membership,
strings and metadata must equal the original. Later old public receipts count
the actual sidecar bytes in their existing aggregate check. Do not increase the
overall budget, clip streams/rows, omit inconvenient members or stuff frames
into the manifest. A small descriptor alone goes in the manifest; its old slots
remain exact.

Implement fixed `SourceReceipts.begin_missing_config` and
`admit_missing_config`/finalization methods, without a general action framework.
They join the existing `missing-config` world root, source and native/helper/
installer pins; require the already accepted public prefix through zero-based
slot 14 (`missing-config` / `uap-repeat`, exactly 15 old public slots); reject
duplicate, reordered or cross-world exchanges; and set
the existing pending fence before launch. A typed, fully joined cleanup receipt
permits retaining that raw exchange as negative evidence even if semantic or
profile admission subsequently fails. Unknown cleanup keeps pending true.

The sidecar uses a closed versioned object: fixed kind/world, source commit/tree,
original root named/held authority, runtime pins, selector/cwd/two-entry env,
one original deadline, already verified owner/revision/DATA joins, the original
complete metadata/hash ledger, and up to two ordered exchange records. Each
exchange contains exact original request and receipt bytes as bounded base64
with byte count/SHA, phase, decoded typed exit, accepted post-ledger or a bounded
metadata-only refusal, and explicit effects/semantic status. Stdout/stderr are
retained exactly once inside the original receipt frame, never reconstructed.
Serialize every original metadata/hash row without the control/scratch
normalization; body hashes come from the existing bounded full content read.
The in-memory comparison still uses the full raw ledger including its bodies.

The exact top-level keys are `schema_version`, `kind`, `world`, `source_commit`,
`source_tree`, `root`, `root_authority`, `pins`, `selector`, `cwd`, `env`,
`deadline_unix_ns`, `owner`, `baseline`, `exchanges`, `measurement_complete` and
`qualification`. Root authority has only `named`/`held`; pins only installer,
native and helper byte-count/SHA pairs; env only HOME/USERPROFILE. Each exchange
has only `role`, `request`, `receipt`, `exit`, `effects_verified`,
`semantic_verified`, `post_ledger`, `refusal`. Raw frame records contain only
`bytes`, `sha256`, `base64`. A post-ledger is either the complete original-shape
metadata/hash projection or null, never a partial projection labeled complete.
Refusal is null or a fixed-reason, fixed-field, relative-member-hash record of
at most 1 KiB; it contains no private path, body or arbitrary exception text.
The owner record carries the original IID/binding/physical/path identities,
source/package/package-revision and mutation/data receipts, plus the original
state and DATA owner-marker hashes, all joined from the already read proof.
The ledger projection names every row and preserves every named/held metadata
value and content hash; omitting duplicate bodies from serialization is not a
normalization of the in-memory authority comparison.

`measurement_complete` is true only for both ordered settled commands, exact
version and selected row, unchanged complete ledgers, source/pin joins and
deadline admission. `product_admitted`, `enabled_recovery_qualified`,
`native_project_qualified`, `ide_activation`, `native_mcp_call`,
`vendor_authentication` and `real_model_inference` remain literal false. A
settled refusal retains only its actual prefix and stops this case; incomplete
post-ledger rows are never labeled full proof. Existing harness/final source,
pending-family, receipt-root and final publication rules still decide whether
any archive may be exposed. No native retry, enable, trust, login, model, help,
import, path fallback or cleanup mutation is introduced.
The measured filesystem scope is this complete original fixture; it is not
a proof of global host write confinement or exact production recovery.

### Required causal contracts before implementation acceptance

1. Exercise the existing helper's actual decode/execute/runner boundary for both
   fixed roles. Assert exact argv/cwd/two-entry env and original source/pins;
   wrong root/binary/selector/action, arbitrary argv/env fields and renewed or
   expired deadline produce zero runner calls. Existing native/public actions
   retain their exact old behavior, including old stderr/exit policy.
2. In the actual parent function, make version fail by nonzero exit, stderr,
   wrong version, receipt/hash/identity failure, expiry or a real filesystem
   effect. All cases have zero list calls. A valid list fixture records an
   observed false as a boolean observation without enablement credit; duplicate
   or wrongly selected rows refuse.
3. Place an unknown member late in real directory enumeration while an earlier
   original file remains readable. Count `os.read`/owned-body and runtime-pin
   reader calls: the metadata failure must happen before every body read.
   Exercise same-byte inode replacement, parent replacement, changed mode/link
   count and changed source/DATA/control/root timestamps; all refuse without
   adopting new directories or launching the next child.
4. Cause an effect between version admission and list launch, and another during
   a post-read final fence. Require unchanged original baseline/held handles,
   zero subsequent child, no false complete ledger and accurate retained prefix.
   An earlier existing UAP refusal-preservation failure must prevent the new
   function from launching at all.
5. Through real `SourceReceipts.publish`, distinguish settled negative evidence
   from an unsettled family that withholds every raw member. Test ordered two-
   role closure, wrong source/world/pin, missing/duplicate/extra sidecar, 75/76
   member policy, pre-child reserve exhaustion and incremental sidecar/ZIP
   overflow without clipping. Reject manifest growth beyond 32 KiB.
6. Preserve all old 14/23 role arrays, five refusal cases and prior contract ASTs.
   Run the affected helper tests and complete embedded Python contracts after
   implementation, then independent source review. Actual final-source native
   originals on the three existing platforms remain necessary; inert contracts
   and this plan grant no product/native qualification.


### Implemented bounded missing-config observation candidate (2026-11-14)

The four-path candidate implements the reviewed pair inside the existing
missing-config refusal fixture. Its two new helper roles use the original
task-owned pinned executable, the exact manifest selector, the fixed User
configuration-directory cwd, and only the two explicit HOME/USERPROFILE
overrides. The preceding fourteen native and twenty-three public action catalogs,
including all five public refusal cases, are unchanged.

The pair captures one original ten-second deadline, including a five-second
cleanup reserve, below the unchanged public journey deadline. The existing
bridge now accepts an optional absolute deadline for this pair; its default
relative-timeout behavior is unchanged. ProfileLedger accepts the same original
deadline during baseline construction. Every later scan closes the complete
original metadata namespace before reading any profile body, then checks every
held file, full bytes/hash and final directory metadata against that baseline.
The original helper-held executable rehash remains intact.

The already captured public_binding_proof now retains one private
authored_source subtree when that exact member exists in its image. No new
source read occurs. The pair requires the original nonempty subtree, state,
owned package and DATA rows to match exactly. Its sidecar retains all metadata
and file hashes, exact request/settled-receipt bytes, separate effect and semantic
results, original source/runtime pins, and the selected IID/binding receipts.
Normal negative exit/stderr is evidence only after the existing cleanup proof;
an unsettled family still revokes the entire raw upload. The single fixed
sidecar reserves eight MiB before either child inside the unchanged thirty-two
MiB archive limit; the opt-in catalog has seventy-six members and the old
seventy-five-member catalog is unchanged.

Local validation is inert. The complete helper package passes and its scoped
all-rule lint reports zero issues. Its new fixed-role contracts fail against
the original helper before the implementation and pass with the candidate.
All fifty-one Python contracts pass, preserving the original forty-eight test
methods. Removing only the new metadata prepass produces three genuine
zero-body-read failures: a late member, a byte-identical file replacement,
and a change between accepted commands. Other new cases cover the absolute
deadline before baseline/body/Popen, unchanged old refusal ordering, original
source continuity, capacity refusal before launch, failed version with no list,
unsettled/invalid receipts, exact inventory identity and unknown sidecar members
or fields. These checks provide no native admission: every product, recovery,
Project/IDE, MCP-call, authentication and model qualification field remains false.

### Independent review and current public-lifecycle evidence

The four existing-file base blobs were checked against published
`f665e332fa944e46e2942119036a076f0381268a` before copying the candidate into an
isolated review worktree. The source worktree remains unchanged. Independent
review reproduced one post-metadata-pass race: an original directory timestamp
changed before the content pass, but the first candidate still read its unchanged
payload before refusing the final comparison. The content pass now rejoins each
original directory's complete row before visiting its files. A causal test changes
the actual root and nested directory separately after the metadata prepass and
requires refusal with zero body reads. No default caller or allowed metadata
transition changes. The existing exchange matrix also retains an observed
`enabled=false` result without granting enablement or recovery qualification.

The corrected candidate passes all 52 embedded Python contracts. The Go helper,
focused Antigravity/provider tests, and public Antigravity CLI tests pass; scoped
helper lint reports zero issues. Final combined-source review, ordinary CI and
genuine three-platform execution of the new measurement remain required.

The earlier published f665 source has independently completed its existing
three-platform public lifecycle in run
[43591958018](https://github.com/777genius/universal-agent-plugins/actions/runs/43591958018).
Original macOS artifact `14113640015`, Linux `14112907762` and Windows
`14112867344` each pass both ZIP CRC layers, their original API SHA-256, the exact
f665 source/tree join, all 75 members, 74 request/receipt-file joins and 60 original
stream length/hash joins. Every archive has 14 settled native commands, 23 public
commands, and true capture/public-effects completion. Agreement-only cold install,
unchanged saved repair with the Skill and MCP inventory witness, locked repeat,
and all five expected public refusals are present. The release-only aggregate is
skipped and is not counted. These originals contain no missing-config measurement
and do not qualify this new candidate, Project native discovery, IDE loading,
MCP calls, authentication, a model request, or production enablement recovery.

R2 also identified a measurement-decoder mismatch with the production native
decoder: the selected row admitted extra fields, and sorting an object-valued
`mcpServers` could accept its keys as a native array. All 21 original f665 list
captures across the three platforms contain the same six-field schema, covering
63 typed rows. The measurement now requires exactly `name`, `path`, `version`,
`enabled`, `skills` and `mcpServers`, with strings, a Boolean and string arrays in
their corresponding fields. The two malformed cases pass the former decoder
and refuse the correction. Both run through the actual pair and sidecar test:
their settled receipts remain available, but measurement completion is false.
All 52 embedded Python methods still pass after these two additional subcases.

## 2026-11-14 — fresh required observation with an originally absent config

This is fresh source work against the Antigravity files in merge
`a50caa828f84657ba24e1195c0e2f4aaf35ea58b`, unchanged through
`c2b9c0766b722a283e901ce7bfc8b766d36e4242`. The earlier six-file local
candidate became inaccessible when the executor stopped. Its old bytes were
later recovered, but this fresh implementation has different parent authority
and public HOME guards. Historical executions of the earlier candidate do not
qualify this composition. The new R1 is Git blob
`b12a712e71a7fc32d313b97fca3ca79227a8d32e`; independent source reviewers
accepted the narrow authority and test boundaries before implementation.

Main #911 preserves native MCP selection and adds intact Project/User repair
journeys. Those exact changes remain in this candidate. It does not change the
User inventory guard: the current guard rejects a missing config before native
observation, including required unchanged/verification requests.

### Original absence and original parent authority

Only `VerifyOnly || PackageUnchanged` may retain an originally absent final
`config.json` leaf as observation authority. Capture the original named parent
before acquiring its one held directory, use a terminal separator plus dot for
directory acquisition, and rejoin both original parent records around the
leaf's no-follow ENOENT check. Parent, backend, alias, replacement and other
acquisition errors are unavailable observations. They cannot borrow the benign
initial-delivery config-absence classification.

The absent lease retains its original root and parent metadata throughout the
same ten-second context, both native commands and final cleanup. It never
reopens, creates or normalizes the missing config. Existing config bodies
still require the exact supported schema and selected Boolean override.
Backend absence is never admitted. A nullable expected value is allowed only
after verifying the explicit absence lease; the native result still passes
the original exact six-field, typed identity/version/component/uniqueness
decoder. Initial or changed delivery with an absent config stays Prepared with
zero native calls.

Actual enabled, disabled and absent inventory observations affect only
transient guidance. Prepared/installation_verified, ownership, original DATA,
scope, consent, cleanup and all activation/authentication boundaries remain.
There is no native enable action, trust write, or runtime capability promotion.

### Public result joined to its actual native observation

The fresh public candidate is recorded in review packet
`0609ba1547d025106c1698b2a7a2e0c31df417f5`. It retains the exact 14 direct
native slots, 23 public slots, two standalone missing-config calls and
76-member archive bound. Slot 13 remains initial delivery; slot 14 must now
complete the public repeat with an observed Prepared result. The following
already-required native list returns its validated selected-row Boolean only
after the original effects, semantic, deadline, receipt-seal and ledger-close
checks. Public enabled/disabled guidance must match that actual Boolean.
The other four refusal worlds and every existing false qualification remain.

Independent R2 found and corrected a new baseline-adoption risk: four existing
pre/post checks covered state, bundle, native-parent membership and identity,
but a successful repeat could also affect another HOME member. The corrected
candidate snapshots the complete original HOME before slot 14 and compares it
after those four checks, before creating the sidecar ledger. A new contract
writes a real other HOME file in the public-repeat callback and requires
refusal before the sidecar runs; unchanged enabled and disabled controls must
reach it once. The earlier lazy-domain regression now keys its injected
change to the actual repeat action, preserving all first-failed-domain and
no-later-read assertions. The superseded first public packet was never
published.

The saved-owned CLI repair case with missing config now requires exactly two
native commands and the existing strict source, installation, binding,
revision, component and transient-guidance DTO checks. Original state,
ownership, DATA, consent, cleanup and unrelated negative cases stay unchanged.
Windows mutation tests may distinguish exact OS prevention from a completed
replacement only by verifying unchanged original objects and the same
operation succeeding after the held lease is released; no broad error or
skip can stand in for that proof.

### Source review and actual local execution

The three runtime/unit/provider files are frozen in packet
`df6faeeb34a0f6df739b806e4b9a9ed41b3bc3de`. Two independent source
reviews accepted that exact composition; the root review is
`64ad69e2050a34130556a21f0e70d420b8618e62`. The public Python/CLI
composition is the corrected packet above; its first version remains
superseded and unpublished.

After the executor returned, the fresh runtime was materialized in an isolated
checkout of `c2b9c0766b722a283e901ce7bfc8b766d36e4242`. Input Git blob,
SHA-256 and byte-length identities were independently checked before execution
and remained unchanged afterwards. Actual Linux results are:

- full Antigravity adapter: 41 top-level methods, 141 passing test/subtest
  events, no skips or failures;
- real-provider/usecase Antigravity coverage: six top-level methods, 21 passing
  events, no skips or failures, including enabled/disabled/absent results for
  both required resume and repair;
- public CLI: all seven top-level methods and 18 test/subtest events passed,
  with no skips or failures;
- public E2E contracts: all 53 Python methods passed, including the actual
  other-HOME-write rejection before sidecar baseline construction;
- `gofmt -d` was empty and `git diff --check` passed for the reviewed
  composition.

The adapter/provider execution receipt is
`22ccdaf39ca4cf99f2f2ef1f87de3ca57c2fcae7`. Original execution-log
SHA-256 values are adapter
`89594356552906b66306ef6819e9cb3a321ede16ef79faec2ee9bcb61cf2d2df`,
provider
`d9fd103bcb0b2881c2729ef4eb14be8ded45949192c966097044e7a2fcbaaf05`,
CLI
`63093d2b0df124c3fbb2455b0fc1570e869b557f9c4f7a99287113041dfaa572`
and Python contracts
`0459a827aeb0d26eac3cf08628c7914be5790bac400d4cb74b8267381a1679fa`.

### Native evidence and remaining acceptance

Original `8d18cfff1e62ae8eeaf896b5b36dd14a8e98e14a` native captures
from run `43607242252` have now been fully decoded on all three OSes:
outer and inner ZIP integrity, exact 76-member closure, 78 request/receipt
frames and 64 stream joins, original source/tool/owner/DATA identities and
the common deadline all pass. Their two standalone commands observed the
original absent config and enabled selected row without changing the original
44-member baseline; cleanup was complete and all seven qualifications stayed
false. Those captures still contain the old public slot-14 refusal. They are
evidence for the captured client behavior, not execution of this fresh
production implementation.

Whole-repository lint/Core checks, fresh native three-OS public lifecycle and
their original archive closure, final-head Required/security and guarded
merge/postmerge acceptance remain separate gates. Local Linux execution does
not qualify the Windows rename-prevention branch or a real native runtime
tool call. No release or support-level promotion is authorized by this entry.

## 2026-11-14 — final literal-dot formatting and composed-tree verification

This update preserves the preceding fresh v1 and `c2b9c076` execution
records. The later local composition is based on
`5ca208a32ae1deee7da64e502144521ecad6bc9a`, including main
`e3943cee826064911a63ceb2e1e4b2a24a124674`. These local observations
remain distinct from acceptance on the eventual published source.

### Final reviewed source and preserved acquisition semantics

The five code/test blobs in the fresh Antigravity composition are:

| Path within its existing package | Final Git blob |
| --- | --- |
| Antigravity `inventory.go` | `1c4925e5d1dbb7ffa874e6fadaf4f00e2768a435` |
| Antigravity `inventory_test.go` | `3c512a2c1c927a8eeb5e4f0e19b5f140d08263c7` |
| Usecase `lifecycle_unchanged_test.go` | `86f85de33ed839d12812100efc9cf5f5d9b8b7fb` |
| E2E `antigravity-native-cross.py` | `526e8f636d24e1184849bf4be9911dffa457f980` |
| CLI `antigravity_public_resume_test.go` | `1307398b7fad3adfe180e6c77bdad550d03d67b4` |

The initial lint followup, review packet
`cd745a82159f5ad81d63a87a25e7a603f811dac1`, made the returned
`record.open` error explicit and passed the original test context to the
real FIFO-creation command. Its first non-cleaning path spelling used
`strings.Join`; the next actual lint pass rejected that spelling under
`stringConcatSimplify`. The final runtime adds only `fmt` and uses
`os.OpenRoot(fmt.Sprintf("%s%c.", parent, os.PathSeparator))`.
Independent R2 verified that reversing those two final substitutions restores
every byte of reviewed runtime `f173795706adde52b1a4280c22467821348951a5`.
The final runtime is 16,595 bytes, SHA-256
`fdd5dc8a4d962f313f850e4162c6ca6c5c0b02323dd7c722fedda9f9fe5c6765`.

The terminal directory component is intentional. Pinned Go 1.26.11
`os.OpenRoot` passes the original name to the Unix open operation with
`O_CLOEXEC`, without `O_DIRECTORY` or `O_NONBLOCK`; directory validation
occurs after that open. A substituted FIFO at the parent name can therefore
block before the incarnation check. The literal separator and dot force
directory traversal before opening that final component. `filepath.Join`
would clean away this required component. Neither the final format change
nor the lint corrections alter the original parent lease, absent-leaf
classification, present-config/backend rules, deadline or cleanup.

### Actual composed-tree checks and causal regressions

On the local `5ca208a3` composition before the final format-only change,
the full adapter again passed 41 top-level methods and 141 test/subtest
events; the selected real-provider/usecase tests passed six methods and 21
events; the public CLI passed seven methods and 18 events. Each had zero
failures and skips, with unchanged input files. All 53 Python contracts
also passed. Their original log SHA-256 values are:

| Check | Log SHA-256 |
| --- | --- |
| Adapter before final formatting | `c7cd3b5d505faa88e0808c1c79bf74fd13ca4d52326dc72afbacb02759dba57e` |
| Real provider/usecase | `1384d14602a5a5d7954a61a6276225a655134c7b36b6914ed463f982921d5681` |
| Public CLI | `2fdbdb394c61fd0592faa06cb6e98be3f69c6f1133b63c7b97c26b5f6ce9a96b` |
| Public Python contracts | `b8060e00e204cba8c3be70cfd7e9f3ca2c85eb9f2022b7ef9c82362329d6c297` |

Incoming `9cc397c42bd1d4060c5cd2458de3da3138f61d35` subsequently
strengthened the existing missing-config-pair negative oracle with eight
exact anchored refusal reasons. The composition retains those assertions
and the fresh positive assertion on the returned enabled/disabled Boolean.
Independent R2 reversed only those test edits to recover every byte of the
reviewed `f8086484` script; all production functions, HOME preservation,
slot counts and false qualification flags remain unchanged. The final
combined script is 274,342 bytes, SHA-256
`da241e6af1787f164f0415f24d31a67f51ab33224509ebbcf75b52fffba697e6`.
All 53 Python contracts then passed again with no skips in 6.218 seconds.
That original 13,255-byte log has SHA-256
`da32a3a40608689ac17d17f1e0d755362d105b9f6af4b85af5f222060cf2ef74`.
This is updated fixture validation, not another native execution.

The final `1c4925e5` runtime was also executed separately: all 41
adapter methods and 141 events passed, with no failures or skips. The
command completed in 1.279 seconds; its log SHA-256 is
`e59015403b38937beb69ff7626b0e8bc70742a5efe1857e923a455f76f88c52b`.

Two actual Go source-overlay counterfactuals then failed their intended
assertions, rather than failing to compile. Setting only the config
`allowMissing` argument to false rejected all six required resume/verify-only
controls across enabled, disabled and absent native rows. Its log SHA-256 is
`e6a0c423c0f2a6b16e65e704cfa62142c30d6864d98601c69bcb33c3cb3679bc`.
Replacing only the terminal-dot acquisition with `os.OpenRoot(parent)`
(and removing the now-unused import) blocked the actual FIFO child until
the test's original five-second context killed it. That one test failed
with the expected blocked-before-incarnation assertion; its log SHA-256 is
`08960a1c1732f84888bb445352a9bdbe5b449aab50537903efd2fdabbf495e43`.
Both overlay files and the exact original-to-overlay differences were
independently inspected. Repository source bytes stayed unchanged throughout.
The combined final-adapter/counterfactual receipt has SHA-256
`267f57854b65e1a95ad39359c40499c79096492d22b015a032716cdbb7b0627e`.

### Lint completion and the remaining local Core limitation

The ordinary full `make lint` against main `e3943cee` first completed
all four size/architecture passes and the changed-line passes for the root,
CLI and integrationctl modules with zero issues. Its final agentplugins
changed-line pass reported the single `strings.Join` style finding above.
That unsuccessful full invocation is retained with log SHA-256
`8924f1cb5c21c92fdb4dddf49619e6ff2d8bc1d6d4015c1a84c65f0fffcfb53e`.

After the reviewed format correction, the official target was resumed for
the only changed Go module using
`make lint LINT_MODULES=install/integrationctl/agentplugins` and the same
`LINT_BASE=e3943cee826064911a63ceb2e1e4b2a24a124674`.
Both module passes and the global shrink-only baseline check passed with
exit zero. The baseline retained 138 exclusion rules; no suppression,
exemption or gate was weakened. The passing followup log SHA-256 is
`026c5d560882a2e21bfd9c36a7595b2117a4a3184d968b5cf23a53e4f8637221`.
A subsequent complete invocation of the same ordinary `make lint` target,
without a module filter and against the same main commit, then passed all
eight module passes and the global shrink-only baseline with exit zero in
214.793 seconds. The final source hashes remained unchanged across that run,
including runtime `1c4925e5`; the 138 exclusion rules remained unchanged.
Its original 20,093-byte log has SHA-256
`583729b1152e24bbfd4ff95dfc5d902b77dfa0bdb3dac4206e87f5ae5e23289e`;
the execution receipt has SHA-256
`338ca53b65540de33f6ba653e00284a42d61aeb65b85067b14288ecb1b233b57`.
This final full invocation closes the local lint gate for the reviewed Go
composition.

The last full local `make test-core CORE_TEST_PARALLELISM=1` attempt on
the earlier `c2b9c076` composition exited 2. Its log reports unavailable
live `/proc/.../task/.../children` interfaces, denied Unix socket fixtures,
malformed or duplicate ambient Codex environment entries, and cleanup
permission failures. Its original log SHA-256 is
`f00a21ea513bbd5284b3279272e2b2cf45b27c4706f72392fd737ab96dc0b925`.
That full target remains a failed local run; the focused passes above do not
turn it into a pass. No containment, privilege, environment or cleanup guard
was changed to bypass those failures. Required hosted Core verification remains.

### Retained native evidence still precedes the new production behavior

The later retained `c2b9c076` native run
[43611712538](https://github.com/777genius/universal-agent-plugins/actions/runs/43611712538)
also completed its Windows, macOS and Linux native jobs successfully.
Original artifacts 14121667115, 14120988841 and 14122495577 were independently
decoded: both ZIP integrity checks, exact 76-member closure, 78 frames,
64 stream joins, original source/tool/owner/DATA identities and the common
deadline all matched. Their two standalone commands left the original
44-member baseline unchanged, observed the exact six-field enabled row and
kept all seven qualifications false. Public slot 14 in those originals
still records the old expected refusal.

Those retained captures do not execute the new originally-absent production
path. Ordinary hosted validation on the published composition must still
prove the observed Prepared slot-14 result, unchanged original HOME and
owner/DATA state, every retained public/native case, all three OSes and the
original archive/source/stream joins. Final-head Required/security and guarded
merge/postmerge checks remain required. No native tools/call, IDE activation,
release or support-level promotion is inferred from these local results.


## 2026-11-14: source 55bf native closure and missing OS adapter execution

The ordinary source commit `55bf868cb98bb07a9b7ba24dfb9abfa63090818b`
(tree `130c1540bc934a39bd7f2497370a580588de581d`) completed all three native
jobs in [run 43620023537](https://github.com/777genius/universal-agent-plugins/actions/runs/43620023537).
These results belong to that source. The release-only aggregate
`132529270759` was skipped and is not counted as native success.

| Platform | Successful job | Original artifact | Outer ZIP SHA-256 | Inner receipts SHA-256 |
| --- | --- | --- | --- | --- |
| macos | `132523405166` | `14123938906` | `7c0a3c7ff6ed77e254c31be460b3f51f37fc1e22d66b3395fd5d5c2b25800600` | `8be376bf04073c7d9fa8e03f01d8565513630f7015556e5bc3c9d0bfe5814e9d` |
| windows | `132523405366` | `14124505960` | `0f073f153b3af6065e5a7e36f3c2e50e8d04f03a1ebc2491671c2c31a9de7459` | `dab1df8e49cbc2f93f318e788fa05962fb0ff8052598ff483e4853c0f280d7d5` |
| linux | `132523405459` | `14124546800` | `2ef490f35a2759559e66637b9dcc28823ccd8f3bdd059f12dabc8d67de0ffa36` | `52962527dca99c2c8c0bd930675993c6527ec1b5273fd5666aa49476fb2ef213` |

Each downloaded original passed the outer digest/size, both ZIP CRCs, exact
76-member closure, all 78 request/receipt frame and 64 stream joins, exact
source/tree/runtime pins, 14 direct slots, 23 public slots and all capture
completion flags. Public slot 14 now exits zero with `mutated=false`,
`Prepared` and `installation_verified`; its installation, physical artifact,
source/package and retained DATA identities join the original owner. Its
user guidance matches the following independently decoded six-field selected
inventory row. All three actual rows report the Boolean `enabled=true`;
this observation does not infer what a future native row will report.

The two missing-config sidecar commands have their original common deadline,
exit zero, completed cleanup and empty stderr. Both post-command ledgers equal
the original 44-member baseline and the config leaf remains absent. The exact
source harness also completes the full HOME before/after check around public
slot 14 before taking the sidecar baseline. That HOME check is an executed
harness gate; a separate pre-slot-14 HOME snapshot is not an archive member.
The four original refusal slots 16, 18, 20 and 22 still exit one; every other
public slot exits zero, including the retained saved and cold lifecycle cases.
All seven sidecar qualification flags remain false. The native logs report
`public_user_refusals=4` and `public_user_missing_config_observed=true`.
Linux and macOS execute all 53 Python contracts with no skips. Windows executes
51 successfully and retains two existing POSIX chmod-domain skips:
`test_scanner_initial_posix_mode_is_exact_before_hashing` and
`test_scanner_same_inode_mode_race_refuses_before_read`.
IDE activation, live MCP execution,
vendor authentication, model inference and release qualification remain
unobserved by this run.

The offline audit receipts and decoded job logs are retained under
`/tmp/antig-55-originals`; their hashes are:

| Platform | Audit JSON SHA-256 | Native job log SHA-256 |
| --- | --- | --- |
| macos | `9753d2f74576a8382febb72aa31898c2c69413fd1c4a1800f56953a5a437676b` | `ad3a282f065394ff076c4d5fc4b658a742c9e87ebcc1e7c5fece91f2f962b779` |
| windows | `3c2db298d4a3ff9e8eaafdf7c845c8f559db2565a68f1a1e349753f22103d5a9` | `e7d4c5bcde8b99a05fd577412f9bea1065d9b5a50dc1f6f1f13aad52da880d61` |
| linux | `80e95f0d446a8b15567ad1493cc53c9aa9b84ba07e8cd4a5affd62dc8da28fde` | `9e88dc8ad0aa853f6d1ceb3de1ba3fcb5d280dfcd1c570325025588967f47d3b` |

### Required Go coverage correction in the existing native steps

Review found that the source-55 native jobs execute the Go admission bridge
package and the 53 Python contracts, but do not execute
`clients/antigravity/inventory_test.go`. Core fast's existing cross-platform
`project-contracts` command includes `clients/...`, but its
`-run 'Test(TargetContext|Project|...)'` selection excludes `TestInventory*`.
The full adapter package's already recorded local 41 top-level / 141 event
execution is Linux evidence. It cannot establish the Windows-specific
held-parent rename-prevention controls or native macOS adapter-test execution.

Root and independent review approved exactly two lines before the existing
bridge test in each of the three existing source-only PowerShell steps:

```powershell
go -C install/integrationctl/agentplugins test -count=1 -timeout=3m -v ./clients/antigravity
if ($LASTEXITCODE -ne 0) { throw 'Antigravity adapter contracts failed' }
```

The existing exact-head checkout, pinned Go 1.26.11, source-only condition,
release branches, jobs, triggers and bridge/native guards remain unchanged.
The command executes the full adapter package, including
`TestInventoryMissingConfigParentLeaseRefusesReplacementBeforeNative` and
`TestInventoryMissingConfigParentRaceRetainsOriginalAuthority`. Their existing
Windows branch accepts only the actual attempted rename's sharing error 32,
unchanged original authority/bytes and absent destination, followed by the
same rename succeeding after lease release. This proves prevention separately
from a completed replacement; it does not skip either parent test. The
POSIX-only FIFO fixture retains its explicit Windows platform skip.

This two-line insertion at three existing call sites changes no Go or Python
implementation or test source. The new command's native Windows/macOS results,
and the final composed commit's ordinary native jobs, remain pending. The
source-55 originals above do not substitute for those later results.
