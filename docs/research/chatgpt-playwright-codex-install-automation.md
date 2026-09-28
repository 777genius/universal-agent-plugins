# ChatGPT Playwright and Codex installation automation - research

Date: 2026-09-27. UAP source: `167aa314833f57bfc368fa77f741cedade63667a` (`main` at research time). Playwright registry source: `979d36608d493bb9254ee1ababd84d76da519eb7`.

Status: research only. The owner has not approved implementation, release, registry changes, or use of a real ChatGPT account.

## What is proven for ChatGPT + local Playwright

- The registry Playwright 0.0.80 package declares one local `stdio` MCP server. Its Node launcher requires an absolute `PLUGIN_DATA` directory and installs the locked `@playwright/mcp` runtime there on first use. The active registry policy does not list `chatgpt` as a target.
- A disposable local run of that launcher completed MCP `initialize` and `tools/list`; 24 Playwright tools were listed. No browser tool was invoked.
- Official `openai/tunnel-client` v0.0.15 (macOS arm64 asset checksum matched its published `SHA256SUMS.txt`) accepted the actual Playwright launcher command in a disposable profile. Its `doctor` stopped at the missing runtime API key, as expected.
- `tunnel-client dev proxy` provided an in-memory local control plane without account credentials. HTTP `initialize`, `notifications/initialized`, and `tools/list` reached the real Playwright `stdio` process and returned 24 tools. This proves the transport bridge, not ChatGPT installation or browser execution. The test process was stopped and the disposable data removed.
- UAP currently rejects `stdio` for ChatGPT (`clients/chatgpt/plan.go`, `ComponentLimitations`), requires a registered app binding, and only permits the personal guided-registration receipt for the hard-coded Context7 identity (`domain/chatgpt_mapping.go`). Adding `chatgpt` to the Playwright catalog policy alone would be incorrect.

## Platform boundaries

- A private local server can connect to ChatGPT developer mode through Secure MCP Tunnel. The tunnel needs a `tunnel_id`, a runtime API key, suitable Platform tunnel permissions, workspace association, and a running `tunnel-client`. The documented ChatGPT flow creates the connection in the Plugins UI. [Secure MCP Tunnel](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels), [Connect and test](https://developers.openai.com/plugins/deploy/connect-chatgpt).
- A developer-mode connection then produces a personal `plugin_asdk_app...` ID used by a local plugin's `.app.json`. The documented installation path still includes installing that plugin in ChatGPT. No supported unattended developer-mode connection-creation API was found in the reviewed official docs; this is a documentation finding, not proof that no API can ever exist. [Package your plugin](https://developers.openai.com/plugins/build/plugins).
- A publicly submitted MCP plugin requires a stable public HTTPS endpoint. A personal Secure MCP Tunnel alone is not eligible. A public HTTPS proxy may forward to a private service, but must enforce per-user authentication, authorization, availability and review requirements. [Build an MCP server](https://developers.openai.com/plugins/build/mcp-server), [Submit plugins](https://developers.openai.com/plugins/deploy/submission).
- Zero user interaction is not a safe or documented promise: ChatGPT installation/connection consent and any required account authorization remain user actions. The practical product goal is to remove tunnel IDs, Platform keys, Developer mode, and manual configuration from the ordinary user's path.

## Product options - not approved

1. Public plugin + hosted authenticated MCP relay + local Playwright companion. UAP installs and supervises the companion; the user installs the published ChatGPT plugin and authorizes the connection. The relay routes only that user's calls to the paired local companion over an outbound connection. This preserves *the user's local browser* and avoids user-managed Platform tunnel credentials. Key risks: account-to-device binding, relay isolation, browser/file capabilities, offline handling, revocation, observability, cost and plugin review. This is an architecture hypothesis, not an E2E-proven design. 🎯 8/10, 🛡️ 7/10, 🧠 9/10; roughly 2,000-4,000 changed lines plus infrastructure. Best fit for the stated low-friction product vision, but not an MVP patch.
2. Guided per-user Secure MCP Tunnel. UAP can prepare the Playwright runtime, validate a tunnel-client profile, supervise its process and prepare a personal plugin after the user supplies the registered app ID. The user still needs tunnel permissions/key and ChatGPT registration, so this is suitable for developer/internal use, not the default mass-user path. 🎯 6/10, 🛡️ 8/10, 🧠 6/10; roughly 800-1,500 changed lines.
3. Public plugin backed by a hosted browser. This can simplify onboarding but changes the product semantics: it does not control the user's local browser. Browser tenancy, storage, privacy and cost become service responsibilities. 🎯 5/10, 🛡️ 7/10, 🧠 7/10; roughly 1,500-3,000 changed lines plus infrastructure.

Do not label any of these paths `AUTO` until its actual activation and verification boundary is proven. No ChatGPT account E2E was performed in this research.

## Codex `! MANUAL STEP` finding

- UAP target `codex` detects `codex` CLI, the `~/.codex` configuration root, and Codex desktop. It is one logical target covering these local surfaces, not a separate desktop-only target (`clients/codex/detect.go`). The adapter activates through the CLI if an executable is available: `codex plugin marketplace add`, `codex plugin add`, then `codex plugin list --json` (`clients/codex/lifecycle.go`). A missing CLI or unrecognized list contract falls back to manual activation/verification.
- The installed Codex CLI v0.152.0 exposes those `plugin` commands and JSON options. Existing focused UAP adapter tests for the explicit-profile activation sequence pass. This confirms the command contract and UAP test path; it is not a fresh real-plugin E2E.
- The preview nevertheless shows `! MANUAL STEP` because the Codex client definition inherits the default `ActivationByUser` in `domain/clients.go`; `clients/codex/plan.go` unconditionally adds `finish installation in Codex Plugins`, and `cli/internal/agentpluginscli/plan_review.go` maps the manual plan to that badge. The preview does not reflect the later automatic CLI activation path. Thus the displayed instruction can be misleading when `codex` CLI is detected.
- Candidate fix after owner approval: make the plan/preview conditional on a usable Codex CLI and verifier. Show automatic *installation* when the CLI path is available, with `start a new session` as a separate post-install usage note. Keep manual setup for desktop-only detection, absent CLI, or an unrecognized/failed verification contract. Do not claim live tool execution from `plugin list --json` alone.

No Codex implementation was changed during this research.
