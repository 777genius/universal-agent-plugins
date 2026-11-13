# Hermes explicit custom-runtime-root MCP: native admission only

## Windows temporal directory refusal: bounded field diagnostics

Original [run 43429238373, job 131898258264](https://github.com/777genius/universal-agent-plugins/actions/runs/43429238373/job/131898258264), source
ee561399d01484dcdd04fce3a504b81839882b7b used actual vendor-managed CPython 3.14.9
from the source installation on Windows Server 2025. Its inert contract stopped earlier
with `inventory directory changed`. The log does not identify the differing
field or the path/handle domain; it is not evidence for dropping any field.

Root R1 approves this diagnostic in the existing admission script and its tests,
with at most 50 changed runtime/test lines. Preserve canonical-path admission
and both complete `stable_file` before/after comparisons. On refusal, report
only fixed `path:st_*` and `handle:st_*` field names from the original metadata
objects; a canonical-path failure reports only `canonical`. Read each current
stat once and do not reread the filesystem to prepare diagnostics. Include the
existing Windows birth-time, attributes and reparse-tag fields where applicable.

The inert contract covers every applicable field in both domains, unchanged
records, canonical refusal and single snapshot reads. It does not override the
host platform or claim Windows execution. The original membership-bound test,
limits, guards, source/profile ownership and qualifications remain unchanged.
A genuine Windows run is still required to identify the actual differing field
before considering any corrective change to directory semantics.

All 39 current inert contracts pass locally on Linux, including the unchanged
membership-bound contract. This is no evidence of a Windows native pass.

Root's scheduling amendment also removes only `needs: native-call` from the
existing workflow's `bundled-custom-root` job. Each job owns a separate hosted
runner, clone, runtime and evidence roots; the bundled job consumes no source-job
outputs or artifacts. This allows independent bundle observations while a source
lane fails. Source failures still fail the workflow; no guard or step is skipped.

## R1 approved correction: separate official bundled runtime

Root approved this plan on 2026-11-13 after reviewing the original `aabcd9b3`
archives. Linux measured 37806 entries and 1711333608 logical bytes; macOS
measured 34823 entries and 1024567265 bytes. Both source-installed runtimes
populate the fresh selected home before the configuration inventory starts.
Their symlinks and large executable files are runtime payload, not admissible
configuration. Windows still refused a directory identity observation. None of
these failures qualifies native custom-root behavior, and no inventory cap is
relaxed to accept them.

Keep the already-proved source-installed User tools/call lane unchanged. After
that lane, build a separate official standalone payload with
`scripts/bundles/native_build.py --source <pinned checkout> --work <owned work>
--cache <owned cache> --out <owned payload> --commit
2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`. Do not patch vendor files, set lazy
install suppression flags, or substitute an ambient interpreter. The builder's
official prepared-input validation must succeed; a build failure stays a failure.
Use the resulting `bin/hermes[.exe]` directly for native version and MCP commands.
Source inspection corrected the first proposal: bundled launchers do not expose
the source-install-only `--print-runtime-command` option. Their actual entry is
`scripts/build/launchers.py` (POSIX) or `launcher_wrapper.py` (Windows), bound to
the manifest's contained Python, source and dependency paths. Observer/guard code
uses that same payload's public `runtime_command` API with its explicit Python.
The native `--version` observation runs inside the selected-root command ledger.

The git-less payload lives outside all disposable User/A/B/configuration roots.
Bind its manifest ref, target and contained runtime paths to the exact checkout,
prepared revision/source/lock, and original manifest/prepared-file byte digests.
Retain the original vendor Git identity separately. Hash every ordinary payload
file with streaming reads, include link text without traversing it, and reject
links that escape the payload. Admit no external link target. A separate finite
runtime budget permits at most 262144 entries, depth 64 and 8 GiB aggregate
regular bytes; scans have a 120-second deadline. This exceeds the measured
1.7 GiB source runtime without making the configuration ledger unbounded.
Keep a held payload root identity through the native family. Compare full bytes
and metadata snapshots before execution and after all child processes settle;
do not reread gigabytes before each small config command. This is immutable
test-owned runtime provenance, not a sandbox for adversarial executables.

The configuration inventory retains its 2048-entry/depth24, 1 MiB/member and
16 MiB aggregate limits, all User sentinels, sibling/root identity checks and
unknown-effect refusals. Retain only small provenance/digest receipts, never raw
payload files. Evidence states `runtime_distribution=bundled` and
`source_install_custom_root_qualified=false`; bundle observations do not qualify
source-installed custom roots, UAP Project support, vendor auth or model inference.

Owned edit allowlist: this plan, `scripts/e2e/hermes-profile-admission.py`, its
existing inert tests, and `.github/workflows/hermes-noauth-tool-call.yml`. Root
also approved the narrow `installation=None` keyword in the existing
`hermes-noauth-tool-call.py` environment helper: the default source-install
allowlist stays identical; explicit bundled calls accept only four real runtime
output paths. Contracts must reject arbitrary environment forwarding.

The standalone build has its own dependent matrix job with a 180-minute limit,
matching the pinned vendor `pm-bundle.yml` allowance for native wheels. Its
builder subprocess has a 150-minute deadline. The source-installed User job
retains 45 minutes. Record build duration/exit separately from the unchanged
720-second native family and 120-second payload scans. Eight GiB is an admission
ceiling, not a measured bundle size. Official build timeout/failure remains an
unqualified failure; no reduced-extras builder or automatic retry is authorized.

Estimate: 180–240 runtime lines, 130 focused test lines and 60 workflow
lines; complexity 6/10, reliability target 8/10, confidence 7/10 before hosted
execution. Tests cover manifest/ref/launcher-path mismatches, large immutable runtime
files, changed bytes/link/root, escaping links and finite budgets while retaining
ordinary configuration refusals. Independent review and final-source original
artifacts on Linux, macOS and Windows are required before any acceptance.

Local verification of this correction: all 38 inert custom-root contracts and
all seven existing provider/wire contracts passed on Linux. Python compilation,
workflow YAML parsing and diff whitespace checks passed. The tests execute no
Hermes vendor process and grant no native/platform qualification. The official
bundle build and final-source cross-platform original evidence remain unexecuted.
Independent R2 source review accepted this five-file correction with no blocking
findings; it verified the pinned official producer, runtime selection, full
payload snapshots, process settlement and unchanged configuration guards. This
review authorizes integration only, not native acceptance or production support.

## Current bounded-inventory refusal and diagnostic correction

Original Windows artifact14020477175 from run43386222522 on
`76f2a8a35bcffeb330409353f80bf57e94a77759` passes its ZIP CRC and all nine
stream byte/hash joins. The earlier cross-API mode mismatch is gone. Native
custom-root capture now refuses the initial whole-tree inventory because a file
does not meet the existing ordinary/single-link/1MiB bound, matching the prior
macOS refusal. Neither original includes the offending member, link count or
size. The dependency preparation messages alone do not identify that member.
Source/runtime hashes stay unchanged; capture and qualification remain false.

The official pinned `hermes_bootstrap.py` invokes
`hermes_cli.venv_sync.prepare_launch`; `pm.environments.dependency_home_root`
scopes dependency state per data root. These sources explain why fresh-root
bootstrap effects must be measured, not silently excluded or rebaselined:
https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/venv_sync.py
https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/pm/environments.py

The narrow diagnostic correction keeps every inventory bound and refuses before
reading an inadmissible member. It records only fixture-relative member spelling
(JSON-escaped and limited to512 characters), the member-name SHA256, mode, link count and
size in the bounded existing failure stream. Longer names retain only their
hash. No file contents, absolute paths or native profile trees are published.
The existing original custom-root identity receipt is written before the
whole-tree baseline; a later inventory failure can therefore complete ordinary
identity-checked cleanup of those exact created roots. This does not grant
cleanup authority for a replaced root or prove a baseline/capture succeeded.

Only admission source, focused inert contracts and this plan change. No vendor,
environment, workflow, dependency, production capability or qualification change.
Original final-source hosted evidence is still required before choosing any
bootstrap treatment. Oversize/hardlink/symlink contracts assert refusal before
content read, bounded metadata only, and existing root replacement refusals stay.

Baseline: PR878 `075c8e7fad31a8d6f5b024c4b683df7ce5cb0d1e`.
Keep official Hermes0.23.0 commit
`2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`; no pin/production change.
Root authorizes this amended admission plan before code resumes. The existing
UAP Project MCP refusal remains in force until separate native admission and
production design approval. Implementation/review workers do not execute native
vendors locally or write remote refs; root owns publication after source review.

## Rejected named-create route

Blank native `profile create --no-alias --no-skills` calls
`profiles.py::_bootstrap_profile_dir -> _seed_model_config` and
`profile_model_seed.py::launch_model_seed`. Nonempty original User model and its
selected provider/endpoint settings are copied. `--no-skills` does not suppress
model seeding, memory hooks, multiplexer notifications or service registration.
This violates the no-copied-model constraint. Named-create is NO-GO; do not remove
original settings or sanitize vendor-created profiles to manufacture isolation.
Its draft code is superseded, not a second candidate or publishable implementation.

## Exact supported replacement

All upstream paths refer to the immutable Hermes commit above:
https://github.com/NousResearch/hermes-agent/tree/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19 .

- `website/docs/reference/faq.md:664` explicitly supports setting
  `HERMES_HOME=/some/path` before each command; named profiles add management.
- `website/docs/reference/environment-variables.md:121`: the environment variable
  selects configuration and user-data home. OS HOME is not the profile selector.
- `hermes_constants.py::_hermes_root_of:99,get_default_hermes_root:302`: an
  absolute custom root OUTSIDE the original native Hermes root, whose immediate
  parent is NOT `profiles`, is its own default root.
- `hermes_cli/auth.py::_global_auth_file_path:492`: selected home equals that
  custom default root, so original User auth fallback is None.
- `hermes_cli/main.py::_apply_profile_override:595`: no `-p` or custom-root
  `active_profile` redirect leaves that root selected. No profile-create/modelseed
  or create/delete hooks are called by this route.
- `hermes_bootstrap.py::_pin_launcher_home:547` and official `_launchers.py`:
  an explicit HERMES_HOME is retained; default pinning occurs only when absent.
- `hermes_cli/config.py::get_config_path`, `mcp_config.py::_get_mcp_servers`,
  `website/docs/developer-guide/session-storage.md:16`: config/MCP and state.db
  follow selected get_hermes_home, not a cwd-discovered project config.

Do not claim all authentication is isolated: provider integrations can consult
other OS-home/ambient credential stores. Admission uses only native MCP list/test
with the existing closed credential-free child environment and no auth/model turn.

## Finite native capture

Reuse existing pinned acquisition/runtime resolution in
`hermes-noauth-tool-call.yml`, unchanged helper imports from
`hermes-noauth-tool-call.py`, existing bounded streams/start gate/POSIX process
ownership/Windows job and PiHTTPMCPProbe. Revise the same small admission sibling,
not a parallel framework. Root owns workflow integration/publication on #878.

1. Keep the parent process environment unchanged. Establish ONE original
   disposable OS User home/profile outside A/B runtime roots. In every child,
   HOME/USERPROFILE remain that exact original User; ONLY the explicit HERMES_HOME
   selects A or B. Native commands contain no `-p`, `profile create/use/delete`,
   auth/model flags, clone or aliases. No HOME-as-project substitution.
2. Before baseline, author inert original User config with unique model/provider
   sentinel, inert auth/history markers and foreign Skill/MCP. Record the actual
   native default Hermes root using the pinned platform resolver under the same
   stable OS home; do not assume POSIX layout on Windows. Freeze complete bounded
   ordinary inventories and identity for original User and project controls.
3. Author separate fresh canonical task-owned runtime roots A/B outside that native
   root, with no immediate parent named profiles, no active_profile or aliases,
   and no existing foreign identity. Each gets ONLY caller-authored minimal MCP
   config (no User config/model/auth/history copied). Freeze exact authored
   source/config receipts; original model/provider sentinel must remain absent
   from every A/B native-authored config/data record after each command.
4. Preflight actual pinned home/default-root/auth-fallback resolutions in each
   explicitly selected child; require selected==custom-default==A/B and fallback
   None, original OS home unchanged. Refuse local/default gateway/multiplexer/s6
   authority signals or external memory provider in the selected root; no
   speculative process scan or private override. Unknown signals fail closed.
5. Use fresh per-root loopback PiHTTPMCPProbe endpoints plus nonce/source identity.
   Native `hermes mcp list`, `mcp test NAME` from A/B project cwd must expose only
   the selected owned MCP. Join native output/selected URL/config/source receipts
   with actual selected initialize/tools-list method ledger and zero sibling
   events. Repeat A through a fresh native process; explicit original User list
   must preserve its foreign config and never contact either project endpoint.
   These ledgers are NOT full protocol wire transcripts or tools/call proof.
6. Native `mcp remove NAME` affects only that explicit custom root. Verify A absent
   while B and original User bytes stay exact, repeat removal/absence as observed,
   then B equivalent. No shared-profile create/delete/registry retirement is used.
   Retire only task-owned outer disposable roots after original identity and
   process cleanup checks; do not rewrite original User/shared metadata.
7. Capture each command's before/after ordinary metadata/hash inventory, including
   failed-command effects. Protected User config/auth/model/history/foreign bytes
   remain exact; A/B contain no copied sentinels. Unknown shared/out-of-scope
   effects retain FAILED unqualified evidence; never renew a whole baseline.

## Evidence, criticisms and review

Stage only a finite ordinary single-link evidence member set <=1MiB/member and
<=16MiB aggregate, after native process/job cleanup, into `evidence-*/upload/`.
Raw profiles/auth/config trees are never uploaded. On early failure emit only a
minimal safe failure report; preserve original failure and all qualification flags
false. Record source SHA/tree/clean, official pin/runtime hashes, actual host/run,
closed stdout/stderr/status, config digests/selected URLs, inventories/deltas and
cleanup. Source/runtime changes refuse. No vendor/source patch or pin relaxation.

Criticism1: explicit root selection is not all-auth isolation. Keep noauth metadata
and initialize/list claims separate; original User auth/model/history and model
sentinel checks remain required even if native mcp test exits successfully.
Criticism2: runtime/bootstrap may write unexpected shared state despite source
resolution. Measure native deltas before ownership-safe cleanup; first capture is
unqualified pending independent raw review, and unknown effects fail rather than
becoming broad exceptions. A passing fixture does not admit plain cwd discovery.

Independent critic reviews this amendment, then root GO before code resumes.
Independently review corrected implementation and final-source genuine hosted raw
capture before any production design. Reuse existing Ubuntu24/macOS15/Windows2025
lane, no duplicate dispatch while current source native work is in flight.

Estimate: revise the existing ~350-line draft down to a bounded custom-root recipe
plus ~100 inert test lines; no additional harness/helper refactor. Complexity5/10;
feasibility7/10 pending actual native root-resolution and original-User evidence.
No support/release/auth/model/tools-call qualification or production change.

## Source-review checkpoint before hosted capture

Root and an independent critic accepted the custom-root plan and corrected
implementation. Review findings required exact authored MCP mapping/config
hashes at every command boundary, retained A/B directory identities, one bounded
ordinary-file reader with held-descriptor checks and no atime comparison, and
private staging followed by atomic evidence publication. The reader uses
O_NOFOLLOW and O_NONBLOCK where supported; a real inert FIFO-swap test verifies
that a changed special object cannot block before descriptor validation.

Final local admission contracts: 14 passed, with zero skips on Linux. The seven
existing Hermes provider/wire contracts also passed. These are inert source
checks only. Genuine hosted execution on each platform and independent original
artifact review remain required. No product or native qualification is inferred.

## Observed Windows inert gate correction

Run43365562797, Windowsjob131689022830 on28633523739cf5f84fc772a39bee6b8990d62987,
failed before vendor installation/native capture: ambient Python lacked PyYAML,
and four ordinary-file contracts refused lstat/fstat equality. The log records no
stat-field telemetry, so the exact differing field is not retrospectively claimed.
Reuse native_client_project_artifacts.open_member/same_opened/identity: Windows
uses a noninheritable OPEN_REPARSE_POINT handle and positive birth/device/inode/
mode/link/size/mtime/attribute cross-check; full ctime remains checked separately
path-to-path and descriptor-to-descriptor. Ownership, alias, bound and link guards
remain. CPython documents evolving Windows ctime semantics and birthtime availability:
https://docs.python.org/3.12/library/os.html#os.stat_result .

Move the inert custom-root contracts after official pinned installation/runtime
resolution, invoking that same official dependency-prepared runtime via its public
runtime_command bootstrap. No ambient pip, new dependency or skipped test is used.
The assumed PyYAML dependency was disproved by the next native runner as recorded
below. Metadata-domain simulation checks valid differing
path/fd ctimes while refusing either within-domain ctime drift, inode/birth changes
and reparse metadata. Only genuine fresh hosted results establish portability.

## Observed official parser correction

Run43368264088, macOSjob131697732241 on3016e7e7e5b2869d01b764131ea5eb221b097df2,
completed official installation and runtime resolution, then passed14 inert
contracts and failed the configuration contract with ModuleNotFoundError: yaml.
No native User or custom-root stage ran. The pinned runtime_command already
activates its committed dependencies through hermes_bootstrap; another activation
or a pip install is not the correction. Pinned hermes_cli/config.py instead uses
the official hermes_yaml adapter over ruamel.yaml. Both admission parser imports
now use that same adapter for safe_load/safe_dump, retaining its YAML1.1 and
duplicate-key policy:
https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_yaml.py .

The small config_document_boundary function checks the parsed mapping and exact
captured-byte digest independently. Inert tests exercise wrong URL, extra server,
model/auth fields and equivalent mappings with changed bytes without importing or
substituting a vendor parser. The hosted child alone reads bounded held files and
parses/emits their contents using the pinned official module; native Hermes then
reads those same files. File authority, root isolation, process containment and
every qualification flag remain unchanged. Fresh hosted originals remain required.


## Measured first-bootstrap state and bounded census proposal

Exact source `38472f31efb038250cce55c9eb2185fb203fa9c2` now retains actual
first-baseline refusal metadata on all three platforms. Original archives have
clean CRCs, eleven closed members and fourteen verified stream/hash joins each;
source/runtime identities remain stable and owned cleanup completes. Native
custom-root admission and every Project qualification remain false.

| Platform | Original artifact / SHA256 | First refused native member |
| --- | --- | --- |
| Windows | 14021734988 / `48165e36f7d2c143c4e1c8300d1b5050a152d4cea8e4bd2cc63894d114c4f16a` | uv cache requests/adapters.py, ordinary, 28064 bytes, two links |
| macOS | 14022647010 / `1bd2d1c2f5a56033dbb4e56bcdb63bcb152fa7aea09537005f590cfed2f72647` | tools/ripgrep-15.2.0-darwin-arm64/rg, ordinary executable, 4014576 bytes, one link |
| Linux | 14022099319 / `c068b6fb95f4c5314b0e0523f08f543755ef532c1db428d410c4308d633d0cda` | uv editable-package cache symlink, 39-byte link metadata, one link |

These names are fixture-relative metadata, not hashes or readings of member
contents. The symlink target was not followed or published. The first offending
member is not a measurement of the complete runtime layout or aggregate size.
The official pinned borrowed-home launcher deliberately creates an independent
dependency generation for each data root. Reusing User state, disabling ordinary
bootstrap, omitting runtime trees or renewing the original baseline would not
establish the required native route.

Separate correction under independent review: preserve the external commit's
lazy bounded enumeration, but bind all POSIX recursive stat/open/read operations
to retained parent descriptors. Windows retains positively joined directory
handles with no reparse traversal and no delete sharing. Plain directory
replacement, not only symlink substitution, must fail without reading foreign
bytes. This correction does not accept runtime links or increase existing bounds.

Proposed next evidence step, pending independent design approval: on the original
initial-inventory refusal only, collect a finite metadata-only census under the
already-owned disposable outer root. Retain original failure and every false
qualification. Never open file contents or follow link/reparse targets. Report
only closed kind counts, logical byte totals, maximum file size/link count and
completion/refusal facts in fixed bootstrap cache/tools/installs/other groups,
plus fixed User/A/B/outer control groups. No raw pathname list, target spelling,
profile contents, new cleanup authority or additional native command is allowed.

The census has an independent five-second budget inside the original lifecycle
clock, at most32768 visits and depth32, with a <=16KiB single closed evidence
member. Bounded partial or changed-root observations explicitly remain incomplete.
It uses held no-follow directory authority, descriptor-relative Unix metadata,
and held Windows directory identities; every traversal rechecks its authority.
It must not mask the original refusal if census collection or publication fails.
Actual final-source hosted originals and a separate reviewed inventory design
remain mandatory before accepting any measured runtime class or new resource cap.


The two independent plan reviews are accepted; root authorizes this diagnostic
only, with the following final fences. The child writes a fresh closed request
only when the original initial inventory raises. A missing or failed request
never causes a fallback scan. The parent handles that request only after actual
whole owned process/job/probe settlement, before existing owned-root cleanup.
Unknown settlement or original root identity skips/refuses the census and cannot
grant deletion authority. A request-publication or census failure preserves the
original inventory refusal and false qualification flags.

Groups are disjoint fixed relative-prefix buckets. Every recorded entry belongs
to exactly one group and kind; kind counts join to group entry counts and group
counts join to the total. Regular logical bytes count each pathname, including
hardlinked names, not unique physical storage. All counters/totals/maxima saturate
at a fixed exactly representable integer cap with an explicit overflow flag;
partial, overflow and changed-authority results cannot be complete. Symlink and
Windows reparse leaves are counted from no-follow metadata only; no resolve,
readlink or content open is performed on them.

The five seconds are explicitly a soft budget checked between metadata operations
inside the existing lifecycle deadline and supervisor, not a new OS-hard deadline
or process framework. The small closed request and the census are the only two
additional allowed evidence members, each <=16KiB. The census carries an explicit
measurement-only designation and never authorizes native/product admission.
Implementation may add up to340 changed code/test lines beyond the separately
reviewed199-line directory authority correction, solely for this finite request,
post-settlement census and meaningful negative contracts. No acceptance-bound,
production adapter, dependency, native-command or workflow change is authorized.

The final source review found and fixed a publication gap: setter bounds alone did
not stop a malformed or oversized request from reaching the old evidence copier.
Both new members now receive exact16KiB/typed-schema admission before publication
and again against the actual staged bytes. Invalid request bytes withhold the raw
upload; they are not overwritten or deleted. The fixed fallback retains the
original failure class. Original evidence directory identity is independently
held before request reads and writes. Per-group regular_hardlinked_entries now
measures the relevant regular-file extent separately from directory link counts.
The measured correction totals525 changed code/test lines, including199 directory
authority lines and326 census lines. Root and independent source review accepted
it;28 inert contracts pass. No native acceptance is inferred before new originals.

## Initial Windows launcher diagnostic after d7

Exact d7 job131802763698 failed at the initial official --version probe before
runtime resolution and both E2Es; neither artifact existed. Its former guard
combined timeout, nonzero exit and empty output without retaining those facts.
The pinned official scripts/desktop-update/runtime.ps1 returns Code, Output and
TimedOut and applies its existing60s bound. Log only those same guard facts as
a fixed JSON record: literal timeout boolean, integer exit and output-present
boolean. Preserve the original refusal, timeout, launcher and helper invocation;
no raw output, path, retry, alternate runtime or cleanup claim is added. Main's
8c3cfa28e later release-launcher settling preflight does not fix this earlier
guard. Actual next hosted facts remain required before identifying its cause.

## Portable census contract after d7 macOS

Original macOS job131802764178 stopped in the inert census contract before
either native capture: its projectA max_links was4 rather than the hardcoded2.
The census deliberately measures all entry kinds, including the directory;
directory link counts are platform-dependent. Compute the expected maximum
from the three authored fixture entries' actual nofollow metadata, while
retaining the independent exact two-regular-hardlinks and logical-byte checks.
No runtime bound, census semantics or admission changes; macOS native evidence
remains pending after this test-only correction.

## Actual 32ee Windows inert contracts

Original Windows job131819394095 passed the unchanged official version probe:
timed_out=false, exit_code=0 and output_present=true. It then refused two inert
contracts before either native capture, so no artifact exists for this platform.
The overflow injection only patched the POSIX dir_fd stat path; Windows uses
Path.lstat and received ordinary fixture metadata instead. Inject the same
authored oversized regular-file metadata through that actual Windows path too.
The real before-open directory replacement correctly raises the existing Windows
reparse-identity ValueError; include that documented refusal class in the test.
No native validator, directory identity, link refusal, runtime limit or timeout
changes. Root plus independent source review and the inert suite precede the
next ordinary hosted lane; a passing unit test is not native admission.

## Bounded 32ee census follow-up

Actual macOS and Linux 32ee census receipts were partial at 32768 entries. The
old combined budget reason checked time first, so these receipts do not prove
which bound stopped traversal. Increase only the metadata census member cap to
131072; keep the original five-second soft budget, depth 32, 16 KiB closed
receipts, no content or link-target reads, settled-child/root authority and
false admission qualifications. This changes no ordinary-file admission limit.

Use distinct closed partial reasons `time-limit`, `member-limit`, `depth-limit`
and `overflow`. A member-limit receipt must join exactly 131072 recorded entries;
all kind/group counters use that same bound. Exhaustive completion requires full
iterator exhaustion, unchanged original root, a final time check and no counter
overflow. An exhausted clock or any refused identity never qualifies completion.
The next genuine run may still be partial; no layout completeness is promised.

Integration note: remote source 23b49ff891513fc601e1c44c8d55f5da0d58ef8d
already contains Windows inert repairs. Preserve their Path.lstat-based Windows
injection while correcting its POSIX fallback: Path.resolve passes a Path operand
and follow_symlinks to os.stat, which cannot be forwarded to Path.lstat. The root
29-test integration run reproduced that TypeError. The fallback now follows the
actual patched API, and the Windows injection is scoped to the exact fixture file.
This follow-up also adds the reviewed census bound, distinct exhaustion reasons
and their contracts on top of that source.

## Closed census stdout readout

Emit one HERMES_BOOTSTRAP_CENSUS measurement statement only after the existing
source/root/membership checks, atomic upload rename and EVIDENCE_CLOSED=true.
Reuse the parsed object returned by the existing census validator and the same
bounded staging read; never cache or print the request nonce or root identity.
The separate readout protocol carries fixed measurement/unqualified flags,
present=true plus the closed census, or present=false without invented totals.

Do not parse lifecycle.stderr or infer a cause from RuntimeError/traceback text.
No native command, filesystem reread, artifact member, budget or acceptance
bound changes. Optional stdout OSError/ValueError cannot delete committed
evidence or replace the original native failure. Successful staging, privacy,
missing report, invalid schema, atomic failure and closed-stdout contracts are
added. These post-outage changes and new contracts are UNEXECUTED at authoring;
current-source hosted CI and original evidence remain required. This statement
is not product/native qualification, and partial census counts remain lower bounds.


### Directory identity across Windows stat APIs

The external `aabcd9b3e763d4341884173d37cdd5b1f7d7caf8` correction is preserved.
A directory's positive device/inode/birth identity, links, bytes and mtime join the
path and held-handle observations. Both records must still be ordinary directories
without reparse attributes or tags. The held handle denies directory deletion;
complete path-domain and descriptor-domain snapshots, including all attribute bits,
are independently rechecked before and after enumeration. Cross-API attribute
equality is not used as object identity. The original fc8 receipt proved a field
mismatch but did not capture the differing bits; synthetic masks in contracts are
not an observation of the hosted values. This does not qualify custom-root native
loading or relax temporal metadata checks within either authority domain.

## Complete regular hardlink closure before bundled payload bytes

Follow-up to official standalone-builder route9c8bfca0 on current672a769c.
Independent inert review showed one payload regular name hardlinked to a sibling
outside payload was previously hashed and accepted. Captured symlink closure did
not close regular inode aliases. Preserve the official builder, canonical bundled
launcher, separate runtime domain and source_install_custom_root_qualified=false;
do not replace this route with the separate developer-runtime proposal.

The first anchored bounded walk now captures metadata ONLY for regular files,
including per-name logical size; no regular bytes are read. Resolve the existing
captured symlink graph, then group all regular names by device/file ID. Require
identical full captured metadata and exact group cardinality == original nlink.
Any outside alias or inconsistent successive observations refuses before hashing.
A second anchored walk joins each original path/ancestor/held-file identity and
exact member set before streaming contents. A late link-count/mtime/identity
change refuses before that file's first os.read; changes while hashing still use
existing held/linked before/after checks. No renewed deadline, baseline, ownership
or native trust is introduced. Existing262144-member/depth64/8GiB bounds remain
exact, with logical bytes charged once per NAME in the metadata pass.

Five meaningful inert contracts cover two internal aliases, outside-alias no-read,
late outside alias before first hash, same-inode metadata divergence, and original
byte/deadline bounds before content. All44 existing/new contracts pass. Production
change78 additions/19 removals and65 test additions remain inside reviewed scope;
no vendor/native execution or workflow/qualification change occurred. Actual
final-source hosted builder/capture and original-artifact review remain required.

## Concurrent closure and Windows reconciliation

Preserve a84a20e7's metadata-only first walk and complete hardlink closure before
any payload content read. That supersedes the earlier unpublished post-hash
closure proposal; there is one alias authority and no duplicated scanner.
The grouping walk checks the original shared deadline and requires positive
integer device, inode and link identities before accepting a complete group.
No deadline or baseline is renewed.

Extended Windows device paths may resolve only to an absolute local drive under
the captured runtime. UNC, GLOBALROOT and drive-relative spellings cannot be
reinterpreted as ordinary relative members. The graph is never OS-followed.
A separate Windows diagnostic contract proves each API's original metadata stays
stable even when path and held-handle domains legitimately differ from each other.
The external fixed-field diagnostic remains unchanged; raw paths/values stay out.
Fresh hosted builder and native captures remain required.
