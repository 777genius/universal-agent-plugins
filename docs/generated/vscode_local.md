# VS Code Local SDK observers

Generated from SDK descriptors and the Local observer contract.

| Runtime platform | Native event | Invocation | Maturity | Carrier | Capability |
| --- | --- | --- | --- | --- | --- |
| `vscode-local` | `Stop` | `VSCodeLocalStop` | public-beta | `stdin_json` | `vscode_local_stop` |
| `vscode-local` | `SubagentStop` | `VSCodeLocalSubagentStop` | public-beta | `stdin_json` | `vscode_local_subagent_stop` |

This optional library API observes the VS Code **Local** harness. Stop means
execution is **about to stop**. It does not establish completed work, success,
idle state, a root session or the outcome of later hooks. SubagentStop observes
a subagent about to stop; custom-agent Stop hooks can run as SubagentStop.
Agent Notifications does not automatically notify for SubagentStop.

Runtime identity is `vscode-local`, installer identity remains
`vscode`, and Agent Notifications identity is `copilot-vscode`.
Copilot CLI, Agent Host, cloud agents and other harnesses have separate wires.
The profile namespace and installed notification transport belong to later
adapter/integration slices. Neither this API nor packaging support proves an
installed Local integration, native availability, Windows/macOS execution,
visible notifications or executable release qualification.

Import `github.com/777genius/plugin-kit-ai/sdk/vscodelocal` and use the
existing engine:

```go
app := pluginkitai.New(pluginkitai.Config{
    Args: []string{"TEST-observer", "VSCodeLocalStop"},
    IO: capturedIO, Env: emptyEnv,
})
app.VSCodeLocal().OnStop(func(e *vscodelocal.StopEvent) *vscodelocal.StopResponse {
    if e.HookEventName != "Stop" || e.StopHookActive == nil || *e.StopHookActive {
        return nil // unknown/recursive/mismatched event: suppress effects
    }
    // Explicit false is an observation, not delivery consent or completion.
    // Validate timestamp and installed identity separately before any effect.
    return &vscodelocal.StopResponse{}
})
app.VSCodeLocal().OnSubagentStop(func(e *vscodelocal.SubagentStopEvent) *vscodelocal.SubagentStopResponse {
    return nil // typed separation; no automatic notification
})
code := app.RunContext(ctx)
```

## Wire and admission

Source binding: official VS Code **1.140.0**, commit
`07f806f999227108933c2e30515b26eecc1fda74`, bundled Copilot Chat source
`extensions/copilot/src/extension/chat/vscode-node/chatHookService.ts`,
plus the [Local hooks reference](https://code.visualstudio.com/docs/agents/reference/hooks-reference)
updated 2026-09-30. The pinned service supplies timestamp via toISOString(),
exact hook_event_name, optional session_id and optional transcript_path.
The documented optional cwd is retained; native process working directory is
not evidence that a cwd property is present in the input.
Stop adds stop_hook_active; SubagentStop adds agent_id and agent_type.
No CLI/Gemini/Claude DTO is reused. No model, result, root or turn ID is inferred.

Timestamp stays a string, without parsing/normalization. Common optional string
fields decode absent/null to empty strings; session_id is never required.
StopHookActive is a pointer: absent/null remains nil, explicit false remains a
non-nil false, and true remains true. Consumers validate timestamp, native event
and observation facts. Unknown fields are tolerated by the existing JSON decoder;
wrong scalar types and trailing JSON return the normal SDK error. There is no
second strict JSON framework or duplicate-key policy.

Fixed invocation names are case folded, while native event names retain their
exact spelling. Argv selects the callback. A different nonempty native name
remains visible and yields the existing mismatch warning; it is not silently
changed to the selector. Consumers must suppress mismatches and missing names.
Bare Stop/SubagentStop remain Claude, and Codex/Gemini/Cursor aliases stay intact.

Input uses stdin JSON through the shared 1 MiB MaxPayloadBytes bound, including
when injected IO returns oversized bytes. Nil and empty responses serialize
exactly {} (ordinary RunContext adds no newline). Public responses have no
fields for decision, reason, continue, system messages, model control, updated
input or context. There is no blocking/permission API in this observer subset.

## Process and qualification limits

RunContext retains ordinary nonzero decode, callback/middleware and IO errors,
including permission exit 2 on the existing permission descriptors. It does not
promise fail-open behavior or bounded interruption of arbitrary blocking IO.
The consumer must pre-read/capture bounded input and own its transport deadlines,
cleanup and redacted diagnostics. No RunVSCodeLocalObserver supervisor is added.
RunCursorObserver and its owned-pipe/SIGPIPE behavior remain CursorStop-only.

The supplied public TEST provider packet reports one genuine native Local Stop
on isolated Linux VS Code 1.140.0, with explicit false and a session identifier.
That source/input seam evidence predates this SDK implementation. Its captured
facts are not a raw input fixture or proof of this executable, installed AN,
restart/revocation, cloud authentication, desktop visibility or complete support.
The independent SDK consumer is injected_contract evidence only. Native
qualification ranges and executable release availability are not promoted here.
