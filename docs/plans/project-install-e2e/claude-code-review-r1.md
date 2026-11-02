# Claude project implementation independent review R1

Reviewer: core_clients, 2026-10-23. Reviewed initial adapter `3505f0ab`,
source-only native gate `c47d3393`, scoped SDK/data/restart contracts `40f108c4`,
and frozen corrective `32dd485e789b57c7825c6c4fb2128263901be78f` in
`work/uap-project-claude`. The original two client-plan GO results remain
separate. No production or test file was edited by this reviewer.

## Decision: GO at 32dd485e

One exact native identity finding is independently verified closed. No
remaining implementation blocker was found in the scoped directory/listing
lane or the authored source-only native gate. This is an implementation review,
not a completed hosted native project qualification.

## R1-1: lexical native paths could conceal a different physical project

At `40f108c4`, project listing accepted filepath.Clean(installPath) equal to the
expected managed path. A native spelling such as skills/alias/../artifact can
lexically normalize to project A's expected path while physically resolving to
project B/artifact if alias points at B/sub. The optional projectPath check also
normalized its input instead of demanding exact selected spelling.

Closed in `32dd485e`. Project entries now require literal installPath equality
to the validated expected path and literal optional projectPath equality to
the selected project. The user-only parser behavior is unchanged. Regressions
include nonclean install/project paths and a real symlink traversal fixture
that proves physical resolution differs despite lexical normalization. The
authored native project fixture also rejects nonclean discovered install paths.
The correction performs no filesystem mutation or trust action.

## Reviewed scope and native evidence boundaries

- Validated original user/project TargetContext selects exact project
  `.claude` authority and `.claude/skills/<physical-artifact>` delivery.
  Layout, plan precondition, projection, native registry listing, activation
  and deactivation revalidate that selected authority. Package staging remains
  beside the watched skills root, preventing precommit staging from appearing
  installed. No user-profile surrogate or package-derived project scope exists.
- Every project listing/removal probe uses fixed selected project cwd and the
  original user config root in CLAUDE_CONFIG_DIR. The bounded environment and
  existing timeout/runner supervision remain intact. The user-only list helper
  remains user-scoped. Project operations do not invoke user uninstall or write
  native settings/trust/account state.
- The existing package projector, standard loader, rendered command skills,
  managed stdio launcher, directory transaction, immutable package receipts and
  persistent data contract are reused. Core's declared project CLI lifecycle
  matches the real directory/listing lane; it does not borrow an invented
  direct-config engine.
- Activated/installed status requires an actual exact enabled project
  name@skills-dir listing at the managed path. Wrong scope/path, disabled,
  duplicate, malformed, load-error and mismatching optional-project rows do
  not establish installation. Missing or failed observations do not become
  success. Project attestation is rejected unconditionally before the generic
  provider shortcut, including prepared component selections and direct reuse.
- Deactivation requires exact selected-project listing before core removes the
  owned directory. Recognized absent evidence cannot silently remove an
  allegedly active binding. No global plugin or whole shared settings document
  is treated as owned.
- Public SDK fixture journeys exercise user/A/B install, unchanged update,
  exact supplied-pin repair after drift despite a moved source, selected update,
  status and exact removal. Distinct stdio data receipts retain marker bytes.
  Postcommit listing failure leaves incomplete/nonactive ownership; a fresh
  installer repairs the recorded pin and reobserves exact identity. Fixture
  runners are explicitly not native client evidence.
- The native gate uses the genuine pinned artifact and actual UAP project
  operations, fresh plugin list/details, command-byte checks and project-B
  absence checks. Its separate opt-in reaches the unchanged disposable guard
  before client launch/acquisition. Exact required-test accounting rejects skips
  or omitted tests; released/draft/prepared inputs cannot select project proof.
  Claude/Codex selector rows must be unioned on integration as documented.

## Independent execution

Go 1.26.10, GOMAXPROCS=3, `-p=2`, bounded RAM toolchain/cache at final SHA:

- Project listing/client contracts: **PASS 0.007s**.
- Composed production provider project contracts: **PASS 0.042s**.
- Public installer user/project-A/project-B lifecycle/data/restart contract:
  **PASS 0.177s**.
- Inert native discovered-skill parser contract: **PASS 0.008s**.
- Native runner Python safety/evidence suite: **56 tests PASS 0.455s**.

An initial SDK run used this reviewer's overly restricted PATH without the
already installed Node runtime; stdio readiness was withheld and the test's
required data assertion correctly failed. Retaining the installed Node path
resolved that harness issue without source, policy or assertion changes.
The existing full client permission-denial suite requires unprivileged execution
as documented by the author; this review used the focused project contracts.

No native Claude process, configured MCP command, model, account mutation,
login, auto-trust or hosted qualification ran in this review. Local official
binary probes previously aborted with Bun134 and remain crash evidence. The
authored native gate and historical user native results do not qualify new
project discovery, skill execution, MCP initialization/tool use, native restart
or release readiness until genuine hosted results are collected.
