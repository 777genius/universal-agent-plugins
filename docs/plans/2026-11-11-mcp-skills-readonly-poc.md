# MCP Skills over MCP — read-only UAP PoC

**Date:** 2026-11-11  
**Status:** Plan v1; design review before implementation  
**Target:** `777genius/universal-agent-plugins`; one isolated, read-only experimental PoC.

## Decision and exact scope

UAP already installs local/GitHub/HTTPS-publisher Agent Skills and configures MCP servers. It does **not** thereby implement the distinct `io.modelcontextprotocol/skills` discovery extension. Codex app-server `skills/list` is not MCP Skills. SEP-2640 is Final but the extension specification itself is **Beta**. OpenAI `mcp-extensions` is a separate set of host-specific UI capabilities.

Implement a **read-only discovery/inspection PoC**, not a replacement for existing UAP acquisition, package manager, lockfile, install or client adapters. The PoC accepts an explicitly supplied, credential-free HTTPS MCP Streamable HTTP endpoint, and optionally a full skill URI. It lists skill metadata, inspects one exact URI, and verifies the returned `SKILL.md` on explicit inspect. It must not execute scripts, send any file to an agent/model, install/activate Skills, modify `uap.toml` or `uap.lock`, register credentials, or persist a server. Supporting resources remain lazy. No scope/target/yes flag is appropriate to a read-only feature.

## Architecture: reuse real UAP boundaries

Read [AGENTS.md](../../AGENTS.md), [architecture](../ARCHITECTURE.md), and [contribution rules](../../CONTRIBUTING.md). Adopt existing Go Clean Architecture, SOLID and DRY. DDD is **pragmatic** here: skill URI + server origin and a validated resource manifest are value concepts; there is no mutable aggregate, repository, ubiquitous-language framework or database to introduce for read-only work.

```text
agentplugins skills mcp list|inspect (CLI only)
           ↓
small read-only application flow (no mutable state)
           ↓
MCP Skills reader boundary (only if it avoids a real dependency)
           ↓
adapters/mcpskills: modern Streamable HTTP and verification
           ↓
external MCP server (read-only calls)
```

Place the cohesive adapter under `install/integrationctl/agentplugins/adapters/mcpskills` using standard library plus existing relevant conformance parsing. The CLI in `cli/internal/agentpluginscli` wires the adapter, renders safe output and changes no installer state. Extract a pure `domain` type **only if shared outside this adapter**; domain must stay stdlib-only. Do not add a process supervisor, Node SDK sidecar, generalized provider framework, feature flag, second resolver or client-ID branches. Do not touch frozen VS Code or generated code. Limit Go files to 500 effective lines, functions to 60 lines/40 statements.

### Protocol contract (MCP `2026-07-28` only)

1. Modern protocol: use `server/discover`, **not** `initialize`; require advertised `capabilities.resources` and `capabilities.extensions["io.modelcontextprotocol/skills"]`. Fail closed on legacy servers; legacy fallback is not in PoC.
2. Per-request `params._meta` includes `io.modelcontextprotocol/protocolVersion`, `clientInfo` and `clientCapabilities`. Streamable HTTP requires POST, `MCP-Protocol-Version`, `Mcp-Method`, `Mcp-Name` for `resources/read`, and `Accept: application/json, text/event-stream`. Support JSON and scoped SSE request responses, bounded by context, response size and time; do not follow redirects or call tools.
3. `skills/list`: preserve exact URIs and origins, paginate `cursor`/`nextCursor` with cycle/count/size bounds, require `resultType=complete` and caching fields. Empty/partial list is **not** proof that no skill exists. `skills/get`: retrieve exact URI even if unlisted, with required metadata.
4. Validate the manifest: `SKILL.md` resource present once, unique file URIs, `frontmatter.name` and `description`, file root containment (not DNS), all SHA-256 digests `sha256:<64 lowercase hex>`, sizes, max 512 resources and 16 MiB total. Alternative URI schemes are permitted by spec, so don't hardcode `skill://` as proof that a resource is a skill. `resources: "dynamic"` is discoverable but cannot be called verified.
5. On explicit inspect, call `resources/read` for `SKILL.md` only, verify raw bytes, size and SHA-256, and compare all parsed YAML frontmatter fields to entry JSON (not just name/description). Other resources are never prefetched. Neither read nor inspect loads a Skill into an agent.
6. Identity is `(UAP-assigned server reference, exact URI)`; two servers can share a URI/name without shadowing. Reject duplicate/conflicting resources, URI traversal, unexpected response URI and invalid/mismatched frontmatter; do not grant any permissions from `allowed-tools`. Hash verification is integrity against untrusted server assertions, **not signing or trust**.

### Proposed PoC UX (unreleased examples)

```bash
agentplugins skills mcp list --endpoint https://mcp.example.com/mcp --format json
agentplugins skills mcp inspect --endpoint https://mcp.example.com/mcp \
  --uri skill://pdf-processing/SKILL.md --format json
```

Default responses summarize source origin, full URI, name, description, resource count and verification state without printing SKILL.md, secrets or raw network errors. Endpoint must be an explicit HTTPS URL without embedded credentials. Tests may inject a loopback HTTP test client; public CLI cannot accept plain HTTP. No special login/OAuth, auto-install or tool calls.

## E2E and security gates

- Disposable compliant modern `httptest` MCP server exercises `server/discover → skills/list` (pagination) `→ skills/get → resources/read`; test both JSON and SSE response paths and direct get of an unlisted URI.
- Reject missing capability, old protocol, malformed response, bad result type, pagination loop, redirect, timeout, cancellations, oversized bodies, dynamic falsely labelled verified, digest/size mismatch, duplicate/unlisted/out-of-root paths, full frontmatter mismatch and cross-server name/URI ambiguity.
- Assert no HOME, XDG, project or installer state effects; no subprocess/tool activation; redact errors/JSON. Fixture E2E must **not** claim real external-host activation.
- Run focused Go tests in nested module and CLI, `go vet`, `make lint`, `make test-core`; check exact final HEAD CI, reviews, conflicts and post-merge main. Never count queued/skipped jobs as PASS.

## Estimate and non-goals

| Slice | Production LOC | Test LOC |
| --- | ---: | ---: |
| Modern HTTP JSON/SSE transport and negotiation | 230–350 | 180–260 |
| Manifest URI/integrity + YAML/frontmatter verification | 190–270 | 230–310 |
| list/get/inspect use case + CLI wiring | 100–170 | 120–180 |
| Docs, fixtures and CI | 40–90 | 50–100 |
| **Total** | **560–880** | **580–850** |

**PoC planning envelope:** ~1,140–1,730 new/changed lines including tests and docs; complexity 4/10, correctness risk 5/10. These are estimates, not measured diff. No stdio, OAuth, automatic transport-version fallback, local snapshot import, dependency/lock migration, native host lazy-loading, new website UI or release publication.

A future *separate* design would add explicit reviewed snapshot import using existing `skillsource.Normalize`, `environment` and `installer`. It needs a distinct trust, source-identity, approval and cold-restore design and must not be smuggled into this PoC.

## Review rounds

1. **Architecture/DDD:** retain existing package boundaries, reject generic frameworks, no second installer.
2. **Protocol:** verify against Beta spec and modern 2026 transport (the 2025 handshake is NOT valid here), SSE/caching/metadata.
3. **Security:** exact server+URI identities, hashes and YAML equality, no automatic trust elevation, redirects or writes.
4. **UX/E2E:** honest read-only wording; negative tests; no incomplete/skipped CI counted as success.

## Sources

- [MCP Skills Beta specification](https://skills.extensions.modelcontextprotocol.io/specification/beta/skills)
- [SEP-2640](https://modelcontextprotocol.io/seps/2640-skills-extension)
- [MCP 2026 Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)
- [MCP versioning](https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning)
- [UAP existing Skills installer](../AGENT_SKILLS_INSTALLER.md)

## Review R1 — actual source and Clean Architecture (2026-11-11)

**Evidence checked:** `AGENTS.md`, `docs/ARCHITECTURE.md`, `cli/internal/agentpluginscli/skills.go` and `root.go`, `install/integrationctl/agentplugins/conformance/{skills.go,author_skills.go}`, `domain/web_skill_policy.go` and HTTPS acquisition. These are existing authorities, not inferred abstractions.

Findings and corrections:

- Register one explicit `newSkillsMCPCommand` under the existing `skills` group. Never call `app.forOptions` for the no-state probe: it can open or validate environment roots and would contradict the zero-mutation promise. Instead validate only compatible global flags (`--target`/project-specific flags refused; `--format` reused).
- Reuse **`domain.ValidSkillName`** and **`conformance.DecodeFrontmatter`** to parse the actual bounded YAML frontmatter; pass the full set of entry keys as its requested root fields, check that the decoded key set exactly equals the advertised object and compare canonical JSON values. Do not write a second YAML grammar or hardcode the six known fields; unknown future keys must still be compared field-by-field.
- Keep transport and manifest checks in one cohesive `adapters/mcpskills` Go package in the nested core module. For this sole consumer, an exported `Client` with `List/Get/Inspect` methods is sufficient: **do not create a speculative interface/use-case package** or a new domain model. Network parsing and pure verification should live in separate small files in that package. Tests inject a private `http.Client` and do not need an exported mockable framework.
- Do not reuse `sourceacquisition`'s HTTPS publisher discovery as MCP Skills: its `.well-known` index is a different protocol, and no repository/lock changes are justified by this PoC.
- Public CLI uses an explicitly constructed, credential-free, no-redirect HTTPS client with **public-address-only dial policy** (same security boundary as existing publisher previews); tests use a deliberately injected loopback client. Never forward ambient proxy settings, cookies or credentials. Errors are reduced to typed safe categories before JSON or human output.

**Disposition:** R1 passed after narrowing the module boundary and removing the speculative application/port layer. No code implemented yet.

## Review R2 — wire protocol / SDK lifecycle (2026-11-11)

**Independent references:** MCP Skills Beta specification; MCP base `2026-07-28` Streamable HTTP, versioning and server/discover; TypeScript SDK 2.4.0 SkillsClient; official Go SDK v1.8.0 and pending Go Skills extension PR #1238. Distinguish the **Final SEP** from the still-**Beta extension**. Do not claim that a speculative `skills/list` handler is a complete MCP client.

Corrections to implementation contract:
- Limit this experiment to `2026-07-28` stateless messages. `server/discover` is capability discovery (not a handshake), and `initialize`/`notifications/initialized` would be wrong. Reject a server whose `supportedVersions` does not include the pinned version.
- `server/discover`, `skills/list`, `skills/get` and `resources/read` must be called through the **same bounded modern JSON-RPC POST transport**. All requests use `params._meta` and mirrored HTTP headers. A server returning `resultType: input_required` is outside the read-only PoC and must be refused; never perform automatic elicitation.
- Both `application/json` and `text/event-stream` are valid response types. The test fixture must send notifications then the matching ID result in the SSE case; ignore only request-related notifications, reject a competing ID, duplicate final responses, truncated stream and oversized event/body. `Mcp-Name` equals the exact URI for `resources/read`; encode non-ASCII header values according to protocol rather than skipping the header.
- Require `resultType=complete` plus `ttlMs`/`cacheScope` from the modern discover, list, get and read results. Discard freshness hints in the PoC: **no caching**, avoids stale permission/context confusion.
- Do **not** fetch `SKILL.md` on list; get-only inspect returns its manifest and on explicit inspect may read the one entry. The proposed CLI inspect is explicitly a user-initiated read, never host activation. Dynamic entries remain discoverable and get-only but unverified.
- Do not add an external Go SDK dependency just to depend on pending typed extension support. A narrow, test-covered, modern-only transport is temporary PoC code; production replacement with official Go Skills client is a follow-up decision, not an API migration in this PR.

**Disposition:** protocol review exposes no blocker for the limited PoC. Implementation must be verified against concrete modern wire fixtures, not old `initialize` samples.
