# Verification record

Updated: 2026-08-09.

## Package conformance

- 26 root `plugin.json` documents pass the Agent Plugins 1.0.0 JSON Schema.
- 25 root `mcp.json` documents pass the Agent Plugins 1.0.0 MCP Schema.
- 4 skills pass `skills-ref` 0.1.1.
- The repository semantic validator reports 26 plugins, 25 MCP servers, and 4
  skills.
- All 26 generated OpenAI compatibility packages pass the repository validator
  and OpenAI's `plugin-creator` validator in CI. The latter is fetched from a
  pinned `openai/codex` commit and verified by SHA-256 before execution.
- The OpenAI adapter preserves the host-specific auth metadata published for
  GitHub, Figma, Linear, and Notion without adding unverified auth fields.

## Dependency verification

The npm registry stable tags were checked before pinning:

- `chrome-devtools-mcp@1.6.0`
- `@upstash/context7-mcp@4.0.0`
- `firebase-tools@15.26.0`
- `@hubspot/cli@8.12.0`

The Docker Hub package is pinned to the multi-architecture OCI digest recorded
in `plugins/docker-hub/mcp.json`.

## Runtime E2E

- Public post-merge run
  [`31332320890`](https://github.com/777genius/universal-agent-plugins/actions/runs/31332320890)
  tested `universal-agent-plugins@0.1.5` at merge
  [`8c2be5a`](https://github.com/777genius/universal-agent-plugins/commit/8c2be5a4740f0cef8b8dd8e57e51757e1f1167ea).
  It completed 26/26 transactional package lifecycles, 25/25 hero projections
  across Codex, Cursor, Copilot, VS Code, and Kiro, and 5/5 native Copilot
  marketplace install/list/remove lifecycles in disposable profiles. These
  checks do not prove client tool runtime or OAuth. Sanitized Actions evidence
  artifacts are retained for 30 days on the linked run.
- Codex CLI 0.144.1 used `universal-agent-plugins@0.1.5` to add `figma 0.1.0`
  for target Codex in a fresh git project and isolated `CODEX_HOME`. The
  generated plugin installed and enabled, Figma OAuth login completed
  interactively, and the read-only Figma MCP `whoami` call succeeded. No design,
  project, team, or workspace was opened or listed; no returned identity fields
  or secrets were recorded. The managed package and disposable profiles were
  removed. This proves Figma OAuth/runtime in Codex only; see
  [`codex-figma-oauth-2026-08-09.json`](../tests/e2e/results/codex-figma-oauth-2026-08-09.json).
- Codex CLI 0.144.1, Cursor Agent 2026.07.09, and Kiro CLI 2.16.0 each
  completed real agent-to-plugin checks for Context7, Cloudflare Docs, Chrome
  DevTools, and Agent Code Navigator in one disposable project. That is 12/12
  no-auth runtime checks across three clients. Codex CLI 0.144.1, Cursor Agent
  2026.08.04, and Kiro CLI 2.16.0 then each completed Notion OAuth and one
  synthetic read-only search, bringing the hero matrix to 15/15. The sanitized
  records are pinned to exact source commits in
  [`agentplugins-hero-runtime-matrix-2026-08-08.json`](../tests/e2e/results/agentplugins-hero-runtime-matrix-2026-08-08.json).
  A fail-closed test proves the stable catalog differs from those tested package
  trees only in `plugins/*/README.md`; separate 26/26 and 25/25 lifecycle runs
  cover the complete current package trees.

- Codex CLI 0.147.0 completed the release-gated public install on Linux: pinned
  `v0.1.1`, installed Context7 into a fresh `CODEX_HOME`, and called
  `resolve-library-id` from the installed package. The sanitized artifact records
  source/workflow commits, reproduction commands, and `/microsoft/playwright`;
  see [workflow run 31212969183](https://github.com/777genius/universal-agent-plugins/actions/runs/31212969183).
- MCP Inspector 2.1.0 completed 12 expected checks with zero unexpected
  results. Context7 and Cloudflare Docs passed representative read calls;
  Chrome DevTools exposed 29 tools from a disposable sandbox.
- Codex CLI 0.144.1 added the local marketplace, installed Context7 and Agent
  Code Navigator, called the Context7 MCP tool, and executed the packaged
  diagnostic skill in fresh disposable repositories.
- Codex CLI 0.144.1 also added the public GitHub marketplace, installed Context7
  from the cloned compatibility package, and returned
  `REMOTE_INSTALL_OK /microsoft/playwright` from a fresh disposable repository.
- Cursor 3.9.16 loaded the portable Context7 package from its local plugin
  directory, started version 4.0.0, and completed the stdio MCP connection in
  an isolated user-data directory.
- Kiro IDE 1.0.288 imported the unchanged Context7 package from a local folder,
  activated it as a Power, called `resolve-library-id` and `query-docs`, and
  returned `UAP_KIRO_E2E_OK` with a React documentation URL. The app profile and
  project were disposable; no real user project was opened.
- ChatGPT web Plugins Directory was verified in a signed-in session. Developer
  mode was enabled with explicit user consent, ChatGPT created a development
  connection for `https://mcp.notion.com/mcp`, completed Notion OAuth, and ran an
  authenticated read-only search for a synthetic probe. ChatGPT returned
  `UAP_NOTION_E2E_OK 0` without page titles or content. This verifies the raw MCP
  endpoint and OAuth flow, not installation of this repository's package. The
  interactive check used a user-approved personal account and workspace, not a
  dedicated test account. The connection was then removed and Developer Mode
  was restored to disabled. Provider-side settings still showed ChatGPT as
  connected. Notion offered only a workspace-wide disconnect that would also
  revoke an unrelated existing MCP client, so it was not used and cleanup is
  recorded as partial rather than complete.

Sanitized structured client evidence is committed under
[`tests/e2e/results`](../tests/e2e/results).

## Remote endpoint reachability

Every configured remote HTTPS origin returned an HTTP response during a
non-authenticated reachability check. Expected results were `401` for protected
origins, `405` for origins that reject a normal GET, and `200` for public web
frontends. The Sentry MCP endpoint was corrected to `https://mcp.sentry.dev/mcp`
after the live handshake exposed the web-root mismatch.

This proves DNS, TLS, and origin reachability only. It does not prove MCP
handshake behavior, OAuth compatibility, account scoping, or tool correctness.

## Deliberately not tested

No destructive tool, write operation, or real user project was used. Successful
Notion OAuth consent was completed interactively on a user-approved personal
workspace, followed by synthetic read-only searches and immediate client-side
credential cleanup. Provider cleanup remains explicitly partial because Notion
did not expose a safe granular revoke that could not affect an unrelated client.
Evidence excludes credentials, account/workspace identity, cookies, OAuth codes,
state, tokens, page titles, and page content. Automated and repeatable OAuth
tests must use a dedicated test account or workspace.
