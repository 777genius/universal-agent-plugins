# Claude project adapter independent implementation review R2

Reviewer: cursor_clients, 2026-10-23. Reviewed the final adapter and source-only
native gate stack through `40f108c43b7dcbd8d5e57c39799072ece048fbc3`, plus the
R1 corrective frozen checkpoint `32dd485e789b57c7825c6c4fb2128263901be78f`, in
`work/uap-project-claude`. Read repository governing/capability-preservation
instructions, the approved two-GO Claude client plan, production adapter,
public installer and provider fixtures, native gate and matrix selection.

## Decision: GO at corrected checkpoint 32dd485e

No remaining implementation blocker found. The original frozen adapter's
project listing path normalization weakness was already identified by the
independent R1 reviewer. This R2 independently checked its final correction:
project `installPath` and optional `projectPath` require literal equality to
the validated selected paths; nonclean traversal spellings cannot manufacture
identity. Legacy user-only listing normalization remains unchanged. The
actual symlink-plus-parent traversal regression demonstrates that the rejected
spelling can resolve into another directory even when filepath.Clean makes it
look equal to the desired managed path.

## Reviewed boundaries

- Native project layout is exactly `.claude/skills/<physical-artifact>`.
  TargetContext, original detected user ConfigRoot, `.claude` authority,
  target anchor/root and active child layout are revalidated. Staging remains
  beside the watched skills directory. Core supplies transactional package
  ownership, project state/data and physical root authority.
- All project registry, activation and removal listing commands preserve the
  original `CLAUDE_CONFIG_DIR` and set Dir to immutable selected ProjectRoot.
  They do not substitute project configuration for a user profile, depend on
  ambient cwd, invoke marketplace installation or perform user uninstall.
  Existing bounded environment and supervised process timeout remain in use.
- Scoped native listing requires the exact expected `name@skills-dir`, exact
  selected project scope/path, enabled status, no duplicate identity, no load
  errors and consistent optional projectPath. Wrong user/path identities are
  collisions, malformed/unrecognized observations remain unknown, and exact
  negative evidence cannot be upgraded by attestation.
- Provider activation preflight runs before generic attestation shortcuts,
  and direct adapter activation invokes the same guard. Project attestation
  is refused even when selected components would otherwise take a prepared
  path. Recognized exact native plugin listing may establish installed plugin
  activation under the existing lifecycle; it does not establish MCP
  initialization, skill execution or model/task completion.
- Project removal uses the same scope/cwd/config roots and verifies exact
  managed native identity before core removes the owned directory. It never
  treats a shared settings file or another scope as owned. User behavior and
  the existing user-only listing entrypoint retain their prior contract.
- Production loader/planner/renderer/stager/provider contracts cover rendered
  commands, wrong/disabled/unknown listings, copied/changed scope/root/user
  authority, attestation refusal and user/project-B isolation.
- Public SDK tests install all three scopes with separate stdio data locators,
  retain data through update/repair/removal, repair from exact supplied pinned
  bytes after source advance, and preserve user/B binding and package bytes.
  A native listing failure injected after commit produces an incomplete,
  non-active owned binding; a fresh installer instance repairs the selected
  pin and reobserves exact project identity. These are production lifecycle
  fixture contracts with an inert runner/launcher, not native Claude proof.
- The new source-only native test runs production UAP project operations and
  genuine pinned read-only plugin list/details processes at project cwd,
  checks project-B absence, exact rendered commands and foreign hashes, and
  emits separate project evidence. The unchanged disposable guard runs first.
  There is no session/model/trust/MCP initialization/login request. Required
  test accounting refuses skips/omissions, and released/draft/prepared inputs
  refuse this source-only selection.

## Independent verification

Using Go 1.26.10, GOMAXPROCS=3, `-p=2 -count=1`, independently ran
`./clients/claude ./providers ./installer -run '^TestProjectClaude|^TestProjectListing'`
at the clean corrected checkpoint:

- Scoped client listing contracts: PASS, 0.008s.
- Composed project provider contracts: PASS, 0.118s.
- Public installer three-scope/data/restart lifecycle: PASS, 0.243s.
- `python scripts/test_native_client_matrix.py`: PASS, 17 tests.

These are reviewer execution results. No native Claude process, package
payload, MCP command, model request, login or trust action was executed.
Existing complete client permission-denial tests were not rerun as task root;
the author's separately recorded unprivileged full-suite run is not relabeled
as reviewer execution.

## Integration and native qualification boundary

The shared native matrix delta must retain the union of independently authored
Codex and Claude project selections/workflow conditions on integration. The
author evidence explicitly records that requirement. Native fixture authorship,
parser/compile checks, historical user runtime evidence and local Bun exit134
probe failures do not qualify the new project runtime. Genuine hosted project
listing/discovery, MCP/skill execution and fresh-session task evidence remain
separate gates. This review makes no release-readiness claim.
