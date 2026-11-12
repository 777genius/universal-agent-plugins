# Grok Project MCP without a model — bounded E2E follow-up

Date: 2026-11-12. Base: `9ef8627e4d18f6a2ff1fd0d6be3b98a84d9b5f81`.

## Owner request and boundary

Follow up merged #810 with actual MCP protocol traffic, without buying a
subscription, providing an API key or invoking a model. Prior proof covers
prepared installation, native discovery and removal, not MCP execution.

The installer must continue to report Prepared. This is test infrastructure,
not permission to grant trust in a real user profile, enable plugins during
installation, publish a release, widen version admission or change the site.

## Acceptance, with distinct authorities

1. Build the ordinary UAP CLI and the existing credential-free native MCP
   probe. Acquire the exact official Grok 1.0.70 through the existing pinned
   acquisition implementation; retain executable/archive identity.
2. Use a new private HOME and Git project A/B under a disposable runner root.
   Never read/copy an ambient login, token, credential store or config. Pass a
   minimal environment with no provider credentials; invoke no model command.
3. Install the probe as a real Project package through the UAP CLI. Read the
   held ownership receipt and the installed native `.mcp.json`; do not bypass
   UAP by independently writing Grok's native MCP configuration.
4. Verify the untrusted/disabled negative first: native discovery sees the
   prepared plugin but cannot start its MCP process. Only then seed explicit
   test-owned folder trust and plugin enablement in the disposable HOME, as
   existing Grok native fixtures already do. UAP must not make these changes.
5. Run real `grok mcp doctor --json` and require original probe events showing
   initialize and tools/list from that process. Neither a doctor exit code nor
   a generated configuration alone constitutes protocol success.
6. Independently exercise initialize, initialized notification, tools/list and
   tools/call against the exact installed stdio command/environment. Validate
   request IDs, the unique run nonce, tool result and plugin/data paths.
   This proves a real MCP call from installed configuration, NOT a Grok-model
   tools/call. Preserve this distinction in both evidence and documentation.
7. Repeat after update and owned repair; verify no-op reinstallation, project B
   and user sentinels, exact owned removal and absence in native discovery.
   Negative probes must reject a bad nonce or stale/foreign installed path.
8. Prefer existing local stdio fixture and transport helpers. Add HTTP coverage
   only with an existing reusable loopback server, not a second test platform.
   Do not claim HTTP execution from a stdio-only proof.

## Observed repair contract

Original hosted runs at `6fa8f03` and `00103e7` proved native doctor and direct
installed-config MCP after installation and update, but failed an incorrect
expectation that repair may overwrite a partially edited package. The existing
`NativeIdentityObserver.verifyOwnedPackage` intentionally refuses a mismatched
package digest; native inspect itself returned valid output with zero MCP servers.

Preserve this safety contract. Delete only the held test sidecar and require the
exact ownership refusal with unchanged state, remaining package, source and data.
Then the test moves its own verified artifact into a fresh private quarantine
(outside native discovery), creating the independently supported missing-package
fault. Ordinary UAP repair must rebuild the exact package from its pinned source;
repeat native doctor and direct MCP calls afterward. Keep the quarantined bytes
and user data unchanged. This is not automatic adoption of partial user changes.

## Architecture and implementation

Keep changes in focused test scripts/fixtures and a path-scoped GitHub Actions
job. Reuse native acquisition, bounded process capture and the existing MCP
probe. No product adapter changes unless an actual observed bug requires one;
then fix it at the existing client boundary with a regression test.

Expected size: roughly 800–1,100 test/workflow lines, zero production lines absent
an observed defect. Complexity 5/10; initial confidence 7/10 until native output
is observed. No new third-party runtime dependency or feature flag.

## Review passes

- Pass 1, scope and architecture: separate native doctor proof from direct MCP
  calls; preserve #810 and its evidence; no paid/provider workflow.
- Pass 2, safety and falsifiability: explicit owned fixture trust only, clean
  environment, bounded child processes, held receipt/config identity, original
  request/response evidence and negatives. Never change a failed assertion
  merely to label a native/authentication failure successful.
- Before merge: review final diff and external review findings, exact-head
  focused native E2E plus applicable Required/security gates, SHA-guarded merge,
  and verify postmerge checks. Queued/skipped work is not a passing test.

## Primary references

- https://docs.x.ai/build/features/mcp-servers (`mcp doctor` and project scope)
- `repotests/agentplugins_grok_cycle_fixture_test.go` (owned fixture trust)
- `repotests/testdata/agentplugins_native_probe/main.go` (no-model MCP probe)
- `scripts/project_native_research.py` (pinned acquisition)
- `docs/ARCHITECTURE.md` and `CONTRIBUTING.md` (unchanged repository gates)
