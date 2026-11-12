# Hermes native tools/call without vendor authentication

Baseline: main `446872e2b98c0e337da37969f0fd6fbc26772972`. The scoped plan is committed before implementation publication. Root verified
that #881 changes only its release-proof checker and tests; none of those paths
are modified here.
Do not modify release workflows, #881 files, #878 automation, production
adapters, vendor source, credential handling or the Project MCP refusal.

## Evidence and supported native route

Keep the genuine official Hermes 0.23 source pin
`2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`, its official installation/launcher
and resolved native Windows runtime. Use the official legacy `hermes-agent` /
`run_agent.py` CLI through the installed runtime and official `runtime_command`
bootstrap, with `--base-url http://127.0.0.1:PORT/v1 --model ID
--enabled-toolsets uap-native-test-server --max-turns 3 --query QUERY`. The
test-owned profile explicitly selects `model.provider: custom`. This runs the
real native agent loop without expanding ownership to normal chat's persistent
gateway. It does not qualify the default chat UI. The pinned
`runtime_provider_backends.py` deliberately supplies the SDK's
`no-key-required` placeholder for a custom endpoint without credentials. No
vendor login, fabricated credential, trust bypass or real inference is needed.

Use the real native CLI and agent loop. A bounded localhost OpenAI-compatible
fixture returns one tool-call completion using the exact tool that Hermes
actually advertises, then one terminal completion after it receives the real
tool result. The expected native wire name is
`mcp__uap_native_test_server__ping`; fail if missing or ambiguous. Do not call
Hermes' tool handler or MCP fixture directly from the test harness.

The official MCP registration provides the raw server-name toolset alias;
native discovery filters selected MCP servers before startup. Selecting only
`uap-native-test-server` therefore preserves the unrelated `foreign` config
without starting its `echo` fixture as an MCP service.

## Proposed publish paths

1. `scripts/e2e/hermes-noauth-tool-call.py` — a separate native case, reusing
   `Lifecycle`, official runtime resolution and `stop_lifecycle` from the
   existing Hermes helpers. A small test-only subclass adds native turns after
   User first add and repair while inheriting the existing User/Project A/B,
   foreign preservation, removal and Project MCP refusal scenarios.
2. `scripts/hermes_local_provider_fixture.py` — a bounded localhost scripted
   Chat Completions endpoint, supporting the actual client's JSON/SSE mode.
3. `scripts/test_hermes_local_provider_fixture.py` — inert finite-state,
   body/request/output/time bounds, wrong tool/result/nonce and evidence tests.
4. `.github/workflows/hermes-noauth-tool-call.yml` — new source-only Linux,
   macOS and native Windows jobs. Reuse the existing pinned official installer
   commands and runtime resolution. No edits to the release workflow or existing
   Hermes release-call inputs. Add path-scoped main PUSH qualification.
5. `docs/plans/2026-11-12-hermes-noauth-native-tool-call.md` — this finite contract.
6. `scripts/pi_project_components_native_e2e.py` — ONLY a separately reviewed,
   optional nonce/observation extension to `PiHTTPMCPProbe`. Default constructor,
   response bytes, method recording and existing Pi/Hermes callers stay unchanged.
   Root checked this shared path against #881; exact default-wire review remains
   required before publication.

## Admission and boundaries

Keep the old discovery-only Hermes case and its no-tools/call oracle unchanged.
The new case uses a fresh owned profile, source/driver identities and bounded
process-tree ownership. Configure the supported custom provider in the test-owned
profile before establishing the YAML/foreign baseline. Child env is closed;
no ambient provider keys, auth stores or proxy endpoints are inherited.

Use a fresh random nonce and explicit stage in the inert ping schema/arguments
and returned text. Declare the inert ping read-only accurately; do not use yolo
or another approval bypass. Require exact joins among the provider's offered
native tool, requested function and call ID, actual MCP tools/call request and
nonce/stage, returned nonce, and the subsequent native tool-result message.
One call per selected stage; reject duplicates, wrong/absent results, extra
completion attempts, retries, failed exit, timeout or missing evidence.

Retain bounded original provider/MCP transcripts, process exit and cleanup,
source SHA/tree/clean state, Hermes pin/runtime identity, run/attempt/OS and
stream hashes. Write an explicit failure receipt on every unsuccessful path.
Reports distinguish `native_agent_turn` and `scripted_provider_exchange` from
`real_model_inference=false` and `vendor_authentication=false`; derive
`tools_call` from observed protocol, never a constant. Project MCP remains an
actual expected-denied boundary. No release/Directory qualification is inferred.

## Estimate and validation

The implementation is approximately 356 native-driver lines (including bounded
capture and owned Windows job cleanup), 214 provider-fixture lines (including
original request/response capture), 169 inert-test lines, 212 new workflow lines
and 51 added optional shared-fixture lines, plus this plan. Complexity 4/10;
source feasibility high, cross-platform
reliability 7/10 pending actual native CI. First hosted run must settle native
CLI streaming, custom-provider startup and toolset/approval behavior; uncertainty
fails qualification rather than introducing a bypass.

Review shared default-wire invariance before publication. Run only meaningful
inert fixture/receipt regressions locally; actual qualification requires original
new-head native results on all three OS, ordinary required checks and independent
artifact admission, then new-source main PUSH proof. No client/toolchain install
or native turn has been executed locally for this feasibility review.

## Exact official source inspected

- `hermes_cli/_parser.py`, lines 237-309: one-shot query, custom provider,
  base URL, toolset and turn/wall-clock budget flags.
- `hermes_cli/runtime_provider_backends.py`, lines 116-204: explicit custom
  endpoint routing, credential isolation and no-key placeholder.
- `hermes_cli/main.py::cmd_chat` and `cli.py::main`: real native CLI dispatch.
- `agent/legacy_cli.py` and `run_agent.py::main`: the selected official legacy
  CLI driver, using the same native agent loop and MCP registry.
- `tools/mcp_tool_schema.py`, lines 147-193: native generated function identity.
- `tools/mcp_tool_registration.py`: server-name alias registration.
- `tools/mcp_tool_discovery.py`, lines 604-630: selected-server native startup.
- `tools/mcp_tool_handlers.py::_call_tool_racing_stdio_death`: actual SDK
  `session.call_tool` dispatch, not a harness-issued protocol call.

All official references use the immutable Hermes pin above. Existing UAP source
references are `hermes-native.py`, `hermes-native-lifecycle.py`,
`hermes-native-e2e.yml` and `PiHTTPMCPProbe` on the baseline main SHA.
