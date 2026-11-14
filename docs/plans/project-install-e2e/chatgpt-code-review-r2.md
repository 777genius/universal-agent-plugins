# ChatGPT project boundary independent implementation review R2

Reviewer: cursor_clients, 2026-10-23. Reviewed frozen checkpoint
`74608d3b4340815d9603bed7030411b15818be2e` in `work/uap-project-chatgpt`.
Read the approved ChatGPT client plan, repository governing instructions and
capability-preservation contract, implementation diff, production CLI scope and
saved dependency entrypoints, direct adapter lifecycle, and added fixtures.

## Decision: GO for the explicit unsupported project boundary

No correctness blocker found. This is approval of the approved negative leaf:
it does not qualify a ChatGPT project filesystem installation, remote app
connection or account/workspace operation.

## Reviewed boundaries

- ChatGPT does not advertise ScopeProject, ProjectTargetLayout or
  NativeRegistryLayout. The implementation adds no fake project native root,
  user-profile substitution or artifact-only bypass. Existing user mapping
  authorization and manual/prepared lifecycle remain intact.
- Production CLI project target admission rejects explicit ChatGPT and mixed
  target selections before detector, acquirer, security evaluator or mutation.
  Imperative/saved add, positional install, personal-registration inputs,
  activation/authentication attestation, exact-ID update/repair/remove and
  invocation-directory root defaults retain the stable unsupported diagnostic.
- Saved lock/update now perform a pure manifest-target preflight before the
  environment lock can create `.uap`. The operation still rereads and validates
  the authoritative manifest under lock, checks pending recovery and compares
  manifest bytes before publishing. The helper extraction preserves existing
  installer-nil and recovery-required behavior. Sync/environment install have
  the same target refusal, with project guidance taking precedence over the
  user-only manual-preparation suggestion.
- The transfer preparation path also validates source agreement targets before
  confirmation, lock/publication or install. Project scope does not gain an
  unsupported import/install escape through that existing generic route.
- Direct activation preflight refuses either project scope or a ProjectRoot
  even with inconsistent user labels, personal preparation or attestation.
  Deactivation similarly refuses unsupported project completion despite an
  external-uninstall assertion. The provider calls preflight before its generic
  attestation shortcut, and direct adapter activation calls the same guard.
- Added E2E contracts assert zero host/acquisition/security calls, exact
  user/A/B byte-and-mode preservation, and absence of mutable namespace, lock
  and user-state creation. Existing intent is preserved. Public guidance keeps
  unsupported project behavior and bounded user preparation distinct.

## Independent verification

With the existing Go 1.26.10 toolchain, GOMAXPROCS=3 and `-p=2 -count=1`:

- Complete `clients/chatgpt` suite: PASS, 0.013s.
- Production composed CLI `./cmd/agentplugins -run '^TestProjectChatGPT'`:
  PASS, 0.442s.

These execute the negative project E2E and direct user/project adapter
contracts. No native client, MCP command, model call, remote API, trust,
registration, authentication or workspace publication was run. The passing
fixtures establish refusal/isolation, not remote product capability.
