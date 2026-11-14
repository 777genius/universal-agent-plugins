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
