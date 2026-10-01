# Cursor stop SDK observer

Generated from the SDK descriptors and observer contract.

Status: **public-beta**, native IDE/CLI qualification pending.

| Native event | Invocation | Carrier | Capability |
| --- | --- | --- | --- |
| `stop` | `CursorStop` | `stdin_json` | `cursor_stop` |

This is a library protocol slice only. It does not install hooks, launch a
client/model, deliver notifications, read profiles or qualify availability.
The Cursor workspace packaging target retains its existing identity and contract.
Only native stop is implemented; there is no permission or task-success event.

Import `github.com/777genius/plugin-kit-ai/sdk/cursor` and register through
`App.Cursor()`. The descriptor-generated `OnStop` method accepts a
`func(*cursor.StopEvent) *cursor.StopResponse`. For cooperative cancellation
and callback errors use `OnStopContext`:

```go
app.Cursor().OnStopContext(func(ctx context.Context, e *cursor.StopEvent) (*cursor.StopResponse, error) {
    switch e.Status {
    case cursor.StopStatusCompleted:
        // An agent loop ended. This is not proof of task success.
    case cursor.StopStatusError:
        // The native stop reported an error; do not expose private input.
    default:
        return nil, nil // Aborted/missing/null/future status: suppress effects.
    }
    // Any consumer work must honor ctx and separately enforce installed consent.
    return &cursor.StopResponse{}, nil
})
ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
defer cancel()
code := app.RunCursorObserver(ctx) // pass code to the process exit after cleanup
```

The process invocation is CursorStop (case folded), with one JSON object on
stdin, terminated by EOF. Input is capped by the public MaxPayloadBytes (1 MiB).
A pipe kept open is interrupted at the dispatch cutoff; even a syntactically
complete frame without EOF is not dispatched. Trailing JSON is rejected by the
existing encoding/json decoder. Unknown top-level fields are tolerated.

The native DTO retains conversation_id, generation_id, hook_event_name,
cursor_version, workspace_roots, model, optional model_id and model_params
(string-valued ID/value pairs), user_email, transcript_path, status and loop_count.
LoopCount, UserEmail and TranscriptPath are pointers: omission/null differs from
an explicit zero/empty value. Status strings, including future values, are never
coerced to completed. Malformed scalar types fail decoding. Native event must be
exactly stop; IDs must be nonempty, at most 256 UTF-8 bytes, without controls;
optional loop_count must be in 0..2147483647. These are observer admission bounds,
not documented native numeric limits or sender authentication. Cursor version,
workspace paths and IDs do not prove a trusted installed profile.

Nil and empty StopResponse encode as {}. No followup_message, continuation,
permission decision, context, task success or delivery receipt can be emitted.
Ordinary App.RunContext emits the normal descriptor bytes without a newline and
retains nonzero errors. Bare Stop and Notification remain Claude; CodexStop,
GeminiAfterAgent and GeminiNotification retain their existing responses.

## Fixed response and IO ownership

RunCursorObserver alone writes exactly {} plus LF once. It discards dispatch
output and SDK diagnostics, including callback/decode/middleware panic or error,
malformed/oversize input and cooperative cancellation. A successful fixed write
returns 0; a broken/full/unsupported output returns 1 and is a no-response
limitation. No response write is retried. Callback/middleware code must not write
process stdout or launch unbounded background work.

The total deadline is the earlier of the caller deadline or four seconds.
Dispatch stops 100 ms earlier, reserving time for output and cleanup. The neutral
write uses its own uncancelled context, for at most 100 ms and within the remaining
total deadline. An already expired context gets at most 100 ms final output grace.
The runner invokes callbacks synchronously: a callback that ignores its context
cannot be forcibly recovered in Go and needs an external process watchdog.
A forced kill cannot guarantee any response or delivery success.

Default App IO transfers exclusive stdin/stdout pipe ownership to this entrypoint.
On Linux/macOS, blocking inherited pipes become pollable owned duplicates; both
originals and duplicates close before return. Input and output deadlines bound
real OS pipe IO. Cancellation moves the poller deadline and joins its short
callback; there is no abandoned IO goroutine. On other systems only already
Go-deadline-capable pipes are supported; synchronous inherited Windows handles
are not qualified by this slice. File/terminal output is outside the pipe contract.

Config.IO is honored. Caller-owned IO remains caller-owned. ReadStdin must honor
its context and stop all IO before returning; a plain WriteStdout must return
promptly. Arbitrary non-closeable caller IO cannot be forcibly interrupted.
To provide cancellable output implement the public CursorObserverIO extension,
whose WriteStdoutContext has the same cleanup obligation. To inject exclusively
owned real pipes directly, use NewCursorObserverPipeIO(stdin, stdout); this
transfers two distinct pipe files, and the runner closes prepared handles before
return. Do not reuse those handles or invoke ordinary RunContext on that IO.

The S-IO consumer is an independent Go1.22 module using only public imports.
It verifies real no-EOF/oversize/malformed stdin, panic/callback error, cancelled
contexts, broken/full output, and process/goroutine/descriptor cleanup after the
runner returns. Legacy invocation checks use the same public consumer. Injected
protocol evidence is not native Cursor/client integration or rendered visibility.
