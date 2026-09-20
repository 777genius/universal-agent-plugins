# Portable MCP Standard Proposal

Date: 2026-03-30

This document proposes the next package-standard authored format for portable MCP in `plugin-kit-ai`.

It is written as a public-facing standard proposal, not just an internal implementation note.

## Goal

Remove the maintenance pain of carrying multiple native MCP configurations across:

- Claude plugin packages
- Codex plugin packages
- Gemini extensions
- OpenCode workspace config
- Cursor workspace config

The goal is not to pretend these tools have one identical MCP runtime model.

The goal is:

- one clear authored MCP source
- one understandable stable core
- several native target projections
- explicit escape hatches where platforms differ

## Sources Reviewed

Local repository sources:

- `docs/research/claude-code-plugins/README.md`
- `docs/research/codex-cli-plugins/README.md`
- `docs/research/gemini-cli-extensions/README.md`
- `examples/plugins/opencode-basic/README.md`
- `docs/generated/target_support_matrix.md`
- `cli/internal/pluginmanifest/manifest.go`
- `cli/internal/platformexec/{claude,codex,gemini,opencode,cursor}.go`

Official vendor docs reviewed during this proposal pass:

- Anthropic Claude Code MCP and plugin docs
- OpenAI Codex MCP and plugin docs
- Gemini CLI configuration and extensions docs
- OpenCode MCP docs
- current public Cursor MCP/rules surface, plus the repo’s own documented Cursor target scope

## Current State

Today portable MCP is already first-class, but still low-level.

What is already good:

- canonical authored path is `plugin/mcp/servers.yaml`
- native import already normalizes back into one portable location
- portable MCP is supported by `claude`, `codex-package`, `gemini`, `cursor`, and `opencode`
- typed authored schema now exists and projects cleanly into current target-native MCP shapes

What is still weak:

- the public contract still needs careful wording about what is truly portable versus merely projected
- env-variable interpolation semantics still differ too much across vendors to be part of one strict portable variable set
- some behaviors intentionally normalize friendly input instead of rejecting every non-ideal shape
- examples and surrounding docs still need to keep teaching the narrow stable core, not the full union of vendor options

Current maturity:

- normalization/generate/import layer: `Увер. 9/10`, `Надёж. 9/10`
- typed authoring contract: `Увер. 9/10`, `Надёж. 8/10`

## Decision Options

1. Keep raw object authoring and only improve docs. `Увер. 7/10`, `Надёж. 6/10`
This is the smallest change, but it does not actually remove multi-target authoring pain. Users still need to know too much about each vendor.

2. Invent a very broad universal MCP schema and force every target into it. `Увер. 5/10`, `Надёж. 3/10`
This looks ambitious, but it will become misleading fast because vendor surfaces already diverge on transport details, trust, OAuth, tool filtering, and variable interpolation.

3. Define a small stable core plus explicit target overrides and passthrough. `Увер. 10/10`, `Надёж. 9/10`
This gives the best balance: understandable for broad users, honest about platform differences, and durable as a public standard.

Recommended choice: option 3.

## Canonical Path And File Name

Recommended canonical authored path:

- `plugin/mcp/servers.yaml`

Why this is the best default:

- keeps `plugin.yaml` small and readable
- matches current discovery behavior
- scales if we later add neighboring files under `mcp/`
- aligns with vendor wording around `mcpServers`

Alternative names:

1. `mcp/servers.yaml`. `Увер. 10/10`, `Надёж. 9/10`
2. `mcp.yaml`. `Увер. 6/10`, `Надёж. 7/10`
3. `mcp/portable.yaml`. `Увер. 5/10`, `Надёж. 6/10`

## What Is Truly Common Across Targets

The real stable shared subset is narrower than "all MCP fields", but strong enough to be useful.

Common stable concepts:

- server alias
- connection transport
- process launch data for stdio servers
- remote URL data for network servers
- basic process environment
- basic HTTP headers
- target selection
- optional author-facing metadata

This is the actual cross-target core.

More careful comparison across Claude, Codex, Gemini, OpenCode, and Cursor shows:

- `command`, `args`, and `env` are the strongest stdio intersection
- remote `url` plus headers/auth shape is a strong intersection, but advanced auth semantics diverge
- `cwd`, `enabled`, and timeout-related fields are not strong enough to call universal across all supported targets
- server naming rules differ enough that the portable standard should choose one conservative alias format

## What Should Not Be In The Stable Core

These fields should not be force-standardized in `v1`:

- OAuth configuration
- trust / approval semantics
- per-server enable/disable controls
- per-server working directory
- Gemini-specific `includeTools` and `excludeTools`
- OpenCode-specific `type: local|remote` naming
- Cursor-native interpolation strings
- Claude-specific plugin-root variables
- Codex runtime-specific MCP config surface
- Codex-specific `required`, `enabled_tools`, and `disabled_tools`
- target-specific auth helpers or secret storage semantics

Reason:

- they are real, but not stable across all supported targets
- if they enter the common core too early, the standard becomes confusing and fragile

## Proposed Standard Shape

Recommended authored shape for `plugin/mcp/servers.yaml`:

```yaml
api_version: v1

servers:
  context7:
    description: Documentation MCP server

    type: stdio
    stdio:
      command: npx
      args:
        - -y
        - "@upstash/context7-mcp"
      env: {}

    targets:
      - claude
      - codex-package
      - gemini
      - opencode
      - cursor

    overrides:
      gemini:
        excludeTools:
          - "run_shell_command(rm -rf)"

    passthrough:
      codex-package:
        startup_timeout_sec: 10
```

## Proposed Schema

Top-level fields:

- `api_version`
- `servers`

`servers.<alias>` fields:

- `description`
- `type`
- `stdio`
- `remote`
- `targets`
- `overrides`
- `passthrough`

`<alias>` recommendation:

- use lowercase letters, digits, and hyphens only
- avoid underscores
- prefer short stable ids that can survive target-native tool qualification

Recommended alias regex:

- `^[a-z0-9]+(?:-[a-z0-9]+)*$`

`stdio` fields:

- `command`
- `args`
- `env`

`remote` fields:

- `protocol`
- `url`
- `headers`

`targets`:

- optional array of target ids
- when omitted, the server applies to every enabled target in `plugin.yaml`

`type` allowed values:

- `stdio`
- `remote`

`remote.protocol` allowed values when `type: remote`:

- `streamable_http`
- `sse`

Implementation note:

- `http` may be accepted as a user-friendly alias and normalized to `streamable_http`

This is the smallest clear vocabulary that still maps well to the vendor set you support.

## Naming Rules Matter More Than They Seem

This standard should be conservative about server ids.

Why:

- Gemini qualifies tool names from the server alias and warns about underscores
- different clients surface MCP tool names differently in prompts and UIs
- user-facing reliability is better when server ids are boring and portable

Recommended naming options:

1. lowercase letters, digits, hyphens only. `Увер. 10/10`, `Надёж. 10/10`
2. allow underscores too. `Увер. 6/10`, `Надёж. 5/10`
3. preserve any target-native alias string. `Увер. 4/10`, `Надёж. 3/10`

Recommended choice: option 1.

## Why The Format Should Use A Discriminated Union

Recommended connection modeling:

1. `type: stdio|remote` plus `stdio:` / `remote:` blocks. `Увер. 10/10`, `Надёж. 9/10`
This is the clearest shape for broad users because invalid field combinations become structurally obvious.

2. generic nested `transport` object with shared fields. `Увер. 8/10`, `Надёж. 8/10`
This is workable, but slightly more abstract and less self-documenting for people who are not thinking in protocol jargon.

3. vendor-shaped raw objects only. `Увер. 4/10`, `Надёж. 4/10`
This preserves fidelity but fails as a public standard.

Recommended choice: option 1.

## Why `type + block` Is Better Than A Flat Vendor-Like Shape

After comparing the native models more carefully, the standard should distinguish:

- whether the server is launched locally or reached remotely
- which remote wire protocol it uses when remote

That leads to:

- `type: stdio | remote`
- `remote.protocol: streamable_http | sse` only for remote servers

Why this is better:

- broad users naturally think "local command" vs "remote URL" first
- OpenCode already uses a local/remote split
- Claude and Cursor expose remote transport variants explicitly
- Gemini separates remote variants by `url` vs `httpUrl`
- Codex package/runtime docs clearly support stdio and streamable HTTP, but not a broad "all remote transports are equal" promise

## Final Cross-Agent Intersection

If we compare the supported CLI agent surfaces conservatively, the strongest shared subset is:

- server alias
- stdio command
- stdio args
- stdio env
- remote protocol kind
- remote URL
- remote headers

Secondary but weaker fields:

- description
- target scoping

Fields that look common but are not strong enough for strict `v1` portability:

- `cwd`
- `enabled`
- `timeout`
- trust / approval flags
- tool include/exclude lists
- OAuth config objects

This is the main correction from earlier drafts: a good public standard for all supported CLI agents must be narrower than the union of their docs.

Recommended connection model options:

1. `type: stdio|remote` plus `stdio:` / `remote:` blocks. `Увер. 10/10`, `Надёж. 9/10`
2. single `type: stdio|http|sse`. `Увер. 7/10`, `Надёж. 7/10`
3. flat vendor-like `command|url|httpUrl` without explicit mode. `Увер. 5/10`, `Надёж. 5/10`

Recommended choice: option 1.

## Variable Standard

The authored standard should not expose vendor-native variables directly.

Recommended standard variables:

- `${package.root}`
- `${path.sep}`

Meaning:

- `${package.root}`: absolute root of the generated package or workspace-owned target output
- `${path.sep}`: platform path separator

Why this matters:

- authors should think in package-standard concepts
- renderers should translate these to vendor-native forms where needed
- examples should stop teaching vendor-specific interpolation as the portable contract

Important constraint:

- `${workspace.root}` should not be part of the strict portable core in `v1`
- different targets treat "workspace" differently, while `package.root` maps more cleanly to both packaged and workspace-config lanes
- `${env.NAME}` should also stay out of the strict portable variable set in `v1`

Why `${env.NAME}` is deferred:

- Claude documents `${VAR}`-style expansion in `.mcp.json`
- Cursor uses `${env:NAME}`-style interpolation
- Gemini strongly documents `${extensionPath}` and path separators, but not one shared universal env-token syntax across all MCP surfaces
- OpenCode and Codex do not give one clearly matching env-interpolation contract here

Recommended env guidance for `v1`:

- use literal values in the portable core when possible
- use target-native env interpolation through `overrides.<target>` or `passthrough.<target>` when needed
- standardize env-token syntax only in a later version if the cross-target evidence becomes strong enough

Recommended variable design:

1. package-standard path variables with renderer translation, while deferring env-token standardization. `Увер. 10/10`, `Надёж. 9/10`
2. preserve vendor-native variables in the authored file. `Увер. 5/10`, `Надёж. 5/10`
3. ban variables in the authored file. `Увер. 3/10`, `Надёж. 4/10`

Recommended choice: option 1.

## Projection Rules Per Target

The portable file should project into each target like this.

### Claude

Portable projection:

- generate portable MCP to `.mcp.json`
- plugin manifest points `mcpServers` to `./.mcp.json`
- `${package.root}` should project to package-local relative paths inside shared `.mcp.json`

Notes:

- Claude supports package-local MCP
- if a repo also targets Codex package, `.mcp.json` stays a shared managed artifact and should not diverge per target

### Codex Package

Portable projection:

- generate portable MCP to `.mcp.json`
- plugin manifest points `mcpServers` to `./.mcp.json`

Notes:

- Codex package lane supports portable MCP
- Codex runtime lane does not and must stay outside this contract
- Codex docs clearly cover stdio and streamable HTTP in `config.toml`
- because the package-specific `.mcp.json` shape is documented less fully than Gemini/Claude, `v1` should keep the common core conservative

### Gemini

Portable projection:

- generate portable MCP inline into `gemini-extension.json` under `mcpServers`
- `${package.root}` can project to `${extensionPath}`

Notes:

- Gemini has more MCP-specific knobs than the proposed portable core
- Gemini-specific filters belong in `overrides.gemini` or `passthrough.gemini`

### OpenCode

Portable projection:

- generate portable MCP into `opencode.json` under `mcp`
- map `type: stdio` into `type: local`
- map `type: remote` into `type: remote`

Notes:

- OpenCode calls this `type: local|remote`
- that naming should remain a renderer concern, not authored-standard vocabulary
- OpenCode OAuth belongs in target passthrough

### Cursor

Portable projection:

- generate portable MCP to `.cursor/mcp.json`
- preserve Cursor-native string interpolation only at projection time if needed

Notes:

- Cursor support in this repo is intentionally scoped to workspace MCP plus rules; root `CLAUDE.md` and `AGENTS.md` are plugin boundary docs, not portable Cursor surfaces
- do not let Cursor-specific rule semantics leak into portable MCP

## Common Core Validation Rules

Package-standard validation should reject or flag:

- missing `format`
- unsupported `version`
- empty `servers`
- duplicate or invalid server aliases
- `type: stdio` without `stdio.command`
- `type: stdio` with a `remote` block
- `type: remote` without `remote.url`
- `type: remote` without `remote.protocol`
- `type: remote` with a `stdio` block
- unknown target ids in `targets`
- normalize an empty `targets` array the same way as an omitted `targets` field

Projection validation should additionally check:

- the target supports portable MCP at all
- the chosen transport is supported by that target
- variable usage is legal for that target
- target overrides do not conflict with generated managed keys
- target passthrough is well-typed JSON/YAML-object data
- server aliases remain valid under the portable alias rule

## Stable Core Field Recommendation

Recommended stable core fields for `v1`:

- `description`
- `type`
- `stdio.command`
- `stdio.args`
- `stdio.env`
- `remote.protocol`
- `remote.url`
- `remote.headers`
- `targets`

Field candidates that should stay out of the stable core in `v1`:

- `enabled`
- `cwd`
- `timeout`
- `oauth`
- `includeTools`
- `excludeTools`
- `trust`
- `required`
- `enabled_tools`
- `disabled_tools`
- vendor-native root variables
- `${workspace.root}`

Why these stay out for now:

- they are useful, but current evidence across all five supported targets is not clean enough to call them stable and universal without misleading users

## Example: Stdio Server

```yaml
api_version: v1

servers:
  release-checks:
    description: Release validation server
    type: stdio
    stdio:
      command: node
      args:
        - "${package.root}/bin/release-checks.mjs"
      env:
        LOG_LEVEL: info
```

## Example: Remote Server

```yaml
api_version: v1

servers:
  docs:
    description: Remote documentation server
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
      headers:
        Authorization: "Bearer DOCS_MCP_TOKEN"

    targets:
      - gemini
      - opencode

    overrides:
      gemini:
        excludeTools:
          - "delete_docs"

    passthrough:
      opencode:
        oauth:
          clientId: "docs-mcp-client-id"
          clientSecret: "docs-mcp-client-secret"
```

## Adoption Plan For The Current Contract

Recommended path:

1. standardize immediately on `plugin/mcp/servers.yaml` as the canonical authored portable MCP path and reject old authored paths early. `Увер. 10/10`, `Надёж. 10/10`
2. require the typed envelope with `format`, `version`, and `servers`; do not keep raw object-map authoring compatibility. `Увер. 10/10`, `Надёж. 10/10`
3. normalize native imports directly into the new envelope, preserving non-portable vendor data under `passthrough.<target>` when needed. `Увер. 9/10`, `Надёж. 9/10`
4. fail fast on old authored paths and shapes so the public contract becomes clear immediately, before users depend on ambiguous behavior. `Увер. 9/10`, `Надёж. 10/10`

## What This Standard Should Promise Publicly

Good promise:

- write one portable MCP file
- generate native MCP artifacts for supported targets
- validate where projection is safe
- use overrides and passthrough when platforms differ

Bad promise:

- one MCP schema with identical runtime semantics everywhere
- total parity of auth, trust, tool filtering, and approval behavior across vendors

## Final Recommendation

Recommended product decision:

1. standardize on `plugin/mcp/servers.yaml` as the canonical human-authored path. `Увер. 10/10`, `Надёж. 9/10`
2. introduce a package-standard envelope with `format`, `version`, and `servers`. `Увер. 9/10`, `Надёж. 9/10`
3. use a small stable core with `type: stdio|remote` and explicit `stdio:` / `remote:` blocks. `Увер. 10/10`, `Надёж. 9/10`
4. keep `plugin.yaml` lean and do not move full MCP into it. `Увер. 10/10`, `Надёж. 10/10`
5. add first-class `overrides.<target>` and `passthrough.<target>` rather than pretending every vendor field is portable. `Увер. 10/10`, `Надёж. 10/10`
6. keep the strict portable core narrower than the union of vendor docs, especially excluding `cwd`, `enabled`, `timeout`, trust, and tool-filter semantics. `Увер. 10/10`, `Надёж. 10/10`

If the goal is broad audience adoption, this is the right balance:

- clear enough for humans
- honest enough for serious users
- narrow enough to stay stable
- flexible enough to remove real multi-target maintenance pain
