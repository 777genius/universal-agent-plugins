# Context7 ChatGPT and Kiro E2E evidence

The 2026-09-10 Context7 qualification covers the guided client paths shipped in
`universal-agent-plugins@0.1.57`. The public release is bound to commit
`6af7f412cb4a35d2e623fcba110f1fe53d9a2a5d`.

| Client | Real client evidence | Supported boundary |
| --- | --- | --- |
| ChatGPT | In a real Pro profile, a disposable Developer Mode app connected to `https://mcp.context7.com/mcp` with no authentication. A new chat explicitly selected it and successfully called `resolve-library-id` and `query-docs`. | The CLI validates the real `asdk_app_...` ID and prepares a personal marketplace package. App creation, marketplace installation, and selection remain visible ChatGPT steps. Availability depends on the account/workspace. |
| Kiro | In real Kiro v3 on macOS, Context7 completed OAuth, loaded after restart, and successfully called `resolve-library-id` and `query-docs` from the installed `context7` server. | The CLI owns only its skill and MCP entries. macOS and Windows use preparation plus an explicit OAuth/restart action; automatic ACP runtime verification is limited to Linux hosts with proven containment support. |

The final release candidate also passed isolated add, repeat add, update,
repair, remove, and receipt-backed reinstall for ChatGPT. The legacy 0.1.56
receipt path was reproduced: it requires a new explicit ID and then migrates
the retained receipt without exposing the private ID in JSON or stderr. The
exact 0.1.57 release pipeline passed native configuration lifecycle proofs on
macOS, Linux, and Windows for amd64 and arm64.

Private app IDs, account details, OAuth values, and conversation URLs are
intentionally excluded from this public evidence.

The older immutable multi-client lifecycle record remains in
[AGENTPLUGINS_CLIENT_E2E.md](AGENTPLUGINS_CLIENT_E2E.md). It is preserved
byte-for-byte because published npm release manifests cryptographically bind
that historical evidence.
