package claude

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func wrapClaudeHandler[T any, R any](name string, fn func(*T) R, mapResponse func(R) any) runtime.TypedHandler {
	return func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*T)
		if !ok {
			return runtime.Handled{Err: internalclaudeTypeMismatch("claude " + name)}
		}
		return runtime.Handled{Value: mapResponse(fn(ev))}
	}
}
