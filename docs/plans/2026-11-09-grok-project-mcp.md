# Grok Build project Skills and MCP — scoped implementation plan

Date: 2026-11-09. Branch: `feat/grok-project-mcp-20261109`. Base: `00bcdf486dabef2dc2a4e6cb239bae818a451ab0`.

## Why

UAP currently advertises Grok Project as Skills-only because the original pinned 1.0.67/1.0.70 native inspect evidence did not qualify bundled MCP, and `grok.QualifyPlan` explicitly marks MCP unsupported. Current upstream Grok documentation supports plugins under `.grok/plugins/` and plugin-bundled `.mcp.json`, subject to project trust. An upstream capability claim is **not** a passed UAP native E2E.

Primary docs:
- https://docs.x.ai/build/features/skills-plugins-marketplaces
- https://docs.x.ai/build/features/mcp-servers
- https://github.com/xai-org/grok-build/blob/main/crates/codegen/xai-grok-pager/docs/user-guide/09-plugins.md

## Scope: one native adapter, no second installer

1. Preserve existing target/project path, version/inspect, source-pin, scanner, transaction, ownership receipt and rollback policy. Do not change User scope, another client, Directory signatures, or CLI target syntax.
2. Add **only** already modeled Grok Project MCP transports (stdio and Streamable HTTP) to the existing per-component capability table. Keep unsupported components (e.g. app/extension, SSE) unsupported; do not silently drop required components.
3. Project a selected MCP set as a standard plugin-root `.mcp.json` document with `mcpServers`, using the existing safe stdio placeholder rewrite / selection path. Do not add server definitions to the user's `~/.grok/config.toml` or trust flags. Existing User bundle projection stays untouched.
4. Permit prepared Grok Project plugin delivery with Skills, MCP or both; require at least one supported component, and continue refusing other supported kinds. `Prepared` / package-valid must not become `Active` by plan status, inspect.enabled, or `--activation-complete`. Review/enable/trust remains a real-user action.
5. Update only direct support statements and focused native/CLI tests; avoid changing generated support artifacts by hand. Do not claim model-authenticated `tools/call` or third-party account consent.

## Safety and E2E gates

- Correct stages in a disposable project root, with exact no-follow/containment and no file mutations outside owned artifact; project A/B and original user remain isolated.
- Signed Directory / direct-source, fresh/add-repeat/update/repair/remove and cold locked restore, with Skills+MCP and MCP-only cases, including rejected unsupported components and foreign/modified file ownership.
- Native `grok inspect --json` at pinned binary on supported OS verifies per-project identity and component discovery; `mcp doctor` or a real native scripted-model `tools/call` only when safe, deterministic and available. Distinguish native discovery from actual invocation.
- Existing Grok project strict-schema, collision, alias, partial-write, CLI and scanner regressions remain green. Required, CodeQL and relevant real cross-platform workflows must pass at PR HEAD. Review findings fixed before Ready/merge.
- No release, signing, registry publication or auto-trust without separate authorization. If native Grok does not discover this bundled MCP format, keep PR Draft and correct implementation/evidence; never weaken assertions to get green.

## Review of scope

- **Architecture:** reuse existing `ClientDefinition`, `grok` adapter and `shared.ProjectMCPServers`; no new core framework.
- **Security:** separate Project authority, no cross-scope fallback, preserve immutable receipt and refusal paths.
- **Compatibility:** leave User Grok CLI install/uninstall behavior unchanged; project old Skills behavior retained.
- **Proof:** project delivery + native plugin discovery + MCP initialize/`tools/list`/`tools/call` are different gates, not interchangeable.
