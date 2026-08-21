# Verification record

Updated: 2026-08-10.

Every committed evidence link below resolves through the exact commit that
contains the cited bytes and is accompanied by their SHA-256. These are
historical observations, not claims about current rolling clients or package
trees. Current stable-launch runtime/OAuth evidence is unavailable until the
protected launch workflow succeeds against one exact signed production
publication and the attested `agentplugins-v0.1.8` release.

Current distribution, release sequence, tree digest, and manifest digest are
generated from [`registry/directory.json`](../registry/directory.json) and are
checked there rather than duplicated in this historical record.

## Package conformance

- 26 root `plugin.json` documents pass the Agent Plugins 1.0.0 JSON Schema.
- 25 root `mcp.json` documents pass the Agent Plugins 1.0.0 MCP Schema.
- 4 skills pass `skills-ref` 0.1.1.
- The repository semantic validator reports 26 plugins, 25 MCP servers, and 4
  skills.
- Released legacy compatibility outputs remain byte-for-byte unchanged. The
  later compatibility output adds the optional ChatGPT binding and pins its evidence path to an immutable
  full Git revision whose exact blob is verified before generation.
- All 26 generated OpenAI compatibility packages pass the repository validator
  and OpenAI's `plugin-creator` validator in CI. The latter is fetched from a
  pinned `openai/codex` commit and verified by SHA-256 before execution.
- Cloudflare Docs is the only package with a generated `.app.json`; its host-only
  development binding matches the exact public app ID, MCP endpoint, and pinned
  direct and personal-app runtime records. Human review still owns the app-ID
  ownership check. A separate desktop control-plane record proves package
  ingestion; the binding alone does not.
- The OpenAI adapter preserves the host-specific auth metadata published for
  GitHub, Figma, Linear, and Notion without adding unverified auth fields.

## Dependency verification

The npm registry stable tags were checked before pinning:

- `chrome-devtools-mcp@1.7.0`
- `@upstash/context7-mcp@4.0.0`
- `firebase-tools@15.26.0`
- `@hubspot/cli@8.12.0`

The Docker Hub package is pinned to the multi-architecture OCI digest recorded
in `plugins/docker-hub/mcp.json`.

## Generated-site browser verification

Pages pull requests generate and CSP-finalize the static production HTML before
loading it in Chromium with `@playwright/test@1.62.1`. Desktop and mobile
projects exercise the hydrated multi-select, combobox, and select by keyboard;
they also fail on browser console, page, request, response, or horizontal-overflow
errors and verify that unsigned review previews expose no copyable install
command. The separate fast accessibility tests are source-contract assertions,
not screen-reader or assistive-technology E2E.

## Runtime E2E

- A registered no-auth Cloudflare Docs connection in ChatGPT Developer Mode
  completed `list_resources` and one read-only `search_cloudflare_documentation`
  call for `Durable Objects SQLite storage API`, returning 7 results. This was a
  direct registered connection, not installation of this repository's generated
  package or the later personal-app UI path. The 2026-08-10 evidence date uses the
  repository operator's `Europe/Kyiv` local calendar. The sanitized record is
  [`chatgpt-cloudflare-docs-direct-2026-08-10.json`](https://github.com/777genius/universal-agent-plugins/blob/fd77a74fa85724a57b328157ab82ef4dd991cda5/tests/e2e/results/chatgpt-cloudflare-docs-direct-2026-08-10.json)
  at revision `fd77a74fa85724a57b328157ab82ef4dd991cda5`, SHA-256
  `050a18c56cf3f6b98d12ad35ac3c4642bd18d9e862956447dc3dad8e3189bcc5`.
- The registered Cloudflare Docs personal app appeared as Installed under
  Plugins > Personal. Its detail action opened a new Chat with the plugin chip
  selected. One read-only prompt made exactly one `list_resources` and one
  `search_cloudflare_documentation` call for `Durable Objects SQLite storage API`;
  the bounded response marker matched `E2E_OK Rules Of_`. This proves UI
  discovery, user-attested manual activation, runtime, and exact `.app.json`
  app-ID linkage.
  It does not prove local `.codex-plugin` ingestion, repository marketplace
  installation, or manager lifecycle. The sanitized record is
  [`chatgpt-cloudflare-docs-personal-app-2026-08-10.json`](https://github.com/777genius/universal-agent-plugins/blob/2ddbb99dd190c1792b79904f9875e6322bccd243/tests/e2e/results/chatgpt-cloudflare-docs-personal-app-2026-08-10.json)
  at revision `2ddbb99dd190c1792b79904f9875e6322bccd243`, SHA-256
  `97ddb41b887eebb7629bff1ae88937448b0c23073688122ab8939c3d96372b37`.
- ChatGPT desktop's bundled Codex backend 0.147.0-alpha.6.5 registered the public
  marketplace at exact merge `d37b49d`, installed and enabled Cloudflare Docs,
  materialized the official `.codex-plugin` package in its cache, and returned
  the exact app binding through `plugin/read`. Its `app/installed` snapshot did
  not contain that ChatGPT development binding. This proves repository
  marketplace ingestion and desktop control-plane parsing, but package-routed
  app routing remains unproved; it does not prove ChatGPT Work UI discovery,
  activation, or runtime. See
  [`chatgpt-cloudflare-docs-desktop-package-2026-08-10.json`](https://github.com/777genius/universal-agent-plugins/blob/fa9d61e1fe49bf3d69f54e451f6320f27930143a/tests/e2e/results/chatgpt-cloudflare-docs-desktop-package-2026-08-10.json)
  at revision `fa9d61e1fe49bf3d69f54e451f6320f27930143a`, SHA-256
  `85abcc90d50f358eb3d216d73b4dd33dbd6493d39d530070032ae6f50ced9990`.
- Public post-merge run
  [`31363316668`](https://github.com/777genius/universal-agent-plugins/actions/runs/31363316668)
  tested `universal-agent-plugins@0.1.6` at main commit
  [`d3941c0`](https://github.com/777genius/universal-agent-plugins/commit/d3941c0ec097a44123eb9c40df940a3cda2a3406).
  It completed 26/26 transactional package lifecycles, 25/25 hero projections
  across Codex, Cursor, Copilot, VS Code, and Kiro, and 5/5 native Copilot
  marketplace install/list/remove lifecycles in disposable profiles. It also
  passed catalog-v2 ChatGPT dry-run, official package projection, State v3
  repair, guarded removal, and cleanup. These checks do not prove ChatGPT Work
  activation, package-routed runtime, Copilot tool runtime, or OAuth. Sanitized
  Actions evidence artifacts are retained for 30 days on the linked run.
- Stable `agentplugins 0.1.6` was built from merge `b9f7353` and passed the
  [release pipeline](https://github.com/777genius/plugin-kit-ai/actions/runs/31343240686):
  exact native binaries for macOS, Linux, and Windows on x64 and arm64, frozen
  checksums and manifest, cold bootstrap, cache behavior, and native lifecycle
  proof. The separate
  [npm pipeline](https://github.com/777genius/plugin-kit-ai/actions/runs/31343525895)
  repeated public-release cold bootstrap on all six targets, published
  `universal-agent-plugins@0.1.6`, and verified the registry tarball, npm
  signature, SLSA provenance, and post-publish arbitrary `plugin.json`
  lifecycle. These release gates do not imply client tool runtime or OAuth.
- Codex CLI 0.144.1 used `universal-agent-plugins@0.1.5` to add `figma 0.1.0`
  for target Codex in a fresh git project and isolated `CODEX_HOME`. The
  generated plugin installed and enabled, Figma OAuth login completed
  interactively, and the read-only Figma MCP `whoami` call succeeded. No design,
  project, team, or workspace was opened or listed; no returned identity fields
  or secrets were recorded. The managed package and disposable profiles were
  removed. This proves Figma OAuth/runtime in Codex only; see
  [`codex-figma-oauth-2026-08-09.json`](https://github.com/777genius/universal-agent-plugins/blob/2132333206f469fd4adb63beeefe8ddbd4991a62/tests/e2e/results/codex-figma-oauth-2026-08-09.json)
  at revision `2132333206f469fd4adb63beeefe8ddbd4991a62`, SHA-256
  `854ccb0d1987e7cc978d81d38e95cabcede60c78bb3baffb924987e7b11f2b53`.
- At exact repository revision `d3c3155285d37aa555615cf4301e3ab5eb347a17`
  and catalog digest `sha256:207df0cd3932d305bbc265357d1a7f6b68ef314ff725629db6ebe27d4c403915`,
  Codex CLI 0.144.1, Cursor Agent 2026.07.09, and Kiro CLI 2.16.0 each
  completed real agent-to-plugin checks for Context7, Cloudflare Docs, Chrome
  DevTools, and Agent Code Navigator in one disposable project. That is 12/12
  no-auth runtime checks across three clients. Codex CLI 0.144.1, Cursor Agent
  2026.08.04, and Kiro CLI 2.16.0 then each completed Notion OAuth and one
  synthetic read-only search, bringing the hero matrix to 15/15. The sanitized
  records are pinned to that exact historical package revision in
  [`agentplugins-hero-runtime-matrix-2026-08-08.json`](https://github.com/777genius/universal-agent-plugins/blob/75658d84ee84f973818d0e0c6b4619eb1e98b624/tests/e2e/results/agentplugins-hero-runtime-matrix-2026-08-08.json)
  at revision `75658d84ee84f973818d0e0c6b4619eb1e98b624`, SHA-256
  `0c8795a67d223424cb612ea25497982145533a6befb5f64f8f0c1e1192520e2e`.
  This is historical evidence for that exact package revision only. It does not
  identify or validate today's Directory defaults; consult
  [`registry/directory.json`](../registry/directory.json) for current release
  identity. Separate lifecycle results remain valid only for the exact package
  trees they record.

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

Sanitized structured client evidence is committed under `tests/e2e/results`;
only the immutable, digest-bound links above are public verification pointers.

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
