# Packed-generated project → injected installer bridge

This is a separate, opt-in **test boundary**, following a successful terminal
`private-npm-native.test.js` run at the final integrated commit. It never builds
or executes candidate binaries, regenerates native projects, installs packages,
invokes an external scanner, reads real client profiles or starts providers.
The existing `add --dry-run` command constructs its real planner; only detection,
security assessment and lifecycle/state interfaces are injected. Fake Cursor,
Codex and Claude directories exist only in Go test temporary directories.

The accepted input is the same-run actual `native-completion.json`, independently
pinned along with its native config. Partial invocations, synthetic test records
and source-generated harness results are never packed-native acceptance.

## Run after the final native gate

Use a clean checkout at ROOT's intended exact 40-character commit. Keep the
candidate, npm pack output, native evidence, native disposable fixture, bridge
config and bridge result outside the checkout. Retain all native temporary
projects, installed prefixes and caches unchanged until bridge verification ends.
Do not run cleanup concurrently. This is the existing quiescent, same-user local
namespace contract, not protection against a hostile concurrent same-UID writer.

1. ROOT runs its final exact-commit native gate with the trusted completed pair:

   ```sh
   UAP_PRIVATE_NPM_NATIVE_CONFIG="$NATIVE_CONFIG" \
     "$NODE" --test npm/agentplugins/test/private-npm-native.test.js
   ```

   Require exit 0 and the terminal completion from that same run. Record SHA256
   of the native config and terminal completion independently. The native config
   must be canonical `JSON.stringify(value, null, 2) + "\n"` JSON, with exactly
   `stage`, `completionDigest`, `evidenceOutput`; stage fields are exactly
   `candidate`, `repo`, `root`, `identity`, `manifestDigest`, `assetScope`,
   `authoringMode`, `go`, `workParent`, `output`, `node`, `npm`.
   Preserve the original config; do not repin a changed or partial run.

2. In a **dedicated external config directory**, write canonical `request.json`:

   ```json
   {
     "expectedCommit": "FINAL_INTEGRATED_40_HEX_COMMIT",
     "nativeConfig": "/absolute/native-config.json",
     "nativeConfigSha256": "INDEPENDENT_NATIVE_CONFIG_SHA256",
     "nativeCompletionSha256": "INDEPENDENT_TERMINAL_COMPLETION_SHA256",
     "fixtureRoot": "/absolute/work-parent/dual-authoring-NATIVE_SUFFIX",
     "disposableEvidence": true
   }
   ```

   `fixtureRoot` is the exact parent of the two `projects` paths in native
   completion, explicitly acknowledged as disposable evidence. It must be a
   direct `dual-authoring-*` child of the configured workParent. Project paths
   are derived exclusively from completion: `<fixtureRoot>/<product> disposable
   projects/<lane>`. No arbitrary project argument is accepted. Both products
   and all five lanes must exist. Every lane must have its successful packed
   `init` invocation; the completed native journey also added `extra-skill`.

3. Seal after completion, immediately before planning:

   ```sh
   "$NODE" npm/agentplugins/scripts/packed-installer-bridge.js \
     seal /absolute/bridge-config/request.json /absolute/bridge-config/sealed.json
   ```

   Save the printed SHA256 independently as `BRIDGE_SHA256`. Publication is
   exclusive: do not overwrite an old config. Sealing reads candidate bytes and
   pins, pack SHA256/size/SRI, completion/identity/mode/scope, invocation records,
   and complete directory/file inventories (including empty entries and modes).
   It does not verify Go build info or rerun native journeys: ROOT's preceding
   trusted native gate supplies that evidence. Config pins both verifier and its
   existing candidate helper. It snapshots the entire native fixture, candidate,
   pack output, native evidence and generated parents. Generated trees reject
   every symlink; the containing npm fixture permits only contained symlinks
   such as installed `.bin` links. All ordinary files must have one hard link.

4. Run only the packed bridge test, using installed Go **1.25.13**, offline cache,
   private HOME/TMP/config/GOCACHE and `GOMAXPROCS=2`. Keep result in a different
   external directory from config and all observed inputs. Example from checkout:

   ```sh
   env -i PATH="$(dirname "$GO")":/usr/bin:/bin \
     HOME="$PRIVATE_HOME" USERPROFILE="$PRIVATE_HOME" APPDATA="$PRIVATE_HOME" \
     LOCALAPPDATA="$PRIVATE_HOME" XDG_CONFIG_HOME="$PRIVATE_HOME" \
     XDG_DATA_HOME="$PRIVATE_HOME" XDG_STATE_HOME="$PRIVATE_HOME" \
     XDG_CACHE_HOME="$PRIVATE_CACHE" TMPDIR="$PRIVATE_TMP" TMP="$PRIVATE_TMP" TEMP="$PRIVATE_TMP" \
     GOCACHE="$PRIVATE_CACHE/go-build" \
     GOMODCACHE="$PRIVATE_MODULE_CACHE" \
     GOPROXY=off GOSUMDB=off GOENV=off GOTOOLCHAIN=local GOMAXPROCS=2 \
     UAP_PACKED_INSTALLER_NODE="$NODE" \
     UAP_PACKED_INSTALLER_CONFIG=/absolute/bridge-config/sealed.json \
     UAP_PACKED_INSTALLER_CONFIG_SHA256="$BRIDGE_SHA256" \
     UAP_PACKED_INSTALLER_COMMIT="$FINAL_COMMIT" \
     UAP_PACKED_INSTALLER_OUTPUT=/absolute/bridge-results/completion.json \
     "$GO" test -p=2 -tags=packedci ./cli/plugin-kit-ai/internal/authoring/commands \
       -run '^TestPackedGeneratedPackagesReachExistingInstallerPlanner$' -count=1 -v
   ```

   Create the private directories and result parent first. `NODE` is an absolute
   trusted installed Node 22/24 path. Go checks HEAD and clean tracked/untracked
   state; don't use `-trimpath`, `-overlay`, alternate build flags or source copies.
   Do not set private directories inside the native fixture. The `packedci` tag
   exposes acceptance only; absent or partial configuration fails. Ordinary
   coverage retains the source harness on Linux/Windows amd64/arm64.

Require exit 0, **ten passing project subtests**, and the exclusive result JSON
with **30 reports** (two products × five templates × three fake targets). Each
report checks successful structured dry-run, exact target, user scope, expected
status and all component names/kinds/support. The scanner compares the real
acquired snapshot to the exact source bytes/entries inside the private acquisition
root. All detector/scanner calls must occur. State writes, staging, discard,
activation and deactivation fail closed through the existing no-effect fixtures.
Source/client trees and acquisition cleanup are checked after each plan; the
entire sealed input is reverified after all subtests. Failed subtests do not
publish a result. Archive the successful Go output with the result and input pins.

Every bridge result keeps `release_eligible`, `platform_acceptance`, `attested`
**false**. This boundary proves injected planner integration only. It does not
prove real installation, profile discovery, external scanning, runtime behavior,
provider access, non-Linux execution, public release eligibility or attestation.

## Disposable harnesses (not native acceptance)

`node --test npm/agentplugins/scripts/packed-installer-bridge.test.js` tests seal
consistency, negative identity/pin/completion/mode/entry/link mutations and CLI
rejection. Its explicitly synthetic records use an in-process `frozenCandidate`
stub only for intake consistency; restoring the real helper and a separate CLI
process both reject them. No stub is reachable from the production verifier CLI.
Fixtures are retained under private TMPDIR for diagnosis.

`go test -p=2 ./cli/plugin-kit-ai/internal/authoring/commands -run
'^TestPackedInstallerSourceHarness$' -count=1 -v` generates five labelled source
fixtures using the existing public authoring test helper, including extra-skill,
and exercises the same planner seam for 15 plans. It writes no packed result.
Use the same offline/private environment above, without any packed opt-in vars.

## Mandatory bounded CI

`authoring-native.yml` adds a separate Linux amd64 packed job, with its own clean
exact-SHA checkout and 45-minute ceiling. It runs `scripts/run-packed-ci.py
"$PACKED_ROOT" "$EXPECTED_HEAD"` after setup. `PACKED_ROOT` must be absent,
absolute and outside checkout. The runner resolves installed Go, Node and npm's
JavaScript CLI; no root-host tool or cache paths are embedded. Go is 1.25.13,
Node is **22.23.2**, and npm is its bundled **10.9.8**, with no npm upgrade.
ROOT selected this pair from the fresh official Node release index (2026-07-28
security release). Versions and tool SHA256 hashes, including stager Node, are
recorded. The private candidate uses the accepted release-contract fixture
versions agentplugins 0.1.91 and plugin-kit-ai 2.0.0; these are not publication.

Relevant Go dependencies warm in a job-owned module cache. Subsequent commands
use allowlisted environments, private profiles, GOPROXY/GOSUMDB off and local
Go tooling. This is an offline command contract, not an OS network sandbox.
Both real npm packs/installations and both named native tests must pass with
741 invocations (739 distinct records; two repeated journeys are intentional).
The canonical seal pins those exact ten projects. Tagged Go discovery must name
`TestPackedGeneratedPackagesReachExistingInstallerPlanner` exactly once before
uncached execution. Ten project leaves and thirty unique product/lane/target
plans must pass; optional product grouping events do not count as leaves.

The read-only terminal checker rejects missing phase exits, skipped named tests,
truncated transcripts, identity/pin/count/tuple mismatches and missing or true
claim fields where declared by each schema. It checks post-planner verification
of the unchanged seal. Failure artifacts retain logs and sealed inputs; the
artifact digest index preserves relationships. Downloaded archives are audit
material, not a fresh verification of relocated live trees.

The always-running aggregate requires **native and packed success**. Its focused
structural controls reject missing/conditional jobs, dependencies, matrix lanes
and explicit tags. All four native lanes and all five source-harness lanes
remain mandatory. Existing Windows concurrent-init failure remains a blocker;
no retry or restricted diagnostic substitutes for that gate. PR167/169 and the
writable macOS owner gate remain upstream requirements. Required-check settings
are external: source controls do not prove GitHub enforcement. ROOT must run the
new integrated SHA and independently inspect its jobs/artifacts and required
check settings before CI acceptance. Release eligibility, platform acceptance
and attestation remain false. Local synthetic/source tests are not this run.

### Explicit public fixture intake

Public preparation is **not** private native completion. The public producer
`test/public-authoring-native.test.js` now emits an exclusive
`public-native-completion.json` (`dual-authoring-public-native/v2`) only after
both products finish five lanes, `extra-skill`, static parity, isolation, cache
recovery, and lifecycle checks. Its 71 authoring/installer invocation records
are ordered and complete. Five successful npm installations separately bind
actual fixture tarball paths and SHA256 values (independent prefixes, shared
prefix, and reinstall). Preparation tarballs remain separate, unqualified
inputs; substituting their hashes for executed packs is rejected.

The terminal binds the configuration, preparation completion, source/engine
identity, candidate, pair marker, projections/checksums, tools, cached native
binaries, invocation/download/result logs, all ten project locators, and full
generated-tree inventories including modes and empty directories. Native binary
and tarball contents are pinned; supplied executables are never launched by the
bridge. The public producer's preceding native verification and the final
clean-SHA Go gate remain required execution/source proof. The bridge checks
read-only byte consistency; it does not authenticate an arbitrary supplied log.

Use the existing six request fields (`expectedCommit`, `nativeConfig`,
`nativeConfigSha256`, `nativeCompletionSha256`, `fixtureRoot`,
`disposableEvidence`) with the additional exact field
`"intake": "public-fixture/v2"`. The completion digest must independently pin
`public-native-completion.json`. Omitting `intake` still selects the unchanged
strict private schema. Unknown intake values and public/private substitution
fail closed. Seal with the same `packed-installer-bridge.js seal REQUEST OUTPUT`
command and pass its digest to the existing tagged planner. Source/evidence
output overlap and existing seal/terminal destinations fail.

For owner-scheduled final-source acceptance, the imported runner has an explicit
public consumption mode. It does not build or repeat the public native run:

```text
python3 -B scripts/run-packed-ci.py --public OUTPUT FINAL_SHA OPTIONS
python3 -B scripts/check-packed-ci.py --public OUTPUT FINAL_SHA
```

`OPTIONS` is JSON with exactly `request` (the request object above), `nativeTap`
(absolute owner-terminal TAP path), `nativeTapSha256` (independent SHA256), `go`,
`node` (canonical absolute tools matching native completion), and `modCache`
(the existing offline module cache). `OUTPUT` must be new, external, and disjoint
from source and evidence. Both checker modes require exact named discovery,
ten projects, thirty unique Cursor/Codex/Claude plans, complete non-skipped
transcripts, and post-planner seal verification. Public planning is offline;
there is no public preparation/build/warmup fallback. The default private CI
runner and mandatory native/packed workflow graph remain intact.

All public qualification, release, platform acceptance, attestation, signed
promotion and public eligibility claims remain false/null. Synthetic acquisition
proves fixture execution only. Historical f68 evidence cannot satisfy this
new schema or a later SHA and must not be enriched, repinned, or rerun here.
The final integrated source (including the independently owned preparation
initialization fix) still needs owner-scheduled public native execution and
packed planner acceptance. These focused synthetic tests do not satisfy that
E2E or the external release/required-check prerequisites.

Public v2 explicitly narrows executable installer evidence to command visibility
and preflight rejection. Rows 1–69 retain the original authoring sequence; row 70
is `agentplugins add --help --format=json` (one successful help JSON document,
empty stderr); row 71 adds `--scope=project` to the original generated Skill
vector and requires exit 1, no signal, empty stdout and the exact user-scope-only
error. Totals are 69 zero exits, one retired-command exit 2, and one preflight
exit 1. Both project trees, client and installer roots retain entries/bytes/modes.
The test-only helper rejects valid installer vectors before spawning; it does
not establish process-level network denial for Go descendants.

The required `installer_boundary` is sealed into `public_evidence` and the
public summary: `executable_observation: help-and-preflight-rejection`,
`valid_add_dry_run: not_evaluated`, `reason: production-security-inputs-not-offline`,
and `argv` retains the original valid add vector without the rejected scope.
Strict v1 intake remains explicitly selectable and cannot accept v2; private
schemas/counts remain unchanged. `check_public(..., require_valid_add=True)`
rejects this evidence as insufficient for successful production add.

The existing ten-project/thirty-plan acceptance still runs the real Go CLI and
planner with injected detector/security/effects. It does not execute production
main, Security Index or lintai. Help, rejection and injected assessments cannot
close Milestone A's still-required successful distributed installer dry-run on
generated projects. That separate legitimate installer acceptance must bind
actual public launcher/native bytes, genuine security inputs, structured plans,
source/client preservation and separately accounted scanner/cache/acquisition
effects. It remains outstanding after v2 core success, independent review and
fresh successor E2E; no historical candidate bytes may be relabelled.

### C2 production acquisition boundary

The same public launchers and kit postinstall accept the fixed v2 descriptor
and complete `native-inputs.json` binding. V1 qualification/null behavior stays
unchanged. V2 alone recognizes `UAP_PUBLIC_AUTHORING_ASSET_FILE` as an untrusted
absolute locator in an owned private custody directory. Every supplied locator
is checked before cache effects, including on warm hits; invalid input never
falls back to download. Outer and inner pins come only from package metadata.
Local and absent-locator anonymous acquisition converge on the existing locked,
verified cache under `public-authoring-v2`. The supplied file is never executed.

Bounded retained metadata/asset snapshots are rechecked before commit and return;
close or cleanup uncertainty returns failure. The child environment removes the
locator and existing proof controls. These checks assume an owned, quiescent
namespace, without hostile same-UID, mount-replacement or all-host guarantees.
Callers must separately keep project/client/evidence outputs outside custody.

C2 source fixtures establish structural and interface behavior only. They do
not authenticate C1 custody, qualify native inputs, or change either existing
bridge intake. Genuine C3 execution needs a separately reviewed authenticated
intake and actual installed launchers/postinstall and installer lifecycle.
N2, C3 E, P/Q, B, anonymous public readbacks, pair channels, PyPI/Homebrew,
full-platform E2E, release and phases 0–11 remain open.

## C3a local authenticated input contract (execution remains closed)

C3a adds a distinct `public-authenticated/v1` request, with exactly
`intake,expectedCommit,journey,journeySha256,admission,admissionSha256,fixtureRoot`.
It does not translate private, public-fixture/v1 or public-fixture/v2 receipts.
Their existing false/null/not_evaluated claims and the offline v2 71-call
boundary remain unchanged. No workflow, promotion, native qualification export,
public launcher, dependency or package closure changes are part of C3a.

`public-authoring-acceptance.js` fixes `public-wrapper-matrix/v1`'s eighteen
host/runtime cells and thirty product/runtime executions. Its J codec binds the
exact canonical I and S, both unchanged packs including SHA256/size/SRI/SHA1,
all twelve native outer/inner subject pins, source F, producer run/attempt/ref,
actual host and distinct controller/npm/shim Node/tool pins. It rejects unknown
fields, alternate canonical spelling, duplicate keys, invalid UTF-8, excessive
nesting and oversized records. Local paths must be canonical and quiescent;
this does not introduce a hostile concurrent filesystem security guarantee.

The fixed local receipt schema is `authoring-public-local-inputs/v1`. Its fields
are `schema,selected,workflow_sha,input_file,stage,repo,work_parent,stage_root,
input_root,journey_root,fixture_root,cell,tools,producer`. It contains comparison
pins, never an authenticated/completed boolean. `readJourneyInputs` genuinely
calls the existing completed `readStage` and `readInputs`, retaining their
three- and nineteen-subject contracts. The existing stage reader supplies source,
pack closure and pack inspection; C3a adds no extractor or verifier engine.
Fresh authenticated subjects are compared with the original retained S/I/packs.
Re-admission scratch is separate from original custody, source, projects and
journey evidence, so later seal checks refer to the same original roots.

J is `authoring-public-journey/v1`, with the plan's exact top-level fields.
Evidence is an ordered table of fixed `commands.json`, `projects.json`,
`npm-lifecycle.json`, `cache-process.json`, `installer.json` path/size/SHA256 rows.
The limit is 1 MiB per record and non-command evidence file, 16 MiB for commands,
and 1 MiB for each command stdout/stderr. The five-file closure therefore fits
within the plan's 128 MiB aggregate cap. Original project snapshots include
empty directories, bytes, sizes and modes. No regeneration or project copy is
implemented. A fixed core inventory contains 55 kit-only or 127 pair command
rows, including the eighteen genuine installer operations required per pair.
Core argv/cwd/status/signal and bounded author result checks are recomputed;
structural true assertions never imply their truth.

**Live J admission deliberately fails.** Full npm shim/postinstall and peer
lifecycle, cache/process/cancellation, complete conformance/parity and genuine
installer result/observation validators belong to C3b. In particular there is
no reviewed public-shim installer validator or whole-descendant observer
interface available at this boundary. `verifyJourney` emits an explicit C3b
missing-capability error even for structurally consistent transcripts. The
three corresponding evidence payloads are bound bytes, not semantically
validated results. No production producer returns synthetic success.
`readJourneyInputs` is input-custody-only; `readJourney` cannot publish a seal.
The local reader CLI exposes only `--read-local-inputs REQUEST` and explicitly
labels that scope. Completed remote `readAcceptance` always fails in C3a.

The bridge has separate `authenticated-options`, `authenticated-intake` and
`authenticated-seal` commands. Its new seal pins the reader source in addition
to the existing verifier/helper and repeats admission on `verify`. The runner's
`--public-authenticated ROOT F OPTIONS` accepts exactly `request,go,node,modCache`.
Python authentic runner, checker and direct reader entrypoints unconditionally
reject with `missing independently provisioned trusted controller; C3b capability required`
before interpreter subprocesses, planner effects, output creation or authenticated
success. Receipt-selected `node` and self-supplied tool/reader hashes are not
independent interpreter authority; no receipt or environment override opens this
boundary. Prepared intake, Linux planner commands, five environment variables and
terminal validations remain for C3b, covered only by explicitly synthetic gate
mocks. C3b must independently provision the controller and make full public
execution work; this closed subcheckpoint is not its completion.
A local J/bridge, successful fixture TAP or injected planner cannot stand for E.

Positive tests explicitly mock custody interfaces and, for bridge-only seal
controls, the unavailable J result boundary. These are source controls only;
they execute no npm, native, installer, scanner, verifier or provider. C3b must
supply the genuine producer, full result validators, fixed workflow, completed
E aggregate/authenticated reader and P adapter together. Its npm/process,
workflow and P test names remain outstanding. Native/N2, genuine E, P/Q/B,
anonymous acquisition/readbacks/channels, stable release and phases 0–11
remain open; independent review and required remote CI are not waived.
