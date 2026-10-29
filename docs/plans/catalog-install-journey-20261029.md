# Catalog installation journey — bounded implementation plan

Date: 2026-10-29. Branch: `feat/catalog-install-journey-20261029`.
Base: `ec85ee7d759944b9799e451c96bfc1859f9d5533`.
Status: implementation complete; both independent plan rounds and both
implementation review rounds approved. Main `60967b8` was integrated by normal
merge `b485cf41abfb179e59960f33e4afbd958024d659`. Exact verified source/evidence
head: local `b3a2a2cdc696fb2c20c695519315cfc6dc243a72`, mirrored in PR 564 as
canonical `445f520b441e3765d05b3a74b240facb6783f6ce` with the identical tree.
Full typecheck, four integrated browser cases, and final visual inspection
passed. Completion follow-up `0f1f8fb` also passed all four focused browser cases
against the finished production static artifact. Monolithic generation hit the
shared memory limit during compression; the unchanged stock Nitro build tail
completed in a fresh process. Full workflow qualification is still queued, as
recorded below. No release, deployment, or merge-to-main claim is made.

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
   help and current CLI contracts before changing a displayed command. Every
   active lifecycle command—add, exact-Git re-add, update, repair, and remove—must
   carry `--scope project` when project is requested. Owned update/remove keep
   their existing whole-alias target semantics; scope must not reintroduce a
   subset target flag. Preserve the public `pluginCommands` third Boolean
   release-qualification argument; add scope as a backward-compatible optional
   fourth argument/options contract rather than silently reinterpreting calls.
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
   stale choices according to existing store behavior. For the same package, a
   scope change, target change, or refreshed feed that removes a selected agent
   retains requested project scope and the visible invalid selection with a clear
   reason. Suppress commands until the user makes a compatible explicit choice;
   never reuse the current watcher fallback of `autoDetect = true` to create a
   copyable user command. A different package still starts from its own default
   user/auto choice. No localStorage/session state subsystem.
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

Implementation uses three small shared files within these existing seams:
`composables/usePluginInstallSelection.ts` owns the duplicated card/detail state
policy; `components/registry/InstallScopeSelect.vue` shares the compact control;
`components/registry/InstallOutcome.vue` shares resolved-release next steps. These
replace repeated selection/watch/presentation code rather than introduce a new
installer or state subsystem. The active homepage consumes the same registry
cards; the unreachable legacy section was not expanded.

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
  Assert exact arguments for every lifecycle command, including project scope and
  existing whole-alias target omissions on owned update/remove.
- Resolver contracts: default supports user but fallback supports project;
  different releases support different scope/target sets; no single release
  supports all; revoked/suspended/failed evidence remains ineligible. Ensure
  selected component/delivery/next-step output comes from the winning release.
- Preference contracts: default user/auto; card/detail navigation retains valid
  explicit choices; same-package feed invalidation keeps requested project scope,
  shows the invalid selection, and cannot silently issue a user command.
- Browser journeys on desktop and mobile: expand card, select scope/agents, verify
  copied command arguments, navigate to detail/back, recover an unavailable
  combination, and verify the active homepage card and final-step wording. Exercise
  keyboard labels and narrow-width wrapping; inspect relevant screenshots.
  Include project selection → unsupported selection/feed invalidation → no
  copyable user command, followed by recovery through explicit agent selection.
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
| Independent plan round 1 | `/root/versions_owner`, current plan `2f8ddcf` | Required: exact project scope on every lifecycle command; preserve whole-alias target semantics; explicit same-package invalidation must never fall back to user commands. Nonblocking: preserve third Boolean API argument. All accepted and specified above in this correction. Parent coordinates gate accounting. | Review received and corrections committed; no source edits |
| Independent plan round 2 | `/root/versions_owner`, corrected plan `35cc888` | Rechecked resolver/release scope facts and active card/detail consumers. All R1 corrections reflected; no remaining material plan defect. No generator/native/API expansion justified. Approved subject to parent publication/gate accounting; no further correction required. | Approved; no source edits before parent gate |
| Early draft PR | [PR 564](https://github.com/777genius/universal-agent-plugins/pull/564) | Separate from versions/outdated PR; published before implementation | Complete |
| Implementation commits | Parent published each checkpoint below | Preserve overlap boundaries; remote mirror trees verified equal | Complete through `b3a2a2c` |
| Independent code review round 1 | `/root/plan_code_critic` reviewed `8f203c1`, `f4fbc5f`, `9497bf0`, and `b11b5e9` | No material source defect. Required actual picker recovery is authored and ran in the first successful browser pass; final corrected/integrated run remains required. Visual findings were fixed as described below. | Source approved; final evidence pending |
| Independent code review round 2 | `/root/plan_code_critic`, `b3a2a2c` / canonical `445f520b` | Explicit GO after inspecting final source, raw browser/dev logs, and all four fresh screenshots. Four cases passed in 55.7 seconds; no Vite CSS warnings. Typecheck exit 0, no casts/suppressions. Selected release/header/project notice and main security/CTA/auth merge preserved. No P1/P2 finding; read-only diff check clean. | Approved |
| Final draft handoff | Parent updates PR 564 | Implementation, independent review, scope, evidence and remaining CI qualification recorded; no deploy/release | Ready for review |

## Implementation and evidence checkpoints

| Local checkpoint | Result |
| --- | --- |
| `8f203c1` | Shared scoped command construction and optional release scope eligibility; legacy facade aligned; source safety and saved alias management preserved |
| `f4fbc5f` | Shared card/detail selection policy, controls, invalidation and selected-release outcomes; nine locales and preference contracts |
| `9497bf0` | Four focused browser cases, missing-scope refusal and existing remote app-binding source boundary; no client capability map added |
| `b11b5e9` | Visual findings corrected: project saved notice and scoped status command; explicit card header agrees with selected release; duplicate component presentation removed |
| `b485cf4` | Parent integrated current main `60967b8` with a normal two-parent merge; PR 561 security wording and PR 562 CTA/auth position changes preserved without conflict |
| `b3a2a2c` | Two-line test-only correction reads a freshly updated computed value with Vue `unref`, avoiding stale TypeScript control-flow narrowing; full typecheck passed without casts, suppressions, or check configuration changes |

The first visual inspection exposed two real inconsistencies. A project command
still displayed the old saved-for-user notice, including an unscoped status
example. It now says that the agreement is in this project and shows
`deps status --scope project`, in all nine locales, while user wording remains
unchanged. The card header also counted delivery modes from the default release
while the expanded outcome used a project fallback. Explicit selection now
derives both from the winning release's selected targets. Unit and browser
assertions cover these corrections.

| Verification | Exact outcome so far |
| --- | --- |
| Landing unit suite, `node --experimental-strip-types --test --test-isolation=none tests/*.test.ts` | 337 passed at `f4fbc5f`; no failures/skips |
| Focused command/scope/resolver/preference checks | 19 passed after the `9497bf0` guard; 8 preference tests passed after `b11b5e9`, including the new header regression |
| `node --experimental-strip-types scripts/check-i18n.mjs` | Passed for all nine active locales after the project notice correction and again on the final integrated tree |
| `nuxt prepare` | Passed using read-only existing dependencies and an already verified registry mirror |
| ESLint for edited production files | Final integrated pass: zero errors; one existing `img` self-closing warning in the untouched card identity markup |
| `vue-tsc --noEmit` | Final full integrated check at `b3a2a2c` passed, exit 0 with no diagnostics. The initial resource-bound attempt ended 137; a later actual test-only TS2339 was fixed by the fresh-value read above. Neither check scope nor production code was weakened |
| Browser authoring/listing | Four focused cases compile/list: desktop and mobile real-picker recovery, lifecycle clipboard bytes, card/detail/back and locale remount, and feed-withdrawal refusal/recovery |
| Initial browser attempt | Renderer crashed before loading the application; shared `oom_kill` counter rose from 20 to 21. Not counted as a functional result |
| Coordinated first browser run on `9497bf0` | Four cases passed in 1.3 minutes, one worker; `oom_kill` remained 21 during that run |
| Initial visual artifacts | Inspected, produced the two corrected wording findings above. Not accepted as final visual evidence: reused dependency realpaths triggered Vite CSS allow-list warnings, and one mobile capture occurred during a visual transition |
| Final integrated browser run | `node node_modules/@playwright/test/cli.js test tests/browser/install-scope.spec.ts --config .catalog-playwright.local.ts --workers=1 --timeout=90000 --max-failures=1`: four passed in 55.7 seconds on committed `b3a2a2c`. The 90-second timeout was a local cold-dev allowance; committed static-artifact configuration is unchanged |
| Final visual inspection | All four fresh 1440px/390px card/detail captures inspected by owner and independent critic. Scope controls, selected-release component/delivery/sign-in guidance and scoped project notice are readable; no body overflow. Header and outcome agree. Main's CTA/auth changes remain present. Precise task/dependency allow-list layer removed all earlier Vite CSS warnings; no production serving guard change |
| Final resource observation | `oom_kill` remained 22 throughout the successful final full typecheck and browser window; no new kill. Preview and browser runner stopped after verification |
| Required CI qualification | Full static generation and the entire production-artifact browser matrix remain the existing PR workflow's responsibility; not claimed as locally run. Focused integrated browser/type/visual gates above passed without deployment |

Resource qualification is explicit: the shared filesystem reached zero free
space and the cgroup has an 8 GiB memory limit, including unrelated retained
tmpfs. Dependency/mirror reuse and single-worker verification avoid creating
another installation or broad test matrix. No foreign artifacts were deleted.
Temporary local harnesses and runtime symlinks were removed after final checks;
none were committed. Final tracked/untracked worktree status was clean before
this documentation-only evidence update. Raw task evidence remains in
`/tmp/uap-catalog-final-type.log`, `/tmp/uap-catalog-browser-final.log`,
`/tmp/uap-catalog-dev-final.log`, and the four
`/tmp/uap-catalog-playwright-results/install-scope-scope-select-*/project-*.png`
captures inspected by the independent reviewer. This PR is a draft
implementation, not a release or deployment claim.

## Production artifact follow-up — 2026-10-29

Production source remained unchanged from `5165ef8`. The only new browser test
change, committed as `0f1f8fb`, scrolls the returned card into view and waits for
the existing catalog scroll-reveal ancestor to reach opacity 1 before capturing
it. The first static run passed all assertions, but its desktop card screenshot
was taken during the real reveal transition. The corrected capture check adds a
readiness assertion; it does not disable animation, replace an observer, extend a
timeout, or alter production behavior.

| Completion check | Exact result and qualification |
| --- | --- |
| Real production client/server compilation and prerender | Unchanged committed Nuxt configuration, `/universal-agent-plugins/` base, signed mirror, and all 512 standard input routes; 971 HTML/API/payload routes prerendered. No route reduction or fixture substitution at build time |
| Dependency reuse | Existing dependency installation reused read-only. Its application lock document is byte-identical to the application document in this branch's `pnpm-lock.yaml`; the extra source lock document only bootstraps pnpm itself. Local Node is 24.22.1; CI retains its pinned 22.21.1 runtime |
| Monolithic `nuxt generate` | Not a completed build gate: the 2048 MiB-heap run reached compression and exited 137. A 1536 MiB retry hit V8's own heap limit before prerender (134). Restoring 2048 MiB with one libuv worker again rendered all 971 routes but was killed during compression. The shared OOM counter rose from 22 to 25 across this window |
| Stock static build continuation | In a fresh 512 MiB-heap process, loaded the same production Nuxt configuration with the normal generate/static options and called Nitro's exported `build(nuxt._nitro)` tail against the completed prerender output. Assertions retained `static: true`, `compressPublicAssets: true`, and the expected output directory. Stock Nitro copied the real compiled client/public assets, completed unchanged max-quality gzip/Brotli compression and wrote `.output/nitro.json`; exit 0. One interrupted unpaired sidecar from the earlier kill was removed and regenerated by the same stock tail; final output has 1014 gzip and 1014 Brotli sidecars with no unpaired files |
| Focused production static browser run | Committed `playwright.config.ts` and static `serve` configuration, repository base, one worker, unchanged standard test/assertion timeouts. The production artifact was staged with hard links to avoid duplicating its bytes. Four cases passed in 31.8 seconds initially; after the screenshot readiness correction, four passed in 36.0 seconds on the exact test tree committed as `0f1f8fb`. No dev server, Vite filesystem allowance, or browser configuration override |
| Visual inspection | All four final 1440 px / 390 px card/detail captures inspected. Owned installation controls, selected-release component/delivery and sign-in wording, project saved-dependency notice, and scoped lifecycle commands are readable. Both real-picker recovery and feed-withdrawal refusal/recovery passed; body overflow assertions passed |
| Resource observation | Stock continuation, sidecar completion, and both static browser runs held the shared OOM counter at 25; all B preview/browser processes stopped after the checks |
| Required workflow gate | [Landing run 40903189097](https://github.com/777genius/universal-agent-plugins/actions/runs/40903189097) still has `test`, `combined-pages-validation (/)`, and `combined-pages-validation (/universal-agent-plugins/)` queued. Read-only job metadata shows runner ID 0, no runner name, and label `nscloud-ubuntu-24.04-amd64-4x8`; none has acquired a runner. The existing `test` job already includes all browser specs after full production generation, including this scoped journey |

The public GitHub release request timed out during local prerender, leaving the
existing honest `ok: false` response in `/api/releases/latest`. Consequently this
local artifact does not qualify the unrelated release-base tests or the full
assembled Pages matrix. The fresh-process stock tail and focused static browser
pass establish production-artifact behavior for this installation journey; they
do not turn either interrupted monolithic generation or queued CI into a pass.
No production guard, signed-feed check, workflow step, native capability, or
release policy was weakened. CI runner availability and the complete Pages
workflow remain the outstanding integration gate, not an identified B source
defect.

Raw follow-up evidence is retained in
`/tmp/uap-catalog-static-generate.log`,
`/tmp/uap-catalog-static-generate-serial.log`,
`/tmp/uap-catalog-static-generate-final.log`,
`/tmp/uap-catalog-static-resume.log`,
`/tmp/uap-catalog-static-resume-repair.log`, and
`/tmp/uap-catalog-static-browser-final.log`. Final captures are the four
`/tmp/uap-catalog-static-results-final/install-scope-scope-select-*/project-*.png`
files. The temporary stock-tail script, runtime symlinks, and task-owned generated
artifacts are removed after verification; no foreign dependencies, caches, or
artifacts are deleted.
