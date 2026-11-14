package codex

import (
	"encoding/json"
	"fmt"
	"strings"

	internalcodex "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/codex"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// RegisterCustomJSON registers an experimental future Codex hook whose payload
// is delivered as a JSON argv argument. The handler remains fully typed.
func RegisterCustomJSON[T any](r *Registrar, eventName string, fn func(*T) *Response) error {
	name := strings.TrimSpace(eventName)
	if name == "" {
		return fmt.Errorf("custom hook name required")
	}
	return r.backend.RegisterCustom(name, runtime.Descriptor{
		Platform: "codex",
		Event:    runtime.EventID(name),
		Carrier:  runtime.CarrierArgvJSON,
		Decode: func(env runtime.Envelope) (any, string, error) {
			if len(env.Args) < 3 {
				return nil, "", fmt.Errorf("decode codex %s input: missing JSON payload argument", runtime.NormalizeHookName(name))
			}
			raw := strings.TrimSpace(env.Args[2])
			if raw == "" {
				return nil, "", fmt.Errorf("decode codex %s input: empty JSON payload argument", runtime.NormalizeHookName(name))
			}
			var dto T
			if err := json.Unmarshal([]byte(raw), &dto); err != nil {
				return nil, "", fmt.Errorf("decode codex %s input: %w", runtime.NormalizeHookName(name), err)
			}
			return &dto, name, nil
		},
		Encode: func(v any) runtime.Result {
			out, ok := v.(internalcodex.NotifyOutcome)
			if !ok {
				return runtime.Result{ExitCode: 1, Stderr: fmt.Sprintf("encode codex %s response: internal outcome type mismatch\n", runtime.NormalizeHookName(name))}
			}
			return internalcodex.EncodeNotifyOutcome(out)
		},
	}, func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*T)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("codex " + name)}
		}
		_ = fn(ev)
		return runtime.Handled{Value: internalcodex.NotifyOutcome{}}
	})
}
