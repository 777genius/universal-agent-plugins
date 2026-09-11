# Phase 6 D2a CLI exporter preparation

Status: prepared reference only; not a public release. Source baseline:
`070663efb27f69ecae8609e6b839f86f843efbb0`.

The separate executable lives at `cli/plugin-kit-ai/tools/authoring-docs` in
module `github.com/777genius/plugin-kit-ai/cli`, allowing existing internal
imports. It adds no runtime registration and changes no production factory.
The old `__docs` exporter, website extraction, navigation, and generated v1
reference remain untouched. D2b integration follows D1 and independent review.

## Existing integration seam

`internal/authoring/commands/version.go:61` exports `App.ReleaseSelection`.
It calls the shared `releaseTree`, constructs implemented command factories
using `App.command` and `skillsCommand`, adds the shared version command,
initializes help flags and completion utilities, and observes selection.
With nil arguments it does not execute commands or completion hooks. This is
the usable API; the unexported `App.command` alone would not be sufficient.
No interception of `App.Execute`, copied definitions, or engine fork is needed.

The adapter sets `PublicContract: true` explicitly and uses
`authoringcli.NewReleasePluginKitRoot` for the plugin-kit-ai surface. For the
second surface it composes `agentpluginscli.NewRoot` and
`authoringcli.NewReleaseAuthorCommand`, matching
`cmd/agentplugins/release_root.go:20`. Only the attached author subtree is
exported; keeping the real ancestor preserves installer persistent flag facts.
No installer service is configured and no product command is executed.

The runtime plugin-kit-ai wrapper in `cmd/plugin-kit-ai/release_compat.go:82`
adds hidden compatibility flags and hidden rejection nodes. Its visible
commands and flags come from the same shared root. The adapter deliberately
uses that shared root without installing retirement shims. Visible Cobra help
and completion utility definitions are included, but their callbacks are never
called. Hidden ancestors, rejection annotations and internal `__` names are
pruned before rendering, including their descendant links.

## Prepared output contract

The caller supplies an exact `--source-sha`, `--checkout` repository root and
an absent `--out-dir` whose parent already exists. `source_sha` is the exact
clean integrated checkout HEAD; `factory_baseline_sha` separately remains
`070663efb27f69ecae8609e6b839f86f843efbb0`. A committed adapter and subsequent
docs-only commits work without repinning their own commit identity. Ancestry
alone is never acceptance: the factory pins and bounded file inventory must
still match. All tracked staged/unstaged changes fail the simple cleanliness
check. Missing or unexpected relevant Go inputs fail even when ignored.

The source inventory in `source.go` covers all non-test Go files directly in
`internal/authoring/commands`, `internal/authoringcli`,
`internal/agentpluginscli` (under the CLI module), and
`install/integrationctl/agentplugins/domain`. This includes constructor
configuration, the installer root's other constructed commands, the
`target_batch.go` → `domain/clients.go` help/registry chain, and other same-package
initializers. Runtime callbacks in these packages are pinned as part of their
files; their transitive engine implementations, YAML assets and examples are
outside the help inventory. Existing `_test.go` files in factory packages do not
contribute to the docs binary. The adapter's complete Go file set, including its
focused tests, must be tracked; additional adapter Go files are rejected.
The two `cmd/*` wrapper pins are composition references, not linked packages.

All five workspace modules' existing go.mod/go.sum, root go.work/go.work.sum,
and the absence of nested workspace controls, extra sum files and vendor
directories are checked. These controls fix workspace use/local replacements
and module version selection, including Cobra v1.10.2, pflag v1.0.9 and the
Cobra/doc dependency go-md2man/v2 v2.0.6. This is a bounded documentation input
contract, not a digest of the repository or installer runtime.

Before any output creation the tool renders the actual loaded trees with the
literal provenance token `SOURCE_SHA`, no source pin array, and no host paths.
It hashes sorted, length-framed filenames and bytes (both manifest facts and
Cobra Markdown), and compares with `reviewedProjection`. Thus a binary compiled
with different help/flags cannot stamp the validated checkout SHA on different
facts. The golden covers untouched factory trees, not the hook-trap trees used
by the no-action test. Normal output uses the caller's exact SHA, including the
author parent source link. Across commits only these provenance bytes differ.

Supported builds use the reviewed adapter source, Go 1.25.13, the root workspace
and the existing checksum-verified module cache. Set GOENV=off,
GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, GOFLAGS empty and GOWORK to the
absolute checkout go.work; use private HOME/TMP/GOCACHE and GOMAXPROCS=2 with
`go test -p 2` / `go build -p 2`. No overlays, build tags, alternate workspaces,
vendor mode, linker substitutions or modified module-cache source are supported.
The fingerprint verifies exported facts even for an ordinary mismatched source
build; it does not authenticate a deliberately altered verifier or arbitrary
binary. Factory changes require a fresh bounded source/help review, updated
input pins and an explicitly reviewed projection golden, not an automatic
regeneration or self-commit SHA constant. Adapter-only changes that preserve
facts need no new factory baseline.

Output lives only under `prepared-authoring-v2/` inside the new destination:

- `manifest.json`: `authoring-docs-manifest-v1` envelope with namespace,
  `prepared-not-release` status, `released: false`, exact source SHA, audited factory baseline SHA and relative
  source file SHA-256 pins, plus the two surfaces.
- Markdown files named from the actual command path with underscores, e.g.
  `plugin-kit-ai_init.md` and `agentplugins_author_init.md`. Every page carries
  prepared status and the source SHA. Cobra's timestamp footer is disabled.
- Command identities use `prepared-authoring-v2:<command path>` and slugs use
  the same prepared prefix. The manifest contains actual Use/Short/Long/Example,
  aliases, deprecation text and separate local/inherited flag arrays with type,
  default, optional-value default, shorthand and help text.

The manifest intentionally differs from the old `docsManifestEntry[]` format,
which had command facts but no source envelope or flag arrays. D2b must consume
this namespace explicitly, never silently replace v1 records or mark these
records public-stable. Ordinary Markdown links remain within this namespace.
The author group's parent link points to the pinned installer root source,
because this bounded export has no installer root reference page.

Flag presence is a help fact, not a promise of accepted execution. In particular,
installer-only inherited flags are retained with the shared rejection wording;
`--dry-run` and `--target` retain their actual help and support qualifications.
The exporter never interprets a successful generation as product validation,
provider readiness, profile discovery, installation, or publication approval.

Output creation refuses any existing directory, including a v1 destination.
A write failure may leave a partial new directory; use another disposable
output directory after addressing the failure. It performs no cleanup or
replacement. Output content is deterministic; filesystem metadata is not part
of the contract. Git is used only for source identity reads.

## Bounded validation and downstream gates

Run only the adapter package with Go 1.25.13, `-p 2`, `GOMAXPROCS=2`, offline
module lookup and private HOME/TMPDIR/GOCACHE. The supplied host contains the
requested version at `toolchain/go/bin/go`; `go1.25.13` is not a filename there.
The external handoff records the exact verified executable and environment.
No dependency was added; the existing Cobra docs dependency renders Markdown.

Focused tests cover fresh-tree and on-disk byte determinism, the exact two
surfaces and utility inventory, Markdown links, local/inherited flag defaults
and help, hidden/rejection/internal descendant exclusion, source identity
rejection before writes, clean committed and later docs-only fixtures, changed
factory/target-registry/workspace/dependency inputs, ignored/adapter Go additions,
changed loaded help/flag/Markdown facts, no v1 overwrite, hook traps and untouched private
profile directories. They do not claim product/native/npm suite coverage or
OS release acceptance. Set DOCS_TEST_CHECKOUT to the absolute clean checkout
being tested to run the disk checks against that full committed tree; otherwise
they use a disposable committed source-contract fixture. Mutation tests always
use private fixture Git metadata and never mutate the source checkout. Existing Windows/macOS holds remain unchanged; no
restricted Windows reproduction was attempted or rerouted.

ROOT mechanical integration and independent medium review remain downstream.
D2b must preserve v1 source and routes, use a separately accepted integrated
source identity, and consume these prepared records only after D1 integration.
No website build, package publication, push, PR, or public activation is part
of D2a. Migration remains unavailable in v2; historical v1 baseline is 1.2.4.
