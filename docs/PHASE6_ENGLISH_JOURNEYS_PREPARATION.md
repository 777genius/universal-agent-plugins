# Phase 6 D1 English journeys preparation

Task: uap-authoring-phase6-english-journeys-20260906.
Status: bounded English content preparation; public activation is not accepted.
Date: 2026-09-06 UTC.

## Scope and immutable inputs

Initial HEAD was exactly `ec0883bf0bfb7f4044321bba9dfd4fd8d6467721` and
`git status --short` was empty. One exclusive Git index-lock creation was
attempted and failed with `EROFS` on `.git/index.lock`. No lock was created,
no existing lock was removed, and no repeat lock or commit attempt was made.
Root reviews and commits mechanically after this terminal handoff.

Future command facts were read with `git show 070663ef:<path>` in
`/var/data/uap-authoring-test-20260906/private-npm-pair-integration`.
The full source pin is `070663efb27f69ecae8609e6b839f86f843efbb0`.
No private source was edited or executed. This is command-source evidence,
not proof that a v2 package is available or a release accepted.

Historical tag `v1.2.4^{}` resolves to
`9beca10448ac50fbe526a52101d1433a12471980`. The annotated tag object is not
substituted for that commit. Historical command context comes from that commit,
not current main. Current installer examples come from the frozen README,
`npm/agentplugins/README.md`, `docs/NATIVE_INSTALL.md`, and the existing
`docs/AGENTPLUGINS_CLIENT_E2E.md` list-command evidence.

Read controls: AGENTS.md, owner preservation contract, implementation-plan owner
clarification, full intake main sections and relevant inventories/appendices,
and `/var/data/uap-authoring-test-20260906/inputs/WORKER_SCOPE_CONTRACT.md`.
Requested hosting policy: gpt-6-astra LOW/default, no fast. No model overrides,
subagents, or fast-mode actions were requested by this worker; this note does
not attest hidden runtime telemetry.

## Route inventory and rationale

These are source route contracts for later integration, not activated URLs.
All 13 pages visibly identify prepared, unreleased authoring content.
Paths below are relative to `website/source`; index routes use trailing slashes.
VitePress frontmatter retains title, description, canonicalId, section, locale,
generated=false, and translationRequired=true conventions.

| Source | Proposed route | Lines | Purpose |
| --- | --- | ---: | --- |
| en/use/index.md | /en/use/ | 81 | One-screen Use/Build/legacy choice and command boundaries. |
| en/use/install.md | /en/use/install | 106 | Existing discovery, explicit target dry run, install and activation. |
| en/use/manage.md | /en/use/manage | 103 | Managed state, update, repair and removal. |
| en/build/index.md | /en/build/ | 82 | Template choice and equivalent future entrypoints. |
| en/build/skill.md | /en/build/skill | 105 | Standalone standard Skill package, useful instruction body. |
| en/build/mcp-remote.md | /en/build/mcp-remote | 99 | Endpoint inputs and remote runtime/authentication boundary. |
| en/build/mcp-stdio.md | /en/build/mcp-stdio | 107 | Node stdio files, prerequisites and static-only evidence. |
| en/build/hybrid.md | /en/build/hybrid | 105 | Skill plus explicit remote or stdio MCP choice. |
| en/build/skills.md | /en/build/skills | 95 | Extra Skill creation, exact root and committed-result recovery. |
| en/build/layout.md | /en/build/layout | 103 | Manifest authority, component roles and optional metadata. |
| en/build/checks.md | /en/build/checks | 118 | Conformance, readiness, compatibility and doctor interpretation. |
| en/build/handoff.md | /en/build/handoff | 102 | Local installer validation and dry-run boundary, no install execution. |
| en/legacy/v1/index.md | /en/legacy/v1/ | 100 | Exact 1.2.4 context and retained YAML/runtime/SDK/design pointers. |

Total website content: 1,306 lines. This note's count and total added line count
are recorded in external HANDOFF.md. The split keeps each tutorial complete
while centralizing report semantics and avoiding copies of historical pages.
No existing public route, page, navigation, generated registry, locale, landing,
metadata, code, test, or workflow file is changed. There is no redirect change.

## Command review evidence

The exact future factories in `internal/authoring/commands/commands.go` and
`skills.go` define supported jobs, flags, explicit roots, and Skill arguments.
`public_contract.go` and `internal/authoringcli/{command,release}.go` distinguish
public parsing from inherited installer flags. Scaffold `plan.go` and
`templates.go` establish actual files, required URL/runtime inputs, Node >=22,
Skill names, optional license inputs, and the absence of install/execution.
`cmd/plugin-kit-ai/release_compat.go` supplies retirement and migration context.
Paths in this paragraph are beneath `cli/plugin-kit-ai/` at the future pin.

Tutorials use explicit names, descriptions, destinations and required template
inputs. `--mcp-template` is the public hybrid flag. All authoring command families
have equivalent `agentplugins author` and `plugin-kit-ai` guidance. Installer
validate/doctor remain separate. Remote example URLs are visibly placeholders.
No future npm acquisition snippet, implicit YAML fallback, deferred runnable v2
job, bulk example migration, or claim of a maintained second engine is added.

## Lightweight verification and limits

Checks ran in an external disposable directory using existing Python, Bash,
and Node only. No dependencies were installed or provisioned.

- Copied existing frontmatter helper and its tests: 3 tests passed with Node.
- Draft frontmatter: JSON-compatible scalar parsing, required keys, English
  locale, unique canonical IDs, and preparation markers passed for 13 pages.
- Markdown: balanced code fences, one prose H1, and trailing-whitespace checks
  passed. Shell blocks passed `bash -n`; no command in those blocks was executed.
- 59 internal draft/historical links resolved to current source pages.
- 10 pinned repository blob/tree links resolved through local Git objects.
  Review caught a nonexistent tagged authoring-doc path; it was corrected to
  the existing tagged CLI README before final verification.
- Static command-block checks reject deferred v2 jobs and legacy/installer flags
  in future authoring examples. Source review, not a candidate binary run,
  establishes the remaining argument semantics.
- Tracked-file diff against HEAD remained empty: all task changes are additions.

These are bounded source checks, not a Markdown renderer or comprehensive linter.
External HTTP links, built routes, anchors, .html aliases, redirects, site base
paths, locale parity and navigation were not exercised. No docs generation,
full build, browser, candidate binary, product runtime, live service, real user
project/profile, provider action, dependency install, push, PR or publication ran.
Owning lanes must perform full docs/locale/landing activation checks later.

## Preservation inventory

| Inventory item | Disposition and unchanged location | Support interpretation |
| --- | --- | --- |
| Existing English guides/reference/releases/API | Preserve in website/source/en and website/generated/en. | Historical/support links are contextual; same-named v1/v2 jobs differ. |
| YAML specification and authoring design | Preserve docs/PLUGIN_YAML_V1_SPEC.md and existing docs. | No standard-root fallback or conversion. |
| Launcher/runtime/SDK source and docs | Preserve cli, sdk, npm and python trees. | Retention does not register deferred v2 commands. |
| Native generation/import, bundle/publication services | Preserve implementation, dependencies and tests in place. | No removal decision or execution claim. |
| External Skills and integration lifecycle services | Preserve existing implementation and tests. | Future authoring exposes only local Skills init/validate. |
| Legacy examples and starters | Preserve all examples without conversion or moves. | D4 may classify descriptions; no new standard-MVP claim. |
| Existing locales, public navigation and deployment | Preserve all existing files. | D2–D5 own integration and activation. |

Deletions: zero. Moves: zero. Owner-accepted removal items: none. No unresolved
capability is retired; retention outside the standard graph remains the rule.

## Handoff dependencies and release boundary

D2 consumes the route table and canonical IDs. Generate future CLI reference
from shared factories at the accepted final SHA; do not invoke/register the v1
export shim. Own navigation, generated groups, old-route/fragment inventory,
static aliases and source/rendered link checks. Preserve old CLI route meaning.

D3 consumes the same English routes. Update RU/ES/FR/ZH or supply explicit
canonical-English fallback with localized context; remove stale runnable current
journey snippets rather than relying on a banner. Verify all five locales,
locale switching, canonical IDs and navigation together with D2.

D4 owns root README, landing, shared strings/SEO and example classification.
Connect Use and Build consistently, preserve create-plugin visibility/noindex
constraints, and retain useful legacy text. Coordinate all five landing locales.
Do not change runtime examples or infer Build publication from installer copy.

D5 must obtain final accepted engine SHA, native/wrapper same-engine provenance,
actual published channel/version evidence, and accepted Milestone A/platform,
upgrade, checksum/cache and launcher evidence from their owners. Regenerate and
recheck commands, full docs, link/alias, locale and landing behavior before
activation. Package metadata copy remains with the package owner.

**Do not merge or activate this standalone preparation branch before locale,
navigation and release gates are satisfied.** Pages auto-deploys main/master;
a visible draft notice is not a deployment gate. No gating framework is added.
Root may review and mechanically commit this bounded patch to a non-deploying
preparation branch after terminal handoff; that is not permission to merge.

Release acceptance=false; platform acceptance=false; public activation=false;
Phase 6 complete=false; global program acceptance=false. D1 content preparation
and bounded source checks do not satisfy these later gates.

External terminal artifacts:
`/tmp/uap-authoring-phase6-english-journeys-20260906-artifacts/`
contains HANDOFF.md, terminal.patch, SOURCE_PINS.json, CHECKS.json, line counts,
and disposable check inputs. Git metadata was read-only, so no commit was tried.
