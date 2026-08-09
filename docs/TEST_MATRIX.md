# Plugin verification matrix

Updated on 2026-08-09. Each column is independent: a schema pass is not an
install, an auth challenge is not OAuth success, and tool discovery is not a
tool call. `Direct harness` means MCP Inspector, not a client installation.

| Plugin | Schema | Installed | Tool called | OAuth | Clients |
| --- | --- | --- | --- | --- | --- |
| `agent-code-navigator` | Pass | Codex; Cursor local load; Kiro CLI resource | Packaged skill route passed | None | Codex; Cursor; Kiro |
| `atlassian` | Pass | No | No | Required - not tested | None |
| `chrome-devtools` | Pass | Codex; Cursor local load; Kiro workspace | `list_pages` | Local browser, not OAuth | Codex; Cursor; Kiro; direct harness |
| `cloudflare` | Pass | No | No | Required - not tested | None |
| `cloudflare-bindings` | Pass | No | No | Required - not tested | None |
| `cloudflare-docs` | Pass | Codex; Cursor local load; Kiro workspace | `search_cloudflare_documentation` | None | Codex; Cursor; Kiro; direct harness |
| `cloudflare-observability` | Pass | No | No | Required - not tested | None |
| `cloudflare-radar` | Pass | No | No | Discovery passed; consent not tested | Direct harness |
| `context7` | Pass | Codex; Kiro import; Cursor local load | `resolve-library-id`; `query-docs` | None | Codex; Kiro; Cursor; direct harness |
| `docker-hub` | Pass | No | No | Optional credentials - not tested | None |
| `figma` | Pass | Codex | Figma MCP `whoami` (read-only) | Passed in Codex | Codex; direct harness |
| `firebase` | Pass | No | No | Local CLI login - not tested | None |
| `github` | Pass | No | No | Discovery passed; consent not tested | Direct harness |
| `gitlab` | Pass | No | No | Required - not tested | None |
| `greptile` | Pass | No | No | Required - not tested | None |
| `heroku` | Pass | No | No | Required - not tested | None |
| `hubspot-crm` | Pass | No | No | Required - not tested | None |
| `hubspot-developer` | Pass | No | No | Local CLI login - not tested | None |
| `linear` | Pass | No | No | Discovery passed; consent not tested | Direct harness |
| `neon` | Pass | No | No | Required - not tested | None |
| `notion` | Pass | Codex; Cursor; Kiro | Authenticated read-only search in Codex, Cursor, Kiro, and raw ChatGPT MCP | Passed in Codex, Cursor, Kiro, and ChatGPT | Codex; Cursor; Kiro; ChatGPT; direct harness |
| `sentry` | Pass | No | No | Discovery passed; consent not tested | Direct harness |
| `statsig` | Pass | No | No | Required - not tested | None |
| `stripe` | Pass | No | No | Required - not tested | None |
| `supabase` | Pass | No | No | Required - not tested | None |
| `vercel` | Pass | No | No | Required - not tested | None |

The automated Codex record includes the public release ref and commit, workflow
URL and commit, copy-ready reproduction commands, and a sanitized three-event
transcript. All client records are under [`tests/e2e/results`](../tests/e2e/results).

## Installer lifecycle

| Installer | Catalog | Package add/remove | Client runtime | OAuth |
| --- | --- | --- | --- | --- |
| [`agentplugins 0.1.5` post-merge run](https://github.com/777genius/universal-agent-plugins/actions/runs/31332320890) | 26/26 pinned | 26/26 passed in an isolated Cursor provider HOME | Not implied | Not tested |
| `agentplugins 0.1.5`, hero projections | 5 pinned hero packages | 25/25 add/remove flows across isolated Codex, Cursor, Copilot, VS Code, and Kiro projections | Not implied | Not tested |
| `agentplugins 0.1.5` + Copilot CLI 1.0.78 | 5 pinned hero packages | 5/5 automatic marketplace registration, native install, verification, uninstall, and marketplace cleanup in an isolated HOME | Not implied | Not tested |
| Interactive hero runtime matrix | 5 local packages | Client-specific test loading in Codex, Cursor, and Kiro | 15/15 checks passed across 3 clients | 3/3 Notion OAuth + read-only runtime passed |
| [Interactive Codex Figma check](../tests/e2e/results/codex-figma-oauth-2026-08-09.json) | `figma 0.1.0` via `agentplugins 0.1.5` | Add, install/enable, and cleanup passed in isolated profiles | Read-only Figma MCP `whoami` passed | Figma OAuth passed in Codex only |

The first three rows come from public run `31332320890` at merge
[`8c2be5a`](https://github.com/777genius/universal-agent-plugins/commit/8c2be5a4740f0cef8b8dd8e57e51757e1f1167ea).
They prove source resolution, package validation, transactional lifecycle, and
native Copilot lifecycle only. They do not prove client tool runtime or OAuth;
those claims stay separate in the interactive row. Sanitized GitHub Actions
evidence artifacts are retained for 30 days on the linked run.

The Figma record is a separate sanitized interactive Codex check. It does not
claim Figma installation, OAuth, or runtime in ChatGPT, Cursor, Kiro, or Copilot.
