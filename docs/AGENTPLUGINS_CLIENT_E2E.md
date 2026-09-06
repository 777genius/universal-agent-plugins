# Agent Plugins client E2E evidence

This record binds the public client-support claims to one exact, disposable
macOS run. It is lifecycle and client-discovery evidence, not a browser tool,
model, OAuth, or login runtime claim.

## Candidate identity

| Field | Value |
| --- | --- |
| Date | 2026-08-30 |
| Installer source | `5630ccd92aa91c8ac8cafb37eea8752fd82edce0` |
| Installer tree | `cf13cbe2f64ae09d93ad34bfc6047fe99d5ca845` |
| Installer version | `0.1.22` |
| Installer binary SHA-256 | `8f417cea031d42b07badbe1b2a37dcd53deb2e5804f99d668f733309ecb4022b` |
| Package source | `ChromeDevTools/chrome-devtools-mcp@cb39d1d835c3baa3eff87501cd8c1de020604789` |
| Package version | `1.8.0` |
| Package tree digest | `sha256:3bd47ccd3f990a6fdd8d3e2fa3dac48ac460a9043e0ccf0c5e14522fb4c472ea` |
| Package manifest digest | `sha256:b34a4dcd71cd536a7f5a3a51d76d53ae5af3d0ce0f18783e71c7f01da865b867` |
| Platform | macOS 15.6.1, arm64, Node.js 24.18.0 |
| Structured transcript | [`evidence/agentplugins-client-e2e-2026-08-30.json`](evidence/agentplugins-client-e2e-2026-08-30.json) |
| Transcript SHA-256 | `437da1bc7423a85b231be139ff9bfbd7e89c942ef216a61ebde668c08a9c2ee3` |

The run used fresh temporary `HOME`, XDG, Claude, Gemini, Cline, and
agentplugins state directories. It did not read or mutate an agent project or
the user's client configuration. A separate read-only `doctor` inventory
confirmed that all five selected client families were present on the host.

## Lifecycle proof

The same exact package acquisition was applied to all five clients:

```bash
agentplugins add ChromeDevTools/chrome-devtools-mcp@cb39d1d835c3baa3eff87501cd8c1de020604789 \
  --target claude,gemini,opencode,cline,windsurf \
  --format json
```

The operation produced one installation ID and reported all five targets as
installed against the same tree and manifest digests. The following client
checks were then run inside the disposable profile.

| Client | Exact check and result |
| --- | --- |
| Claude Code 2.1.205 | `claude plugin list --json` returned the enabled `chrome-devtools` package, its six skills, and the `chrome-devtools` MCP server. |
| Gemini CLI 0.36.0 | `gemini mcp list` returned the configured `chrome-devtools` stdio server; `gemini skills list` discovered all six skills. The MCP process was `Disconnected`, so browser/runtime execution is not claimed. |
| OpenCode 1.18.4 | `opencode debug config` returned the exact managed MCP command, working directory, `PLUGIN_ROOT`, and `PLUGIN_DATA`. Runtime tool execution is not claimed. |
| Cline | The isolated native MCP settings contained the exact managed server and all six skills were projected. The real host detector found the installed Cursor extension/config surfaces; Cline runtime and login were not started. |
| Windsurf / Devin | The isolated legacy MCP settings contained the exact managed server. The real host detector found existing Devin/Windsurf configuration surfaces; skills remained prepared-only and runtime/login were not started. |

`doctor --format json` completed read-only and found no projection drift. Its
only findings were the expected `authentication_not_checked` notices because
this run intentionally performed no login or OAuth flow.

An immutable full-SHA installation deliberately has no update channel. The
multi-target update failed during preflight with no mutation and directed the
user to `switch --to` with a new reviewed SHA. A repair then reacquired the
exact recorded revision once and completed for every client:

```bash
agentplugins repair chrome-devtools \
  --target claude,gemini,opencode,cline,windsurf \
  --format json
```

The transcript records the exact update preflight and confirms the installation
was unchanged. Directory-backed updates are a separate release/publication
canary and are not inferred from this immutable-source run.

Finally, one multi-target remove succeeded for all five clients. Post-removal
checks proved:

- `agentplugins list` returned no installations;
- Claude Code returned an empty plugin list;
- Gemini CLI reported no MCP servers and no skills;
- OpenCode returned an empty MCP map;
- Cline and Windsurf MCP maps were empty;
- the projected Gemini and Cline skill files were gone.

Plugin data was retained by default, matching the CLI's documented safe-remove
contract. No browser, model, tool call, consent screen, or OAuth session was
used in this evidence run.

Compatibility work dated 2026-09-06 is tracked separately in
[release preparation](AGENTPLUGINS_RELEASE_PREPARATION.md). This historical
transcript does not prove the unreleased candidate's cwd or transport behavior.
