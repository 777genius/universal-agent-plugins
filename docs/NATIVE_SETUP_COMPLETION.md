# Native setup completion

A confirmed add/update/repair operation can complete supported native setup
beyond file delivery. The existing review displays the exact project trust or
selected MCP approval that will persist. It also discloses when a native command
starts other enabled MCP servers. There is no second confirmation flow.

Repeating an unchanged Skills selection refreshes the observed installation
result. Plain saved status remains host-free. The shared use-case capability
uses the existing repair operation only for an owned, verified, allowed Prepared
installation with certain native effects. It checks a fresh immutable preview
and checks again under the existing lock; it does not restage source content or
introduce another transaction protocol. Clients opt in, and other clients retain
their existing behavior. An unclosed probe stops subsequent client mutations.

| Client | Observation and automatic setup | Limit |
| --- | --- | --- |
| Codex | Raw Skills use finite native discovery without changing project trust or reading configuration. Selected project MCP setup reviews exact project trust, observes effective app-server configuration, and verifies owned Skills and selected stdio MCP status. Native user plugins require exact identity and version. | Selected-server MCP observation requires stable app-server version 0.162.0 or newer, observed in the same process. Older, unknown and prerelease versions cannot use this capability. No model turn or downstream service authentication is established. |
| Claude Code | Native marketplace registration, exact scoped plugin/cache identity, reviewed project trust and selected MCP approval, and native Connected health | Explicit disablement, ambiguous same-name plugins and managed policy remain authoritative. |
| Gemini | Reviewed exact project trust, native Skill paths/content and selected MCP inventory | Native MCP inspection can start all enabled servers in the selected profile/project. Review discloses this; verify-only inspection does not connect them. Qualification applies to observed native schemas. |
| OpenCode | Effective config, exact Skill content and selected local MCP connection in user/project scope | The observer requires an immutable qualified V1 profile. Its normal MCP command can start all enabled profile/project servers and is disclosed in review. Verify-only inspection does not connect them. V2 and unknown profiles cannot run V1 commands. Connection evidence does not establish a tool call. |
| Cursor | Exact selected MCP fingerprint approval and native server/tool inventory | The qualified approval schema is version-gated; unresolved interpolation stays Prepared. Skill/plugin discovery requires a signed-in native session. |

Trust and approval preferences survive plugin removal. Updates preserve unrelated
settings through locked conditional file transactions. A failed write or rollback
retains its observed native effect; an uncertain effect cannot be reported as
unchanged or followed by another native probe. There are two exceptions, both
for the Claude Code project commands (`plugin marketplace add`, `plugin install`,
`plugin uninstall`, `plugin marketplace remove`) and the `plugin list` that
verifies them:

- A failed or interrupted command whose whole process tree was proven closed:
  the adapter may read the client's own registry once, read-only, and report
  only an exactly recognized state. This also covers a verifying `plugin list`
  that completed but did not confirm the exact install, including after an
  install that had no effect: it no longer blocks by itself. The binding is
  marked failed, and the attempt is closed only when every owned receipt read
  back is exact. Unproven process cleanup, a failed read or an unrecognized
  state stay uncertain and keep blocking the next mutation.
- An attempt left by a killed agentplugins process (SIGKILL, OOM, power loss),
  confirmed by an explicit `repair` or `remove` with `--yes` (`--dry-run`
  previews the same checks and writes nothing). Here the process tree is not
  proven closed. Under the mutation lock the confirmation runs one more
  read-only probe after a process census that is a heuristic, not a proof: on
  macOS and Linux it refuses while a live process of the user looks like Claude
  Code and runs a `plugin` command, carries the same
  `AGENTPLUGINS_CLAUDE_PROFILE` marker (or, from earlier versions, the same
  `CLAUDE_CONFIG_DIR`), or hides both its arguments and environment; it
  recognizes no other process. On
  Windows it relies on the kill-on-close job that ends every command with its
  invocation and lists no processes. Only an exactly recognized state is
  accepted, and it is never recorded as unchanged: the effect is committed with
  the recognized ownership, the binding is marked failed, and the ordinary
  command then finishes. An unrecognized state needs manual recovery.

Cursor writes only the approval store and never invokes native `mcp enable`,
which can clear explicit disablement.
Existing deny settings are preserved and checked again before readiness is
reported. Unknown schemas, foreign ownership and uncertain cleanup never become
successful activation.

Fresh native discovery/connection can qualify local stdio authentication as not
required. It does not erase explicit pending/failed authentication or replace a
catalog requirement. Setup does not execute arbitrary plugin tools; deterministic
tool calls belong to isolated native E2E fixtures.

Terminal and browser results report the actual per-item installation outcome.
Verified results use neutral installation wording. Remaining manual actions are
sanitized, deduplicated and attributed to the corresponding agent, including
partial failures. A generic doctor suggestion is used only when no concrete
action is available.

Qualification uses disposable profiles and projects without vendor API keys or
remote provider calls; isolated tool-call fixtures may use a local scripted
model. Native discovery, MCP initialization and fixture tool calls are distinct
evidence. Cursor's login boundary remains visible. Historical pinned
CI evidence and released-client availability labels retain their original meaning.
Source review alone does not qualify a reconstructed candidate: full core, lint,
native lifecycle and visual checks must run on the published commit.
