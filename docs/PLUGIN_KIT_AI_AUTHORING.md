# plugin-kit-ai

[![Required](https://github.com/777genius/plugin-kit-ai/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/777genius/plugin-kit-ai/actions/workflows/ci.yml)
[![Docs](https://github.com/777genius/plugin-kit-ai/actions/workflows/docs.yml/badge.svg?branch=main)](https://github.com/777genius/plugin-kit-ai/actions/workflows/docs.yml)
[![Polyglot Smoke](https://github.com/777genius/plugin-kit-ai/actions/workflows/polyglot-smoke.yml/badge.svg?branch=main)](https://github.com/777genius/plugin-kit-ai/actions/workflows/polyglot-smoke.yml)
[![Release](https://img.shields.io/github/v/release/777genius/plugin-kit-ai?label=release)](https://github.com/777genius/plugin-kit-ai/releases)
[![npm](https://img.shields.io/npm/v/plugin-kit-ai?label=npm)](https://www.npmjs.com/package/plugin-kit-ai)
[![Go Reference](https://pkg.go.dev/badge/github.com/777genius/plugin-kit-ai/sdk.svg)](https://pkg.go.dev/github.com/777genius/plugin-kit-ai/sdk)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

Build one plugin and ship it to many AI agents.

`plugin-kit-ai` keeps authored source under `plugin/`, generates the supported outputs you need, and helps you validate the repo before handoff.
The honest promise is `one repo / many supported outputs`, not fake parity everywhere.

## Install Agent Plugins 1.0

Want to use a plugin rather than author one? `universal-agent-plugins` is the
public npm package. It installs the `agentplugins` binary; both names run the
same installer and lifecycle manager (Node.js 22+):

```bash
npx universal-agent-plugins add context7
```

Short names resolve to immutable releases through the signed
[Universal Agent Plugins Directory](https://github.com/777genius/universal-agent-plugins).
The command shows the selected publisher, source, and verification status, so a
community bridge is not mistaken for its upstream project. Direct local paths
and exact GitHub revisions work without Directory submission.

```bash
npx universal-agent-plugins search docs
npx universal-agent-plugins validate ./my-plugin
npx universal-agent-plugins add context7 --dry-run --target cursor
npx universal-agent-plugins add context7 --target codex,cursor,kiro
npx universal-agent-plugins outdated --all
npx universal-agent-plugins update --all
npx universal-agent-plugins update context7 --target codex,cursor,kiro
npx universal-agent-plugins repair context7 --target codex,cursor,kiro
npx universal-agent-plugins remove context7 --target codex,cursor,kiro
npx universal-agent-plugins switch context7 --to upstash/context7
npx universal-agent-plugins add ./my-plugin --target cursor
npx universal-agent-plugins add owner/repo@0123456789abcdef0123456789abcdef01234567//plugins/my-plugin --target cursor
```

Replace the example SHA with the full lowercase 40-character commit SHA you
reviewed; branches, tags, and abbreviated SHAs are rejected. `repair` reapplies
the recorded revision; `update` selects a newer eligible release. `switch`
moves the whole installation to a qualified Directory distribution or exact
source.

### Choose clients

In an interactive terminal, `add` without `--target` detects installed clients,
checks which of them the selected package can serve from one release, and skips
incompatible clients before showing the confirmation. One compatible client is
selected automatically. With several compatible clients, the CLI shows a
multi-select with all of them selected by default; press Enter to keep all of
them. Scripts and CI must choose explicitly, for example
`--target claude,gemini,opencode`. The same comma-separated syntax works with
`add`, `update`, `repair`, and `remove`.

Detection checks installed executables and documented configuration surfaces.
It does not start an agent, log in, complete OAuth, or prove browser/tool
runtime behavior.

The following matrix describes the current release source. New client support
is available through `npx` once a release containing this source is published.
The exact package, source revisions, commands, and limitations are recorded in
the [isolated client E2E evidence](docs/AGENTPLUGINS_CLIENT_E2E.md).

| Client | Evidence |
| --- | --- |
| Claude Code | Claude Code 2.1.205: real isolated exact-SHA `add`, native list, `repair`, safe update preflight, and `remove` lifecycle |
| Gemini CLI | Gemini CLI 0.36.0: real isolated configuration and the same exact-source lifecycle |
| OpenCode | OpenCode 1.18.4: real isolated configuration and the same exact-source lifecycle |
| Cline | Real locally detected client; isolated native configuration and exact-source lifecycle; no client runtime or login |
| Windsurf | Real locally detected configuration and exact-source lifecycle; no client runtime or login |

`search` combines the reviewed Directory with a separately signed Discovery
Index. Unreviewed results use publisher-qualified `discovery:owner/repo//path`
selectors and show their exact indexed commit before install. The CLI
reacquires and validates those package bytes before mutation; the index
signature authenticates metadata but is not a package or runtime endorsement.
`outdated` is read-only, while `update --all` keeps every installation's
recorded source and targets and fails the batch preflight before mutation if any
planned update is unsafe.

For Agent Plugins 1.0, the root `plugin.json` is the install authority. A
`plugin.yaml` file is legacy `plugin-kit-ai` authoring input only: it is not
merged with, converted into, or allowed to silently override `plugin.json`.

Before changing anything, the CLI resolves and validates the source and
preflights every selected client. `--dry-run` prints that same read-only plan.
`doctor` only inspects paths and filesystem metadata; it never launches a
discovered client executable, and neither does a `--dry-run`. Non-dry-run
lifecycle resolution may explicitly run a bounded `--version` probe when an
exact client version is needed for signed Directory evidence. That probe
receives an empty stdin and credential-free environment and runs outside the
caller's working directory. If it cannot obtain a matching version,
version-bound evidence remains unavailable rather than being inferred.
Managed files and state are staged and committed together; a failure rolls back
changes when ownership can be proven, or stops with recovery guidance without
claiming success. Observed unowned entries fail closed before mutation. Portable
filesystems cannot provide a linearizable content compare-and-swap against a
non-cooperating client that writes in the final syscall-sized window, so the CLI
rechecks and reports identity conflicts instead of promising impossible
concurrency isolation. Client-controlled activation happens separately: some
clients activate automatically, while others finish as prepared and print a
manual activation step. OAuth and consent prompts stay visible and
user-controlled; cancelling one preserves the package and reports
authentication as pending or cancelled.
Kiro skills are installed directly into the documented global skills path.
For packages with MCP servers, agentplugins atomically merges only its owned
entries into Kiro's global `mcp.json`, preserving unrelated user configuration,
then checks supported Kiro CLI versions through a bounded structured ACP v1
initialize/session handshake.
It sends no prompt or model/tool turn and does not inject package endpoints;
Kiro must load its installed native configuration and report connected servers
with enabled tools. The verifier drains a bounded quiet settlement window,
rejects EOF, partial bytes, or trailing contradictions, then stops and reaps the
long-lived ACP process through supervised containment. Automatic Kiro ACP
verification is available only on Linux after capability preflight proves
delegated cgroup v2 creation, atomic CLONE_INTO_CGROUP placement, and
cgroup.kill. macOS, Windows, and Linux hosts without every required proof fail
preflight before any MCP mutation. On those hosts, use Kiro's documented manual
skill and MCP configuration paths; that workflow is outside this automatic CLI
path. Failure to start ACP
after a supported preflight may still leave a manual verification action. This
is activation evidence, not a runtime tool E2E claim.
Once the ACP process starts, companion-launch failure (including a missing
`kiro-cli-chat`), EOF, timeout, authentication failure, malformed or partial
output, contradiction, and non-clean exit are authoritative activation
failures; the managed package remains committed for explicit repair and is
never reported active.
Verification is reported for the exact plugin, client, runtime, and OAuth
evidence available—not as a claim that every combination has been tested.

`universal-agent-plugins` is an independent community installer, not an official
OpenAI CLI.

Docs site:

- overview: [plugin-kit-ai documentation](https://777genius.github.io/plugin-kit-ai/docs/en/)
- fastest start: [Quickstart](https://777genius.github.io/plugin-kit-ai/docs/en/guide/quickstart.html)
- choose by job first: [Choose What You Are Building](https://777genius.github.io/plugin-kit-ai/docs/en/guide/choose-what-you-are-building.html)
- one repo, many outputs: [What You Can Build](https://777genius.github.io/plugin-kit-ai/docs/en/guide/what-you-can-build.html)
- delivery model guide: [Choose A Target](https://777genius.github.io/plugin-kit-ai/docs/en/guide/choose-a-target.html)
- honest caveat: [Support Boundary](https://777genius.github.io/plugin-kit-ai/docs/en/reference/support-boundary.html)

Project policies:

- support boundary: [docs/SUPPORT.md](docs/SUPPORT.md)
- contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- security: [SECURITY.md](SECURITY.md)
- code of conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## Choose What You Are Building

### Connect an online service

Use this when the plugin should connect to a hosted service like Notion, Stripe, Cloudflare, or Vercel.

```bash
plugin-kit-ai init my-plugin --template online-service
cd my-plugin
plugin-kit-ai inspect . --authoring
plugin-kit-ai generate .
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform claude --strict
```

Real examples:

- [notion](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/notion)
- [stripe](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/stripe)
- [cloudflare](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/cloudflare)
- [vercel](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/vercel)

### Connect a local tool

Use this when the plugin should call into a repo-owned tool or CLI like Docker Hub, Chrome DevTools, or HubSpot Developer.

```bash
plugin-kit-ai init my-plugin --template local-tool
cd my-plugin
plugin-kit-ai inspect . --authoring
plugin-kit-ai generate .
plugin-kit-ai generate --check .
plugin-kit-ai validate . --platform claude --strict
```

Real examples:

- [docker-hub](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/docker-hub)
- [hubspot-developer](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/hubspot-developer)
- [chrome-devtools](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/win4r/chrome-devtools-codex-plugin)

### Build custom plugin logic - Advanced

Use this when the product is defined by hooks, runtime behavior, or custom plugin code.

This path is more powerful and more engineering-heavy than the first two starters.
Plain `plugin-kit-ai init my-plugin` still exists as the legacy compatibility path for the older Codex runtime Go starter.

```bash
plugin-kit-ai init my-plugin --template custom-logic
cd my-plugin
plugin-kit-ai inspect . --authoring
plugin-kit-ai validate . --platform codex-runtime --strict
plugin-kit-ai test . --platform codex-runtime --event Notify
```

Guide:

- [Build Custom Plugin Logic](https://777genius.github.io/plugin-kit-ai/docs/en/guide/build-custom-plugin-logic.html)

## Quick Start

Want to try a real Agent Plugin without installing a CLI permanently?

```bash
npx universal-agent-plugins add context7
```

Building a plugin instead? Use `plugin-kit-ai` for authoring and generated
output delivery. Its older `plugin-kit-ai add` lifecycle is not the default
installer for portable Agent Plugins 1.0. Start with
[Choose What You Are Building](#choose-what-you-are-building).

Recommended authoring CLI install path:

```bash
brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai
plugin-kit-ai version
```

Then create a repo from the path that matches the job:

```bash
plugin-kit-ai init my-plugin --template online-service
plugin-kit-ai init my-plugin --template local-tool
plugin-kit-ai init my-plugin --template custom-logic
```

Plain `plugin-kit-ai init my-plugin` still exists for backward compatibility, but it is no longer the recommended first start for new repos.
Use one of the three job-first templates above unless you are intentionally maintaining the older Codex runtime Go path.

## What You Get

- one plugin repo that stays the source of truth
- authored files under `plugin/`
- generated root files that stay managed
- supported outputs for Claude, Codex, Gemini, Cursor, and OpenCode where the repo shape allows it
- a clean readiness check through `generate`, `generate --check`, and `validate --strict`

## Works Across Multiple Outputs

- Claude
- Codex package
- Codex runtime
- Gemini
- OpenCode
- Cursor

That depth stays available, but you do not need to understand the whole target model before creating the repo.

## What To Do Next

- run `plugin-kit-ai inspect . --authoring`
- edit the repo under `plugin/`
- regenerate after changes
- validate the supported output you actually plan to ship first
- only then add more outputs when the product really needs them

Other supported CLI install methods:

- npm: `npm i -g plugin-kit-ai` or `npx plugin-kit-ai@latest ...`
- pipx (`public-beta`, only when that release is published to PyPI): `pipx install plugin-kit-ai`
- fallback installer: `curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh`
- fallback one-shot command: `curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh -s -- add notion --dry-run`
- source build for maintainers of this repo: `go build -o bin/plugin-kit-ai ./cli/plugin-kit-ai/cmd/plugin-kit-ai`

## Keep This Rule In Mind

- start with the job you need today
- keep authored source under `plugin/` for new repos
- let generated root files stay managed
- add deeper target-specific behavior only when you need it
- use advanced docs when you need exact target and support details

## Deep Product Details

Everything below this point is for people comparing delivery models, import paths, and detailed support boundaries. If you only needed the main promise and first path, you can stop above.

## Go Deeper By Goal

### Fast Local Plugin

Choose this when the plugin stays local to the repo and your team already works in Python or Node.

- Main flow: `init -> doctor -> bootstrap -> generate -> validate --strict`
- Runtime note: the execution machine still needs Python `3.10+` or Node.js `20+`
- Delivery options: vendored helper by default, shared `plugin-kit-ai-runtime` when you want a reusable dependency, bundle handoff when the repo must travel

This is a supported non-Go path, not a hidden fallback.
Use the starter templates for Codex and Claude across Go, Python, and Node/TypeScript when you want the fastest copy-first path into a working repo.

Start here:

- [examples/starters/README.md](examples/starters/README.md)
- [examples/local/README.md](examples/local/README.md)
- [docs/CHOOSING_HELPER_DELIVERY_MODE.md](docs/CHOOSING_HELPER_DELIVERY_MODE.md)

### Production-Ready Plugin Repo

Choose this when you want the strongest supported release and distribution story.

- Online service path: `plugin-kit-ai init my-plugin --template online-service`
- Local tool path: `plugin-kit-ai init my-plugin --template local-tool`
- Advanced runtime path: `plugin-kit-ai init my-plugin --template custom-logic`
- Package/config expansion later: `codex-package`, `gemini`, `opencode`, `cursor`
- Real multi-target MCP-first example: [`context7` in universal-plugins-for-ai-agents](https://github.com/777genius/universal-plugins-for-ai-agents/tree/main/plugins/context7)

### Already Have Native Config

Choose this when you are migrating existing Claude/Codex/Gemini/OpenCode/Cursor native files into the repo-owned workflow.

```bash
./bin/plugin-kit-ai import ./native-plugin --from codex-runtime
./bin/plugin-kit-ai normalize ./my-plugin
./bin/plugin-kit-ai generate ./my-plugin
./bin/plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
```

## Stability Snapshot

Stable by default:

- the main public CLI contract
- the recommended Go SDK path
- Go scaffolds for the default Codex and Claude runtime lanes
- the stable local Python and Node subset on `codex-runtime` and `claude`
- `doctor`, `bootstrap`, `validate --strict`, `export`, and bundle handoff for that stable local subset

Use carefully:

- `generate`, `import`, and `normalize` are still `public-beta`
- package and workspace-config targets have different guarantees than runtime targets
- `shell` remains a bounded `public-beta` escape hatch

For the precise contract:

- [docs/generated/target_support_matrix.md](docs/generated/target_support_matrix.md)
- [docs/generated/support_matrix.md](docs/generated/support_matrix.md)
- [docs/SUPPORT.md](docs/SUPPORT.md)

## Path Summary

- Go is the recommended path when you want the strongest production story and the least downstream runtime friction.
- Node/TypeScript is the main supported non-Go path for repo-local runtime plugins.
- Python is the supported Python-first repo-local path.
- Package and workspace-config targets are for packaging and configuration outputs, not for pretending every target behaves like a runtime plugin.

## SDK And CLI

Go SDK packages:

- `github.com/777genius/plugin-kit-ai/sdk`
- `github.com/777genius/plugin-kit-ai/sdk/claude`
- `github.com/777genius/plugin-kit-ai/sdk/codex`
- `github.com/777genius/plugin-kit-ai/sdk/gemini`

Useful starting points:

- [sdk/README.md](sdk/README.md)
- [docs/generated/support_matrix.md](docs/generated/support_matrix.md)
- [docs/SUPPORT.md](docs/SUPPORT.md)

Common CLI commands:

```bash
./bin/plugin-kit-ai init my-plugin
./bin/plugin-kit-ai doctor ./my-plugin
./bin/plugin-kit-ai bootstrap ./my-plugin
./bin/plugin-kit-ai generate ./my-plugin
./bin/plugin-kit-ai validate ./my-plugin --platform codex-runtime --strict
./bin/plugin-kit-ai import ./native-plugin --from codex-runtime
./bin/plugin-kit-ai capabilities --format json
```

`plugin-kit-ai install` stays intentionally narrow: it installs third-party plugin binaries from GitHub Releases, verifies `checksums.txt`, and does not act as a self-update path for the CLI itself.

For automation, `plugin-kit-ai validate --format json` now emits the versioned `plugin-kit-ai/validate-report` contract with `schema_version: 1` and explicit outcomes `passed`, `failed`, or `failed_strict_warnings`.
For Codex lane selection, use [docs/CODEX_TARGET_BOUNDARY.md](docs/CODEX_TARGET_BOUNDARY.md). For the validation ABI itself, use [docs/VALIDATE_JSON_CONTRACT.md](docs/VALIDATE_JSON_CONTRACT.md).

## Build And Test

Requirements:

- Go `1.25.13` for this monorepo workspace and its CI lanes
- generated Go plugin projects created by `plugin-kit-ai init` remain on Go `1.22+`

Common commands from repo root:

```bash
go run ./cmd/plugin-kit-ai-gen
go build -o bin/plugin-kit-ai ./cli/plugin-kit-ai/cmd/plugin-kit-ai
./bin/plugin-kit-ai version
make test-polyglot-smoke
go test ./...
```

## Repository And Docs Map

Main repo areas:

- `sdk`
- `cli/plugin-kit-ai`
- `install/plugininstall`
- `examples/starters`
- `examples/local`
- `examples/plugins`
- `repotests`
- `docs`

Canonical docs:

- [docs/generated/support_matrix.md](docs/generated/support_matrix.md)
- [docs/generated/target_support_matrix.md](docs/generated/target_support_matrix.md)
- [docs/SUPPORT.md](docs/SUPPORT.md)
- [docs/CODEX_TARGET_BOUNDARY.md](docs/CODEX_TARGET_BOUNDARY.md)
- [docs/VALIDATE_JSON_CONTRACT.md](docs/VALIDATE_JSON_CONTRACT.md)
- [docs/PRODUCTION.md](docs/PRODUCTION.md)
- [docs/INSTALL_COMPATIBILITY.md](docs/INSTALL_COMPATIBILITY.md)
- [docs/STATUS.md](docs/STATUS.md)

Maintainer-only historical context:

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- [docs/FOUNDATION_REWRITE_VNEXT.md](docs/FOUNDATION_REWRITE_VNEXT.md)
- [docs/adr/README.md](docs/adr/README.md)
