# Portable package-manager E2E qualification

## Owner request and boundaries

Create one reusable, deep package-manager acceptance suite for Linux, Windows and
macOS. Execute it locally on Linux and in Windows CI; provide the same command
for the owner's macOS run. Investigate first, review the plan independently,
then implement and correct reproduced defects. Keep this pull request a draft
while qualification is in progress. Publish each commit immediately.

Use GPT-6.1 Sol with xhigh reasoning for researchers, critics and implementers.
Do not change disputed product semantics or introduce a large new subsystem
without the owner's decision.

## Research baseline

- Repository main at `77ed6c0412f67907609a43db9b5eeff111485b19`.
- Existing composed CLI dependency tests live in
  `cli/internal/agentpluginscli/deps_*_test.go`; the optional real-binary harness
  lives in `repotests/uap_dependencies_e2e_test.go`.
- Existing native-client qualification and VS Code-deferred work are separate.
- Open PR #563 owns the saved dependency update lifecycle. Recheck its status
  before asserting or changing update-selection behavior.
- The GitHub CLI is installed but has no authenticated account in this workspace;
  use the authorized GitHub connector for branch, commit and PR publication.

## Work stages

1. Inspect current resolver, lockfile, acquisition, store, transactions and E2E
   helpers. Compare suitable package-manager contracts using official sources.
2. Have independent critics review the scenario matrix, portability, expected
   failure behavior and scope. Record product decisions needing owner input.
3. Extend the existing test infrastructure with deterministic, disposable fixture
   repositories/registries and public CLI journeys. Prefer shared helpers to a
   parallel installer or resolver.
4. Run on local Linux, fix demonstrable defects with narrow regressions, and
   review the actual changes. Add a scoped Windows CI execution and one
   platform-independent local command, including macOS.
5. Publish exact execution evidence and instructions. Distinguish a true CLI
   process run from composed in-process CLI tests, and a passed run from an
   unavailable environment or skipped test.

## Proposed scenario families (subject to code research)

- Direct, transitive, diamond, wide and deep graphs; shared-node deduplication.
- Compatible constraints, incompatible constraints, candidate backtracking,
  deterministic selection, exact pins and prerelease policy.
- Self-cycle and longer dependency cycles according to the existing contract.
- Locked/frozen replay, stale or corrupt lock rejection, cold restore, immutable
  source identity, integrity errors, and intentional source unavailability.
- Idempotent repeated sync, explicit update, selected-root preservation, removal
  and shared-dependency retention.
- No durable writes on preflight rejection; honest partial outcomes and recovery
  when effects have already started; preservation of unrelated/user-edited files.
- Paths containing spaces and Unicode, platform environment isolation, portable
  digests, Windows filesystem behavior, bounded execution and actionable logs.

## Acceptance criteria

- One documented entry point runs the identical semantic scenarios on all three
  operating systems using disposable roots and no model/account credentials.
- Expected graph/package identities and resulting managed bytes are asserted;
  exit code alone is insufficient.
- No production safety guard is disabled for this environment. Unsupported host
  capabilities are reported explicitly and do not count as a pass.
- CI is path-scoped, bounded and reusable without changing repository protection
  or unrelated workflows.
- No new dependency solver, persistence format, daemon, UI, native-client feature,
  release or VS Code change.

## Research conclusions and implementation design

The existing contract is one selected version per canonical source in an
environment, compatible cycles, lowest-compatible stable fresh resolution with
backtracking, and retention of compatible existing pins. This is not Go MVS.
Keep those rules unchanged. Explicit Registry update policy belongs to #563.
`--locked` forbids a new solution; cold restore may fetch immutable bytes. UAP
does not currently expose an offline/frozen flag. Do not invent one in tests.

Graph dependencies express co-installation, not service startup order. Cycles
must terminate with complete edges and target inheritance. Archive cycle fixture
declarations omit optional checksums (already supported); the resulting lock
records the acquired checksums. Mutually embedded archive checksums are not a
constructible fixture and must not be faked.

### Test layers and command

- A stable `TestPackageManagerE2E` catalog in the existing CLI test package calls
  production Cobra, resolution, acquisition, loader, store and transactions.
  Named scenarios use explicit independent expected graphs and payload bytes.
  Reuse existing focused tests where they already prove the requested contract.
  The test-only `uappm_e2e` build tag prevents duplicating those scenarios in
  ordinary package tests. A shared JSON case inventory binds runner expectations
  to the Go catalog; missing or unregistered cases fail.
- New graph fixtures serve exact URL-to-ZIP mappings over a real local TLS
  server, returning 404 for every unknown route. Use a representative Gemini
  configuration target to exercise real projection writes without launching a
  native agent. Test security evaluation and verified Directory inputs remain
  declared fixtures; no production trust or containment bypass is introduced.
- Registry range/backtracking and real Git journeys reuse existing fixture
  machinery and run on supported hosts. They remain mandatory in the full suite.
- A small Python-standard-library runner builds the unchanged production CLI,
  runs a subprocess smoke for help and preflight refusals, executes the explicit
  Go catalog, and records source/OS/tool identity plus Go JSON output. A valid
  local package validation is not an offline smoke: it invokes production
  security evaluation. Do not silently replace that boundary.
  The existing production-composed ZIP user/project lifecycle and cold restore
  also run, exercising successful main composition with explicit fixture host
  and security boundaries. The report separates these from the unchanged-binary
  subprocess smoke and from any live-agent or production-security claim.
- The common entry point is `python scripts/package-manager-e2e.py`. Full is the
  default. An explicitly named archive subset is useful for restricted hosts,
  but its report must say subset and cannot claim full qualification. Missing
  required tests, skips, timeouts and nonzero exits make the selected run fail.
  Isolate HOME, USERPROFILE, XDG and Git configuration before any fixture setup,
  including Git hooks and inherited repository overrides. Record dirty-source
  status and the executable hash, not just a commit name.
- One path-scoped workflow runs the full catalog on hosted Linux and Windows.
  The owner runs the same default command on macOS. No schedule, new repository
  protection, paid model, credentials, public source dependency or native-client
  matrix is required.

### Scenario matrix

| Family | Required observations |
| --- | --- |
| Graph shape | Depth 32, width 64, shared dense DAG; exact nodes, edges, versions, required-by facts, target assignment and installed payload bytes |
| Cycles | Self-cycle, three-node cycle, remaining-root retention and unreachable-cycle removal; bounded tree display |
| Deduplication | Diamond, shared root removal, explicit/transitive promotion and demotion without owner-identity churn |
| Constraints | Compatible and incompatible Registry ranges, a new forced candidate-backtracking CLI case with discarded-branch cleanup, ineligible candidate versus hard acquisition failure, documented caret boundaries |
| Identity/order | Equivalent input ordering preserves selected graph; source identity is distinct from alias and digest; pinned releases remain immutable |
| Replay | Missing lock refuses in locked mode; stale/corrupt existing lock refuses in both ordinary and locked install; warm repeat makes no source requests; a cold root restores only from the two agreement files |
| Integrity | Changed remote bytes, missing remote leaf, damaged local store and forged/missing lock edges refuse before active installation changes |
| Lifecycle | Dry-run, selected update, drift refusal, recorded-pin repair, foreign-file/data preservation, truthful partial activation and retry |
| Publication | External edits to manifest/lock while resolving are preserved, including creation of an empty lock where none existed |
| Portability | Disposable paths with spaces/Unicode, independent profiles, native path handling and identical semantic assertions on each OS |

The graph fixture is not a second solver. Expected node/edge sets come from the
scenario declaration, not from production output. Acquisition assertions measure
relevant requests; do not impose an accidental global exactly-once guarantee on
all previews or error paths. Runtime limits have ample headroom and are hang
guards, not cross-machine performance thresholds.

### Publication defect to reproduce before fixing

`deps lock` captures only the manifest preimage before resolution and can
overwrite an externally edited lock. `resolveDependencyLock` also publishes to
the store before the final manifest recheck. Reproduce the lost edit, then use
the existing agreement byte/absence comparison to check both inputs. Read each
preimage once, resolve from those bytes, and recheck before store publication
and immediately before lock publication. Preserve unchanged-lock cold-store
hydration and dry-run behavior. Distinguish missing files from existing empty
files; do not add a transaction framework or promise atomic compare-and-swap
against an arbitrary editor between the last comparison and rename.

### Failure boundaries

Resolution/preflight refusal must preserve the active installation and agreement.
Valid immutable cache objects can exist after a later publication failure.
Native activation failures may already have committed a binding and must report
partial/retry state; UAP has no global all-graph rollback guarantee. Assertions
must match the phase rather than require an unchanged entire filesystem for
every nonzero result.

## Primary research sources

- [Cargo resolver](https://doc.rust-lang.org/cargo/reference/resolver.html):
  constraints, lock preference, backtracking and its different cycle policy.
- [Cargo fetch](https://doc.rust-lang.org/cargo/commands/cargo-fetch.html): locked,
  offline and frozen are separate documented contracts.
- [pnpm install](https://pnpm.io/cli/install): frozen lock, offline store and
  integrity refusal are useful separate scenario dimensions.
- [npm install](https://docs.npmjs.com/cli/commands/npm-install/) and
  [npm folders](https://docs.npmjs.com/cli/v11/configuring-npm/folders/): nested
  copies/Node resolution are not UAP's singleton installation model.
- [Go modules](https://go.dev/ref/mod#minimal-version-selection): MVS does not
  resolve arbitrary upper-bounded version ranges.
- [Nix profiles](https://nix.dev/manual/nix/2.32/package-management/profiles):
  atomic generation switching is an architectural example, not a portable
  promise for UAP's multiple native configuration directories.

## Independent review checkpoint

Three GPT-6.1 Sol/xhigh researchers independently inspected industry contracts,
resolver/state risks and portable execution. Their subsequent plan critique
corrected checksum-cycle construction, overly broad atomicity/offline claims,
valid-local validation security assumptions, unknown fixture-route handling,
missing-versus-empty agreement checks, wrapper duplication, early Git isolation,
and false archive request counters. A second review accepted the corrected
scope, including a mandatory partial-commit/retry case and one target-union
journey. Implementation follows this corrected
scope. Alias-collision backtracking, latest-versus-minimum fresh selection,
side-by-side versions and a global transaction redesign are not implemented.

## Initial environment evidence

On the Linux workspace, the existing real HTTPS ZIP, TAR, TAR.GZ, TGZ and
extensionless archive lifecycle tests passed when loopback was permitted.
Production real Git acquisition refused before execution with
`ordinary_unsupported`: this host cannot provide its required procfs process
observation. Preserve that failure and require genuine supported-host Linux and
Windows runs for the full catalog. These exploratory tests do not qualify the
new implementation; they were recorded before implementation began.

## Implementation findings

The new public archive journey reproduced a second defect: `add` could not
promote a retained transitive Git/archive source to a direct dependency because
the absent root declaration was compared as a zero-value selector. Restore the
recorded requirement from its immutable pin when that declaration is absent;
keep selector/alias-change refusal and the existing exact-Git-commit exception.
Independent review accepted this as a correction to existing promotion behavior.

A related regression proved that a valid graph can use a declaration key equal
to another package's stable alias. Direct promotion must refuse an occupied key
belonging to a different source rather than returning its requirement. Default
regressions and the reusable agreement catalog retain these source/identity
checks. No alias renaming or resolver policy change is introduced.

The longer catalog test names also exposed an old transfer assertion that
mistook the word `invalid` in a temporary path for a real error diagnostic. It
now inspects the structured TOML diagnostic field. All transfer state/byte
assertions are retained.

The first full hosted run exposed the same oracle hazard in the reused
partial-installation/retry journey: the word `partial` in its temporary path
was mistaken for an active lifecycle fact. Status checks now inspect exact
labels in `facts[].facts`, with a regression that places diagnostic words in
unrelated metadata. The transfer rollback assertion likewise requires the
actual `rollback incomplete` diagnostic instead of a word present in its path.
