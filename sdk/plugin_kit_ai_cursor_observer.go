package pluginkitai

import (
	"context"
	"time"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime/process"
)

const cursorObserverBudget = 4 * time.Second
const cursorObserverOutputReserve = 100 * time.Millisecond

// CursorObserverIO is an optional extension of Config.IO for a context-bounded
// fixed observer response. WriteStdoutContext must honor ctx and return only
// after all its IO work has stopped. ReadStdin has the same obligation. Neither
// method may abandon a blocked goroutine. RunCursorObserver uses this extension
// when present; ordinary RunContext continues using IO.WriteStdout.
type CursorObserverIO interface {
	IO
	WriteStdoutContext(context.Context, []byte) error
}

// RunCursorObserver dispatches ONLY the beta CursorStop through the existing
// engine, discards its result/diagnostics and attempts exactly one {} plus LF.
// Decode/size/handler/middleware panic/error and cooperative cancellation stay
// neutral (exit 0) when the fixed response is written; output failure returns 1.
// It uses the earlier of ctx's deadline or four seconds, reserving 100 ms for
// output/cleanup. An already expired context receives at most 100 ms output
// grace. Cancellation stops dispatch but does not cancel the neutral response.
//
// Default process IO transfers stdin/stdout IPC ownership to this call; the
// handles are closed before return. On Linux/macOS they are made pollable and
// deadline-capable: pipes or connected anonymous UNIX stream sockets with empty
// local/peer names only, including Node/libuv socketpair stdio. Named/network
// sockets and other files are unsupported. Other hosts require pollable pipes
// or injected CursorObserverIO; unsupported/broken/full output is a no-response
// limitation.
// Config.IO is honored: arbitrary caller IO remains caller-owned and runs
// synchronously. Its read must honor ctx; its ordinary WriteStdout must return
// promptly. Non-closeable IO cannot be forcibly interrupted by this API.
//
// Callbacks/middleware must cooperate with the supplied context and never write
// process stdout. A non-cooperating callback can block this call and requires an
// external process watchdog. Forced kill cannot guarantee a response. This API
// proves a protocol observation, never task success or notification delivery.
func (a *App) RunCursorObserver(ctx context.Context) (code int) {
	engine := a.cursorObserverEngine()
	switch owned := engine.IO.(type) {
	case process.IO:
		prepared := newCursorProcessIO()
		engine.IO = prepared
		defer prepared.close()
	case *cursorPipeIO:
		defer owned.close()
	}
	deadline := time.Now().Add(cursorObserverBudget)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	workCtx, cancel := context.WithDeadline(ctx, deadline.Add(-cursorObserverOutputReserve))
	defer cancel()
	// The outer recovery includes decode/resolver/middleware construction, which
	// ordinary engine recovery deliberately does not cover. No private diagnostic
	// or buffered encoder output can escape this Cursor-local boundary.
	defer func() {
		_ = recover()
		cancel()
		now := time.Now()
		outputDeadline := now.Add(cursorObserverOutputReserve)
		if deadline.After(now) && deadline.Before(outputDeadline) {
			outputDeadline = deadline
		}
		outputCtx, stop := context.WithDeadline(context.Background(), outputDeadline)
		defer stop()
		code = writeCursorNeutral(outputCtx, engine.IO)
	}()
	if workCtx.Err() != nil {
		return 0
	}
	inv, err := engine.Resolver(engine.Args, engine.Env)
	if err != nil || inv.Platform != "cursor" || inv.Event != "stop" {
		return 0
	}
	_ = engine.Dispatch(workCtx)
	return 0
}

func (a *App) cursorObserverEngine() runtime.Engine {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runDone = true
	return runtime.Engine{
		Args: append([]string(nil), a.args...), IO: a.io, Env: a.env,
		Logger: runtime.NopLogger{}, Resolver: a.resolveInvocation, Lookup: a.lookupDescriptor,
		BuildEnvelope: process.BuildEnvelope, Handlers: a.handlers,
		Middleware: append([]runtime.Middleware{runtime.RecoveryMiddleware(runtime.NopLogger{})}, a.mws...),
	}
}

func writeCursorNeutral(ctx context.Context, hostIO IO) (code int) {
	written := false
	defer func() {
		_ = recover()
		if !written {
			code = 1
		}
	}()
	response := []byte("{}\n")
	if bounded, ok := hostIO.(CursorObserverIO); ok {
		if bounded.WriteStdoutContext(ctx, response) == nil {
			written = true
			return 0
		}
	} else if hostIO.WriteStdout(response) == nil {
		written = true
		return 0
	}
	return 1
}
