# Architecture boundaries that keep covering new clients

Date: 2026-10-30. Base: `ae068426706d5f2c9d624ae94c6504ac8b7f553e`.
Branch: `refactor/architecture-boundaries-20261030`.
Status: implementation plan; publish this checkpoint before implementation.

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
point. An external consumer test may import its own adapter, preserving the
existing Gemini consumer tests. Existing contract-test helpers remain usable.
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
3. Add negative fixtures in the guard's own test data and exercise the real
   scanning/checking path. Cover a future unregistered client, a peer import, a
   shared-helper import of a concrete adapter, nested client packages and
   platform/custom build-tagged production files. Add positive cases for the
   root contract, shared helpers, `clients/all` assembly, own external consumer
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
