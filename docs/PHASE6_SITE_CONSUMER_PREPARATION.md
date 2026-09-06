# Phase 6 D2b website consumer preparation

Task: `uap-authoring-docs-site-consumer-20260906`.
Status: bounded consumer implementation and focused checks complete; full website
build/browser validation and public activation remain **unaccepted**.

## Exact inputs and execution boundary

Initial HEAD: `25f0b01985daaecc52e6dae4303cb49a052099aa`; initial porcelain status
was empty. This explicit temporary base mechanically includes D1 PR171
`914737b568ec9956ce59cae831536dd9e5ef0645` and independently accepted D2a PR175
`e2297fc7528c316d5bae158f09ab65e5f0fdd410`. Neither merge proves release.
The one uniquely named `.git/d2b-probe-*` exclusive creation failed read-only;
no lock was created, removed, or retried. No commit/history mutation was tried
in the source worktree. Root owns mechanical commit of the terminal patch.

Read controls: repository AGENTS.md, capability preservation contract, owner
clarification and Phase 6 of the implementation plan, D1 and D2a preparation
notes, the external D2b integration contract, and all 547 lines of the intake.
Requested worker policy is gpt-6-astra low/default, no fast; no worker was spawned
or model override requested. This does not attest hidden hosting telemetry.

The adapter was first executed on the clean exact base, then twice through the
new extractor against a clean disposable shared-object clone at that same SHA.
Its 34 command records comprise 20 `plugin-kit-ai` records and 14
`agentplugins author` records, including actual utility commands. All 52 source
pins and factory baseline `070663efb27f69ecae8609e6b839f86f843efbb0` survive in
the model. No product command, native/npm suite, Windows reproducer, provider,
user project/profile, install, dependency upgrade, push, PR, or publication ran.
Windows/macOS holds are unchanged.

## Consumer and preservation contract

`extractCLI` explicitly combines two distinct inputs:

- `prepared-cli.mjs` runs the actual reviewed Go docs adapter. It requires the
  caller's exact `DOCS_AUTHORING_SOURCE_SHA`, an optional explicit clean
  `DOCS_AUTHORING_CHECKOUT`, and a fresh absent output destination. It validates
  the versioned envelope, namespace, preparation status, two surface roots,
  command identities, flag arrays, source pins and Markdown provenance/links.
  It does not accept the old array as v2 and does not filter additional commands.
- `historical.mjs` reads committed generated artifacts and registry entries from
  `9beca10448ac50fbe526a52101d1433a12471980`, the verified peeled `v1.2.4` tag.
  Original CLI URLs, canonical IDs, headings and command code fences survive.
  Generated output receives explicit historical 1.2.4 identity and pinned
  source links. There is no execution of historical product commands.

Prepared routes are `/en/api/cli/prepared-authoring-v2-<command-path>` and
identities are the adapter's `prepared-authoring-v2:<command path>`.
They cannot replace `/en/api/cli/plugin-kit-ai-*`. Every prepared entity has
`released: false`, `status/stability: prepared-not-release`, `maturity: prepared`
and `publicVisibility: preparation`; it never inherits public-stable defaults.
The full unmodified command object retains Use/Short/Long/Example, aliases,
deprecation, local/inherited flags, defaults, types and help. Markdown retains
command links and the exact installer ancestor source link. Text outside code
fences escapes angle brackets so `skills/<name>` is not a Vue component; command
syntax inside fences remains literal. Go module import identities are unchanged.

Historical platform/capability reference also reads the pinned artifacts; it
cannot execute the current product's retired `__docs` command during generation.
The old CLI/platform extractor implementations remain in place with explicit
legacy function names outside the preparation call graph. Python generation
requires an already provisioned pinned environment; the prior provisioning
helper remains available outside the offline preparation path.

Preservation inventory: legacy CLI and support exporters **preserve/defer** at
`website/tools/extractors/{cli,platform}.mjs`; YAML engine/assets/examples,
SDK/runtime implementation, dependencies, tests and design docs **preserve in
place, untouched**. Historical CLI and support artifacts are **adapted only at
generation**, preserving their useful facts and original routes. No capability
removal, YAML fallback, source move or second supported engine is introduced.
Migration remains unavailable in v2.

## Navigation, aliases and locale gate

All five configured navigations have first-class Use and Build entries. English
has the two journeys, all 13 D1 source pages and both actual prepared command
surfaces in sidebars. Other nav entries explicitly identify the English preview.
Source entity paths come from real files; generated locale fields are bound only
after corresponding output pages exist. ES/FR/ZH prefix replacement alone never
establishes a path. Prepared references are English-only and are not mirrored.

`website/tools/config/routes.json` owns alias policy: preserve existing
locale-less English page aliases, exclude the home/404, and add a versioned
historical CLI index alias `/en/legacy/v1/cli/` → `/en/api/cli/`. Old authoring
pages are retained rather than redirected to semantically different v2 jobs.
The generator materializes this inventory instead of overwriting it with `{}`.
VitePress rewrites are no longer misused as HTTP redirects. Postbuild validates
actual target HTML, rejects collisions/chains, and emits real `.html` aliases
with canonical URLs and query/fragment-preserving scripts. Index aliases use
`index.html`; extensionless serving still needs hosting/browser verification.
With JavaScript disabled the no-script refresh/manual link reaches the canonical
page, but fragment/query preservation is not claimed for that fallback.

Focused evidence includes emitted alias HTML and execution of its actual inline
scripts against query and encoded/unencoded fragment inputs. This proves those
script transformations, not VitePress rendering or browser anchor behavior.
Real full-site anchor/clean-URL/browser evidence remains a mandatory build gate.
The existing `docs:smoke-ui` now also calls the focused built-site consumer
smoke: both entrypoint roots, five navs, and six aliases in both URL spellings
using headings read from actual rendered targets. It intercepts canonical-origin
requests to the local sandbox server. This browser code was not executed here.

D3 still owns RU/ES/FR/ZH stale runnable prose and the existing theme
`LocaleSwitcher.vue` prefix-guess fallback for pages absent from the registry
(and its hiding of missing translations on registered pages). The new registry/nav do not claim
that this widget has been fixed. D3 must consume actual registry paths, label
canonical-English fallback truthfully and verify all five languages together;
a banner above conflicting commands is insufficient. Existing locale parity
checks must not be weakened to claim release acceptance.

Both generation and direct VitePress configuration fail unless
`DOCS_PREPARATION_PREVIEW=1`. This flag is only for disposable, non-published
previews, not approval to publish inconsistent locales. Noindex is not the gate.
Do not merge this preparation to auto-deploying main/master. D3/D4 and D5 must
satisfy locale, landing and release acceptance before changing the build gate.

## Verification and exact remaining gates

Focused command (existing Node 24.18.0; not the pinned full-build Node):

```sh
. /tmp/uap-authoring-docs-site-consumer-20260906-artifacts/environment.sh
node --test website/tools/lib/frontmatter.test.mjs website/tools/lib/site-consumer.test.mjs
```

All 14 tests passed, none skipped. Coverage includes actual adapter invocation
twice, determinism, exact flags/provenance and parent link, malformed contracts,
all D1 source links, real source navigation, five-locale path binding, every
pinned historical CLI page's headings/code fences, collision/chain rejection,
and emitted alias script behavior. `git diff --check` passed. These checks are
supplemental; they do not substitute for the full docs build or browser.

The attempted full generation entrypoint stopped at its offline prerequisite
check before any extractor/generated-tree writes. Exact missing prerequisites:
Node 22.21.1 (available 24.18.0), installed website packages mermaid 11.17.0,
playwright 1.62.0, typedoc 0.28.20, typedoc-plugin-markdown 4.12.0,
vitepress 2.0.0-alpha.18, vitepress-codeblock-collapse 1.0.0,
vitepress-mermaid-zoom 1.0.2, vue 3.5.40; private Python environment with
pydoc-markdown 4.8.2; cached gomarkdoc 1.1.0. Pinned pnpm 8.15.1 and an installed
Playwright Chromium must also be verified before the next full build/browser.
No missing tool was installed, fetched or upgraded.

Go execution used the existing Go 1.25.13 at
`/var/data/uap-authoring-test-20260906/toolchain/go/bin/go`, private HOME/TMPDIR/
GOCACHE, GOMAXPROCS=2, `-p 2`, explicit clean-clone GOWORK, GOFLAGS empty,
GOENV=off, GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, and existing module cache
`/tmp/uap-authoring-native-integration-old-20260906-artifacts/windows-modules`.

After the prerequisites exist, root must run full generation, model/boundary/
locale checks, VitePress build, postbuild, rendered link/anchor/output checks,
all-five-locale browser smoke and landing smoke in a disposable sandbox. Add
checks for both prepared roots, Use/Build traversal, historical 1.2.4, old
`.html` and extensionless deep links, canonical base, and truthful locale
switching. Regenerate against the final clean accepted source and review drift.
No tracked generated fixture is included in this patch: all generated evidence
is external, and the complete generated registry/site must be reviewed then.
D5 additionally requires final same-engine/native/wrapper release evidence;
this patch does not release Phase 6 or alter platform holds.

Terminal patch, per-file SHA-256 manifest, exact base, environment, test log,
prerequisite failure, generated consumer/redirect evidence and root handoff are
under `/tmp/uap-authoring-docs-site-consumer-20260906-artifacts/`.
