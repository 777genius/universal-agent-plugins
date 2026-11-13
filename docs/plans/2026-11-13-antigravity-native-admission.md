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
