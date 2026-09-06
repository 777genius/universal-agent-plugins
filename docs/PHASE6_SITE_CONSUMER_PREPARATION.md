# Phase 6 D2b website consumer preparation

Task: `uap-authoring-docs-site-consumer-20260906`.
Status: bounded consumer implementation and focused checks complete; full website
build/browser validation and public activation remain **unaccepted**.

The sections through the original terminal artifact location record the initial
consumer implementation run. The correction contract and new validation below
supersede its source-selection and test-invocation instructions.

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
  checked-in accepted source pin and a mandatory explicit clean
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
switching. Regenerate against the fixed accepted adapter input pin described below and review drift.
No tracked generated fixture is included in this patch: all generated evidence
is external, and the complete generated registry/site must be reviewed then.
D5 additionally requires final same-engine/native/wrapper release evidence;
this patch does not release Phase 6 or alter platform holds.

Terminal patch, per-file SHA-256 manifest, exact base, environment, test log,
prerequisite failure, generated consumer/redirect evidence and root handoff are
under `/tmp/uap-authoring-docs-site-consumer-20260906-artifacts/`.


## Review corrections (exact base 801f846cac40f47c344983ccb61122cbdd3ae2f8)

Historical platform extraction includes the complete pinned former output inventory:
platform-events, capabilities and all five `reference/target-support.md` pages.
The production alias registry retains `/reference/target-support` to its English
page. Tests compare every platform page's body facts against the historical Git
blob, allowing only historical metadata/banner and pinned source-link changes.
Preserved exporter implementations and YAML capabilities remain untouched.

### Immutable adapter input and writable site output

`tools/lib/source-contract.mjs` owns the accepted adapter source SHA, currently
`801f846cac40f47c344983ccb61122cbdd3ae2f8`. This is a reviewed input revision,
not the commit that will contain generated output. `DOCS_AUTHORING_CHECKOUT` is
mandatory and must be a separate clean checkout of that revision. There is no
same-checkout default. An optional `DOCS_AUTHORING_SOURCE_SHA` must equal the
checked-in pin. A new accepted adapter revision requires an explicit reviewed
source-contract change; generating or committing the website never advances it.
The adapter's own tracked staged/unstaged clean-source guard remains unchanged.
The consumer additionally rejects overlapping physical source, site, output and
tools paths, wrong HEAD, and tracked source changes before extraction/assembly.

Ordinary `docs:gen`, `docs:build`, and `docs:dev` write the site's `generated/`
and `.site/` while reading the separate immutable adapter checkout. For bounded
assembly checks, `DOCS_SITE_OUTPUT_ROOT` selects a disposable directory containing
those two trees; this is an assembly-only override, not a relocated VitePress
build contract. Full builds should use the ordinary site output directory.
SDK/runtime input remains the writable site checkout; use its absolute GOWORK.

Checked-in baseline policy: generate with the accepted input pin, review the
complete generated diff (including added/deleted files and registries), then
commit generated output on the non-publishing integration branch. Keep the input
pin fixed across that commit and subsequent builds. `docs:check-drift` must then
pass against that baseline; it must fail for changed tracked generated bytes.
It is not an inventory review: separately inspect untracked generated additions
with `git status --short --untracked-files=all -- website/generated`. The existing
pre-correction tracked output is not a newly accepted baseline, so initial drift
is expected and must be reviewed, not suppressed. No self-referential HEAD value
or timestamp enters prepared provenance. A future source-pin update and its
reviewed generated baseline can share a downstream commit without changing that
input revision again.

### Mandatory bounded integration and offline SDK tool

`pnpm docs:test` now runs `tools/quality/test-site-consumer.mjs`; `docs:check`
already requires that command. There are no optional/skipped consumer acceptance
tests. Missing pinned Go/private offline environment fails clearly. The runner
creates local disposable source/output clones, exports a fresh real adapter
fixture, verifies every source hash, and runs the complete consumer tests.
It calls the production `assembleBundles` seam three times with fresh actual
CLI exports and historical platform bundles, writing generated pages, registries,
aliases and the runtime tree between exports. It compares all generated/runtime
file hashes, commits a disposable output baseline, verifies the actual drift
check across another assembly, rejects intentional drift, and verifies clean
source throughout. Same-output provenance and dirty source are negative checks;
the unchanged real adapter is also exercised against dirty source. These bounded
assemblies intentionally exclude SDK/runtime extractors and are not full-site
or browser acceptance.

Required environment: Go 1.25.13; private absolute HOME, TMPDIR, GOCACHE and
DOCS_TOOLS_ROOT; an existing GOMODCACHE; GOMAXPROCS=2, GOENV=off,
GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, empty GOFLAGS. The runner supplies
its own source checkout, accepted pin and adapter GOWORK. Evidence is retained
under its printed TMPDIR child. No absolute worker fixture is required.
Full generation additionally requires the unchanged pinned Node/dependencies,
Python and preview prerequisites, with `DOCS_GOMARKDOC` pointing to an absolute
installed executable whose `--version` is exactly `v1.1.0`. Both preflight and
SDK extraction verify that version. SDK extraction invokes that binary directly;
it never uses version-suffixed `go run`, whose deprecation metadata lookup fails
offline even with the module installed. No download or tool pin change is used.

Correction evidence and terminal handoff are in
`/tmp/uap-authoring-docs-site-fix-20260906-artifacts/`. The mandatory package gate passed all 15 tests with zero skips, followed by
real assembly repeatability and source/drift negative checks. The actual pinned offline
SDK extractor was run twice: 12 pages and five entities, identical bytes.
Full generation on the terminal integrated SHA, complete generated inventory,
locale/model/boundary/drift checks, VitePress dead-link/render/output checks and
real browser acceptance remain ROOT's separate gate. D3 locale prose/switching,
D4 landing and D5 release/platform evidence remain independent. This correction
does not authorize publication or claim full-site/browser acceptance.
