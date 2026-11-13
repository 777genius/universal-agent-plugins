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
refinement, intact repair and cold observation preserve full owned bundle
metadata and receipts; actual replacement compares original contents/modes
while retaining binding and data authority. Historical authoring changes only
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
