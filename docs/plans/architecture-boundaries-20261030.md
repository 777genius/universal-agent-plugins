# Architecture boundaries that keep covering new clients

Date: 2026-10-30. Base: `ae068426706d5f2c9d624ae94c6504ac8b7f553e`.
Branch: `refactor/architecture-boundaries-20261030`.
Status: implemented locally after plan checkpoint `e738d4f`; ready for independent review.
Draft PR: https://github.com/777genius/universal-agent-plugins/pull/591.

## Problem and priority

The client registry has grown from eleven to fifteen client identities, but the
architecture checks still repeat the original eleven adapter package names.
`internal/archtest/boundaries_test.go:37-49` and the
`generic-no-concrete` / `clients-no-concrete-clients` rules in `.golangci.yml`
omit Grok, Kimi, Qoder and Pi. A new concrete import in a generic dispatcher can
therefore pass the guard that promises the dispatcher only sees the registry.
The plain Go test also does not mirror the client-peer import rule at all.

Priority: P1 architectural regression prevention. No current forbidden
production import was found by the initial search; this is a demonstrated gap
in the protection for the next client change, rather than a claimed runtime bug.
The value is protecting the existing extension seam without adding a framework.

## Intended behavior

Generic providers, planning and client detection may import the client contract
and shared helpers, but may not import concrete client implementations or the
assembled registry. Every client implementation is covered as soon as it appears
under the client namespace; coverage must not depend on updating another client
identity list.

The client contract, shared helpers and concrete adapters may not import a peer
adapter or the assembled registry. `clients/all` remains the explicit assembly
point. The existing external Gemini consumer tests may import their own exact adapter
package, preserving the narrow existing test exception. Existing contract-test helpers remain usable.
The check must preserve every current prohibition while detecting previously
uncovered peer imports and adapters added in the future.

The AST file scan continues reading production Go source directly and ignoring
host build selection. A Windows-only or custom-tagged production file must be
checked during the normal Linux Go test. Test fixtures remain outside production
scans. The production client-ID budget and its existing limitations are unchanged.

## Bounded implementation

1. Replace the handwritten concrete-client list in the Go guard with one
   namespace classification policy. Explicitly identify structural packages
   such as the root contract, shared helpers, test helpers and assembly package;
   treat additional implementation package names as clients automatically.
2. Add the missing executable peer boundary using the same classifier. Keep
   diagnostics specific to the importing source file and forbidden import.
3. Add temporary synthetic checkout fixtures in the guard tests and exercise
   the real scanning/checking path. Cover a future unregistered client, a peer import, a
   shared-helper import of a concrete adapter, nested client packages and
   platform/custom build-tagged production files. Add positive cases for the
   root contract, shared helpers, `clients/all` assembly, the existing external consumer
   tests and test-only dispatcher assembly.
4. Align depguard with the stable client namespace where its supported
   prefix/exception matching can express the same policy. Verify configuration
   behavior with the installed linter before replacing lists; the AST guard is
   the host-independent, automatically extending enforcement. Do not weaken a
   current rule solely to simplify YAML.

Files owned by this lane: `install/integrationctl/agentplugins/internal/archtest/`
and, only if necessary, `.golangci.yml`. This plan is also owned by the lane.
No client implementation, dedicated client fixture, VS Code source or dedicated
VS Code tests will be changed. No release, merge or dependency migration is
included. Legacy authoring capabilities remain preserved.

## Validation and review

- Run the architecture test package uncached against the complete checkout.
- Prove the new guard rejects the negative fixtures and accepts the positive
  cases through the same scanner/checker used for repository enforcement.
- Include build-tagged negative fixtures so a host-limited scan cannot silently
  replace the current full-source behavior later.
- Run the relevant depguard check if `.golangci.yml` changes, including a
  negative import against a newly named client. Run formatting and diff checks.
- Obtain an independent review of rule equivalence, the future-client cases,
  peer boundaries and exclusions before the lane is considered complete.

Tests must exercise forbidden architecture changes, rather than assert the
literal contents of a new client list. A green repository scan alone is not
sufficient evidence that a guard can reject a regression.

## Remaining risks and deferred work

This change enforces package imports. It does not prove semantic purity, prevent
all string-based client dispatch, or replace behavioral client contract tests.
The existing domain-selector budget already parses client constants dynamically
and scans all build-tagged production files; it does not need a replacement.

Other priorities from the same audit are supervised npm cancellation and moving
the existing shared dependency application protocol out of the CLI. They are
separate implementation lanes. Existing PR owners retain their E2E, dependency,
project, SDK and landing work.


## Implementation and validation evidence

The executable guard now classifies the client import namespace, with only
`shared` and `contracttest` as structural subpackages. It requires no adapter
identity list, filesystem discovery list or domain registration to reject a
future implementation import. Client-peer enforcement runs in plain Go tests as
well as depguard. The installer facade now also rejects `clients/all` outside
its tests, matching its documented injection requirement.

Depguard uses `list-mode: lax`, a broad `clients/` deny and more-specific
structural helper exceptions. Exact `$` suffixes and separate trailing-slash
subtree exceptions prevent names such as `sharedfuture` from inheriting the
shared-code exemption. The existing external Gemini own-package exception is
retained in a separate narrow rule. Existing test exclusions and all-client
assembly remain explicit. No production client source was changed.

Validation used Go at `/tmp/uap-e2e-tools/go/bin/go`, golangci-lint 2.13.2, and
existing module/build caches. The following checks passed:

- `go test -count=1 ./internal/archtest` from the installer module: PASS, final uncached run 0.647s.
  This includes the real repository scan and 49 synthetic checkout cases using
  the same rules, directory walker and import parser. Cases cover every current
  adapter, an unregistered future adapter, nested packages, peer production and
  test imports, unknown helper-name prefixes, registry assembly, installer/CLI
  injection and the existing external consumer exception. Windows-only and
  custom-tagged source is checked on the Linux host.
- `golangci-lint run --enable-only=depguard ./internal/archtest/... ./clients/...
  ./providers/... ./planner/... ./adapters/clientdetect/... ./installer/...`:
  PASS, zero issues against the actual affected module packages.
- `golangci-lint run --enable-only=revive,funlen,gocyclo,gocognit,dupl,depguard
  ./internal/archtest/...`: PASS, zero issues.
- A compilable synthetic module was checked with both the committed base config
  and the proposed config using the real installed linter. All 36 controls
  matched their expected results: the original config rejected 14 forbidden
  source files; the new config rejected 28. Fourteen additional regressions are
  rejected, zero previous rejections were lost, and positive helper, assembly,
  dispatcher-test and existing external-consumer cases remained accepted.
  This includes future-client imports absent from every registry/list, current
  adapters omitted by the old lists, helper-name prefix collisions, peers,
  shared helpers importing implementations and installer registry assembly.
- `golangci-lint run --allow-parallel-runners ./internal/archtest/...`: PASS,
  zero issues with the full configured linter set after addressing its local
  naming, boolean-expression and preallocation findings.
- `gofmt` was applied to the two changed Go test files; `git diff --check` passed.

The manual linter controls, logs and structured result are reproducible during
this session at `/tmp/uap-architecture-lint-controls/` with
`/tmp/uap_architecture_lint_controls.py`. An initial concurrent linter attempt
hit its process lock; rerunning with a lane-specific cache and
`--allow-parallel-runners` completed both controls. No unsuccessful check was
counted as a pass.

The persistent Go fixtures are the regression gate; the temporary synthetic
linter module is validation evidence, not a new repository tool or runtime
dependency. Independent review and normal PR CI remain pending. No release or
merge was performed.

## Ranked remaining audit followups

The three immediate slices are supervised npm cancellation (runtime correctness,
separate PR), transport-independent environment application (shared protocol,
separate PR) and this client-boundary guard (future-change protection).
Additional issues below are deferred, rather than expanded into this patch.

| Priority | Evidence in the audited base | Future-change risk and bounded next step |
| --- | --- | --- |
| P2 | `usecase/native_attempt.go`, `nativeDialectProjectionChanged`; `installer/native_recovery.go`, `inspectNativeIntents`; `installer/group_remove.go`, `planRemoveGroup` | Generic code still embeds OpenCode dialect/object identifiers and historical Claude/Codex admission policy as string comparisons. The zero client-selector budget cannot detect these semantics. When these behaviors next change, move them behind a narrow adapter capability or an explicit, tested historical-compatibility policy. Mechanically replacing strings with constants would only move the metric. |
| P2 | `installer/identity.go`, `reserveNewBinding`; `installer/project_scope.go`, `nativeRegistryRoot` | Target resolution receives `context.Background()` rather than the operation context; identity reservation also suppresses resolver errors. Current target adapters were not shown to perform long-running work here, so no present cancellation failure is claimed. A new adapter that probes the host could expand that limitation. Review a context-aware identity/target API before adding such work, retaining compatibility for current callers. |

This plan does not claim that import rules enforce all semantic architecture.
The unchanged contract dependency-closure test still sweeps its declared
platform matrix; the layer scanner tested here reads all production build-tagged
files without depending on that matrix.
