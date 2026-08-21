# Plugin verification matrix

Updated on 2026-08-10. Each column is independent: a schema pass is not an
install, an auth challenge is not OAuth success, and tool discovery is not a
tool call. `Direct harness` means MCP Inspector, not a client installation.
All passes below are historical. Current stable-launch runtime/OAuth status is
explicitly unavailable until the protected launch workflow completes against
the exact signed production publication and attested `agentplugins-v0.1.8`
release.

Current distribution, release sequence, and digest identity are generated from
[`registry/directory.json`](../registry/directory.json), not copied into this
historical matrix.

| Plugin | Schema | Installed | Tool called | OAuth | Clients |
| --- | --- | --- | --- | --- | --- |
| `agent-code-navigator` | Pass | Codex; Cursor local load; Kiro CLI resource | Packaged skill route passed | None | Codex; Cursor; Kiro |
| `atlassian` | Pass | No | No | Required - not tested | None |
| `chrome-devtools` | Pass | Codex; Cursor local load; Kiro workspace | `list_pages` | Local browser, not OAuth | Codex; Cursor; Kiro; direct harness |
| `cloudflare` | Pass | No | No | Required - not tested | None |
| `cloudflare-bindings` | Pass | No | No | Required - not tested | None |
| `cloudflare-docs` | Pass | Codex; Cursor local load; Kiro workspace; ChatGPT registered personal app; ChatGPT desktop manager | `search_cloudflare_documentation` | None | Codex; Cursor; Kiro; ChatGPT direct, personal app, and desktop control plane; direct harness |
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

The [Cloudflare Docs ChatGPT record](https://github.com/777genius/universal-agent-plugins/blob/fd77a74fa85724a57b328157ab82ef4dd991cda5/tests/e2e/results/chatgpt-cloudflare-docs-direct-2026-08-10.json)
(revision `fd77a74fa85724a57b328157ab82ef4dd991cda5`, SHA-256
`050a18c56cf3f6b98d12ad35ac3c4642bd18d9e862956447dc3dad8e3189bcc5`)
proves only a direct registered no-auth development connection: `list_resources`
and one read-only search passed. The separate [personal-app record](https://github.com/777genius/universal-agent-plugins/blob/2ddbb99dd190c1792b79904f9875e6322bccd243/tests/e2e/results/chatgpt-cloudflare-docs-personal-app-2026-08-10.json)
(revision `2ddbb99dd190c1792b79904f9875e6322bccd243`, SHA-256
`97ddb41b887eebb7629bff1ae88937448b0c23073688122ab8939c3d96372b37`)
proves Installed discovery under Personal, user-attested manual activation,
exact app ID linkage, and one read-only prompt with exactly two tool calls. It
does not by itself prove local `.codex-plugin` ingestion, repository marketplace
installation, or package-routed runtime. The separate
[desktop-package record](https://github.com/777genius/universal-agent-plugins/blob/fa9d61e1fe49bf3d69f54e451f6320f27930143a/tests/e2e/results/chatgpt-cloudflare-docs-desktop-package-2026-08-10.json)
(revision `fa9d61e1fe49bf3d69f54e451f6320f27930143a`, SHA-256
`85abcc90d50f358eb3d216d73b4dd33dbd6493d39d530070032ae6f50ced9990`)
proves exact-revision marketplace registration, official manager installation,
enabled state, cache materialization, and desktop backend parsing of the app
binding. It does not prove ChatGPT Work UI activation or package-routed runtime.

## Installer lifecycle

| Installer | Catalog | Package add/remove | Client runtime | OAuth |
| --- | --- | --- | --- | --- |
| [`agentplugins 0.1.6` release](https://github.com/777genius/plugin-kit-ai/actions/runs/31343240686) + [npm verification](https://github.com/777genius/plugin-kit-ai/actions/runs/31343525895) | Embedded legacy compatibility data, digest `66199c87...357050` | Public cold bootstrap passed on macOS, Linux, and Windows for x64/arm64; native lifecycle and published-Directory verification passed | Not implied | Not tested |
| [`agentplugins 0.1.6` post-merge run](https://github.com/777genius/universal-agent-plugins/actions/runs/31363316668) | Catalog v1 26/26 plus catalog-v2 Cloudflare Docs | 26/26 Cursor lifecycle; ChatGPT v2 dry-run, projection, State v3 repair, guarded removal, and cleanup passed | Not implied | Not tested |
| `agentplugins 0.1.6`, hero projections | 5 pinned hero packages | 25/25 add/remove flows across isolated Codex, Cursor, Copilot, VS Code, and Kiro projections | Not implied | Not tested |
| `agentplugins 0.1.6` + Copilot CLI 1.0.78 | 5 pinned hero packages | 5/5 automatic marketplace registration, native install, verification, uninstall, and marketplace cleanup in an isolated HOME | Not implied | Not tested |
| [Interactive hero runtime matrix](https://github.com/777genius/universal-agent-plugins/blob/75658d84ee84f973818d0e0c6b4619eb1e98b624/tests/e2e/results/agentplugins-hero-runtime-matrix-2026-08-08.json) (`75658d84ee84f973818d0e0c6b4619eb1e98b624`, SHA-256 `0c8795a67d223424cb612ea25497982145533a6befb5f64f8f0c1e1192520e2e`) | 5 packages at exact revision `d3c3155…` and catalog digest `207df0cd…c403915` | Client-specific test loading in Codex, Cursor, and Kiro | 15/15 checks passed across 3 clients for that exact historical revision | 3/3 Notion OAuth + read-only runtime passed |
| [Interactive Codex Figma check](https://github.com/777genius/universal-agent-plugins/blob/2132333206f469fd4adb63beeefe8ddbd4991a62/tests/e2e/results/codex-figma-oauth-2026-08-09.json) (`2132333206f469fd4adb63beeefe8ddbd4991a62`, SHA-256 `854ccb0d1987e7cc978d81d38e95cabcede60c78bb3baffb924987e7b11f2b53`) | `figma 0.1.0` via `agentplugins 0.1.5` | Add, install/enable, and cleanup passed in isolated profiles | Read-only Figma MCP `whoami` passed | Figma OAuth passed in Codex only |
| [ChatGPT desktop package check](https://github.com/777genius/universal-agent-plugins/blob/fa9d61e1fe49bf3d69f54e451f6320f27930143a/tests/e2e/results/chatgpt-cloudflare-docs-desktop-package-2026-08-10.json) (`fa9d61e1fe49bf3d69f54e451f6320f27930143a`, SHA-256 `85abcc90d50f358eb3d216d73b4dd33dbd6493d39d530070032ae6f50ced9990`) | Public marketplace at merge `d37b49d` | Official manager add, enabled state, cache, and app-server read passed | Not implied | No auth required; Codex backend app snapshot did not include the ChatGPT development binding |

The historical 15/15 row applies only to its linked revision and catalog
digest. It does not identify today's Directory defaults or package trees.

The three `0.1.6` lifecycle/projection rows come from public run `31363316668` at
main commit [`d3941c0`](https://github.com/777genius/universal-agent-plugins/commit/d3941c0ec097a44123eb9c40df940a3cda2a3406).
They prove source resolution, package validation, transactional lifecycle,
catalog-v2 ChatGPT package preparation, and native Copilot lifecycle. They do
not prove ChatGPT Work activation, package-routed runtime, Copilot tool runtime,
or OAuth; those claims stay separate. Sanitized Actions evidence artifacts are
retained for 30 days on the linked run.

The Figma record is a separate sanitized interactive Codex check. It does not
claim Figma installation, OAuth, or runtime in ChatGPT, Cursor, Kiro, or Copilot.
