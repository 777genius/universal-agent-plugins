# Devin 3000.12.57: ordinary metadata gate and public UAP Global E2E

Status: Step 1 candidate implemented locally; hosted execution and original
evidence review remain pending. Step 2 is still prospective. The plan was prepared
from commit `672a769c8954365726e96485bff3868e1936e41b`; the Step 1 implementation
uses `a84a20e70110f7ee035eda0d364507b554e08c41`. The existing Devin files and this
workflow are unchanged through `bb5b9c033ddf5d5d03139bbc71ef42d7f3a3cb4f`; the
coordinator owns current-source integration, hosted execution and merge. The
narrow effect predicate was accepted for this checkpoint. No native PASS or
production version admission follows from the local implementation.

## Purpose and existing evidence

Exercise the real public UAP Global add/repair/remove route with the pinned
Devin 3000.12.57 executable, then verify the exact UAP-written Skill, stdio and
HTTP sources through genuine structured native inventory. Preserve Prepared,
the original profile, legacy Cascade routing and all policy boundaries.

The existing D0 lifecycle is the reference sequence, not code to copy wholesale:
`.github/workflows/devin-global-d0.yml` already builds production UAP, installs
one mixed Global package, repairs a removed owned Skill, removes the package,
and checks discovery in A/B. Its older 3000.11.205 pin and historical evidence
stay unchanged. Reuse the current structured research acquisition, Context,
process containment, strict JSON, fixture effects and original-stream publisher.
Do not introduce a second native runner, a client wrapper or an unwired parser.

Original metadata run
[43420259044 / job 131868182977](https://github.com/777genius/universal-agent-plugins/actions/runs/43420259044/job/131868182977)
on `4994e4e1c780a7f45d65ed26aa576ebd8a429355` contains all eight A/B/User/A-repeat
MCP/Skills streams. Its historical job failed pending schema review; it is not a
new PASS. The current validators replay those originals and pass 31 inert tests.
Fresh current-source metadata CI and independent original-artifact review remain
a prerequisite. That route uses an explicit private config and is not ordinary
startup or UAP installation evidence.

The new ordinary route invokes no `--config` operand. Its independently authored
User fixture intentionally chooses `read_config_from.cloud=false`, as in the
existing research. UAP never creates or changes that policy. This is ordinary
discovery for that local-policy profile, not the cloud-enabled default or an
authenticated organization policy qualification. Official documentation says
cloud=false excludes Cloud MCP/personal plugins while organization-required
plugins still load; team restrictions remain independent.

Primary documentation reviewed on 2026-11-13:
- https://docs.devin.ai/cli/reference/configuration/read-config-from
- https://docs.devin.ai/cli/reference/configuration/global-vs-local
- https://docs.devin.ai/cli/extensibility/mcp/configuration
- https://docs.devin.ai/cli/reference/commands

## Step 1: ordinary metadata effects before any UAP mutation

This gate is the only next implementation checkpoint; Step 2 remains deferred.
Implement and execute it separately before the production version change.
It uses the existing independently authored original User, A/B, local override,
legacy fixtures and zero-effect probes. There is no private replacement config,
seeded version field, copied account state, login, prompt, trust change or retry.
Both owned Git excludes contain the exact previously observed ignore line at
fixture construction, before baseline 0; native changes to them remain refused.

The small runnable checkpoint is one sibling script
`scripts/devin_ordinary_metadata_admission.py` and its focused
`scripts/test_devin_ordinary_metadata_admission.py` contracts. It imports the
existing acquisition, `AdmissionContext`, fixture, census, strict JSON, bounded
reads, zero-effect and publisher helpers. The new leaf owns only the ordinary
argv sequence and the exact changed effect predicate; it has no product parser
or production callsite. Keep the current private-config recipe unchanged.

After plan and current original-artifact review, the existing
`client-automation-native.yml` gets one Linux-only ordinary-inventory job using
the same checkout, closed hosted environment and publisher. No Go build is
required at Step 1. Its ten-minute job contains the existing 360-second case:

```sh
python3 -m unittest discover -s scripts -p 'test_devin_ordinary_metadata_admission.py' -v
python3 scripts/devin_ordinary_metadata_admission.py --output "$RUNNER_TEMP/devin-ordinary-inventory"
```

Publish only that fresh output's admitted `upload/research.zip`. The original
private-config job keeps its command, artifacts, budgets and acceptance unchanged.
The command is implemented for the next hosted checkpoint; it has not been
executed against the native vendor here.

Capture the existing five version/help commands, then ordinary `mcp list --json`
and `skills list --json` in A, B, outside-project User and fresh-process A-repeat.
Retain exact source/bin/archive identities, argv/cwd, original streams, complete
root-relative before/after inventories and the same zero-connection/stdio guards.
Any unadmitted effect stops before UAP add, repair or remove can run.

### Proposed additional effect: one native User-format migration

The d7 run 43400739054/job 131802659003 already measured a User config version-field
addition and a Git exclude edit. This plan proposes a prospective predicate for
the former only; it does not reinterpret that failed run or relax the existing
private-config research. The coordinator accepted this exact additional effect
authority for Step 1, without extending the other bootstrap allowances.

Only the first ordinary MCP inventory may change
`home/config/devin/config.json` from the exact authored 37-byte object to a strict
unique-key JSON object equal to that same policy plus literal integer `version:1`.
The after file is at most 256 bytes, ordinary, single-link, original UID/GID and
0644 mode, under the original held no-follow parent/root chain. Native atomic leaf
replacement is allowed only at this one boundary; record its before and after
file identities, complete known-fixture body digests, mode and size separately.
No extra field, type change, policy change, alias, missing file or later rewrite
is allowed. After the first accepted native result, freeze its exact identity
and bytes for every remaining command. Baseline0 and the migration delta remain
in the evidence; do not restore the file, discard the delta or renew baseline 0.
If the measured first result differs, preserve a failure and stop for review.

All original User MCP/Skills, Project main/MCP/local files, legacy files, Git
metadata, source/probe/runtime files and original directory authorities remain
unchanged. Only exact auxiliary-parent size changes implied by admitted children
are tolerated; original IDs, owner and mode remain fixed. Unknown writes,
unproved cleanup or nonzero native exit never become qualified absence.

### Finite allowlist and budgets

| Authority | Gate limit and allowed effect |
| --- | --- |
| Pinned runtime | Existing archive 72925933 bytes; 250 members, 234 regular files, 214280841 expanded bytes. Archive SHA256 `0946784ef886d716e82a8026f76933bf0a6e4f0a804a23722bb62047e8c07d4a`; binary SHA256 `3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d`. Full immutable byte verification unchanged. |
| Complete outer census | Existing 4096 entries/depth 32/no aliases or special members; complete root-relative membership before/after every child. No excluded mutable subtree. |
| Native bootstrap directories | The exact nine current `BOOTSTRAP_DIRS`; 0755, original owner/filesystem. No other children by directory wildcard. |
| Fixed bootstrap files | Exact current telemetry, plugin lock, empty lock and definition-cache paths/bodies; retain their hashes. Definition cache stays separately pinned 674340 bytes/0600 and immutable after creation. |
| Variable bootstrap files | Current installation UUID grammar with lifetime digest continuity; current maintenance-lock grammar; logs with existing validated timestamp/positive numeric suffix grammar. Ordinary single-link 0644; 16 KiB/member and 256 KiB aggregate excluding only the separately pinned definition cache. |
| Gate log membership | At most one new log per metadata command, at most 8 logs and 24 new native-bootstrap entries total. |
| Original User config | The one proposed migration above, then exact freeze; no other original-file mutation. |
| Time/processes | Existing 360-second gate budget; each native command at most 30 seconds; existing tree settlement and cleanup authority. Five help/version commands must remain effect-free. |
| Publication | At most 128 ordinary single-link evidence members, 1 MiB/member, 16 MiB total; only bounded original streams and closed receipts, no runtime/cache tree upload. Full cleanup precedes atomic publication. |

Step 1 completion requires all eight exact JSON source checks, unchanged A-repeat
outputs, every declared effect predicate, complete settlement and original stream
digest/length joins. Its result means only ordinary metadata for the declared
local-policy fixture. Independent original review is required before Step 2.

Required focused negatives cover the migration during help or any later command;
duplicate/extra JSON keys, boolean or floating-point version, changed cloud policy,
oversize/mode/link/owner violations, substituted parent/root/leaf authority, and
the frozen after-image changing bytes or identity. Assert that baseline 0 and both
original fingerprints survive every case. A normal nonzero first command fails
even if its migration is otherwise allowed; missing process settlement withholds
raw publication. Additional controls reject unknown bootstrap paths, changed
pinned cache/runtime bytes, excess logs, endpoint activity and JSON source swaps.
The Step 1 script has no UAP mutation branch; its finite argv allowlist excludes
`--config`, prompt/auth/trust actions and all native mutation subcommands.

## Step 2: exact version admission and the real Global lifecycle

Only after Step 1 review, add exactly 3000.12.57 beside 3000.11.205 in
`clients/windsurf/project_validation.go::requireCurrentDevin`. Preserve primary
executable selection, normalized host-observed version, original XDG/APPDATA,
legacy receipt precedence, all migration/override refusals and offline removal.
Do not unlock a version range, replace the old collector pin, spoof the observed
version, modify generic observer flags or broaden other platform claims.

The same candidate must pass the existing version/admission and Global receipt
tests plus the genuine Linux cycle below before merge. SDK ClientVersion remains
an embedding-host observation, not executable provenance.

Build ordinary production UAP from exact current source with the pinned existing
Go setup before native fixture baseline 0. Record source SHA/tree/clean state,
compiler version, build result and UAP binary hash. The built binary and full
Devin payload remain immutable throughout the case. Use a separate empty owned
AGENTPLUGINS_HOME and scratch directory inside the fully observed task root;
preserve the same original Devin HOME/XDG and real selected PATH for all commands.

Run the complete Step 1 gate again in the lifecycle case before invoking UAP.
The isolated mixed source contains one tiny Skill, one managed stdio server and
one HTTP server, with unique UAP names. Existing same-name foreign User/A/B/local
controls stay independent of those UAP names and prove precedence preservation.
The probes observe effects only; the harness sends no MCP requests.

Use the existing D0 public command shapes, with real selected Devin version:

1. `agentplugins add SOURCE --no-save --target windsurf --scope user --format json`.
2. Native MCP/Skills inventory in A and B, each as a fresh real process.
3. Public `doctor` on the returned installation ID; no mutation or recovery.
4. Rename only the exact UAP-owned Skill leaf into a predeclared private quarantine
   to induce repair. Use the original held parents and source identity, refuse an
   existing destination and verify the same moved inode/body. Record this one
   deliberate harness delta; do not use pathname-recursive deletion.
5. `agentplugins repair ID --target windsurf --scope user --format json`.
6. Native MCP/Skills inventory in A and B and a second public `doctor`.
7. `agentplugins remove ID --target windsurf --scope user --format json`.
8. Native MCP/Skills inventory in A and B; require exact selected-source absence.

The initial gate supplies the before-add A/B inventories. Thus the lifecycle
retains D0's four phases/eight A/B checkpoints, plus User/A-repeat gate controls:
20 inventory commands total, five version/help commands and five UAP commands.
With source/Git checks this remains below 40 recorded commands and the shared 60
command ceiling. No model/auth/status/native mutation subcommand is added.

### UAP writes and native readback must remain distinct

Before each UAP command declare its permitted paths and expected transition;
after settlement require every observed delta to match that command's receipts.
Never adopt all files beneath AGENTPLUGINS_HOME as implicitly allowed.

- Add may create only the exact managed package/launcher, installation state,
  existing typed transaction/ownership artifacts and selected Global components.
  The managed launcher must equal the prebuilt UAP binary hash; its metadata must
  bind protocol/version/hash. Package bodies bind the original source digest.
- Doctor and every native listing may not change UAP state, receipts, package,
  launcher, managed components or foreign policy. Native auxiliary effects remain
  independently governed by the declared bootstrap predicate.
- Repair may restore only the deliberately quarantined owned Skill and the exact
  supported existing transaction/state transition; original ownership stays bound.
- Remove may remove only recorded owned Global components/package and update the
  matching installation/receipt state. It neither invokes native remove nor relies
  on current authentication/version for the recorded exact cleanup.
- Inspect complete before/after outer membership and held-file hashes. Ordinary
  UAP metadata is at most 1 MiB/member and 16 MiB aggregate; at most 256 additional UAP
  members. The only larger allowed files are the exact prebuilt UAP executable
  and managed copy, each at most 128 MiB and with the same predeclared hash. The
  existing 4096 outer-entry/512 MiB-member ceiling is not enlarged. Any unexpected
  cache, transaction remnant or unresolved journal refuses the candidate.

For native JSON, derive expected selected paths/launcher argv from exact UAP
ownership, not substring membership or handcrafted replacement files. MCP source,
enabled/disabled facts, transport, argv/url and disabled tools must match. Env and
headers are absent from this native schema and remain file/receipt assertions.
Skill identity is provider/base_dir/display_name, with expected description and
triggers; User/Project shared names do not collapse into one row. Preserve exact
foreign/builtin controls. Following remove, require the UAP source to disappear,
not every unrelated item that happens to share a display name.

The lifecycle recipe prospectively permits at most 20 logs/36 new native auxiliary
entries because it performs20metadata commands. Preserve 16 KiB/member and 256 KiB
aggregate mutable bootstrap bytes, pinned definition-cache exception and all
identity checks. This separate recipe does not enlarge the existing research
limits. Use a 900-second case deadline, native commands 30 seconds, UAP commands 90
seconds and the existing settlement mechanism; build/acquisition have separate
finite workflow budgets. One hosted job gets 20 minutes, without automatic reruns.

## Product claims, scope and verification budget

Keep ActivationPrepared, ActivationAttested=false, installation verification,
unknown authentication/effective policy, native-call/model qualifications false.
Do not remove DevinGlobalAction's manual native-inspection instruction merely
because CI performed the inspection. Production automatic readback is a separate
later wiring decision; no unused Go parser or session capability is added here.
Legacy Cascade behavior, Project qualification on the old version, VS Code,
ChatGPT Web, hooks, release and remote actions remain outside this candidate.

| Change | Prospective handwritten budget |
| --- | ---: |
| Step 1 ordinary recipe and narrowly reused effect primitives | 140–210 lines |
| Step 1 meaningful effect/cleanup/policy negatives | 80–120 lines |
| Step 2 D0 lifecycle extension and exact source/receipt joins | 160–240 lines |
| Step 2 focused lifecycle/negative recipe contracts | 80–120 lines |
| Exact production version admission | 15–35 lines |
| Existing Go admission/receipt test extension | 40–80 lines |
| Existing CI hook/setup and plan |small scoped additions |

No remote execution in this planning task. Before each implementation checkpoint,
independent review checks complete scope and realistic budget. Run existing Devin
31 contracts plus new focused negatives, applicable Go tests/lint/test-core for the
version change, then genuine exact-source CI and original artifact review. A
queued job, historical replay, failed native guard or visible config alone is not
completion. Failure evidence never authorizes a broader allowlist after the run.

## Independent plan review

The Devin metadata reviewer independently accepted Step 1's source/policy,
one-boundary migration, complete original fingerprints, after-image freeze,
cleanup and finite budgets. The reviewer emphasized that even a permitted first
migration cannot turn a nonzero exit or unproved cleanup into success. The Step 2
fault injection was narrowed to an exact owned quarantine rename following the
review. This is a plan review, not implementation or hosted qualification.

## Step 1 implementation checkpoint

The new leaf reuses the concrete collector through a fixed recipe/effect seam;
the private leaf's source changes, while its default argv, effect predicates,
budgets, acquisition and publisher behavior remain unchanged. There is no generic
runner or artifact-schema change and no selectable CLI callback or recipe mode.
The two source entrypoints and artifact names distinguish private and ordinary
observations, including early bounded failures. Common publication refusals keep
their existing minimal failure schema and never claim either route completed.

The ordinary route creates no private replacement config. Its original User
identity/body remain baseline 0; the one first-inventory after-image is retained
separately and frozen at subsequent boundaries and the final metadata fence.
Before/after fingerprints are observed effect evidence, including on a settled
nonzero command, not success receipts. Nonzero stops the sequence and unproved
cleanup withholds all raw captures. Final ordinary completion also joins every
recorded native argv and cwd to the exact thirteen-command ordinary allowlist.

Local validation runs the existing 31 contracts and six new ordinary contracts.
The original 4994 `research.zip` SHA256 is
`d61be4edb2597537f7c5ccfd31f766feaba4f39c2fff823ad015ea68bdbaaf22`; its eight
original MCP/Skills streams are replayed through the unchanged source validators,
with all recorded length/digest joins and private argv/cwd checked. Historical
replay does not prove the new ordinary route. Its dedicated Linux hosted job
must still finish, and its original artifact must pass independent review before
any Step 2 mutation or exact production-version admission.
