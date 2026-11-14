# Gemini Runtime Audit

This note records the current production-ready runtime boundary for the Gemini Go runtime lane.

## Supported Runtime Surface

The Gemini Go runtime is currently production-ready, audited, and covered by the stable runtime promise for:

- `SessionStart`
- `SessionEnd`
- `BeforeModel`
- `AfterModel`
- `BeforeToolSelection`
- `BeforeAgent`
- `AfterAgent`
- `BeforeTool`
- `AfterTool`

## Runtime Evidence

The production-ready Gemini runtime is backed by:

- typed Go SDK surface in `sdk/gemini`
- descriptor-backed runtime metadata and generated support tables
- scaffolded Gemini Go runtime repos via `plugin-kit-ai init --platform gemini --runtime go`
- strict generate/validate contract checks
- deterministic repo-local runtime gate via `make test-gemini-runtime`
- dedicated opt-in real CLI runtime smoke via `make test-gemini-runtime-live`

The deterministic Gemini smoke now covers:

- lifecycle input decoding
- runtime control semantics such as `deny`, `continue:false`, `systemMessage`, `clearContext`, `suppressOutput`
- runtime transform semantics such as request/response rewrite, tool selection config, turn-local context injection, tool-input rewrite, tool-result context, and tail tool calls
- tool payload observability including `tool_input`, `tool_response`, `mcp_context`, and `original_request_name`

The live Gemini runtime smoke uses an explicit tool-use prompt for the tool path. On current Gemini CLI builds this is materially more reliable than injecting `@README.md` content and hoping the model still chooses a tool call. The live gate now checks real CLI scenarios across the stable contract:

- happy-path tool execution with `response: "OK"` plus successful `read_file` vendor stats and zero tool failures
- blocked-tool control semantics where `BeforeTool` denies `read_file`, Gemini reports a failed `read_file` call in the vendor JSON envelope, and the trace proves `AfterTool` never fired
- blocked-model control semantics where `BeforeModel` denies the turn, Gemini returns an empty `response`, records zero tool activity, and the trace proves neither `AfterModel` nor tool-selection/tool execution hooks fired
- model transform semantics where `AfterModel` replaces the model output, Gemini returns rewritten response text, records zero tool activity, and the trace proves tool-selection planning may already have fired but no tool execution occurs
- agent retry semantics where `AfterAgent` denies once, Gemini retries the turn, returns corrected response text, and the trace shows the second `AfterAgent` pass with `stop_hook_active=true`
- tool-selection allow-list semantics where `BeforeToolSelection` restricts execution to `read_file`/`list_directory`, Gemini records successful `read_file` stats with zero failures, and the trace proves the runtime took the `allow_only` branch before `BeforeTool`/`AfterTool`
- tool-selection `mode:"NONE"` semantics where `BeforeToolSelection` disables all tools, Gemini records zero tool activity, still emits `AfterModel`, and never reaches `BeforeTool`/`AfterTool` even if the model text still mentions a tool-style plan
- transform semantics where `BeforeTool` rewrites a missing `read_file` path to `README.md`, Gemini records successful `read_file` stats with zero tool failures, and the trace proves the runtime took the `rewrite_input` branch before `AfterTool`

Additional live probes against the current `gemini 0.36.0` vendor build did not justify widening the release gate further:

- `BeforeModel synthetic_response` was ignored in the live CLI path and the session still returned the original `OK` response
- allowlist / `mode:"ANY"` tool-selection probes did not stay boring-stable in the live CLI path: one run ended in vendor `AbortError`, while the corrected allowlist shape then fell into loop/capacity exhaustion

Those paths remain covered by deterministic runtime evidence, but they are not currently counted as release-gating live proof.

## Stable Promise Boundary

The current stable Gemini runtime promise covers only:

- `SessionStart`
- `SessionEnd`
- `BeforeModel`
- `AfterModel`
- `BeforeToolSelection`
- `BeforeAgent`
- `AfterAgent`
- `BeforeTool`
- `AfterTool`

It does not automatically widen to future Gemini hooks just because the upstream CLI adds them. Any future Gemini runtime surface still needs:

- descriptor-backed metadata
- scaffold and validate alignment
- deterministic smoke coverage
- production docs alignment
- sufficient live evidence for the intended stable promise

`Notification` and `PreCompress` remain outside the stable Gemini runtime promise.
