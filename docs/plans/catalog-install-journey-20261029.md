# Catalog installation journey — bounded implementation plan

Date: 2026-10-29. Branch: `feat/catalog-install-journey-20261029`.
Base: `ec85ee7d759944b9799e451c96bfc1859f9d5533`.
Status: corrected reconnaissance checkpoint; implementation waits for two
independent review rounds and disposition of their findings.

## Outcome and boundaries

Let a catalog visitor choose supported agents and user or project installation,
copy a command accepted by the released CLI, and understand what is delivered and
which sign-in or activation step remains. Keep the existing homepage installation
journey and Directory installation journey consistent without building a generic
installer, a new browser service, a second capability database, or an authoring
system. Preserve current automatic agent detection and user scope defaults.

This work is independent of the separate version/outdated CLI work. It targets
released 0.1.94 behavior, including explicit project scope and bare project
installation, and must not advertise unreleased version features. No production
deployment or release is included; create an early draft PR and keep it current.

Explicit exclusions: CLI/backend/domain/version DTO/OpenAPI changes; native
adapter policy; VS Code; Pi availability policy; scanner behavior; project APIs;
authoring engines/kits; security-card wording, auth-lock position, button text
color, or unrelated theme changes. Existing PR 561 security-card work, PR 415
installer-confirmation work, and PR 520 native work retain their ownership.

## Existing implementation and sources of truth

- `landing/utils/commands.ts` builds Directory commands and validates exact
  community Git source selectors, but has no scope input.
- The active homepage is `pages/index.vue` → `RegistryDirectory.vue` →
  `UnifiedCatalog.vue` and the shared registry card surfaces. Legacy
  `PluginInstallSection.vue` and `getResolvedPluginInstallSpec` have no production
  consumer at this base (only existing tests). `landing/data/pluginInstall.ts`
  still separately builds those legacy commands. Align its command facade with
  the shared builder, but do not create a new visible legacy homepage journey or
  broaden its curated five lanes.
- `RegistryPlugin.client_support.scopes` is the normalized per-client scope map
  (the feed/domain uses `scopes_by_client` elsewhere). `ReleaseTarget.scopes` is
  the release-specific authority. A union of client support is insufficient to
  promise one release for a multi-agent selection.
- `resolveDistribution` already selects one eligible release/distribution for an
  entire target set and reports fallback diagnostics. Scope must participate in
  this selection rather than be checked after an unrelated release is chosen.
- `clientCapabilities.generated.json` already describes client component and
  saved-dependency capabilities. `SAVED_DEPENDENCIES_CLI_RELEASED = true` is the
  existing release qualification; retain one authority instead of adding manual
  per-feature booleans. The existing generator can access `clients.SupportsScope`,
  but B needs no global scope export: active surfaces already have release-level
  scope facts. Leave the generator and client metadata unchanged. Global client
  admission could never override package-specific release scope anyway.
- `InstallPanel.vue`, `RegistryPluginCard.vue`, `PluginInstallSection.vue`, and
  `installPreferences.ts` already implement selection, command copying, default
  detection, and navigation state. Extend these seams rather than adding a new
  manager. Preserve signed-snapshot freshness checks and read-only discovery.
- Nine landing locales are maintained: en, ru, es, fr, zh, ar, hi, pt, uk. Add only
  the small set of journey strings needed here to all nine.

## Intended design and file ownership

1. **Selection and eligibility.** Extend the existing resolver with an optional
   scope argument. Keep its public target-only behavior backward compatible.
   Release eligibility intersects selected clients, requested scope, active
   release state, component requirements, trusted blocking failures, and existing
   upstream evidence. A successful selection always represents one release;
   never combine separate distributions to manufacture multi-agent support.
   Keep small pure scope-selection helpers beside existing command utilities
   only when both UIs need them; no rules engine or injected framework.
2. **One command construction seam.** Reuse a small typed command builder from
   `commands.ts` in `pluginInstall.ts`, retaining each facade's current invocation
   convention (`npx -y` versus selected installed CLI). Build flags once for add,
   update/re-add, repair, and remove. Preserve exact Git pins, source safety,
   dependency-saving policy, owned-package update/remove semantics, and explicit
   homepage target selection. Confirm each scope/lifecycle flag against released
   help and current CLI contracts before changing a displayed command.
3. **Directory controls.** Add a compact scope control to the expanded card and
   detail InstallPanel. Label choices in ordinary terms: for this user versus
   this project. Project requires an explicit compatible agent selection when
   automatic detection cannot guarantee a truthful target set. Explain a disabled
   project choice with one short, specific reason. Selecting project turns off
   automatic detection and preserves the selected agents; automatic detection is
   unavailable while project is selected. Do not silently omit any agent. Show
   selection controls even if the combination has no command, so the visitor can
   recover. Derive explicit target availability from one-target release resolution
   across eligible releases, including fallback distributions, rather than only
   the normalized default release's aggregate clients. Disable project entirely
   with a clear reason when no package release qualifies any target for project.
4. **Active homepage and legacy compatibility.** Shared registry cards deliver
   the same scope behavior on the homepage and Directory without a separate
   implementation. Legacy `pluginInstall.ts` retains its public facade and
   existing lane metadata and delegates command assembly to the shared builder.
   No changes to unreachable `PluginInstallSection.vue` are planned. Project
   guidance on active surfaces says to run from the project folder; never show a
   user install path as the project destination.
5. **Preferences.** Add scope to existing package preferences with user as the
   default. Preserve explicit target/scope choices between card and detail when
   still valid. Changing package or losing supported targets invalidates only
   stale choices according to existing store behavior. Switching targets while
   project is selected must not silently install into user scope; keep the
   requested scope with an actionable explanation or explicitly reconcile before
   issuing any copyable command. No localStorage/session state subsystem.
6. **What happens next.** For explicit selections, derive component/delivery
   summaries and authentication/activation guidance from the resolved release's
   selected targets, not the plugin-wide default aggregate. Use existing labels
   for managed, prepared, and manual activation; keep the broad-user summary
   concise, with detail only where it resolves a decision. Automatic selection
   must describe checks that occur in the CLI rather than pretend to know the
   local machine. Never infer successful sign-in or activation from preparation.

Owned production files: `landing/utils/commands.ts`,
`landing/data/pluginInstall.ts`, `landing/utils/registry.ts` (resolver only),
`landing/types/registry.ts`, `landing/stores/installPreferences.ts`,
`landing/components/registry/InstallPanel.vue`,
`landing/components/registry/RegistryPluginCard.vue` (installation region only), and new keys in
`landing/locales/*.json`. Touch plugin detail page only if necessary to wire
existing models; avoid its security and general visual sections. Do not change
the capability generator, native adapters, or generated client metadata.

Owned tests: focused command/scope/resolver/preference tests in `landing/tests/`,
one focused browser journey spec plus minimal fixture additions under
`landing/tests/browser/` and `landing/tests/fixtures/`. Do not regenerate large
signed registry fixtures or overwrite other journey plans. No production line
count target; extract only real shared behavior.

## Source and selection rules

| Source/selection | Scope and command rule | User-facing boundary |
| --- | --- | --- |
| Published Directory alias | Requested scope must be supported by one eligible release for all explicit targets | Explain fallback or unsupported combination; retain freshness gates |
| Exact external Git/community source | Keep validated exact commit selector and existing save/no-save decision; use only declared supported scopes | Discovery remains unreviewed and is checked again by CLI |
| Unavailable discovery or retired package | No new install command | Preserve existing source/vendor recovery guidance |
| Native package/distribution | Use signed release scope/component facts and existing generated client capability contract | Describe delivery without promising app loading |
| ChatGPT app binding | User/manual preparation only; no project promise | Finish connection/setup in ChatGPT using existing guidance |
| Automatic detection | Preserve user-scope default; qualify what CLI decides at runtime | Do not imply all catalog agents are installed locally |
| Explicit multiple agents | Intersect scope and release capability for complete selection | Never silently drop agents or mix sources |
| Missing/unknown scope facts | Fail closed for project; preserve compatible user behavior | Say project installation is unavailable for this selection |

## Implementation checkpoints (after both plan review rounds)

1. Commit shared scope eligibility and command contract with focused unit tests.
2. Commit Directory selection/preferences and scoped next-step presentation.
3. Commit legacy command-facade alignment and all nine locale strings.
4. Commit focused browser regressions, record visual verification and limitations,
   update this plan's completion ledger, and request final independent code
   review through the parent. Fix review findings in small commits.

Commit each coherent checkpoint immediately and notify the parent with SHA,
changed files, verification, and risks. The parent publishes each checkpoint to
the separate early draft PR. Do not touch other worktrees or remote refs.

## Validation and acceptance

- Table-driven command contracts: user/project; explicit one/multiple targets;
  Directory aliases; exact Git pin including safe unusual path; saved/no-save;
  add/update-or-re-add/repair/remove; legacy unreleased-saving parameter; invalid
  target/scope combinations; mixed scope support; ChatGPT; missing scope data.
- Resolver contracts: default supports user but fallback supports project;
  different releases support different scope/target sets; no single release
  supports all; revoked/suspended/failed evidence remains ineligible. Ensure
  selected component/delivery/next-step output comes from the winning release.
- Preference contracts: default user/auto; card/detail navigation retains valid
  explicit choices; invalidation cannot silently change project to user command.
- Browser journeys on desktop and mobile: expand card, select scope/agents, verify
  copied command arguments, navigate to detail/back, recover an unavailable
  combination, and verify the active homepage card and final-step wording. Exercise
  keyboard labels and narrow-width wrapping; inspect relevant screenshots.
- Run relevant landing unit tests, i18n consistency, lint for edited files, and
  build/type checks required by existing landing workflow. Use existing browser
  harness rather than inventing a browser toolchain. Record concrete blocked
  checks; do not claim live sign-in, native activation, or public release proof.
- Acceptance: user can tell where installation goes, which agents/components
  receive it, whether an app step remains, and why a project choice is unavailable;
  every displayed command matches the chosen released scope and target set.

## Review ledger and PR checkpoints

| Gate | Reviewer / checkpoint | Findings and disposition | State |
| --- | --- | --- | --- |
| Initial plan | This commit | Bounded scope, ownership, source rules, release authority, validation | Ready for draft PR |
| Reconnaissance correction | Follow-up plan commit | Active homepage shares registry cards; legacy install section is unreachable; no generator changes; explicit project/auto transition and all-release target availability | Before independent review |
| Independent plan round 1 | Parent assigns independent critic | Pending; amend and commit before round 2 | Required before implementation |
| Independent plan round 2 | Parent assigns independent critic | Pending; amend and commit before source edits | Required before implementation |
| Early draft PR | Parent publishes initial plan commit | Separate from versions/outdated PR | Pending |
| Implementation commits | Parent publishes each checkpoint | Preserve overlap boundaries | Pending |
| Independent code review | Parent assigns reviewer | Resolve correctness, UX, architecture, and test findings | Pending |
| Final draft handoff | Parent updates PR | Evidence, scope, limitations; no deploy/release | Pending |
