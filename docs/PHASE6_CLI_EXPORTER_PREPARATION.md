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
an absent `--out-dir` whose parent already exists. The source must equal the
explicit audited baseline and checkout HEAD. Tracked changes, staged changes,
source pin mismatches, and untracked Go files outside this adapter are rejected.
Updating the baseline requires owner integration, a source review, updating the
constant and file pins, and rerunning the focused checks. This exporter does
not accept a newer commit merely because it is a descendant of this baseline.
Build this tool from the reviewed patch against that source; checkout validation
is not cryptographic attestation of an arbitrary externally supplied binary.

Output lives only under `prepared-authoring-v2/` inside the new destination:

- `manifest.json`: `authoring-docs-manifest-v1` envelope with namespace,
  `prepared-not-release` status, `released: false`, source SHA and relative
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
rejection before writes, no v1 overwrite, hook traps and untouched private
profile directories. They do not claim product/native/npm suite coverage or
OS release acceptance. Existing Windows/macOS holds remain unchanged; no
restricted Windows reproduction was attempted or rerouted.

ROOT mechanical integration and independent medium review remain downstream.
D2b must preserve v1 source and routes, use a separately accepted integrated
source identity, and consume these prepared records only after D1 integration.
No website build, package publication, push, PR, or public activation is part
of D2a. Migration remains unavailable in v2; historical v1 baseline is 1.2.4.
