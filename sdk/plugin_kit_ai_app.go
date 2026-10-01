package pluginkitai

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/claude"
	"github.com/777genius/plugin-kit-ai/sdk/codex"
	"github.com/777genius/plugin-kit-ai/sdk/cursor"
	"github.com/777genius/plugin-kit-ai/sdk/gemini"
	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/gen"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime/process"
)

// Use appends middleware that wraps all subsequent handler dispatch.
func (a *App) Use(mw Middleware) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.runDone {
		panic("plugin-kit-ai: Use after Run")
	}
	if mw == nil {
		return
	}
	a.mws = append(a.mws, mw)
}

// Claude returns a registrar for Claude-specific hook handlers.
func (a *App) Claude() *claude.Registrar {
	return claude.NewRegistrar(registrarBackend{app: a})
}

// Codex returns a registrar for Codex-specific event handlers.
func (a *App) Codex() *codex.Registrar {
	return codex.NewRegistrar(registrarBackend{app: a})
}

// Gemini returns a registrar for Gemini-specific hook handlers.
func (a *App) Gemini() *gemini.Registrar {
	return gemini.NewRegistrar(registrarBackend{app: a})
}

// Cursor returns a registrar for the beta native stop observer.
func (a *App) Cursor() *cursor.Registrar {
	return cursor.NewRegistrar(registrarBackend{app: a})
}

// Run dispatches the current process invocation with context.Background().
func (a *App) Run() int {
	return a.RunContext(context.Background())
}

// RunContext dispatches the current process invocation using the supplied context.
func (a *App) RunContext(ctx context.Context) int {
	a.mu.Lock()
	args := append([]string(nil), a.args...)
	io := a.io
	env := a.env
	logger := a.logger
	mws := append([]runtime.Middleware(nil), a.mws...)
	handlers := a.handlers
	a.runDone = true
	a.mu.Unlock()

	engine := runtime.Engine{
		Args:          args,
		IO:            io,
		Env:           env,
		Logger:        logger,
		Resolver:      a.resolveInvocation,
		Lookup:        a.lookupDescriptor,
		BuildEnvelope: process.BuildEnvelope,
		Handlers:      handlers,
		Middleware:    append([]runtime.Middleware{runtime.RecoveryMiddleware(logger)}, mws...),
	}
	res := engine.Dispatch(ctx)
	if res.Stderr != "" {
		_ = io.WriteStderr(res.Stderr)
	}
	if res.ExitCode == 0 && len(res.Stdout) > 0 {
		if err := io.WriteStdout(res.Stdout); err != nil {
			_ = io.WriteStderr(err.Error() + "\n")
			return 1
		}
	}
	return res.ExitCode
}

func (a *App) resolveInvocation(args []string, env runtime.Env) (runtime.Invocation, error) {
	if inv, err := gen.ResolveInvocation(args, env); err == nil {
		return inv, nil
	}
	if len(args) < 2 {
		return runtime.Invocation{}, fmt.Errorf("usage: <binary> <hookName>")
	}
	raw := args[1]
	a.mu.Lock()
	defer a.mu.Unlock()
	if inv, ok := a.custom.byRaw[strings.ToLower(raw)]; ok {
		inv.RawName = raw
		return inv, nil
	}
	return runtime.Invocation{}, fmt.Errorf("unknown invocation %q", raw)
}

func (a *App) lookupDescriptor(platform runtime.PlatformID, event runtime.EventID) (runtime.Descriptor, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if desc, ok := a.custom.byDesc[customKey{platform: platform, event: event}]; ok {
		return desc, true
	}
	return gen.Lookup(platform, event)
}
