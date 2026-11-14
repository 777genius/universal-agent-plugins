> **Owner-approved acceptance revision (2026-11-14):** No provider key is needed for this PR's final credential-free Project MCP lifecycle E2E. The original `tools/call` research below still documents an **unqualified** capability; it is *not* a mandatory gate and must not become a false-green claim.

# Cursor / Qoder genuine MCP `tools/call`: source + vendor capability boundary

Read against UAP `main` at `f32c9ee880170b8fa9a2ea3408ef69571d2feecf` and official client documentation on 2026-11-08. **No new native `tools/call` is claimed by this note.**

## Existing UAP probes are intentionally narrower

- `scripts/project_native_collectors/cursor.py`: exact pinned Cursor Agent CLI `2026.10.22-27dcf5f` has `--version`, `mcp --help`, and read-only `mcp list` probes. Its collector marks `cursor.mcp-tools-call` **unobserved**. Do not treat CLI help or inventory as a call.
- `scripts/project_native_collectors/qoder.py`: exact pinned Qoder CLI `1.1.78` supports `mcp list` + A/B project selection, but explicitly sets `mcp_tools_call_qualified=false`, and keeps `qoder.mcp-tools-call` unobserved.
- `scripts/project_native_qoder_installed_mcp.py` already verifies a genuine Qoder process sees **UAP-installed** mixed Project stdio/HTTP config and A/B isolation, then update/repair/remove; it intentionally does not claim authenticated transport or tool call. Reuse this existing harness.
- `docs/PROJECT_CLIENT_SUPPORT.md` accurately limits Cursor to `initialize`/`tools/list` and Qoder to native source/listing observation. Keep these boundaries until actual selected-client call receipts exist.

## Official CLI contracts

- Cursor [CLI MCP documentation](https://prod.cursor.com/docs/cli/mcp) exposes `agent mcp list`, `list-tools`, `login`, `enable` and `disable`. It describes **model-driven** tool use during `agent -p`; listing tools is not executing them.
- Qoder [MCP reference](https://docs.qoder.com/cli/mcp-reference) describes project MCP authorization, explicit allowlisted servers and tool permissions; lazy loading has meta tools `mcp_list`, `mcp_get`, `mcp_call` **inside a model session**, not an unauthenticated management `qoder mcp call` command. [Qoder tools](https://docs.qoder.com/cli/tools) documents `-p` / allowed-tools with real sessions.
- **False alternative to avoid:** an apparent “agent-sdk tool call” documentation at `cursor.com/docs/agent-sdk/reference/tools` currently describes `@grok/agent-sdk` and hosted server-tool direct calls, **not the pinned Cursor Agent CLI**. Running that SDK call would be a different product and does not qualify Cursor MCP.

## Smallest truthful E2E extension

1. Keep the existing exact UAP Project A/B stdio MCP fixture and native pinned executable SHA. First prove the selected real CLI can invoke the already configured MCP tool **without vendor-model credentials** on an authorized test path. If not, record the authentication refusal as negative evidence and do **not** simulate the native client or fake transcript.
2. If supported, use a selected-client read-only, explicit allowed tool plus a fresh random nonce. Require the server to receive actual `initialize`, `tools/list`, **`tools/call`** on the same session, and return the nonce; record exact native process identity, project A/B state and absence in B.
3. On remove/restart, prove new session cannot call the removed tool and foreign MCP bytes/trust are unchanged. Include denied-trust, mismatched source, unapproved/unsupported transport, and cleanup outcomes.
4. Exact HEAD real Windows/macOS/Linux (as supported by pinned client), original signed/sha-verified evidence, independent review, merge and postmerge proof. If private auth is mandatory, qualify only a legitimate disposable account supplied through approved CI secrets; no log of credentials, no mock-provider green.

## Current outcome

No auth-free selected-client invocation path verified for these pinned CLIs, and no new hosted E2E run has been launched. Draft PR #793 correctly remains pending; this note narrows the implementation so future attempts cannot confuse inventory, synthetic MCP requests or another SDK with a genuine native client call. VS Code #530 and Kiro #686 remain separate, and their unqualified native gates must not be silently included as PASS.

## 2026-11-12 independent vendor-doc contract check (source-only)

Official [Cursor headless](https://cursor.com/docs/cli/headless) says `agent -p` refuses unapproved MCP calls; `--force`/`--yolo` bypass that approval boundary and cannot qualify this PR. An authorized test must use exact named-tool permission in a disposable, explicitly consented profile; `CURSOR_API_KEY` is the documented CI credential name. It is not present or used by this research.

Official [Qoder CLI auth](https://docs.qoder.com/cli/authentication) uses `QODER_PERSONAL_ACCESS_TOKEN` for CI (not `QODER_API_KEY`). Official [Qoder MCP reference](https://docs.qoder.com/cli/mcp-reference) states `--strict-mcp-config` ignores the original project `.mcp.json`, and `--mcp-config` can substitute a different server. A **positive** native UAP-installed Project E2E must use neither override, must prove selected Project A source and explicit per-server trust / per-tool permission, and must retain independent server-side `tools/call` evidence. `--allowed-mcp-server-names` is a loading filter, not consent. Strict/alternate-config modes may serve only as separately labelled negative controls. Never use all-server approval, `--yolo` or `bypass_permissions` to claim default native behavior.

Three proof-boundary reviews: (1) separate inventory from actual model-initiated server call; (2) reject alternate-source trust/config bypass; (3) require genuine vendor auth and only narrowly consented named tool in disposable A/B roots. All leave `cursor.mcp-tools-call` and `qoder.mcp-tools-call` **unqualified** until original exact-HEAD native results exist. No executable, credentials, native-run, CI policy or site/release status changed by this documentation correction.

## Final revised qualification boundary

On exact source HEAD `14e87e5a5476cc5b38c686e86f7cb1c64547b97b`, the original two-client no-auth run [43617742434](https://github.com/777genius/universal-agent-plugins/actions/runs/43617742434) completed SUCCESS. Original proof artifacts were independently inspected in the PR review: genuine pinned binaries, source-built UAP, selected native MCP inventory, owned Project A install/removal, B/outside negatives and foreign-byte preservation. The artifacts explicitly state `native_tools_call_qualified=false` and `vendor_authentication_qualified=false`. Strict parser/runner contracts (20 tests) [43617742476](https://github.com/777genius/universal-agent-plugins/actions/runs/43617742476), Required [43617742544](https://github.com/777genius/universal-agent-plugins/actions/runs/43617742544), and Dependency Review [43617742427](https://github.com/777genius/universal-agent-plugins/actions/runs/43617742427) succeeded on that exact prior HEAD. These are **not** receipts for native model-driven `tools/call`.

For the owner-approved narrower acceptance, repeat the required no-auth lifecycle/contract checks on the final edited head, then merge and verify the protected-main no-auth push run. The optional secret-bearing vendor model experiment is manual-only on main. This does **not** upgrade `docs/PROJECT_CLIENT_SUPPORT.md`, Directory grants, or website capability labels for `tools/call`.
