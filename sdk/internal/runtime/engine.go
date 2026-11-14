package runtime

import (
	"context"
	"fmt"
)

type Engine struct {
	Args          []string
	IO            IO
	Env           Env
	Logger        Logger
	Resolver      Resolver
	Lookup        DescriptorLookup
	BuildEnvelope EnvelopeBuilder
	Handlers      *HandlerRegistry
	Middleware    []Middleware
}

func (e *Engine) Dispatch(ctx context.Context) Result {
	inv, err := e.Resolver(e.Args, e.Env)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error() + "\n"}
	}

	desc, ok := e.Lookup(inv.Platform, inv.Event)
	if !ok {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("unknown invocation: %s/%s\n", inv.Platform, inv.Event)}
	}

	env, err := e.BuildEnvelope(ctx, inv, desc.Carrier, e.Args, e.IO, e.Env)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error() + "\n"}
	}

	event, hookName, err := desc.Decode(env)
	if err != nil {
		return Result{ExitCode: 1, Stderr: err.Error() + "\n"}
	}

	handler, ok := e.Handlers.Lookup(inv.Platform, inv.Event)
	if !ok {
		return Result{ExitCode: 1, Stderr: fmt.Sprintf("no handler registered for %s/%s\n", inv.Platform, inv.Event)}
	}

	run := func(ic InvocationContext) Handled {
		return handler(ic, event)
	}
	handled := Chain(e.Middleware, run)(InvocationContext{
		Context:    ctx,
		Invocation: inv,
		Descriptor: desc,
		Env:        e.Env,
		Logger:     e.Logger,
	})
	if handled.Err != nil {
		return Result{ExitCode: 1, Stderr: handled.Err.Error() + "\n"}
	}
	res := desc.Encode(handled.Value)
	return attachMismatchWarning(res, inv.Platform, inv.RawName, hookName)
}
