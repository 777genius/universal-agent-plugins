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

## Bounded metadata for an external payload hardlink refusal

The b79 Linux [job 131933748437](https://github.com/777genius/universal-agent-plugins/actions/runs/43439520227/job/131933748437) finished the official builder, then refused before native start or regular payload bytes. Its exact offending alias is unknown; the external build-cache explanation remains a hypothesis.
The same closure refusal now retains one metadata-only record through existing cleanup and atomic publication: canonical payload-relative member (ASCII-escaped spelling ≤128 bytes, otherwise null), SHA256 with explicit UTF-8 surrogatepass encoding, observed device/file ID, links, captured alias count and UID/GID.
The record is ≤1 KiB; non-uint64 integers become null with a saturation flag. Windows zero UID/GID are observations, not ownership proof. Invalid diagnostic input cannot replace the original refusal; no outside traversal, new file reads, materialization, native action or qualification change is added.
Diagnostic inputs must be the captured 9/12-field identity tuple and a nonempty list of at most PAYLOAD_MEMBERS string names; malformed containers retain the original refusal without reads or diagnostic authority.

Focused contracts cover actual outside hardlink no-read, retained internal-pair success, long/non-ASCII/surrogate names, malformed containers, oversized and wrongly typed link counts, saturation and existing publisher retention. Local execution is unavailable: this candidate has static review only; exact-source hosted contracts and native evidence remain required.

## Fixed historical original archive diagnosis

Add one read-only job and a Hermes-only reader for exactly two original artifacts:
macOS artifact14046328482/run43439520227 at b79c2ac0b0bf2021b3697bb691108df24438c01d
(9945 bytes, SHA256 ed0d57c5bea2e29991d7ff59d752d62209ce7842088bad2291d880a22f3af877),
and Windows artifact14052063757/run43449616323 at
0a1ecf16d41708b5c3cfc4add1e3cf8f0c6c96d8
(1077 bytes, SHA256 1be40362fd5f15c4e71519e2d73eb0dcbc102a92add9453a44496a9e59c01c55).
Use the ordinary official gh API artifact download with actions:read/contents:read;
no local executor, native binary, bootstrap, new action or trust operation.

Before ZipFile allocation check exact original bytes/hash and fixed bounded EOCD
count/central-directory limits. Read only frozen flat historical MEMBERS,
bounded each/aggregate, regular metadata, full CRC, unique membership; never extract.
Require original source/tree/run/attempt/pin, unchanged before/after identities,
all false qualification fields, completed cleanup and every proof.streams
length/hash/member join. Original bytes remain unchanged. Project only fixed
failure class/guard index, chronological known command label where present,
exact harness basename/line frames, counts/hashes and the closed Windows
hardlink metadata record (no relative member text). No raw exception, profile,
nonce, code, path, config or stream contents are printed. Unknown decode emits
one fixed diagnostic refusal, never native qualification.

Four paths: scripts/hermes_original_archive_readout.py, its inert unittest,
the existing Hermes workflow and this plan. Reader187 lines, tests102, workflow27;
source-only tests are unexecuted until genuine CI. Three review passes: author
source/closure review, independent critic, then root exact immutable diff review.
This diagnostic measures historical failures and does not change inventory
admission, pins, process limits, runtime qualification or production behavior.

### Review corrections for temporary original reader

The job is restricted to pull_request number878, with two fixed matrix cases
and fail-fast:false; expiry cannot create a permanent main failure. A refused
diagnostic prints its fixed JSON and exits1. Before reading ZipExtFile, validate
local headers/names/size/CRC/data descriptors and exact contiguous central-bound
ranges, then reuse read-authoring-evidence-zip.check_expansion for actual deflate
EOF, full expanded bytes/CRC and trailing-data refusal. Catch struct/zlib decode
errors as fixed refusal. Accepted readout includes fixed artifactID, original
size/SHA, member count, verified CRC and stream-join count. Runtime presence,
before/after equality and second runtime-error presence are separate booleans,
so a second failure is retained without leaking runtime contents. Preserve the
already approved closed Windows canonical bounded relative member alongside
its digest/numeric metadata; it is not a raw profile or stderr path.
Reader223 lines, tests147; workflow34-line job. Added forged declared expansion,
trailing compressed data, runtime-second-failure and refused-exit contracts.
All new contracts remain unexecuted.

## Deterministic midstream mutation contract after Windows 850

Original source `850a4bea44f1321ded9be8e1e1cec7a878a35583`,
[run 43461070487, job 132013249130](https://github.com/777genius/universal-agent-plugins/actions/runs/43461070487/job/132013249130),
installed the official Hermes source runtime with CPython 3.14.9 on Windows.
It then failed one of 50 inert contracts before the native User tools/call stage:
`test_payload_replacement_and_midstream_mutation_are_refused` did not observe
its expected capture refusal. This is not a successful current-source native run.

The test overwrote `one` with same-length `two` immediately after a real read,
but did not establish that the filesystem exposed a changed timestamp. Make
that injected mutation explicit: retain the real same-size write, set its mtime
two seconds beyond the pre-write value, and assert that the filesystem reports
the changed mtime before returning the read. Keep the original refusal assertion
and root-replacement negative. This makes the fixture precondition deterministic
without sleeping, skipping Windows or changing any production capture guard.
The log alone does not identify which timestamp observation remained unchanged.

The concurrent `136900cdfcc6ace6c4e025221931ef08507eb4d5` correction
already advances mtime; preserve it and add the explicit observed-delta assertion.
Only this existing test and plan change. All runtime hashes, metadata fields,
link-closure checks, native commands and qualification rules remain unchanged.
The corrected contract and fresh cross-platform/native CI are unexecuted.

## Pinned builder environment finding (2026-11-13)

Remove the ineffective workflow-only `UV_LINK_MODE=copy` preparation override;
restore the unchanged official `native_build.py` invocation. At pinned Hermes
`2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`, the initial environment preserves
that key (`scripts/bundles/desktop_toolchain.py:27–65`), but application venv
construction passes it through `scripts/bundles/native.py:243–249` and
`pm/operations.py:53–60` into `pm/environment.py:242–256`, whose
`_base_environment` removes UV settings not accepted by
`pm/index_config.py:24–44`. That exact allowlist excludes `UV_LINK_MODE`.
This is source proof of an ineffective override, not a native repair or proof
of the member responsible for any closure refusal. Keep complete official
payload, original closure guards, false qualifications and existing deadlines.
No vendor source change, environment-filter bypass, cache cleanup or custom
repackaging is authorized by this correction. Workflow execution remains pending.

## Owned official-build alias teardown (2026-11-13)

Actual original850 archives show Linux `venv/share/man/man1/qr.1` with two
links/one payload alias and Windows `pm-runtime/Lib/site-packages/packaging/dependency_groups.py`
with three links/two payload aliases; historical0a1 repeats the Windows member.
No payload body hash is inferred from the diagnostic member-path hash.

Allocate fresh0700 work/cache/payload roots exclusively before the unchanged
pinned official builder, retain their original directory handles and identities,
and author only the builder's documented `.native-preparation` owner marker.
After the builder returns, one original120-second cleanup budget covers the
complete metadata-only union census, targeted unlink and final full payload
snapshot. At most3x262144 entries/depth64/8GiB logical regular bytes across the
union are admitted; the prior macOS payload alone had123925 entries. Bounds are
finite refusals, not evidence that every future cache fits. Profile caps stay
unchanged. Unknown external runtime aliases refuse before the first unlink.

Only external regular aliases whose complete inode group is accounted inside
those original fresh held roots are unlinked. Internal payload aliases and bytes
remain intact; all other work/cache entries are retained. Every operation joins
original roots, no-follow held ancestors and immediate leaf identity; exact own
nlink decrements/ctime transitions and immediate-parent metadata changes are
tracked only for successful individual unlinks. Any observed drift stops and
retains remaining state, without a stale rollback or whole-tree deletion.
POSIX unlinkat is descriptor-relative, not atomic inode-CAS against arbitrary
same-UID attackers: authority is exclusively this fresh private settled-build
fixture lifetime, never an existing user/foreign cache. Windows uses DELETE
access plus OPEN_REPARSE_POINT and FileDispositionInfo on the held leaf handle,
with rename-denying held ancestors; unknown/API failures refuse.

No vendor patch, custom repackage, cache relocation, raw binary upload or native
qualification is introduced. Genuine final-source Windows/macOS/Linux capture
remains mandatory. Local inert tests exercise actual Linux aliases, replacement,
unknown/late outside aliases, shared deadline and membership/byte bounds; Windows
handle behavior requires the ordinary hosted Windows lane.


## Original archive closure and observer-only bytecode correction

All five originals were downloaded through the official Actions artifact API
and independently verified locally: exact API size/digest, whole ZIP CRC,
closed member paths, source/source-after and vendor-pin joins, and every
recorded output stream size/digest. The fixed historical CI reader expected
flat members, while the real uploads retain their exact owned
`hermes-profile-admission-evidence-<token>/upload/` prefix. Its two diagnostic
jobs refused before reading the native evidence; those jobs are not native
failures or passes. Remove the completed temporary jobs/reader/tests now that
the originals have been properly audited. Genuine User/custom-root jobs remain.

| Original | Artifact ID | Bytes | SHA-256 |
| --- | ---: | ---: | --- |
| 850 Linux | 14058673342 | 1039 | `832a926c4cc7589695e15ac6b9ba6b0d957029ab9b08c2fa35eb94d0c759a8a3` |
| 850 Windows | 14058990969 | 1077 | `84d8ae6694ba1131427c6143b65cf4b932b8412d164fd4cbfd164d5d59b0257d` |
| 0a1 Windows | 14052063757 | 1077 | `1be40362fd5f15c4e71519e2d73eb0dcbc102a92add9453a44496a9e59c01c55` |
| 850 macOS | 14059226751 | 9946 | `03c61239c8dc39362f23d424a6b631b91c5f285d216d10270bb06f07595c27c6` |
| b79 macOS | 14046328482 | 9945 | `ed0d57c5bea2e29991d7ff59d752d62209ce7842088bad2291d880a22f3af877` |

Both macOS originals pass the literal native `--version` command, then refuse
an original custom-root identity change and report changed runtime inventory.
Both add 13 entries and 405670 logical bytes. Their aggregate-only inventories
do not identify those members or the changed root identity field. The 920
private-root correction remains prospective; no old failure cause is inferred.

The exact pinned `hermes_cli/_launchers.py:39–72` runtime API emits
`[python, -I, -c, bootstrap, *args]`. Isolated Python ignores the existing
`PYTHONDONTWRITEBYTECODE=1` environment setting. Preserve that complete
vendor-generated bootstrap and argument tail, validate its exact prefix,
and insert `-B` only for the UAP observer and UAP resolution probes. The literal
native launcher argv and official payload are unchanged. Native POSIX launchers
use `-P` in `scripts/build/launchers.py`; no source-installed or native command
is silently rewritten by this observer helper.

A real isolated-Python regression demonstrates that the original `-I` control
creates bytecode for a new inert module despite the environment setting, while
the observer command leaves its module bytes intact and creates no cache.
Additional shape negatives refuse changed interpreter/flags/tail/container.
All 52 existing/new profile contracts pass locally, independently repeated.
This proves an observer defect and its correction, not that the historical
13 runtime members were bytecode. Complete runtime inventory guards and all
native admission/qualification gates remain unchanged. New genuine captures
must establish the resulting native behavior.

## Original 920 custom-root removal postimage

Run43471155964 used source `92026773461a1b8e1a2856c4fd32efaf0cb8de71`,
tree `b65d840a66409f1fa8505d727aaa9139f32fd1f5`, and the unchanged pinned
Hermes source `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`.
The three original bundle archives pass API size/digest, whole ZIP CRC and
source/vendor before/after joins. Linux14063173258 (1042bytes,
SHA256 `65c76242ec86513d60bda14159848a46215e7e144aeaced156284eb455d2773b`)
and Windows14064123045 (1079bytes,
SHA256 `7699c8ac0b05b32373ceb0f2433d8bafb612620ac0b02c22845fd5052b9041e0`)
repeat the exact prior external-hardlink refusal members. They predate the
4ad owned-build alias cleanup; they do not test or qualify that correction.

macOS14063198477 has24154bytes and SHA256
`53369c03443743b08dc9f9086391e848dd1a68a651d502090d6f4a039d470f09`.
Its43 closed members and42 stream joins pass. Literal native version, selected
A/B inventory, three selective initialize/list sessions, independent native
root-resolution guards and the original User inventory finish before the first
`mcp remove`. That successful native command replaces the63-byte authored
config with a private20-byte ordinary file; the observer then refuses its fields.
The recorded postimage hash is
`8bddefe963a867c332591ef2f50ec056944d1a323229b6294a4de7d9718a69cb`.
Pinned `hermes_cli/mcp_config.py:_remove_mcp_server` removes the last server
key and calls the official save path, which preserves schema version52 from
`hermes_cli/config_defaults.py`. The source-derived bytes
`_config_version: 52\n` exactly match the original length and digest. Original
raw YAML was not uploaded; this is a verified reconstruction, not a raw capture.

Admit only that exact integer/byte postimage after the existing successful
remove transition. Initial/pre-remove authored config authority is unchanged;
wrong versions/types/bytes, extra settings or servers still refuse. The new
regression fails on the prior observer before the correction. The full native
journey must still pass: 920 stops before removed absence checks and B removal,
and reports runtime inventory growth of76 entries/1407659 bytes. That aggregate
does not identify the changed runtime members. The later observer-only `-B`
correction remains unqualified by this old run; immutable payload guards and
all native/product/authentication/model qualification boundaries stay intact.

## Separate final User control and exact Project config observations

Root approved a bounded correction after inspection of the original 920 macOS
archive. All 17 recorded shared effects belong to the deliberate `list-user`,
not the preceding Project commands or `remove-mcp-a`. Pinned `config_home.py`,
`config.py`, `config_backups.py`, `default_soul.py` and `hermes_logging.py` explain
the finite cold User bootstrap. This original remains a failed partial journey.
The fresh authored User fixture uses explicit UTF-8 bytes, keeping the same
279-byte config preimage on Windows without text-mode LF-to-CRLF translation.

Move the same User control after both Project removals and absence checks.
Every Project command must preserve the complete original User snapshot before
and after execution and may change only its exact selected runtime subtree.
The original baseline is never renewed. The final User control admits only
the three same-object directory permission transitions, ten exact new directory
names, the pinned 667-byte SOUL/hash, two empty logs, and exactly one timestamped
backup with the original 279-byte config/hash/mode and a distinct inode. Every
other original User file and both Project trees remain exact. Windows uses its
source-supported portable stat mode domain; this does not prove ACLs or claim a
Windows native pass. Unknown effects still refuse. Keep original raw deltas;
only the validated final control is excluded from unadmitted shared effects.

Add exactly four prospective first-party observations: `config path` and
`config get mcp_servers --json` once in A and once in B, after each root guard.
The path must be the exact selected config plus LF or CRLF; JSON must have unique
keys and equal the whole selected loopback MCP map. No `--raw`, User/default
fallback or human-list parsing. These commands may initialize their selected
runtime; they are bounded observations with pre/post effect guards, not a
blanket promise of no writes. The existing protocol ledger requires zero probe
events for all four commands. Each uses the same source/root/config authority.

The closed command catalog gains exactly four labels and twelve fixed
stdout/stderr/status member names (62 to 74 total declared names). The 720-second
family deadline, 60-second command limit, 1 MiB/member, 16 MiB aggregate,
configuration inventory and immutable payload bounds remain unchanged. No new
workflow, exporter, artifact type, native runtime policy or product gate changes.
Real inert children exercise the actual effect and stream validators, including
Project-phase leakage, foreign changes, alias/identity, copied config digest,
exact readout framing and zero-endpoint negative cases. Their success cannot
qualify native Hermes; genuine three-platform original evidence remains required.

## Original 8a Windows inert fixture corrections

Source8a run43480021547, Windows User job132076721507 stopped before the native
lifecycle: 53 inert contracts reported one error and one failure (three separate
POSIX-only cases were skipped). The mocked official argv used `/payload/python`
while the real Path argument used Windows separators, so the strict
observer shape check correctly refused. Build that inert argv from the same
platform Path string. The real bytecode fixture wrote text-mode CRLF then
required exact LF bytes; write its authored module as explicit bytes. Keep both
production guards and the real isolated child/bytecode assertions unchanged.
These corrections are not native Windows qualification or a cleanup-census fix.

## Windows owned-build census attribute diagnostic

The original 8a Windows bundled job132076721240 refused during its first complete
build-root census, before alias planning or unlink, on `path:st_file_attributes`.
Keep that equality and refusal. Only this Windows failure path may append a
closed JSON diagnostic (at most 512 ASCII bytes): fixed work/cache/payload role,
SHA256 of the already-computed relative member, and four uint32 attributes from
the existing path/handle before/after snapshots. No new stat, file read or unlink
is added; malformed context omits the diagnostic and preserves the refusal.
No bit meaning, affected role or native qualification is inferred before capture.

## Whole owned-build domain budget (actual 8a Linux)

Run43480021547/job132076721458 completed the unmodified official payload build,
then refused the metadata-only initial union census with `owned build byte bound`.
The observer never ran and no alias was unlinked. The prior code budgeted entries
for three roots but mistakenly budgeted their logical regular bytes as one root.
No exact offending path or full union byte count was retained; do not infer one.

Keep the payload's existing262144-entry/depth64/8GiB limits unchanged. Each of the
three exact fresh owned domains (payload/work/cache) receives that same finite
entry/byte ceiling; the disjoint metadata union is bounded by786432 entries and
24GiB. Count logical bytes for every regular name, including internal/external
aliases, rather than deduplicating inodes to hide census work. The original120s
cleanup deadline is shared across all roots and phases. Complete alias accounting,
held identities, unknown-alias refusal and final full payload closure are unchanged.

A fixed closed census line reports only fixed roles, entry/regular-byte counters,
limits and complete/refused status. Refusal occurs before alias planning/unlink;
no raw path, body, exception or profile value is emitted. Tests prove a complete
three-domain union exceeding a single-domain budget, independent domain/union
refusals before reads/deletes, and a per-domain member bound that cannot borrow
sibling capacity. Existing replacement, outside-alias, Windows disposition and
cancellation negatives remain required. Native Windows attribute diagnostics from
babb are preserved; this change does not reinterpret or relax their equality.

## Actual 8a macOS cleanup-contract refusal order

Run43480021547/job132076721092 stopped in the eleven inert cleanup contracts,
before the official build or custom-root observer. In the actual moved-parent
negative, the held-parent guard raised the exact fixed message
`owned build unlink held parent changed` before the named-ancestor guard could
raise `owned build ancestor link changed`. The test accepted only the latter.
Accept exactly these two production refusal messages for this one negative;
keep the actual parent move, failure requirement and payload-preservation check.
No production guard, ownership rule or native effect allowance changes.
This one-line test correction is source-reviewed through the official API after
executor loss; it has not been executed locally and needs genuine CI validation.

## Windows named-directory attribute correction (2026-11-13)

The original Windows bundled job `132127261446` in run `43495420584`
refused the owned build census: a named directory changed attributes from
`0x10000010` to `0x10`, while both held-handle snapshots stayed `0x10`.
The reported member-name SHA-256 was
`b0f5646479f0ae4474e9ed3c91bf6b21700e9d02c2e5aa754ea3036fc6914325`.
No native profile admission followed that refusal.

[CPython issue 126253](https://github.com/python/cpython/issues/126253)
records the same NTFS discrepancy. The
[Microsoft file-attribute contract](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/ca28ec38-f155-4768-81d6-4bfeb8586fc9)
defines the directory bit as `0x10`; the extra `0x10000000` is not a public
file attribute. Normalize only this one observed bit on ordinary named
Windows directories already carrying `0x10`, with no reparse flag or tag.
Every held-handle attribute, other named bit, file identity, owner, timestamp,
mode and link guard remains exact. Regular files are unchanged. The payload
metadata and subsequent hash pass use the same named-directory identity;
the build cleanup ancestor checks use that same helper.

Scope: the existing profile collector, its tests, build cleanup and this plan.
Estimate: 25 runtime and 40 test lines; complexity 2/10, reliability 8/10,
confidence 8/10 before genuine Windows execution. Regression cases exercise
the original temporal comparison in both directions, unrelated attribute and
ownership changes, reparse substitution, exact held-handle refusal and
unchanged regular-file semantics. Independent source review and hosted native
execution remain required; no Windows pass is inferred from inert tests.

The inert official-bootstrap contract also compares the native `Path` string,
not a POSIX-only literal, so Windows can reach the genuine admission stage.
Independent R2 source review accepted the exact normalization scope, unchanged
held-handle comparisons and regular-file guards. The combined correction passed
57 profile contracts and all 14 owned-build cleanup tests locally on Linux.
These contracts do not qualify the pending Windows native execution.

## Windows diagnostic fixture uses a representable directory mode

Source `95f8648b4ffe03b729c16c3a8b1506f922f33f4e`,
[run 43505052472, job 132163259773](https://github.com/777genius/universal-agent-plugins/actions/runs/43505052472/job/132163259773),
passed the prior official-argv fixture but failed the temporal field diagnostic
before the native User lifecycle. Its synthetic `st_mode=123456789` raised
`OverflowError: mode out of range` in the real Windows `stat.S_ISDIR` call.
The missing `caught.exception` error is a consequence of that unexpected type.

Give this existing fixture a real directory mode and change only its execute
permission bit for the mode negative. All other field mutations, exact refusal
messages and single-snapshot assertions remain unchanged. Runtime code and
Windows directory normalization are unchanged; no additional skip is added.
The original hosted failure is the red evidence. Linux contracts can validate
the revised fixture, but cannot reproduce or qualify the Windows runtime.
On the unchanged Hermes sources at `f8025bfabdfbe40ea2a0f6e5b2a09f6273d97308`,
the corrected 57 profile tests and all 14 cleanup tests passed locally on Linux,
with no skips; the diff whitespace check passed. Fresh Windows CI is required.

## Finite owned-build cleanup budget after the Windows census

Original source `95f8648b4ffe03b729c16c3a8b1506f922f33f4e`,
[run 43505052472, job 132163259326](https://github.com/777genius/universal-agent-plugins/actions/runs/43505052472/job/132163259326),
completed the official Windows build and the full initial census. The payload
held 134259 entries and 6258975024 logical bytes; cache held 57890 entries and
4836389126 bytes; work held 263 entries and 3179853 bytes. The census finished
after about 20 seconds. The original 120-second cleanup deadline then expired
inside the payload-anchor join during alias detachment, before final closure
and before any native profile observation. No identity or alias refusal occurred.

Allow one fixed 600-second budget at the existing cleanup entrypoint. Initial
census, exact alias unlink/rejoin, final census and payload hash all share that
same deadline; no phase renews it. Preserve every ownership, inode, alias, member
and byte guard. The native family remains 720 seconds and admission scans remain
120 seconds. The existing 180-minute job and 9000-second build elapsed receipt
remain unchanged; this does not add 600 seconds beyond either outer limit.
The new budget is prospective, not a measured native pass. A fresh task-owned
Windows build must complete cleanup and admission before it can qualify.

## Owned build cleanup deadline after the original Windows 95f failure

Run 43505052472, job 132163259326 built the official pinned payload. The initial
metadata census completed at 21:21:13 UTC: payload 134259 members / 6258975024
logical bytes, cache 57890 / 4836389126 and work 263 / 3179853. At 21:22:53 UTC
the 120-second shared cleanup clock expired while rejoining the payload ancestor
after an alias unlink. The native profile observer never ran. This is a real
cleanup deadline failure, not a successful Windows custom-root result.

The source correction gives the complete census, detach and final closure at
most 600 seconds, and makes build plus cleanup share one absolute 9000-second
clock. The official builder gets only positive remaining time; cleanup receives
the earlier of that original deadline and entry time plus 600 seconds. It cannot
renew the total budget. The 180-minute workflow, 720-second native family,
120-second payload scan budgets and all entry, byte, ownership and alias limits
remain unchanged.

Closed progress lines contain only fixed status, selected-alias count, verified
alias count and elapsed seconds. They appear at admission, each 1024 fully
verified aliases and completion or refusal; no paths or file contents are emitted.
A failed post-unlink join can leave a removed alias uncounted; verified_aliases
is a lower bound on fully verified transitions, never proof that no other effect
happened. Only final census and full payload closure permit completed status.

Root and independent R1 accepted this bounded correction. Actual-file regressions
expire the clock immediately after the first unlink and require the original
payload and remaining alias to survive. Build-clock tests prohibit a renewed
cleanup allowance after the total deadline. All 16 cleanup contracts passed
locally on Linux. Original hosted execution must still establish Windows
completion and performance. No native qualification changes.

## Original Windows profile progress and exact removal newline

Source `b7f96040e26e09a0c12a64eae8b909a90a8a32bc`, tree
`fc9a3ed45c87e166a51f8a149651bdebc211e76d`,
[run 43515526775, Windows job 132197261357](https://github.com/777genius/universal-agent-plugins/actions/runs/43515526775/job/132197261357)
completed owned build cleanup with 33071 detached aliases. The original payload
remained 134259 members / 6258975026 logical bytes. The subsequent native profile
observer failed after successful A/B config/list/test and A restart commands,
at the first final-server removal. This advances the earlier cleanup timeout;
the complete Windows profile lifecycle is still failed and unqualified.

Original artifact `14084285338` has 27008 bytes and SHA-256
`1cdcfeda716f8ae3faca92f2061e0a902447f22eefc6cff312989842f7463f34`.
Root independently verified its CRC, all 52 members, 51 complete stream hashes,
15 status/output joins and 13 command deltas. Both source identities and the
complete before/after bundled runtime match; cleanup completed. The original
method ledger contains selected initialize/list sequences for A, B and A restart,
without tools/call. It records no unadmitted shared effects. All final native,
authentication, inference and ordinary-source qualification flags remain false.

The `remove-mcp-a` command exited zero, but `config_document_boundary` then
refused `unadmitted removed config stamp`. Its recorded postimage is 21 bytes,
SHA-256 `9d2f633e8e0a5553913286ecc1b88d2511a69e545da69760b24135d02db75f3e`,
which exactly joins the candidate bytes `_config_version: 52\r\n`. The Unix
20-byte LF spelling has the separately established digest
`8bddefe963a867c332591ef2f50ec056944d1a323229b6294a4de7d9718a69cb`.
The original raw YAML was not uploaded; this conclusion uses its exact length
and digest, rather than inferring bytes from the runner OS alone.

The oracle now requires the measured CRLF spelling only on Windows and the
existing LF spelling elsewhere, with the same strict integer, exact mapping,
removal-only and authored-byte checks. It does not normalize arbitrary whitespace
or accept both spellings on every platform. A portable inert contract joins the
original Windows hash and refuses the opposite platform newline, missing newline,
duplicate carriage return and trailing content. Native effects, process bounds,
cleanup, publication and qualification gates are unchanged. A corrected-source
original Windows run is still required; no historical archive is reclassified.


## Public bundled Project lifecycle and native cold recovery

This prospective collector extension composes the reviewed durable Project
adapter with the existing pinned bundled observer. It does not qualify an
ordinary source installation, vendor authentication, model inference or
tools/call. The original raw A/B metadata/MCP family remains intact, with its
separate exact A=2/B=1 initialize/tools-list oracle. New public A/B probes and
ledgers cannot contribute to that historical count.

Before the immutable runtime interval, the workflow builds the actual current
UAP, the compiled existing Cold2 test and the existing fixed MCP probe. The
collector receives their exact paths and SHA-256 values, verifies them against
the same clean source head, and holds their original identities. All three
remain outside the disposable home, evidence, vendor source and payload.
Compilation and its caches run before the native/profile baseline.

The public fixtures are independent real Git projects. Ordinary public add
creates Prepared, installation_verified bindings at the exact selected root
PROJECT/.hermes/uap-session. The collector derives launcher read authority from
the exact state, package ownership, committed package receipt, standalone
native-root receipt, original marker, source revision and retained data receipt.
It recomputes the complete package digest, including streamed launcher bytes.
No path pattern alone admits a copied executable.

Ordinary members retain the original 1 MiB per-file and 16 MiB aggregate caps.
At most four receipt-backed launcher roles can stream at most 128 MiB each,
under a separate 512 MiB total. Each must match the original compiled UAP
size/SHA. Windows launcher mode joins the original named UAP file's mode,
rather than assuming a synthetic numeric mode. The sole additional large
scanner leaf is the existing pinned lintai 0.1.3 platform asset at the exact
public-uap-state security path. Its existing POSIX mode contract and Windows
within-domain identity checks remain intact. Unknown large members refuse.

The public sequence covers add A/B, byte-stable repeat A, explicitly consented
public run --inspect, actual native HTTP and stdio initialize/tools-list on A/B,
a fresh A restart, changed-source update A, restoration of missing owned Skill
and selected YAML from intact staged source, another explicit inspect and both
MCP transports, then scoped removal A/B. Removal retains the original selected
root and parent, its standalone publication receipt and owned persistent data.
Subsequent native selected lists and refused removed-server tests cannot launch
the removed server. Original User, foreign Skills/files, the other Project,
each Git tree and unrelated settings remain unchanged. Unknown bootstrap
members or unmeasured effects fail the collection.

The stdio peer is the repository's real fixed MCP test server, compiled before
the interval. Its bounded retained-data events join exact nonce, PID, fresh
session and initialize/tools-list order; no tools/call or substitute Hermes is
inferred. Public HTTP counts remain separate from the raw family.

Cold2 executes the precompiled TestProjectHermesBundleColdRestoreAndRepair
inside that same original 720-second interval, with the selected actual bundle,
genuine hosted flags and a fresh report, from the current checkout. Its public
Cobra entrypoint runs in the Go test process with the production composition,
detector and session planner. Only the existing StartNativeSession callback
executes the real selected native plan through the existing OS runner. This
keeps native children under the same live Go owner for context cancellation.
The report explicitly records entrypoint=public-cobra-in-process and
installer_executed=false. It does not claim to execute the compiled UAP binary
during Cold2; that binary is exercised separately by the warm public family.

Each of the three Cold records separately contains the public request arguments,
actual native executable/arguments, actual working directory and selected_root
from the HERMES_HOME passed to the native process. They must join the selected
Project, restored binding and retained root publication. Closed original stream
hashes, cleanup receipts, exit zero, the exact PASS/source marker and absence of
SKIP remain required. Empty nil Go stderr is accepted only with strict zero
length and the exact empty digest; stdout must contain native inventory.

The collector chooses the smaller of 360 seconds and original remaining time
minus ten seconds once, and refuses a budget of 40 seconds or less. It passes
that same deadline as UAP_HERMES_COLD_DEADLINE_UNIX_MS. The Go cancellation
context retains a 30-second reserve before both that deadline and the testing
timeout. This does not renew the original monotonic 720-second outer fence.

The parent precreates exactly public-cold-tmp/hermes-cold and holds the original
parent and child directory identities/descriptors before any Go child starts.
TMPDIR/TEMP/TMP all name that existing empty child. Go creates its private
journey directories there without automatic test cleanup. Only after the real
test exits successfully, both server families close, the closed report joins
and compiled inputs remain unchanged may the parent remove that original
owned child. It checks held identities and exact parent membership, reuses the
existing owned-root cleanup and requires the parent empty afterward. A newly
observed directory name is never adopted as cleanup authority.

Capture completion is set only after public and Cold completion, native
process settlement, owned-home cleanup, both source identities and full
before/after payload and compiled-executable closure. Failure is not promoted
to admission. If the public family has started without a fully validated Cold completion
report, the parent retains the disposable home and withholds raw evidence
before census, deletion or runtime-after claims. Empty observer process-group
membership alone cannot prove cleanup after a Go supervisor is killed.
Cancellation, a missing/partial report or an unjoined directory never
authorizes cleanup. No native result is inferred from these contract tests.

Before the execution-environment interruption, 64 profile contracts passed
locally. After recovery, the source review corrected nil Go stderr handling,
kept outer-root identity separate from selected-root identity and deferred
capture_complete until full closure. Further contracts reject forged Skill
ownership/digests, wrong native/public command roots, missing or incorrect
entrypoint flags, partial cleanup, skipped tests, renewed deadlines, an extra
temporary sibling and replacement of the originally held temporary directory.
The Cold command callback in those tests is inert; it proves collector behavior
and makes no claim of local native execution. Fresh original native artifacts
on the final integrated source remain necessary on all three OSes.

The earlier pre-integration Python checkpoint passed all 66 profile contracts and
all 16 owned-build cleanup contracts on Linux; the whitespace diff
check passed. These are contract results, not a bundled native acceptance.

## Ordinary installed-source capture with the original owner preserved

This six-path research change starts from
`4ab2e3099e885fed2357b06cb628852566b1b353`. It preserves the reviewed directory
transaction/session candidate, the Windows removal oracle and the absolute
9000-second build budget. Public Hermes Project registration, source bootstrap,
native recovery and actual Windows product qualification remain refused.

The existing source-installed `native-call` job now installs the pinned official
checkout under its real default User home: `user/.hermes/hermes-agent` on POSIX,
or `user/AppData/Local/hermes/hermes-agent` on Windows, within the same disposable
`RUNNER_TEMP/hermes-native-home` fixture. The official installer, immutable vendor
commit, actual published `HERMES_BIN`, and official runtime-command output are
retained. `HERMES_NATIVE_SOURCE` passes that exact checkout to existing helpers;
their previous location remains the default in other workflows. There is one
additional bounded source observation step in the same job, not another matrix
job or a private replacement installer.

The source input boundary verifies the original HOME, USERPROFILE, native User
root, canonical source/tools/installs directories, actual launcher and store
Python. It closes the complete captured link and hardlink graph before reading
runtime bytes. The actual PM Python entry, dependency-generation fact and
`pyvenv.cfg` are then joined back to that capture. Both facts documents come
only from the existing held content-hash pass, with the same single-link and
bounded-file checks; no path-based provenance-body reread follows the census.
The original outer identity and captured facts ancestors/leaves are rejoined
before decoding; the selected provenance, generation and `pyvenv.cfg` chains are
rejoined again before returning the snapshot. Moving the original User tree
and unchanged facts into a replacement outer directory refuses before any later
provenance read. The saved native runtime argv
must equal the pinned official `runtime_command` API output; its code is not
evaluated as an unvalidated captured string. No PM facts or dependency generations
are manufactured, adopted, copied or installed by the collector.

Before any preparation, the parent retains the complete original post-installer
census privately. It prepares only the existing inert config/auth/history/foreign
sentinels and the closed environment's known directories. Official config and
environment preimages retain their original file identities, bytes and exact
rename metadata under fresh task-owned names; an existing auth/history/foreign
member refuses. The parent compares every original record with the prepared
census: only these authored files, recorded preimage renames and explicitly
created directories may differ. Existing directory authority stays exact, with
child-membership metadata changes allowed only where declared children changed.
An unrelated User or source write during preparation refuses.

The parent sends the child only a bounded preparation receipt with the nonce,
original outer identity, original/prepared complete User digests, fixed fixture
names and preimage metadata. Its hash comes from the bytes actually authored by
the parent. The raw member ledger never passes through the environment or public
artifact. The child does not write User or select a new User baseline; its first
capture, after observer bootstrap and Project setup but before the first native
Project command, must equal the parent's prepared User digest. The parent checks
that same digest and the unchanged preparation receipt after process settlement.
The receipt has a closed metadata-only schema, also checked during publication.

The complete User digest covers caches and every file outside the three PM trees.
Source/tools/installs have separate complete digests; protected files come from
the same captured records. The remaining large runtime partitions are hashed,
while small Project configuration/bootstrap metadata remains explicit. No file
is excluded from the full capture or counted twice in its partition.

The existing 262144-member, depth-64, 8-GiB logical-byte and 120-second scan bounds
apply to this complete fixture. The ordinary Project metadata/output bounds stay
2048 members and 1 MiB per file/report. A Project link may reach its own runtime
or the exact installed source/toolstore; it cannot reach the other Project,
original User configuration, or another captured sibling merely because they
share an outer directory. Intermediate link hops are checked too. Complete
hardlink alias closure and domain checks precede every content read. User sentinel
copies are detected across streaming chunk boundaries without exporting bodies.

Only HERMES_HOME selects A or B for the actual native commands; the installed
OS User and launcher stay fixed. At this pinned source revision a cold
`--version` deliberately skips dependency preparation, so the first A/B command
is the actual `config path`, followed by the measured source version, ownership
guard, `config get`, list and native initialize/tools-list exchange. A repeats
its test from a fresh native process, then both roots remove their final server
and verify absence. The original 60-second command and 720-second owned process
family limits remain unchanged. The ordinary source banner must match the
original observed `v0.23.0`/pinned-commit grammar; bundled `vunknown` is not used to
qualify this path.

Every Project command compares the entire original User, source, toolstore,
installation state, sibling Project and outside-selected ledger against the
original baseline. The already-installed final User control has no new cold-User
allowlist: any changed User state is recorded and refused. Its baseline is never
renewed. Unexpected observer effects are likewise retained in before/after
runtime evidence and refused against the parent's prepared User state, including
effects before the child's first capture. The parent separately compares the
immutable source, toolstore, installation state and named launcher/provenance
across the observer family. Its complete outer inventory digests are retained
too. The original post-installer census is never replaced by an unchecked
post-preparation baseline. This capture does not qualify an unmeasured general
bootstrap/recovery footprint. The existing bundled cold-User policy remains
separate and unchanged.

After process-group/Windows-job settlement the source fixture and any measured
client-owned PM state are retained. The collector does not recursively delete the
installed source or claim ownership of a native dependency tree. The closed
source artifact has its own prefix and metadata/command-stream envelope; it
does not upload runtime trees or User file bodies. Every source/bootstrap/
production qualification remains false even if the finite observation completes.
Normal interrupted-sync recovery, actual repeat/restart state, effects outside
the selected root and the public version/runtime gate still need original native
evidence before Project promotion. These inert contracts do not supply it.

### Complete native source version readout

The independently audited original macOS User artifact `14086135237` from
run `43515526775`, job `132197261526`, source `b7f96040e26e09a0c12a64eae8b909a90a8a32bc`,
contains a 217-byte `native-version.stdout` with SHA-256
`b4f09e15b3940ad8739fa1aa3bc99c802ab162f9112e798edf3557afffaf1911`.
Its first line is the pinned `v0.23.0`/stable/local-commit banner, followed by
install directory, install method, Python, OpenAI SDK, and an update notice.
The independently joined Windows original has the same six fields with CRLF:
209 bytes, SHA-256 `da4ae42f21fc02f2f0967394bbfbaaa0f45cad7ec01272c5a9e8f46d8add0691`.
The v2 collector incorrectly compared the whole output to the banner alone.

Pinned `hermes_cli/_startup_fast.py::print_fast_version_info` confirms this
multiline format. The source-only decoder now requires the exact banner,
selected installed source path and `git` install method; Python 3 and OpenAI SDK
metadata must each contain concrete three-part numeric versions with bounded
components. These metadata fields are observations, not an additional runtime
qualification. Only the exact captured optional update notice is admitted.
Additional/duplicate lines, unobserved status messages, malformed or missing
metadata, a different source/version/commit, incomplete output and the opposite
platform newline refuse. The original 217/209-byte hashes are retained in the
regression, with source-path substitution explicit for the portable fixture.

Command count, existing output/time budgets, source ownership, configuration
guards, public registration and all source/bootstrap qualification flags remain
unchanged. Original User and bundled evidence does not qualify the new ordinary
source Project route or interrupted native bootstrap recovery.

### Combined source-v3 and bundled public collector review

The public/Cold collector is reconciled against exact
`2b557b30e57206485aa35b79986da2ecff75f691`. The installed-source branch keeps its
original `.uap/hermes-runtime` roots, preparation envelope, retained original
User preimage, captured source payload documents, alias closure, source runtime
before/after comparison and retained-home policy. It does not require compiled
public UAP, probe or Cold inputs. Only the bundled branch captures those three
inputs and invokes the separate public A/B and Cold families. An inert behavioral
contract runs both child routes and confirms that source capture opens only its
two original probes and never calls the public observer.

Independent R2 replay found two concrete weaknesses in the first public report
reader. Python truthiness accepted false, zero and an empty list as an absent
native activation attempt. It also accepted malformed or foreign Cold stdout
after its base64/count/SHA fields were consistently recomputed. Both readers now
accept only an omitted/null attempt or an actual empty string. Every Cold stdout
passes the existing duplicate-key/nonfinite-refusing JSON reader, contains the
exact owned `docs` MCP map and joins its restored native-object digest using the
same canonical digest helper as the warm family. The three original, restored
and repaired maps must be equal. Rehashed malformed, foreign and changed-last
inventories remain negative contracts, not native evidence.

The standalone root receipt additionally joins its recorded
`publication_parent_authority` facts: exact version, selected parent path,
bounded ancestry and active/staging/backup sibling paths. The collector keeps
opaque recorded volume/object IDs and the complete receipt through removal; it
does not issue a new authority token or claim to repeat the product's physical
revalidation. Existing Store/Kernel authority validation remains responsible
for that check. Missing authority, extra fields, a wrong parent, an invalid
version, a changed final ancestry path or an escaping sibling are refused.

The first combined local run passed all 84 profile contracts and all 16 owned-build
cleanup contracts on Linux. AST comparison confirms that all 73 test methods
from exact `2b557b30` are preserved unchanged and that all existing source-v3
helper bodies, evidence admission and staging are unchanged. Of the original
functions, only the shared bounded file/inventory readers and the child/parent
dispatch gain the reviewed bundled extension. `git diff --check` passed.
These are inert contract results. Genuine final-source bundled/native originals
on Linux, macOS and Windows, with complete source/runtime/process joins, are
still required before any product or admission qualification.

The second independent review found a retained-marker gap after warm removal.
Once the recorded installation had `clients={}`, the binding reader correctly
returned no active binding, but the finite native profile check no longer joined
the marker bytes. A real inert filesystem regression preserved the original
root/parent identities and standalone receipt while altering, shortening or
deleting `.uap-owner`; all three cases were wrongly accepted by the old helper.
The existing profile check now always requires the exact fixed marker length
and digest for that original Project, including after removal. It adds no root
adoption or cleanup authority and leaves the existing receipt/identity guards
intact. The new regression was red in all three negative cases before the four
runtime lines, then the complete 85-contract suite passed; the unchanged marker
remains a positive control. This is collector verification, not native evidence.


### Windows final User control: unopened lazy log files

Original Windows bundled run `43526178468`, job `132234865220`, on source
`999be5b1206df4d96a30c2aacc94913461560647` completed all eighteen Project
commands. Both removals produced the exact 21-byte CRLF config stamp with
SHA-256 `9d2f633e8e0a5553913286ecc1b88d2511a69e545da69760b24135d02db75f3e`.
The previous newline correction is therefore now observed in the native run.

Artifact `14089152690` is 34,325 bytes with SHA-256
`d5ea24f4616fa8b55aecbae5402d9e89a9b1efecd05fd3baca714d89db826860`.
Its closed 70-member inventory, 69 stream joins, 21 command status joins and
nineteen chronological command deltas were checked. The final User `mcp list`
returned zero but its filesystem guard refused: the observed addition set was
exactly the ten declared directories, pinned `SOUL.md` and the independent
byte-identical config backup. Both empty `logs/agent.log` and `logs/errors.log`
were absent. All original User records were unchanged in that observation;
Project commands had preserved User and the sibling Project throughout.

The pinned vendor [logging implementation](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_logging.py)
selects `ConcurrentRotatingFileHandler` only on Windows and explicitly describes
its lazy file opening. POSIX retains eager stdlib handlers. `setup_logging`
registers the two CLI handlers, and `_new_file_handler` does not eagerly open the
Windows files. The dependency is pinned to `concurrent-log-handler==0.9.30`
under the Windows/Python 3.14 marker in the same revision's `pyproject.toml`.

Only the bundled final User-control oracle changes: Windows accepts the two
closed variants of the same pair, either both existing exact empty logs or
both absent. One log alone, nonempty or altered log metadata, an extra object,
a missing declared directory/file, changed original User data, or an aliased
backup still refuses. POSIX requires both logs. Ordinary-source capture keeps
its strict original prepared User baseline, and Project commands still cannot
adopt any User delta.

The regression includes the exact original metadata-only User projection:
18 original records and 12 additions, canonical SHA-256
`a8a525f0ef4b05fcf6d37a7f475850a6bf367c4fb649cf32118579e96c088a89`.
It reproduces the published refusal before the correction and passes after it,
with separate negatives for partial/nonempty/foreign changes and Project,
ordinary-source and POSIX scope. No native execution is simulated by this test.

The failed original remains `capture_complete=false` and `lifecycle_exit=1`.
Its separate final inventory and post-User native-method join were not produced
because the filesystem guard refused first; the audit does not invent either.
The complete bundled runtime was unchanged and cleanup completed. This narrow
oracle correction changes no limits, production registration, native bootstrap
policy or qualification flags. New native evidence remains required.

### Source-v3 read and absolute-link refusal diagnostics

Run `43535108885` at `2b557b30e57206485aa35b79986da2ecff75f691` produced two
different source-only refusals. Windows job `132262352093`, artifact `14089868523`
(9,766 bytes, SHA-256 `d33ba823747fac62e4ff237609cd19c55fabda5eee80c847d992dd10265e3c93`),
failed on `os.read` with `PermissionError` during the observer's initial payload
capture. macOS job `132262352082`, artifact `14090174800` (847 bytes, SHA-256
`4850505e39d77da46abfa9bde76294a282f75b4083760dd29b74acd4d04b7807`), refused an
absolute intermediate link target before the observer started. Both original
ZIPs passed CRC and source joins; neither retained the offending member.

The collector now attaches an observation to those original exceptions using
only the member name and metadata already captured by the existing reader.
The original exception, traceback, file-descriptor cleanup and refusal remain.
Nested-link diagnostics name the actual intermediate link. They retain only
the target's UTF-8-surrogatepass digest and byte count, never its absolute text.
The file-read observation records the completed byte count and bounded OS error
numbers. Relative names have a 512-byte ASCII-JSON ceiling, otherwise a digest
and null name; captured numeric metadata uses uint64 or null with saturation.
This records no file body, environment value or new ownership authority.

One fixed `source-payload-refusal.json` member is limited to 2 KiB. The existing
held evidence-directory writer creates it with `O_EXCL`; later failures cannot
overwrite it. Both admission and staging enforce its closed schema. Staging
joins the two already-read bodies to the same nonce and failed, source-only,
unqualified `profile-admission.json`; it does not reopen that proof. Existing
process settlement, evidence identity, membership, second-read and atomic
publication checks remain. Unknown process cleanup still publishes only the
separate inert failure envelope. A partial diagnostic is neither repaired nor
deleted: publication refuses, retaining the original failure as its cause.

Two new inert contracts first failed on the old code because the actual owned
FD read failure and captured intermediate-link refusal lost their member facts.
A real partial-write regression separately reproduced replacement of the
original cause by a JSON error. After the narrow changes, all 92 profile
contracts and all 16 owned-build cleanup contracts passed locally on Linux,
with no skips; all 86 previous profile test bodies are AST-identical to `12ed`.
The tests cover original exception identity, closed FDs, no additional payload
access, fixed source scope, exact staged nonce/flags, integer and name bounds,
replaced evidence directories, first-write preservation and no partial upload.
These are diagnostic contract results, not native Windows/macOS acceptance.

The Windows immutable-install digest change and the separate Linux prepared-User
baseline refusal remain unexplained by this metadata-only amendment. No baseline
is adopted, link or read allowance added, source repaired, or qualification
promoted. Each original stays failed; new genuine source evidence is required.

### Source User differences after confirmed process settlement

The Linux source-only job `132262351679` in the same `2b557b30` run failed at a
third boundary: its initial payload read succeeded, then the prepared User
digest comparison refused before the first Project native command. Artifact
`14091083977` is 9,643 bytes with SHA-256
`15a0a730b619ced2715413c049db51b5041627711ba742b0c3007cdacb55c774`.
Its prepared and after-settlement User digests differ although both count
100,256 members. The original retains no complete private map or leaf delta;
it does not establish that this change affected metadata alone.

A controlled Linux producer observation used the exact pinned
[`lease_directory`](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/runtime_state.py)
function and the collector's actual snapshot/digest functions. With an existing
empty `.leases` directory, the pinned function created one empty, locked UUID
file. Explicit release removed that file; the directory's mtime and ctime still
differed. User counts were 11, 12 and 11 before, during and after the lease;
retained file-body hashes were unchanged. All five imported vendor source files
matched Git `2bd378ed` before and after, bytecode writes were disabled, and the
environment was unchanged. This ran no Hermes CLI, installer, model or hosted
admission. Its report SHA-256 is
`17abda82d6e429dbee9073143591cbdc105921a8f8ca8176b26d0ad8c4116260`.
It demonstrates a possible mechanism, without identifying the old CI member.

The collector now retains the already captured prepared map only after the
preparation relation and receipt write succeed. It retains the final map only
after the complete source snapshot returns. An already failed source run with
confirmed process settlement can project their User-only difference into one
fixed `source-user-delta.json`: at most eight rows and 16 KiB, with the total
change count and an explicit truncation flag. The same absolute 120-second
budget covers the final snapshot and this comparison. No payload file is
opened, read or traversed again for the diagnostic. A failed comparison remains
the original failure, including when it is the first failure in the parent.

Rows contain bounded relative names or their hashes, captured stat fields and
existing file/link-target digests. Bodies, environment values and link-target
text remain private. Numeric values outside uint64 become null with an explicit
saturation flag. Equal projected rows are refused unless saturation has hidden
the changed value; distinct wide Windows inode values can legitimately produce
that case. Such a row remains an incomplete numeric observation, never authority.
The closed staging reader joins both User digest/count pairs and the nonce to
the single already-copied failed source proof. Existing physical evidence
authority, first-write, process, partial-file and publication guards remain.

All 97 profile contracts and 16 cleanup contracts passed locally on Linux with
no skips; the original 92 profile test bodies are unchanged. New controls cover
real captured file and metadata differences, complete-return/receipt ordering,
missing captures, unknown settlement, expired budget, original Windows/Linux
record projection, bounds, copied-proof joins and partial-write refusal. An
identical unsaturated-row packet was accepted by the initial draft, then refused
after the one-line guard; the wide-inode saturation control remains positive.
After-settlement maps cannot reconstruct a vanished live lease file. The old
native failure remains failed, and this diagnostic grants no new source or
Project acceptance, baseline, cleanup permission or qualification.

### Original public installation and retained data continuity

The warm observer now compares each accepted installation with the preceding
accepted state. The complete source, installation identity, binding authority,
data receipts and physical data directory must continue. Only the authored local
update may advance its tree and package revision; timestamps and operation-group
metadata may advance during a committed operation. The original committed
receipt prefix cannot be replaced or truncated. Normalized records are compared
as typed JSON, including removal, so numeric values cannot substitute for
booleans. Independent snapshot validity alone does not prove this continuity.

Cold restoration additionally compares the complete accepted package and source,
requires a new installation and disjoint data authority, and preserves the
restored identity across locked replay and missing repair. Both original and
cold data trees are captured once with held file identities before each world's
first native inspection. The captures remain live through removal; existing
test cleanup closes the handles before the parent can dispose of its completed
fixture. Removal must retain the complete original source/package/data record.

The new inert contracts use real state files and directories to demonstrate that
two individually admissible snapshots can still have detached source, binding,
data ownership, receipt history or a byte-identical replacement directory.
These comparisons add no native commands, extend no timeout, change no recovery
permission and do not qualify a native run.

### Source snapshot of an already-proven empty readable file

Original Windows source run43542221829/job132285537607/artifact14093934143
retains a zero-byte ordinary member beneath the original selected generation's
`.leases` directory. The exact 10576-byte outer SHA256 is
`e505ec63a838fee28933e5137c5528bca02a0611221960a937ce76ba814a45d6`.
All 15 members passed physical ZIP/CRC admission; 14 original member hashes,
four command-stream hashes, source/vendor trees, nonce and preparation join.
Its original no-follow readable handle and first metadata check succeeded; the
first `os.read(fd, 1)` failed with errno13 before any Project native command.
Pinned `lease_generation` calls `lease_directory(environment.parent)`, matching
the retained selected `venv` and lease path. The vendor creates an empty UUID
file and Windows `lock_fd` locks offset0/length1, which may extend beyond EOF.

Only source capture now hashes an initially empty file without a positive-size
read, after the original successful open and held identity/owner checks. The
deadline and final named/held metadata and ancestor checks remain unchanged.
No lease path is exempted; open failures and all nonempty read failures still
refuse. The immutable User/install ledger is neither renewed nor relaxed, so
the separately observed after-settlement install change remains a refusal.

The inert contracts preserve the original 99 methods and cover a real empty
file, unchanged non-source EOF refusal, growth, replacement, parent replacement
or exact Windows sharing protection, and initial-open refusal. A Windows-only
control takes a real CRT byte-range lock, first demonstrates the failing
one-byte read, then captures the empty file; a locked nonempty control refuses.
Local Linux execution cannot qualify that Windows branch or a native source
journey. Existing hosted jobs supply those remaining observations; no command,
catalog, native budget, production capability or qualification is added.

### Public stdio fixture uses the original pinned executable through authored PATH

The public package's absolute stdio command was invalid at the standard loader
boundary. A local causal check evaluates the actual `observe_public` authoring
expression and passes its document to the real loader and embedded schema
registry: the HTTP server remains, but stdio is excluded with
`mcp_server_invalid`. The same original selector also fails the real encoded
managedstdio dispatch with exit126 before any probe event. This establishes an
invalid fixture; the opaque failed bundled539 envelopes do not establish that
it was their first internal failure.

The public recipe now uses the original emitted probe basename and an authored
server `PATH` containing only that probe's original pinned build directory.
Arguments and the three existing `UAP_TEST_*` values are unchanged. Existing
readiness honors this authored PATH, so the outer process PATH is unchanged.
The pinned vendor later prepends its owned managed-launcher directory; this is
not a claim that the final child PATH literally contains only the build root.
No executable is copied into the package and no runtime pin is renewed.

The positive local check uses the same normally built, source-verified inert
probe through the real loader and managedstdio encoded dispatch. Initialize
and tools/list responses and nonce/session events join; the original probe's
inode, metadata and hash remain unchanged. This is a Linux fixture contract,
not a vendor launch or native qualification. The Python contract covers both
platform basenames, both Project recipes and same-name executable replacement
refusal. All prior contracts, native commands, limits, source/profile guards
and the HTTP-only cold fixture remain unchanged.

### Closed observer diagnosis when public cleanup remains unproved (2026-11-14)

Original run [43542221829](https://github.com/777genius/universal-agent-plugins/actions/runs/43542221829)
on `539e6a54a05cc297370f388116e681fbcdafa443` completed bundled Linux,
macOS and Windows with the original `native profile observation failed` and
subsequent `native process cleanup unproved; raw evidence withheld` refusals.
Artifacts 14094469725, 14094526111 and 14094322467 each contain one 422-byte
failed, unqualified summary. Their original ZIP SHA256 values are respectively
`528a3570fa7dbc19ca0f22808b3e3cead266e2f7aa9f6fe638ad60a987bfff15`,
`6790aa8c8afdeb25198ee3d131b74e0aa19fd9d63eae48b904687e4327878eaa`
and `212de71a0fbb58e4876eb6dd43ee2afdd6ea35df23fb33550127c6aacbc83eed`.
Those summaries do not identify the child predicate or prove a leaked process.
The public cleanup guard still requires the complete validated cold receipt
once the separate public family has started.

This correction preserves that guard and the private raw-evidence boundary.
On an exception, the observer may write one canonical JSON record of at most
2 KiB to its existing stdout. It contains only finite phase/reason/exception
names, the original nonce and exact UAP source head, and an optional numeric
caller line plus function name/definition line. A nullable command label is
selected only from the existing closed COMMANDS set, and only when the failing
stack contains the exact own-source nested command code object. Unknown or
non-string labels are omitted. Caller information comes only
from code objects already loaded from this exact profile script, including its
nested functions. No traceback paths, error messages, configuration, native
stderr, environment values or external source contents enter the record.
Unavailable diagnostics never replace the original exception.

Before starting the observer, the parent duplicates its original, exclusively
created stdout file descriptor. It retains that descriptor until the existing
Windows job or POSIX primary group has positively settled. Only then may it
read at most 2049 bytes, require the original ordinary-file identity and stable
metadata, and accept the single closed, canonical record with the same nonce
and source head. It never reopens the capture pathname. Missing, malformed,
unknown, duplicate, oversized, aliased or changing output produces no observer
diagnostic. If primary closure is not proved, it does not read the descriptor.
The descriptor is closed without replacing any existing native refusal.

The fresh safe-failure envelope may additionally include the finite cleanup
phase and observer exit code already held by the parent, the primary-settlement
boolean, and the validated observer record. Primary settlement is distinct from
public-family closure: this envelope is produced through the unchanged failed
public-cleanup branch, all existing qualification fields remain false, and no
raw artifact, home deletion or Project admission is authorized by diagnosis.
The 720-second native, 120-second scan, payload byte/count, original User,
source/toolstore and lease checks remain unchanged.

Six new inert contracts exercise original exception identity, exact closed
command-label selection, nested own-source
caller joins, the real retained descriptor with an inert Python writer, pathname
substitution, unsettled-family no-read, duplicate/unknown/source/nonce/caller
refusals, overflow, short or failed reads, hardlinks and concurrent file changes.
The safe envelope still refuses public cleanup and keeps every qualification
false. All original 97 profile test bodies remain unchanged; 103 profile and
16 cleanup contracts passed locally on Linux. These are collector contracts,
not a successful native rerun or a source/bootstrap qualification.

### Run the source observer through the recorded PM interpreter (2026-11-14)

Original [run 43546125344](https://github.com/777genius/universal-agent-plugins/actions/runs/43546125344)
on `e5facca91c705f18ff1629b0dae9336daa1b73c1`, tree
`72bb64428987b75dceec6ebc8984f43d958cf614`, now includes the complete settled
User difference on both Linux and Windows. Linux artifact 14096690344 has
15 members and ZIP SHA256
`76b1e30f63b19ceb680b6dd911d86f90a55bf8caa0f95ae6f55a34aa73463a07`;
Windows artifact 14096635177 has 16 members and ZIP SHA256
`33543be01e8a2fcd56dce72c968cd717918d6ea0de9987a3455fa3413cb0b17f`.
Independent size/CRC, 29 master-stream, 4 command-output, exact-source,
nonce/preparation/census and closed delta joins pass. Both native observations
remain failed before their first Project command.

The source-bound producer reports exactly two settled changes, without
truncation: metadata on `.leases` under the selected dependency generation and
under one `pm-runtime/generations/<id>` directory. Linux changes only mtime and
ctime; Windows changes only mtime. The Windows initial scan additionally names
an unreadable zero-byte UUID lease under the selected dependency generation,
with errno 13 at the first read. This identifies the observed member and the
complete settled difference; it does not identify the kernel lock owner, prove
which PM runtime generation was selected, or retroactively identify the missing
member detail in older Linux 539 archives. No lease is adopted or ignored.

The pinned official
[`pm.environments.venv_command`](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/pm/environments.py)
provides a dependency interpreter for a script or `-c` probe. Its source-install
branch returns the supplied venv interpreter and explicit Python options.
The vendor also uses this interface for a
[plugin capability probe](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/plugins_cmd_catalog.py).
By contrast, the existing official `runtime_command` deliberately starts
`hermes_bootstrap`, whose normal source startup selects and leases dependencies.
Using that full native startup merely to host the measurement script creates a
live source-User effect before the observer can enforce the prepared baseline.

Only the source observer now uses `venv_command`. It takes the selected
environment from the already-held source snapshot/facts, never from a new
selection or baseline. The collector checks the pinned API's source ownership
and accepts only `[recorded-venv-interpreter, -I, -B]`, then its own `-c` observer
entry and private child argument. An alternate generation, store interpreter,
payload trampoline, wrapper, extra option, foreign API or non-string argv is
refused. The native launcher's original `runtime_command` provenance check,
actual `HERMES_BIN` commands, source-resolution guards, OS HOME/USERPROFILE and
A/B HERMES_HOME selection remain unchanged.

The interpreter's link chain is resolved only against the original captured
link graph, under its existing source-domain boundaries. Before and after the
API call, the original facts, API source, pyvenv.cfg, executable and all ancestor
identities are rejoined. Pinned bytes are checked through the existing bounded
ordinary-file reader while the original directory handles remain held. The
existing 128 MiB executable bound and original caller deadline remain enforced;
no complete payload is scanned again and no PM state is written. Link targets
must also equal their original captured text. The resulting small observer
receipt names the original selected environment and captured input hashes; it
is separate from the unchanged native launcher provenance and is unqualified.

Five new inert contracts cover the selected interpreter and actual stdlib-venv
observer startup without importing a bootstrap trap or creating a lease, exact
full User/runtime preservation in that fixture, alternate generations and argv,
API ownership, changed input bytes, a replaced generation ancestor with retained
child identities, captured links and foreign-target refusal, retained parents
and the original read deadline. All 103 existing profile test bodies remain
unchanged; 108 profile contracts pass locally on Linux. The stdlib fixture is
not a native Hermes result. The genuine source A/B first/repeat/restart commands,
final User control, native bootstrap recovery and public Project admission must
still satisfy their original guards. In particular, no exception is introduced
for a later genuine User command's lease-directory changes.

### Composition with the retained lifecycle contracts (2026-11-14)

The source-observer and closed-failure changes above are composed with checkpoint
`744a9bb1287cb72a36c4e17d648ea90c03a260fa`. All 104 existing profile test
methods retain their original AST, including full lifecycle continuity, the
known-empty source-file read boundary and the portable pinned stdio recipe.
The new stdlib-venv child writes its fixed output through `sys.stdout.buffer`
so the exact LF assertion does not depend on Windows text newline translation.
The combined 115-method Python suite passed on Linux (114 executed, one existing
Windows-only lease contract skipped). This verifies the composed collector and
inert fixtures; Windows execution and genuine source/bundled native outcomes
remain separate required evidence, with no new qualification from these tests.

### 2026-11-14 — join the public PLUGIN_DATA locator to CLI composition

The completed bundled macOS and Linux jobs of run `43559083280` at exact
checkout/head `c5b94a6157391974de6c3ce3178ec197b15a74a1` retain one closed
failure summary each. Original artifact `14100980339` is 771 bytes with SHA-256
`a813e0b1515a33d10ab7289f1da79d229d833c5d5ababcf5f563661b2cf6778c`;
artifact `14100501544` is 770 bytes with SHA-256
`6b920a18f83db4db3c71a90b09e10c1b5cb141f88eb336ce49b2c81340ee7743`.
Both source/nonce-joined diagnostics identify `public-add-a`,
`public_binding` (function line 1903), and the retained data receipt predicate
at line 1965. The primary observer family settled; public-family cleanup was
not proved. The original raw public state and command output remain withheld.

The public Cobra composition selects `<project>/.uap` as its state namespace
and `plugin-data` as the PLUGIN_DATA base (`cli/cmd/agentplugins/compose.go`);
`PluginDataManager.EnsureData` appends the physical artifact ID and writes the
complete receipt. The installer default uses the same base, and
`bindDataReceipt` retains the produced receipt. The collector and its inert
fixture instead expected `.uap/data`. That incorrect expected locator rejects
a legitimate public receipt before reading its marker.

The collector now requires exactly
`<project>/.uap/plugin-data/<physical-artifact-id>`. Its fixture uses that same
production namespace. It does not accept both paths or follow a locator from
state as a new authority. Existing binding, physical artifact, scope, state,
marker, publication ancestry and package/source checks remain intact. No
production writer, cleanup rule, budget or qualification flag changes.

An inert replay of the original function rejects the production-shaped
receipt with `public retained data receipt differs`; the corrected function
accepts it. The new contract also rejects mutually matching state and marker
at the old `.uap/data` root, a foreign root or a sibling Project's data root,
and rejects a changed marker at the selected root. Wrong-root markers are
never read. These contracts are not a reconstruction of withheld native state
and do not establish a successful bundled or cold lifecycle. The next ordinary
native run must still close every public stage and its original cleanup proof.

### 2026-11-14 — retain the first ordinary source-profile bound refusal

Original source artifacts `14100548260` (Linux, 13,705 bytes, SHA-256
`b7a6b4e7e232de4e81eac43c0afdb54eadc8de0ad705e023e619a0a8f62f9dac`)
and `14100478450` (Windows, 14,102 bytes, SHA-256
`210d2da437ffe4fc81a3629a9cb897af0d9a59e1217b2178bf3e756c2d50ecce`)
belong to the same exact `c5b94a6157391974de6c3ce3178ec197b15a74a1`
source run. Both reach the genuine first `config-path-a` command, which exits
zero and reports the selected A configuration path. Its retained stderr reports
dependency preparation for that data root. The next capture fails with the
original `source ordinary profile bound` predicate in `source_ledger`; the
parent's settled capture fails at the same bound. These originals do not retain
the offending member or distinguish the ordinary-member count from file size.
They do not prove a cache location or authorize another large-tree partition.

The existing first-refusal record now has one additional closed operation,
`ordinary-profile-bound`. It is attached during the same sorted ledger pass,
using the already captured member record and ordinary counter. It reports the
relative member (or its digest when the existing 512-byte representation bound
would be exceeded), typed metadata, selected A/B, original User or other fixture
scope, object kind, ordinary count, and the exact failing count/file-size
predicate. The decoder cross-checks the represented path and scope, mode and
kind, counter and unchanged limits. No file contents, native raw error message,
new read, walk or independently selected baseline supplies these fields.

The original RuntimeError and traceback remain the outcome. The existing
2-KiB closed record, source/nonce/stream joins, original held evidence authority,
exclusive first write and failed-source upload rules also apply. File-read and
absolute-link refusal documents retain their existing closed schemas and byte
representation. The scan deadline, 2,048 ordinary-member and 1-MiB ordinary-file
limits, source/runtime partitions, link and hardlink closure, cleanup rules and
all false qualification flags remain unchanged. In particular, this change
does not exclude leases or add an inferred cache subtree to runtime authority.

Inert contracts exercise all four scopes and each count/file-size predicate
without filesystem reads, reject false or mismatched metadata and bounds, and
preserve the first record and original exception across a second failure or
replacement of the evidence directory. Successful decoding is diagnostic only;
genuine source bootstrap, repeat/restart and recovery still require the complete
native result under the original guards.

## 2026-11-14 — classify the observed official source PM cache

The original Linux source-profile artifact `14106517402` from run
`43574321311`, job `132389771695`, source `440f0666dad9255c1667c86f95654f7d960ffeb6`
is 14,506 bytes, SHA-256
`f6347c58b44213a87220bcd2664be21e8c34120886a798a100a75e693c2ea6db`.
Its retained first `config-path-a` command exits zero. The native family settles
after 64.429 seconds, and the source snapshot refuses ordinary file bytes at
`project-a/.uap/hermes-runtime/cache/uv/archive-v0/1VwFzUGycm98fsvq/watchfiles/_rust_notify.abi3.so`:
1,160,728 bytes, two hard links, ordinary member 210. The diagnostic is attached
after the complete metadata, symbolic-link, hard-link and file-hash capture,
when `source_ledger` classifies captured records. Its null `bytes_read` field
does not assert that the producer never read the file. The original archive
does not embed this file or name its second alias, and retains no complete
accepted post-command User/profile image. All qualification flags stay false.

At the pinned official Hermes commit `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`,
[`pm/packages.py::uv_cache_path`](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/pm/packages.py)
selects `get_default_hermes_root()/cache/uv`. The active custom Project root is
that root. `Venv.apply` uses this cache, while
[`pm/environment.py`](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/pm/environment.py)
removes ambient UV cache settings and assigns the selected cache itself.
Borrowing the real owner's tool store does not relocate this dependency cache.
No cache environment override, copied cache, early native warm-up or changed
User HOME supplies authority for this correction.

The source measurement ledger now groups only each already-selected runtime's
exact `cache/uv` subtree alongside its existing `tools` and `installs` runtime
trees. Every member, original identity, link target and content hash remains in
the digest and coverage accounting from the same complete capture. The `cache`
parent, unknown cache siblings and protected members remain individual ordinary
rows. The full 262,144-member, depth-64, 8-GiB payload ceiling, 120-second capture
deadline, complete link closure, User/sibling preservation, ordinary 2,048-member
and 1-MiB-file bounds, process settlement, native command catalog and evidence
schemas are unchanged. This classifies measured runtime bytes; it does not grant
recovery or production authority over those bytes.

The causal inert filesystem contract creates the observed 1,160,728-byte shape
with a real second hard link in the same Project's dependency generation. The
original classifier refuses it; the corrected classifier preserves complete
member/byte accounting, content-sensitive digests, the pre-effect User/sibling
baseline and protected rows. Separate cases retain refusal for large ordinary
cache siblings and protected files, unknown or cross-User/cross-Project aliases
before hashing, and original root/cache-parent or membership replacement. The
ledger grouping itself performs no additional read, stat or traversal. Genuine
current-source bootstrap, repeat/restart and recovery remain unqualified until
their complete native original is accepted under these unchanged guards.

## 2026-11-14 — preserve the original public inspection control lock

The three original bundled captures at source
`440f0666dad9255c1667c86f95654f7d960ffeb6` reach `public-run-a` and retain the
existing unmeasured-path refusal. Their single-member failure envelopes do not
retain the changed member or native command streams. They therefore do not
identify the historical changed path.

A separate causal replay executes the real public Cobra `run --inspect
--confirm-client-start --target hermes --scope project --project-root P` path
with the existing Project composition and process lock. Only session execution
ports are inert; no vendor process starts. The command succeeds and refreshes
the original `P/.uap/mutation.lock` PID/UTC acquisition record. Its named and
held identity remains unchanged, as do every other fixture member, mode and
byte. The actual Go log has SHA-256
`6551fead920e329e3162e621ab020e55c84d3df20686ad9822b2dd849cbb5915`.
Replaying those exact old/new record bytes through the original Python
inventory and inspect predicate reproduces the refusal for precisely that key.
The replay is a control-lock oracle diagnosis, not Hermes native qualification.

Only the existing `public-run-a` and `public-run-b` inspection calls now hold
the original outer, Project and `.uap` directories and the original single-link
lock across the child command. Before reading, the named and opened objects
must join the original inventory. Complete mode, owner, link count, device,
file identity and applicable birth/Windows attribute/reparse facts remain
separately pinned in both stat domains. The existing no-follow directory/file
openers remain responsible for platform handle admission.

The lock body must be at most 256 bytes and exactly the existing writer's
ordered compact JSON plus one LF: a positive integer PID below `2**32`, and a
valid UTC RFC3339Nano acquisition timestamp with the writer's fractional-second
spelling. Duplicate or extra keys, Boolean/numeric coercion, alternate spacing,
offsets, invalid dates and extra data refuse. The same held readable descriptor
captures the postimage before the ordinary whole-tree snapshot. Its original
named/held authority and exact captured size/hash must still join afterward.
Only then may that exact file key differ in the existing inspect predicate.
File size/mtime/ctime may change with this record refresh; no sibling, state,
receipt or entire `.uap` subtree is admitted by this rule. Direct native MCP
commands receive no such allowance. Child and capture failures keep their
original exception, and all held descriptors close.

Inert contracts execute the actual nested command through its delta guard for
both Projects, replay the recorded writer bytes, and preserve refusal for
native-call lock writes, other labels/argv, malformed records, state or sibling
changes, replacement and aliases before body reads, and post-capture drift.
All 123 prior test methods remain unchanged. POSIX replacement tests exercise
open original parent handles; Windows directory handles retain their existing
no-share-delete policy, with no close/reopen workaround. Native execution,
command counts, evidence schemas, deadlines, payload/profile limits and every
qualification flag remain unchanged. Current complete native originals are
still required for public lifecycle acceptance.

## 2026-11-14 — show only already closed failure metadata in the job log

The current `40c6bb2d27a9282755ec3a739c236eb9dc2875f2` macOS source job
`132408965958` from run `43582150782` again refuses the absolute intermediate
link at `payload_links.parts`, before the payload hash pass or observer start.
Its existing source artifact `14109111276` is 1,666 bytes, SHA-256
`2a451fbcf95ab361c2f04b9664bf43c677f46cc8967b47c50b67db00808cb961`.
The job log and official metadata were read; this change does not claim to have
decoded that ZIP or identified its member. The existing diagnostic already
retains the bounded relative member, target digest and original nonce.

The same run's macOS bundled job `132408965881` completed official build cleanup
and then refused public native cleanup. Its separate safe artifact
`14109300946` is 764 bytes, SHA-256
`7cb9e279038fe0382a116d324c4fcbc5397263b76b5c56ee5d35285c210dbb79`.
Linux source job `132408965934` reached an observer failure with settled cleanup;
artifact `14109567941` is 17,208 bytes, SHA-256
`2120cc3d3f678d4fc2441dcdcac3e78463a53a30440f20a412cbda2b620aa47f`.
Neither inner failure is inferred from these metadata alone.

The existing stage loop retains its already read admission-proof body in memory.
Only after the original membership/identity checks, staging rename and
`EVIDENCE_CLOSED` does one optional `HERMES_ADMISSION_FAILURE` line project
validated failure facts. The complete ASCII line, including its fixed prefix
and newline, is at most 2 KiB. Source refusal output retains only its existing
closed fields: bounded relative member, digests, counters, metadata and finite
operation/stage. The full failed source proof must carry the same nonce and
unchanged clean source identity. No target text, error message, environment,
configuration body, traceback path or raw nonce is printed.

For an unclosed native family, the existing `native_failure_context` validator
supplies the finite cleanup phase, settlement flag, exit code and optional
observer phase, exception kind, command and own-source caller. For a settled
observer failure, the same context is passed from retained parent locals through
an optional stage argument and joined to the staged proof's source and nonce.
This argument adds no artifact field and reads no stream or path. Both uses
share the one context assembled from the existing locals; original safe-failure
artifacts and their validators remain unchanged.

Malformed, absent, mismatched, qualified or successful inputs omit the optional
failure line. Log failures cannot replace the original native error or undo
publication. Successful output and the existing census line remain unchanged.
All output is measurement-only with native/admission qualification false.
There is no new reader, workflow job, artifact schema, scan, retry, ownership,
link allowance, cleanup authority or changed deadline.

The six added inert contracts cover the actual captured intermediate-link
exception without target reads, nullable and mismatched observer context,
source/nonce/caller joins, post-rename emission with exact existing read counts,
settled-source context without artifact changes, unchanged successful output,
failed publication and failed stdout preserving the original error. All 130
existing test bodies are byte-for-byte retained. These new tests, Python
compilation and genuine native outcomes are unexecuted in the unavailable local
executor; source review and immutable source readback do not replace those gates.


## 2026-11-14: hash-only public bootstrap refusal detail

The original Hermes jobs at source
`1412f416b9bf70b227d8f517e1881075f2b05a38` now identify the same
`public_profile` membership predicate on all three bundled platforms. Linux
job `132428496650` and macOS job `132428496354` stopped at `public-stdio-a`;
Windows job `132428496528` reached `public-http-a`. Each genuine invocation's
closed stdout record identifies the original function at line 2125, predicate
at line 2135, `cleanup_phase=public-family`, settled primary observer family,
and `RuntimeError`. These facts do not distinguish more than five admitted
backup names from an unexpected profile member. Public cleanup, bootstrap
admission, and native qualification remain unproved.

The raw original logs are respectively 233617, 200763, and 281477 bytes, with
SHA256 `f8dd78af06278ef1c8efcf453cb7bcd65bb7c77128d2c514cf2a883531b70538`,
`9ed5410ea8a5f18b0a1fbba9ddb8183c37e0719300e5c3df0a692fd813e88d1a`,
and `a82ba8fe18a99b8e0e40b04b9a8dc3de911467c62fef847e900f90452d54b64a`.
The lines were isolated within each actual profile-admission invocation,
excluding the synthetic diagnostic output from the preceding inert tests.
The bounded original ZIPs have not been read in this source-only work.

This change retains the exact predicate, error, inventory limits, and
allowed members. On that predicate's existing `RuntimeError`, it can attach
an optional closed `public_bootstrap` detail derived solely from the already
captured ledger. Its fields are the selected `a` or `b` scope, a finite
`backup-count`, `unmeasured-member`, or `both` predicate, exact captured member,
backup, and unexpected-member counts, and at most the first lexicographic
unexpected row. Count/predicate consistency is mandatory. The unexpected
name is represented only by SHA256 over the domain separator
`uap-hermes-public-bootstrap-member-v1\0` and its UTF-8/surrogatepass bytes.
No raw member name, target, profile content, absolute path, or nonce is added
to the detail. A strictly shaped ordinary file or directory row contributes
only type, mode and byte count; directory byte count is null. The private
captured ledger retains its exact content hash for integrity validation, but
no foreign file-content digest is exported: a deterministic digest could
expose low-entropy data by offline guessing. Invalid optional metadata is omitted.

The existing observer record gains this one optional nested field; old
records keep their exact closed shape. Its validator requires the actual
owned `public_profile` code, public phase, `RuntimeError`, and an existing
closed public command for the same selected scope. The observer emitter
falls back to the original diagnostic if the optional detail is malformed
or cannot be bound. The original exception still escapes. The original
retained descriptor, positive primary-family settlement, source/nonce joins,
canonical encoding, and 2 KiB observer bound continue to govern transport.
The existing failure stdout projection carries the validated detail only
after the same closed-evidence stage. Its 2 KiB line bound and false native
and admission qualifications remain unchanged. There is no new file,
filesystem read, traversal, process, cleanup permission, or recovery rule.

Four appended inert contracts exercise the actual `public_profile` guard
for each finite predicate, exact first-row hashing and no additional I/O;
strict count/metadata negatives; actual owned nested command, caller, phase,
and scope binding with optional-field fallback; and retained-descriptor plus
safe/settled failure projections with identity, settlement, and size bounds.
These new contracts have been authored for source review only; local Python
and new native execution have not been claimed.

The incoming peer's separate `extra-lock-prefix` fixture correction is
preserved in full. In macOS original job `132428496678`, the stricter earlier
`public control original parent changed` check rejected both `a` and `b`
variants before the previous final snapshot predicate. The production
parent guard, other variants, exception types, call counts, and original-lock
identity checks are unchanged by this diagnostic patch.

## 2026-11-14: join the pinned stdio log and repaired inspection

The pinned vendor `tools/mcp_tool_config.py::_write_stderr_log_header` writes
`logs/mcp-stderr.log` for each stdio session. Direct execution of that unchanged
writer from Hermes `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19` reproduced the
81-byte timestamp/server header. Its source-file SHA256 is
`02d05318c60b5f203610185e9ca152ec15276420d3d11060cbbef206cd651797`.
This is a causal producer replay, not a new native lifecycle result.

The public fixture now admits only this exact header-only member after the
existing nonce/session-correlated stdio event check succeeds. Original captured
file identity, mode, byte count and SHA remain required; the file is capped at
512 bytes. Every byte must match the pinned timestamp/server grammar and host
newline, with one header per completed owned session (at most three A and one B).
Foreign names, child stderr, extra headers, invalid dates and post-snapshot
changes refuse. No unmeasured directory or other log receives an exception.
The verified count survives binding removal with the retained profile, while
every command without stdio must preserve the exact existing log row.

The already implemented repair journey invokes `public-repaired-run-a`; its
inspect validator previously accepted only `public-run-a` and `public-run-b`.
The allowlist now admits that one existing A action with the same fixed argv,
original held control lock and all former effect checks. Repaired B and mismatched
scope/role variants still refuse before execution.

All 140 incoming test bodies are preserved. The three new contracts cover the
actual profile predicate, the real command's protocol/profile join across
removal, and repaired inspection through the existing lock fixture. The final
suite ran 143 methods successfully, with one existing Windows-only skip on
Linux; that skip is not Windows evidence. Source review and this replay do not
qualify the new native path. The incoming closed bootstrap diagnostic is retained
for the distinct unmeasured Windows HTTP effect. The source-installed original
User bytecode/UV-lock refusal remains unchanged and unresolved.


## 2026-11-14: existing settled source User delta in closed stdout

Original source-native run `43588580245` at
`1412f416b9bf70b227d8f517e1881075f2b05a38` now has the same first cold
`config-path-a` refusal on Linux job `132428496636` and Windows job
`132428496582`. The exact owned call site iterates scope `a` then `b` and
passes that same scope to `profile_effect_boundary`. The closed records
identify its original line 1514, where the scope predicate is true and the
captured User subtree differs from the original baseline. They do not name
the changed member. Raw log SHA256 values are respectively
`b20df2acf51069427af8ce2b5b614889e9cd004ca8931831213d225410c81d4b`
(117627 bytes) and
`45697ec053c5e09b0c704a0eb59f697818e58786c58383e9123a1de68d145f16`
(138457 bytes). Source artifacts `14113135153` and `14112575305` remain
unread here; no member identity or specific bootstrap cause is inferred.

The existing source User delta writer already compares complete retained
before/after records under the original deadline. It keeps at most eight
lexicographic changed rows, exact total count and truncation, normalized
metadata and digests, and User summary hashes. Its 16 KiB schema, held
evidence publication, existing member encoding, and every admission guard
remain unchanged. The existing staging loop already reads and validates
this file, joins its nonce to the failed source proof, and joins both User
summaries to the prepared and final runtime snapshots before publication.

This change passes that same validated in-memory value to the existing
closed failure readout after the evidence commit. An optional
`source_user_delta` stdout field repeats the failed source, unchanged source
identity, nonce, both User summary, and positive primary-family-settlement
joins internally. Public output contains only the before/after entry counts,
not the aggregate User inventory digests. It selects the first retained row
and the earliest retained file/link
row on either side, deduplicating index zero. The output retains exact
indices, total/retained counts and truncation; absence of a file/link among
truncated retained rows never means none changed elsewhere.

Each selected row exposes only the existing `member_sha256`, before/after
finite kind, saturation flags, and names of changed normalized fields. The
existing `utf-8-surrogatepass` hash is preserved; it is not the domain-separated
hash used by the separate public bootstrap diagnostic. Raw member names,
targets, contents, identity numbers, nonce, aggregate inventory digests and
individual file-content or link-target digests are omitted. Only the names
of changed digest fields may appear. Saturation is
explicit, including the case where changed raw captured identities become
equal normalized records and no changed field can be reported.

The original failure line is constructed and checked first. Missing,
malformed, unjoined, or oversized optional data returns that exact original
line, including any existing public bootstrap detail. The 2 KiB stdout cap,
false qualifications, retained original error, artifact membership, writer,
schema, process/cleanup decisions, source User baseline and native command
catalog do not change. There is no new read, walk, member or process.

Four new inert methods cover the real captured-map delta producer with
directory-first/file-second selection and no-I/O sentinels; retained
truncation, create/remove, link and saturated-identity cases; malformed and
mismatched inputs plus actual line-size fallback preserving the public
detail; and existing stage read counts, commit-before-output, exact copied
bytes, nonce mismatch and late-read refusal. All preceding 143 test bodies, including the bd63 public content-hash privacy
correction and the incoming 880d stdio-header/repaired-inspection contracts,
are preserved. These new tests and native execution are pending; this
source-only correction supplies diagnosis and grants no new admission.

## 2026-11-14: official source runtime candidate with explicit bytecode suppression

This candidate retains the failed direct-launcher observations. It does not
qualify the unchanged published shell/exe launcher, source recovery, or public
Project capability. The implemented candidate uses the selected installation's
official runtime export, with the single explicit interpreter option `-B`.

At vendor `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`,
`hermes_cli._launchers.print_runtime_command` is the machine boundary for an
exact published launcher. Its readout branch sets `sys.dont_write_bytecode`
before importing vendor modules, prints the command, and exits before ordinary
`hermes_bootstrap`. Interrupted-update recovery is an explicit exception; no
new runtime readout is performed inside the measured lifecycle. The workflow's
existing pre-capture readout remains the original authority.

The actual pinned API, called only as a generator with the original Linux
artifact 14113135153's recorded source and store Python, reproduces the complete
readout bytes with SHA256
`bd17811b72e8568b3a9d3b098d35df99f14a088e93ade2cf547a5c7b560a9512`.
That is the original report's `runtime_command_sha256`. This generator replay
is not native execution. The output is exactly `[storePython, -I, -c, code]`;
`code` retains the source path, home pin, `hermes_bootstrap`, dependency selection
and real `hermes_cli.main`. Insert only `-B` after `-I`, then append the unchanged
CLI arguments. The vendor's `venv_sync.relaunch_command` preserves that option
through `sys.orig_argv`. Never substitute the observer's venv-only route.

The parent freezes this original prefix once after the original source/API
join, including the readout identity, bytes and original parent authority. The child joins its original source inputs to the frozen prefix; it cannot
choose a new interpreter from a later receipt read. Retain the first child
capture's already collected private records only after its User digest equals
the parent's prepared digest. Before each source native launch, keep the full
original User/source capture check and rejoin the original launcher, selected
store-Python link chain, both PM facts, runtime API source and their ancestors
with the existing `payload_links` and `verify_source_capture_paths` helpers.
Hash the captured regular inputs through those original held authorities before
and after API evaluation, with the original lifecycle deadline. Require the
API's complete argv to equal the frozen original prefix plus exact CLI tail.
A replaced interpreter, receipt, source/API, link, ancestor, fact or command
shape refuses before launch. Even a same-bytes readout replacement between
parent selection and the first child capture refuses. No new resolver, owner or baseline is created.

Author one private `tmp` directory inside each original selected A/B runtime
root before the baseline. Record and recheck its original directory authority.
For source Project commands and resolution probes only, pass TMPDIR/TMP/TEMP
pointing to that directory before bootstrap. Keep HOME, USERPROFILE and selected
HERMES_HOME unchanged. PM preserves explicit temp variables; the vendor's later
scratch hook respects them. Final source User control uses the same official
`-B` runtime prefix with the original User environment and strict User equality;
it receives no newly authored User temp root. Bundled native commands retain
the existing literal launcher route, and the source observer retains its current
selected-venv measurement route.

The source runtime snapshot and final report distinguish the original
published launcher/readout provenance from the actual official `-B` argv.
Actual command status records retain full argv and original streams. No new
artifact, cleanup permission or capability flag is needed. Meaningful contracts
cover exact prefix/source/interpreter joins, substitution before launch, bytecode
suppression through the real isolated Python mechanism, and A/B temp authority
without User or sibling redirection. The native matrix remains required.

The original Linux source refusal has seven complete User deltas: the PM
pycache directory, two new pyc files, User tmp metadata and three empty UV locks.
Windows has those plus mtime on the shared source `.hermes/bin` directory. The
pinned source joins config-path bootstrap to `expose_cli`, then
`_expose_windows_user_bin` and `ensure_install_launchers`: the same owner Python
is deliberately not kept by `_kept_shared_launcher`, and mint stages a temporary
candidate under the shared bin even when executable bytes match. The original
Windows artifact also records new A/bin launchers. This is a source/effect join,
not a new native call trace. `-B` and selected tmp routing do not authorize that metadata change.
Any repeated shared-bin effect remains a genuine refusal; no warming, mtime
normalization, disabled lazy bootstrap, or shared-User allowance is proposed.

The fixed resolution probe shares the same original-input fences. It admits
only the existing constant `GUARD_CODE` with no CLI arguments, and its command
record names `fixed-resolution-probe`. Every actual source CLI command, including
final User list, retains the complete exported main bootstrap. TMP directory
authority is rechecked immediately before spawn after API evaluation and after
child settlement; legitimate selected TMP children are captured and permitted.

Local verification on this candidate ran 150 Hermes methods: 149 passed and one
existing actual-Windows CRT lock test skipped on Linux. All 143 preceding methods
remain present. Seven new methods cover the source command and original readout
fences, API/tail/interpreter/ancestor substitutions before and during generation,
original store links, parent-to-child same-bytes readout replacement, actual nested
command TMP/User routing, and selected-temp/sibling/User refusals. The existing
real isolated-Python test now also exercises the source prefix: an environment
bytecode setting is ignored under `-I`, while explicit `-B` prevents pyc writes.

The pinned vendor `relaunch_command` was also extracted without imports from
`hermes_cli/venv_sync.py` (file SHA256
`7688851ccadc994bc3b799a4a25577edaf465b7553b8887892c8b36e1811ffe0`,
function SHA256 `7dae8448b8e0fceedd549770300f61e3cd63997934d82b280bdd56092b5def93`).
Its actual generator preserved `[-I, -B, -I, -c]`; a real isolated Python child
reported `True 17` and created zero pyc files. This verifies option propagation,
not native Hermes acceptance.

`native_launch` is added inside the existing source provenance document; the
original `native_command` remains the published-launcher path. The existing
`admit_evidence`/`stage_evidence` consumers retain the bounded closed member set
and copy this report unchanged. A contract stages before/prepared/after source
provenance with the added field and verifies byte-identical publication and all
qualification flags still false. No diagnostic schema, artifact member, public
capability, cleanup permission, or source User effect allowance was added.
Native execution of the frozen candidate is still required; Windows shared-bin
metadata remains an expected strict refusal unless independently resolved.

Independent R2 accepted the exact four-path packet relative to
`880d4c885191de21a8c64c0aeaead006fab06f6f`, patch SHA256
`e4ab34496c2ee783cc97787ce9ad766eee030f3aa1c0872b8d303d8af8bce1c3`.
The reviewer rejoined all base/current blobs, reran the seven new methods
successfully, and checked the frozen hashes again. Root reviewed the same
packet, applied it with exact content-hash verification and ran 150 methods:
149 passed and the same existing Windows-only case was skipped. The workflow
YAML parses and diff checks pass. These checks do not waive any native gate.

The final composition preserves concurrent commit
`9b9a94040d2b6ec84375e1aabaf593740abf5ac9` and all four new captured-delta
contracts. Independent review accepted its source/nonce/settlement and complete
User-summary joins alongside the new provenance field. Root verified unchanged
added/removed source-runtime lines after composition and ran 154 methods:
153 passed, with the same Windows-only skip. The closed diagnostic remains
optional; unknown effects still refuse and no native acceptance is inferred.

## 2026-11-14: require the calendar error type in the stdio-header contract

The original macOS User/source job `132449810970` in run `43595758794`
checked out `880d4c885191de21a8c64c0aeaead006fab06f6f`. Its original
113,620-byte job log has SHA256
`3ffc0605e1114b09d554bff7c339edcf50a58b8492babc841cd957fc8c37358f`.
The 143-method prerequisite failed only the `bad-date` subcase of
`test_public_stdio_log_joins_only_completed_owned_sessions` (one existing
Windows-only case skipped). The guard correctly rejected the authored date
`2026-02-30` with `ValueError`; its message was
`day 30 must be in range 1..28 for month 2 in year 2026`, whereas the test
required the older substring `day is out of range`. The enclosing runner
traceback identifies CPython 3.15.2, while the native version output and the
actual child argv identify the selected CPython 3.14.9; those observations do
not identify the enclosing interpreter as the source of the child message.

The same test method remained byte-identical through the current Hermes source
on `087b8fed2c76dd2e2c22715451f5ca4ea9b8e44d`. The correction requires
`ValueError` for exactly the existing `bad-date` mutation without matching its
wording. Every other negative retains the `stdio|unmeasured` message check and
now requires `RuntimeError`; the two positive cases, all authored mutations,
and the production calendar/header checks remain unchanged. No new bootstrap
member, stdout content, or failed native observation is admitted.

This two-path candidate has source and exact preimage/inverse checks only.
Execution of the corrected method and current native workflows is still
pending. The older 880d failure does not exercise the later source User
readout or the official source bootstrap `-B` correction.


### 2026-11-14 — prospective public stdio spawn-ledger evidence

The original bundled macOS run at source
`880d4c885191de21a8c64c0aeaead006fab06f6f`
(run `43595758794`, job `132449810814`) remains a failed admission.
Its closed first-refusal report reached `public-stdio-a` and
`public_profile`: 22 selected-root members, one config backup, and two
unmeasured members. The first was an ordinary 0600 file of 486 bytes.
The report's domain-separated relative-name digest
`171f4c723dfacb549c6650b433e39cab3cb614dbe559c070f56d4e3493f87bcf`
matches the exact pinned vendor literal `spawn-ledger.json`.
The original full log is 200,059 bytes, SHA-256
`467da768bf8085635cf32916a0820671601f42dd4e68597115b738086b4dad39`.
Artifact `14115072289` is 920 bytes, SHA-256
`b1110a3e661c0659b17667a611d4ca3b402324ee319e3c00bab29499dc3fff8e`;
its ZIP has not been decoded in this review. The second member's name,
mode, and bytes are not established by that report.

Pinned Hermes `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`
registers each stdio child through `process_identity.register_child`.
The sole ledger writer prunes on append, writes a fixed sixteen-field
entry, and uses an empty `spawn-ledger.json.lock` sidecar. The pinned
`pm.filesystem.lock_fd` implementation locks without writing a byte.
Normal stdio retirement clears in-memory tracking; it does not remove or
rewrite the machine ledger. These source facts support the following
prospective, bounded oracle. They do not establish the old artifact's
unobserved sidecar, current process ownership, process birth, or cleanup.

The public fixture now opts in to one additional fact on its existing
`initialize` and `tools/list` event records:
`{"kind":"hermes-public-parent-v1","parent_pid":<positive integer>}`.
Only exact `UAP_TEST_HERMES_PUBLIC_PARENT=1` selects it. All other
values preserve null facts on these methods; the six top-level event
fields, RPC sequence, replies, and existing `tools/call` facts remain
unchanged. The existing real compiled-probe subprocess test exercises
absence, empty, disabled, malformed, padded, and exact values. The
bundled workflow explicitly runs that probe package's unit tests, with
a 300-second command limit, on its existing three OS rows before the
existing build. Its two existing path filters include the probe directory;
jobs, acquisition pins, permissions, and native deadlines are unchanged.

Admission reads only these two fixed names below the original
receipt-owned `public-a/.hermes/uap-session` or
`public-b/.hermes/uap-session` root. The complete already captured
directory chain and file rows are joined before bytes are read. POSIX
reads remain relative to held directory descriptors; Windows keeps the
existing no-delete-sharing ancestry handles. Both files must be ordinary,
single-link, same-owner objects with exact captured identity, size and
SHA-256. The ledger is at most 4 KiB and exactly one JSON entry with the
sixteen pinned fields. The lock must be empty, 0644 on POSIX or 0666 in
the Windows stat domain; ledger mode is 0600 on POSIX or 0666 on Windows.
Unknown, partial, corrupt, temporary, extra, or substituted members refuse.

Every accepted row must join the current native command's exact status
argv, positive PID, zero exit, absent error, and both complete stream
receipts. It must also join exactly the two new events of that command,
with the original nonce, new session, same event PID and parent fact.
POSIX joins the ledger child to the probe PID and its parent to the native
status PID. Windows joins the ledger child to the probe's observed parent;
the probe remains a distinct child. The only allowed complete argv forms
are the receipt-owned eight-token encoded managed launcher, and its
one-token pinned probe after POSIX exec. There is no name-only, substring,
argv-only Windows, or previous-session fallback.

The expected installation ID uses the parent-selected bundled code path
passed as `HERMES_PROFILE_RUNTIME_ROOT`, with its exact bundled bin sibling.
That selection originates in the existing `runtime_before` payload
contract. A fresh directory tuple records continuity within the public
phase; it is not a persisted original receipt. The pre-existing complete
`runtime_before`/`runtime_after` payload comparison still supplies the
original immutable runtime check. The official source checkout is never
substituted for the selected bundle's `__file__`-derived code root.

The first owned stdio requires absence of both files in the preceding
measured image. Later owned stdio may replace the single ledger row only
after a current-command event/status join, while retaining the original
lock's full metadata and bytes. All non-stdio commands, sibling commands,
update, repair, removal, removed inventory, and the final cold check keep
both accepted file rows and full captured stat identities unchanged.
The accepted baseline advances only after all existing command/profile
checks pass. Ledger PIDs, timestamps, pruning, and parent facts never
authorize signalling, deletion, cleanup, or a production capability.

The new Python contracts cover current versus replayed events, POSIX and
Windows parent chains, all fixed schema/default/type bounds, exact
command and stream joins, held original-parent substitution before body
reads, changed/aliased/oversized/partial leaves, preserved sidecar identity,
non-stdio metadata drift, complete launcher arguments, and unknown
bootstrap members. Two existing test methods and the existing inspect
command fixture receive only the required opt-in/ledger contexts; their
previous attack sequences and assertions remain. The original completed
stdio/removal test executes the real new guards and baseline update.
No guards are omitted from its command AST.

This candidate has source review only while the execution environment is
unavailable. Go tests, Python tests, formatting, and a new genuine native
replay have not run on these bytes. Existing primary-family, public-family,
cold-recovery, full-source, original-User, sibling, and final admission
checks remain mandatory. This change grants no production or native
qualification and does not relax the separate source-runtime boundary.

Two older prerequisite failures are separate from the ledger observation:
macOS job `132449810970` ran 143 methods in 5.190 seconds with one failure
and one skip; Windows job `132449810996` ran 143 methods in 13.511 seconds
with one failure and four skips. Both used the authored invalid date
2026-02-30 and reached a changed `ValueError` message before User/source
capture. Their selected child interpreter was Python 3.14.9; the outer
wrappers were respectively 3.15.2 and 3.12.10. The existing correction in
`282995` is retained, without another fix or a native-pass claim.
The corresponding raw-log SHA-256 values are
`3ffc0605e1114b09d554bff7c339edcf50a58b8492babc841cd957fc8c37358f`
(113,620 bytes) and
`aba9d666a3287104354ee723f09b2af0d82ae9f7669eaed97d4cb4a08a0ae378`
(134,694 bytes).


The subsequently completed original Linux bundled job `132449811032`
from the same `880d` run independently reached the same first predicate:
22 members, one backup, two unmeasured members, with the same
`171f4c72…` relative-name digest for a 0600 file of 514 bytes.
Its raw log is 233,900 bytes, SHA-256
`3be584d2df266ba55f7b87bb78bc80b6e55d803cbe0bad71afbdbb16a4431020`;
artifact `14117135844` is 919 bytes, SHA-256
`d82d58d9181ed8afc830b3d0992b53efd71caada855ed882b6a073ba1db619c3`.
Independent evidence receipt `2ac527483bf85c70000734d50c50414da0f80039`
records this source/metadata join. The ZIP is unread, the second member
is still unknown, and final cleanup/admission remain unproved.

## 2026-11-14 — Windows fixed agent-log sidecar after an owned native MCP test

The completed original Windows bundled job
[132449811109](https://github.com/777genius/universal-agent-plugins/actions/runs/43595758794/job/132449811109)
used UAP source `880d4c885191de21a8c64c0aeaead006fab06f6f` and vendor
`2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`. Its original raw log is
280,300 bytes, SHA-256
`c4581d254270fd61d2f14a6590c78604239bb0fe7c6847767a865b4b711bdbfb`.
The actual closed failure frame, isolated from inert prerequisite output,
reached `public-http-a` / `public_profile` with 19 selected members, one
backup, and one unmeasured ordinary file: mode 0666, zero bytes, relative-name
SHA-256 `d6503b34655ee420b82058507655f28a09177f18851cd32ea54eb4fbe234d256`.
This is exactly the existing domain-separated hash of
`logs/.__agent.lock`. Artifact `14117700734` is 917 bytes, SHA-256
`15d825ee3ba2456613a3a1ad7147b94c9b48d5324e718e25581b9fd59c009234`;
its ZIP remains unread here. The primary family had settled, but final
public cleanup was unproved and raw evidence was withheld. This is a
distinct first HTTP failure, before the later stdio spawn-ledger gate.

The fixed name and empty-file behavior are joined to primary source:

- Pinned Hermes [hermes_logging.py](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_logging.py),
  Git blob `2651bda7fa524c8ad6c2aeb26a7c5b144751d212`, selects the Windows
  concurrent handler, supplies selected-root `logs/agent.log`, and names
  `logs/.__agent.lock` explicitly. `hermes_cli/main.py` initializes this
  logging route before the actual `mcp test` dispatcher. The particular
  log record that caused lazy creation is not present in the safe frame
  and is not inferred.
- The pinned pyproject and actual build output agree on
  `concurrent-log-handler==0.9.30`. Its official tag resolves to commit
  `63d1566acf898586580a70a947900b6822e9f5a2`;
  [handler source](https://github.com/Preston-Landers/concurrent-log-handler/blob/63d1566acf898586580a70a947900b6822e9f5a2/src/concurrent_log_handler/__init__.py),
  blob `29ce51b62cdf34253cbef7e889324561aca5c23a`, derives
  `.__agent.lock`, opens it lazily with exclusive creation or existing
  `r+`, and keeps it after normal unlock. The selected non-timed handler
  does not write a rollover stamp into this sidecar.
- Actual dependency `portalocker==3.2.0` resolves through its official tag
  to commit `7415a5d20aa64ac347b0c734915ddbe49ce844f3`;
  [Windows locking source](https://github.com/wolph/portalocker/blob/7415a5d20aa64ac347b0c734915ddbe49ce844f3/portalocker/portalocker.py),
  blob `006cb57571e1f0e3d86cf7f1d8370c02200d961f`, locks/unlocks the
  existing handle without writing file content.

The collector admits only this one Windows member, only after prior
absence and one of the eight successful owned HTTP/stdio test labels:
the initial A/B tests and A restart/repaired tests. Existing native exit,
selected configuration, exact HTTP initialize/list or fresh stdio event
joins, source receipts, and sibling/data guards run first. Inspect,
add/update/repair/remove, removed inventory/tests, sibling operations,
POSIX, and all other labels cannot introduce it. In particular,
`logs/.__errors.lock` is not admitted.

Admission reuses the existing ordinary-file and held-directory helpers.
The original outer, Project, selected-parent, and selected-root authorities
are rejoined before body access. The logs directory must match its
original captured row and retain its directory identity, mode, and the
existing owner fields. Each read holds and rejoins all five directories
with their full before/after metadata. The exact sidecar must be an
ordinary single-link 0666 file with zero bytes and the empty digest.
Its full stat identity and bytes remain unchanged across every later
command and the final cold check. Logs-directory size/mtime may change
between commands when already admitted members such as `mcp-stderr.log`
are created; they remain exact within each held read. This adds no
general lock registry, file deletion, process signalling, or cleanup
authority. Existing spawn-ledger and final admission requirements remain.

Seven focused inert methods cover the original name hash and all eight
roles, rejected roles/flags and POSIX, the unchanged rejection of an
unproved or errors sidecar, original ancestor substitution before bytes,
ordinary-file metadata/body/alias failures, full sidecar continuity,
a directory change during its held read, and the actual command's
post-protocol admission block with missing protocol/owner/preimage and
sibling attacks. Four of these methods explicitly require Windows for
its actual named/held filesystem semantics. The prior 164 methods and
all their assertions remain; two existing command-AST fixture contexts
only gain the new helper definitions and initially empty sidecar state.
The incoming peer's Windows row regression gains the original-directory
context and a fixture wrapper that forms the held receipt; all its prior
assertions and hostile rows remain. There are now 171 methods, without claiming they have executed.

This is a source-only candidate based on `bcb5182f35a464e83e5c79abb020cd257a65bda9`.
The runtime, Python tests, and genuine native replay still require CI.
Original source/User invariance, payload identity, budgets, public-family
cleanup, cold recovery, false qualification flags, and the existing
raw-withheld failure path are unchanged. Original audit receipt
`4a0f77c9f647c8152188eea0a21d2318345f078b`, R1
`3be91ae8d8e14a7a627d6c8ecd94703ecc69fe94`, and independent R1
`bb6d7299c9c4070edb8021898bef0fb2f5035d78` preserve the source/evidence
boundary; none is a new native-pass receipt.

The intervening peer correction in `bcb5182f` is preserved as one route:
its `PUBLIC_NATIVE_LOG_LOCK`, `native_log_locks`, and `log_lock` parameter
remain. Its leaf checker is called only from the original held-directory
capture and returns the full stat receipt. The previous row-only early
state update is replaced by the successful command's prospective receipt,
committed after all profile guards. The peer's selected-session plan is
unchanged; no parallel sidecar registry or second admission path is added.

## 2026-11-14 — Windows readout-parent negative fixture

Original source/User job
[132473514608](https://github.com/777genius/universal-agent-plugins/actions/runs/43603223618/job/132473514608)
on `9278aed97af3278cc50f6423bea9bb738e5354c3` ran 163 inert methods and
failed only the `during / readout-parent` subcase of
`test_original_readout_runtime_files_and_ancestors_are_rejoined_before_and_after_api`.
The original log is 138,637 bytes, SHA-256
`224bbaa1a7a81ce83a812e02d2715a449ff9ded1c53de3ca4bda355c5382f06c`.
Windows returned `PermissionError / WinError 32` at the attempted directory
rename. The existing `held_census_evidence` handle deliberately omits
delete sharing, so the attempted substitution never happened.

The test now requires exactly that Windows kernel refusal for this one
subcase, including the original rename source/destination, absent moved
directory, unchanged original parent/file identities and readout digest.
The same rename must succeed and be restored after the method exits,
checking that the held-directory handle was released. The other nineteen
before/during substitution cases retain the existing RuntimeError checks,
and every case retains its exact API-call count. No subcase is skipped and
no exception is swallowed.

Only this test method and this note change. The runtime, native argv,
readout capture, source/User guards, cleanup and qualification remain
unchanged. There are still 171 authored methods. This is a source-reviewed
candidate; Python and genuine Windows/native execution remain pending.

### 2026-11-14 — retain the original command and pair through admission

The exact incoming `9278aed97af3278cc50f6423bea9bb738e5354c3` was executed
locally: 163 Hermes methods ran in 6.764 seconds, with 162 passes and the existing
actual-Windows CRT-lock test skipped on Linux. Its unchanged compiled Go probe
package passed, including all six parent-fact opt-in cases. This supersedes the
preceding source-only execution status; it is not native Hermes acceptance.

Independent review then reproduced three remaining boundaries on those exact
incoming bytes. A real inert bounded child's same-byte status or evidence-parent
replacement was accepted by the late status reader; changing only its status PID
returned a value different from that original child. This was a status-helper
reselection, not a demonstrated bypass of the separate event/ledger chain.
A ledger growing after open consumed 33,260 bytes under a 1,048,577-byte read
request before refusal, despite the advertised 4 KiB member bound. Finally, an
in-place ledger mutation after its read and before the lock read left directory
metadata unchanged and passed that pair/entry boundary with stale bytes. Later
whole-tree checks could detect persistent drift, but did not close this local
admission boundary. The original counterexample report has SHA256
`f2413616beb4bf4b9d1940f87667ca3cc0373e175c155d6bb3764d016b7981a1`.

Root and independent R1 accepted the exact correction plan, SHA256
`124c738ac286eff1dd28bbc128e2f97a9dd4cbc71cde0ef78335d9322c0dd9e4`.
The existing status helper now freezes status and both stream files immediately
after the completed bounded stdio call, before the following snapshot. It uses
the original prepared evidence-directory authority, exact actual argv/server,
first integer PID, zero exit, absent error, complete stream receipts and returned
stdout. Every later admission check rejoins those original file identities,
metadata and bytes under the retained parent. No later JSON decode selects a PID.
Missing original authority remains a refusal.

The existing ordinary reader gains only an optional narrowing bound for unpinned
reads. Explicit zero handles the empty sidecar; defaults and pinned runtime
reads retain their old behavior. The status read requests at most 16 KiB plus
one overflow byte, the ledger 4 KiB plus one, and the empty sidecar one byte.
Original stream counts bound their reads. All new reads and parsing retain the
same original lifecycle deadline, checked before and after use.

Both fixed members are rejoined after their initial reads, including a final
full metadata comparison so an earlier member cannot change during the later
read. After validating the current nonce/session/platform parent chain, the
command rejoins that exact pair, original status/streams and original bundled
code authority before advancing the prospective baseline. It cannot adopt a
later receipt, PID, file incarnation or deadline as new authority.

The incoming entry, continuity, parent-facts, launcher, install, rows, profile,
events and recipe predicates remain byte-identical. So do the workflow and both
Go probe files. The exact fixed names, one-entry schema, sidecar incarnation,
non-stdio/sibling/update/repair/removal/cold boundaries, full source/User closure
and process settlement remain mandatory. The earlier optional second-member/
nine-count diagnostic packet is preserved as a separate reviewed checkpoint;
it is not overlaid on this admission path or used to weaken the new parent facts.
Published first-member and source User failure projections remain unchanged.

The composition also retains the stronger reviewed test oracles: exactly the
protected Windows parent rename can report sharing violation 32 only while its
original directory/file identities and bytes remain and the destination is
absent; the impossible-date ValueError must originate in the real stdio parser
frame with the exact captured date match. Every other invalid header still
requires the incoming RuntimeError predicate. Neither change relaxes runtime
validation or treats a blocked substitution as native success.

Four new methods exercise a real inert child's original status/streams/PID,
same-byte replacements and missing authority, actual growth read caps including
an empty lock, between-read file drift and original deadlines, and the real
command tail's after-parse gates before baseline publication. All 163 incoming
methods remain. The complete corrected suite ran 167 methods in 7.386 seconds:
166 passed and the same actual-Windows case skipped on Linux. The unchanged Go
probe package passed again in 0.458 seconds. Three actual in-memory
counterfactuals failed their regressions: restoring the old read request caused
three assertions; omitting pair rejoin caused three; omitting after-parse gates
advanced the baseline and failed its assertion. Repository source stayed intact
during those counterfactuals.

The original Windows bundled job `132449811109` in the old `880d` run completed
separately and still failed at `public-http-a`: 19 members, one backup and one
unknown empty 0666 file, relative-name digest
`d6503b34655ee420b82058507655f28a09177f18851cd32ea54eb4fbe234d256`.
Original artifact `14117700734` is 917 bytes, SHA256
`15d825ee3ba2456613a3a1ad7147b94c9b48d5324e718e25581b9fd59c009234`;
ZIP CRC and its sole 1,612-byte report were independently verified. The report
SHA256 is `7f9277e08757ef0ec1b18311170485420dc1386030f0a51a0f683eb454813d49`.
This is a distinct non-stdio refusal, outside the two-file admission correction.
Its producer audit and fresh final-source native execution remain separate;
all original qualification flags remain false. No native pass, Windows source
shared-bin allowance, cleanup permission or production capability is added.

### 2026-11-14: bounded Windows agent-lock read on the original lifecycle deadline

This source correction composes with `8d18cfff1e62ae8eeaf896b5b36dd14a8e98e14a`
and the independently preserved stdio authority correction. The full reviewed
composition bases are runtime blob `6e774555430b97ae66876f74e8076448493c5ce4`,
test blob `b982b26dc309564f62eae405d5303245bdbac27f`, and this leaf's prior blob
`bf8447cd53a6de5b632c989f9f51674c07620937`. Source R1 plan SHA-256:
`2f5debc012c2ebe2bf2afdbf00f8c48b4c3c6c29cd218ca0295638c80f649871`.

The exact empty Windows `logs/.__agent.lock` read now uses the existing
`ordinary_bytes(..., maximum=0, deadline=deadline)` boundary. The metadata
bound is zero; the existing oversize probe requests at most one byte and must
observe EOF. The required deadline is the one already captured by
`observe_public`, passed through the two private helpers without renewal.
Entry, leaf-body and final rejoin fences use that same value. The existing
held-directory `finally` verification still runs when the budget expires.

All incoming original outer/Project/parent/selected/logs identities, full lock
stat continuity, exact six-field captured row, original absence, owned
HTTP/stdio role checks, prospective publication after `public_profile`, and
pre/post-command plus final cold continuity are retained. No other filename,
foreign member, source effect, writer behavior, platform or qualification is
admitted by this change.

Three focused contracts are authored: expiration before any filesystem
acquisition on every platform; a real Windows empty-file read with an explicit
one-byte request and growth refusal; and real Windows pre-body/final-rejoin
expiration while all five original held directories are still rejoined.
The existing command-block test asserts propagation of the exact supplied
deadline. The 175 previous methods remain, with only the necessary direct
caller/fixture argument updates. These inert fixtures do not constitute a
vendor/native observation.

The executor is unavailable. These new methods and the composed 178-method
source have **not been executed locally**. Independent source R2, ordinary CI
execution (including actual Windows cases), fresh native originals and final
source acceptance remain required. Historical Windows HTTP evidence explains
the fixed member's role; it does not qualify this new source.
