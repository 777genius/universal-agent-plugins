---
title: "UAP client compatibility evidence"
description: "Exact client versions, source identity, tested runtime layers, and release limitations."
canonicalId: "page:reference:client-compatibility"
section: "reference"
locale: "en"
generated: false
translationRequired: false
---

# UAP client compatibility evidence

## Released installer 0.1.53

[Public release 0.1.53](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.53)
uses installer commit `28cf05af0a1e4fea642825dd34b78f9c99094ab5`.
[Native run 34166817934](https://github.com/777genius/universal-agent-plugins/actions/runs/34166817934)
passed all nine jobs and 18 required tests without skips. It downloaded the public
binaries, checked their manifest and attestations before execution, and used
separate source-built test/probe helpers at harness commit
`9c33cfac81fc00e61205591321c89c5675fedb60`.

| Tested client | Linux arm64 | macOS arm64 | Windows amd64 |
| --- | --- | --- | --- |
| Codex 0.153.4 | Passed stdio/HTTP and skill-context lifecycle | Same scope passed | Same scope passed |
| Claude Code 2.1.263 | Passed stdio/HTTP/skill lifecycle | Same scope passed | Passed HTTP/installed-skill lifecycle; managed stdio unsupported |
| OpenCode 1.18.29 | Passed lifecycle, collision-observation and extended stdio/skill suites | Same scope passed | Same scope passed |

Lifecycle includes install, update, same-version refresh, repair and removal.
OpenCode JSON/JSONC lifecycle and HTTP collision fixtures are separate from its
global JSON stdio/skill-body suite; JSONC stdio/body coverage is not inferred.
The observed colliding OpenCode tool names remain a limitation, with
`opencode_tool_namespace_not_evaluated` advisory. The required Codex suite's Git
acquisition seam is not a new public-Git native runtime claim.

These are maintainer-run observations in fresh GitHub-hosted profiles with
scripted loopback providers and network access. They do not prove real-model
quality, OAuth/login, desktop applications, live services, independent adoption
or vendor certification. Package validation, installation, activation/discovery
and runtime remain separate evidence layers. The separate
[six-platform launcher run](https://github.com/777genius/universal-agent-plugins/actions/runs/34164699278)
does not extend this native-client table to other architectures.

[Released evidence identities](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-released-0.1.53.json)
record all nine lanes and exact hashes. A verified archive contains 529 original
files plus its summary, SHA-256
`0d315f3a24bb6c41d184f0632d76a7228e18a6c1c41be2774e23c8066dfd3af6`.
[Public evidence archive](https://github.com/777genius/universal-agent-plugins/releases/download/native-evidence-0.1.53-34166817934/uap-0.1.53-native-34166817934.zip)
and [checksum sidecar](https://github.com/777genius/universal-agent-plugins/releases/download/native-evidence-0.1.53-34166817934/uap-0.1.53-native-34166817934.zip.sha256)
were re-downloaded and hash-checked after publication on the
[separate evidence release](https://github.com/777genius/universal-agent-plugins/releases/tag/native-evidence-0.1.53-34166817934).
[Reproduction and publication procedure](https://github.com/777genius/universal-agent-plugins/blob/main/docs/released-native-client-proof.md)
keeps those assets separate from the immutable installer release.

## Historical source-built observations

Snapshot: **2026-09-07**. These are bounded native CLI observations for the
`agentplugins` installer. They do not establish support for every version of a
client, Desktop application, operating system, package, or authentication flow.
UAP is independent community tooling; this matrix is not vendor certification.

### Historical source and release boundary

The macOS/Windows checkpoint below is `ef20b93`; Linux evidence retains its
separate historical identity. No result is implicitly carried to another SHA.

The Linux runs below used a **source-built installer**, binary SHA-256
`3e5d4b8ee25a16d274b0f3bbdaf35f6963416650fed972995e65817f274cf8d8`,
from product tree `dcd9090458b036515c9aed77ee5d3e66f0dcb4f5`, base commit
`1c831fa8b2016fda68c73fc60fe6f691581d0b13`, and binary/full-index patch SHA-256
`d768da15903e0db5138dab72d2dbf8c94fbb5ca2e3b46f06492cbd3ac2bcaa42`.

The delivery was merged in [PR #184](https://github.com/777genius/universal-agent-plugins/pull/184)
as [`7d17c77599306d662e0bec0e0fd77d35a4f8941c`](https://github.com/777genius/universal-agent-plugins/commit/7d17c77599306d662e0bec0e0fd77d35a4f8941c).
These historical runs identify their tested premerge product tree, not a runtime
rerun of that merge commit. The latest release verified for the 2026-09-07 source snapshot is
**0.1.51 (2026-09-06)**, which does **not** contain PR #184. Do not apply these
candidate runtime results to that published binary.

[Compact evidence identities](https://github.com/777genius/universal-agent-plugins/blob/main/docs/evidence/client-compatibility-2026-09-07.json)
record image, client, installer, test, probe, invocation, and log hashes for
`codex-03`, `claude-03`, `opencode-03`, `git-01`, and the independently
audited `opencode-extended-11`. Raw transcripts are not
bundled with this summary; hashes identify retained artifacts, not a publicly
replayable transcript archive.

### Historical Linux arm64 native CLI matrix

Each run used a fresh test project and isolated HOME/client/XDG roots in a
container. Runtime used loopback MCP servers and scripted providers, without
user credentials or a real model. Container image digest:
`sha256:df6ab45e95c4425fcdccfc9a143bec29e2bf08293b63482c7843f398e06baabb`.
The compact record does not specify a distribution version.

| Evidence layer | Codex 0.153.4 | Claude Code 2.1.263 | OpenCode 1.18.29 |
| --- | --- | --- | --- |
| Standard package validity | Separate loader/conformance gate | Separate loader/conformance gate | Separate loader/conformance gate |
| Installation route | Local package, managed marketplace/native plugin | Local package, personal `@skills-dir` | Global JSON and JSONC, separate fixtures |
| Install, version update, same-version refresh, repair, remove | Passed | Passed | Passed |
| Activation/discovery | Native app-server plugin and skill discovery | Native CLI skill discovery | Native config/discovery; runtime HTTP catalog |
| Native MCP tool execution | Default/explicit stdio and HTTP | Two stdio servers and HTTP | Default/explicit stdio and HTTP, separate suites |
| Stdio cwd, argv, env, data continuity | Passed through update/refresh/repair | Passed through update/refresh/repair | Passed through update/refresh/repair in extended JSON suite |
| Skill body in outgoing context | Passed, explicitly selected skill with scripted Responses provider | Passed, native Skill tool with scripted Messages provider | Passed, native skill tool with scripted provider |
| Real model behavior | Not evaluated | Not evaluated | Not evaluated |
| OAuth/login | Not evaluated | Not evaluated | Not evaluated |

Installation, activation, and runtime are separate observations. None replaces
validation against the published Agent Plugins contract. The historical Linux adapter
cycle did not rerun the pinned conformance corpus and makes no new conformance
claim. Scripted-provider requests prove the observed client behavior and tool
calls; they do not prove real-model reasoning or successful OAuth.

The extended OpenCode run uses image
`sha256:cf4b5019fbb6d887a96ff066402594b367b7df75c90a86813d9ab9ef5fa491cb`
with offline ripgrep 14.1.1. It reuses the exact installer identity above; its
test source SHA-256 is
`55314bbed0587390965914a2d685a3b16d0ea926950625cc4dd91ab95ea2b0d4`.
Independent review verified all 21 referenced transcript hashes and 81 source
manifest hashes. The skill BODY nonce reaches outgoing tool context and is
absent from initial metadata/tool definitions; this does not establish model
comprehension. The run does not prove new installer code or another OS.

### Historical limitations and narrower proofs

- **OpenCode tool-name collision:** logical keys `api/server` and `api server`
  each work alone, but together expose one `api_server_inspect_runtime` callable
  name even though both MCP catalogs are visible. Config-key
  preservation is not callable-name uniqueness. Source checkpoint `c154185`
  adds the advisory `opencode_tool_namespace_not_evaluated` to CLI delivery
  output. It warns about unsupported colliding runtime combinations; it does
  not rename tools, prove uniqueness, or invalidate a standard package.
- OpenCode HTTP fixtures establish JSON/JSONC foreign-config preservation and
  collision refusal. The separate `opencode-extended-11` global JSON run proves
  stdio cwd, argv/env, once-only token expansion, and data continuity. Both tools
  and skill metadata are absent after removal, with no probe activity and a
  real main provider request observed. JSONC stdio/body coverage is not inferred.
- Codex excluded SSE, invalid skills, and missing runtimes; isolated executable
  startup failure; kept healthy calls working; and refused all-unsupported input
  without mutation. Claude did not exercise that full failure matrix. OpenCode
  proved SSE exclusion with healthy siblings, not the full Codex matrix.
- Codex observed HTTP protocol/header priority and a redirect that the client
  did not follow cross-origin. Claude and OpenCode proved HTTP calls, without
  independently testing redirect/header-priority behavior.
- Codex and Claude proved static foreign-content/drift protection within their
  fixtures. Separate regression tests cover publication rollback races; this is
  not a guarantee against arbitrary concurrent filesystem writers.
- `git-01` proved actual public immutable Git acquisition, installation,
  activation, native Codex skill discovery, and removal. Its skill-only fixture
  was [AP-6.2-MISSING-LOCATION-OK at the pinned corpus commit](https://github.com/Booyaka101/agent-plugins-conformance-kit/tree/4d163c6281a9b4929f482f387efe340b4dc175a1/fixtures/core/AP-6.2-MISSING-LOCATION-OK/plugin).
  Network access ended before native discovery. Git MCP execution and immutable
  switch/update were not evaluated. Claude/OpenCode native Git runs were not
  evaluated. Local and immutable Git installation do not require the Directory.

### Historical macOS arm64 and Windows amd64 source checkpoint

[GitHub Actions run 34158664586](https://github.com/777genius/universal-agent-plugins/actions/runs/34158664586)
passed all six required native client lanes without skips on source commit
[`ef20b9314b69d4094f4922946b8ad39024371202`](https://github.com/777genius/universal-agent-plugins/commit/ef20b9314b69d4094f4922946b8ad39024371202),
tree `4ba999db51304b056ec559c5429ed9aab9aace60`. All **348 indexed artifact
hashes** were verified. This is native evidence for that exact source-built
checkpoint, not the historical Linux installer, a published binary, or a claim
that all repository quality/release gates passed.

| Tested client | macOS arm64 | Windows amd64 |
| --- | --- | --- |
| Codex 0.153.4 | Native lifecycle, stdio/HTTP runtime and scripted skill context passed | Native lifecycle, stdio/HTTP runtime and scripted skill context passed, including removal, repeat-removal checks and foreign-content preservation |
| Claude Code 2.1.263 | Native discovery and scripted stdio/HTTP runtime lifecycle passed | HTTP and installed-skill lifecycle passed; managed stdio explicitly unsupported |
| OpenCode 1.18.29 | All three suites passed: JSON/JSONC lifecycle, extended stdio/skill body, HTTP tool collision | All three suites passed: JSON/JSONC lifecycle, extended stdio/skill body, HTTP tool collision |

For Codex and OpenCode, passing lifecycle includes install, version update,
same-version refresh, repair and remove. OpenCode's extended global JSON suite
separately proves stdio cwd/argv/env/data continuity and outgoing skill body;
that does not infer JSONC stdio/body coverage or resolve the observed tool-name
collision. Codex also exercises immutable acquisition within its native suite.
Standard package validity remains separate from these client observations.

**Claude on Windows has a narrower runtime contract.** The native HTTP and
installed-skill stages passed through A/B/C, repair and remove, with installer
data retention verified. Managed stdio reports the exact readiness diagnostic
`managed_stdio_platform_unsupported`. Stdio runtime is observed unsupported;
stdio cwd/argv/env/data behavior remains not evaluated. This explicit platform
limitation does not invalidate the portable package or block the tested healthy
HTTP/skill components.

These were disposable GitHub-hosted machines with fresh profiles and test
projects. The OS versions were not captured in the compact runner records.
**Network was not blocked**; model endpoints were scripted loopback services.
Exact client/build and runner-evidence digests are in the compact record. No
real-model, login, OAuth, or Desktop claim follows from these runs.

The native fixtures, runner wrapper and native workflow are byte-identical to
the independently reviewed `a6462a7` checkpoint. Their semantic review is reused;
the new run artifacts were independently hash-verified. This does not substitute
for review of other product changes between the commits.

Earlier checkpoints, including failed Windows lanes, remain in the evidence
identities as history. This run supersedes their platform summary; their
failures have not been relabeled as passes.

## Other platforms and clients

| Surface | Evidence boundary |
| --- | --- |
| macOS arm64 source checkpoint | All required native suites passed at `ef20b93`, including OpenCode extended runtime |
| Windows amd64 source checkpoint | All required native suites passed at `ef20b93`; Claude managed stdio remains explicitly unsupported |
| Linux amd64 | Not evaluated by these final runs |
| Desktop applications | Not evaluated by these CLI runs |
| Historical macOS 15.6.1 arm64, UAP 0.1.22 | Discovery/lifecycle only; Claude 2.1.205, Gemini 0.36.0, OpenCode 1.18.4; Cline/Windsurf config projection, client versions unrecorded |

The [historical macOS record](https://github.com/777genius/universal-agent-plugins/blob/main/docs/AGENTPLUGINS_CLIENT_E2E.md)
has its own installer, package, and transcript digests. It proves no browser
tool call, model, OAuth, or runtime claim for the current candidate. New results
must identify their own source SHA and client/OS versions before replacing any
pending cell.
