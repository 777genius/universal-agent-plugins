package qoder

import (
	"fmt"
	"strings"

	internalqoder "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/qoder"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// RegisterCustomJSON registers an experimental future Qoder hook whose payload
// is delivered as stdin JSON (Claude-style hook protocol). The handler remains
// fully typed and responds with the shared context response family.
func RegisterCustomJSON[T any](r *Registrar, eventName string, fn func(*T) *ContextResponse) error {
	name := strings.TrimSpace(eventName)
	if name == "" {
		return fmt.Errorf("custom hook name required")
	}
	return r.backend.RegisterCustom(name, runtime.Descriptor{
		Platform: "qoder",
		Event:    runtime.EventID(name),
		Carrier:  runtime.CarrierStdinJSON,
		Decode: func(env runtime.Envelope) (any, string, error) {
			dto, err := runtime.DecodeJSONPayload[T](env.Stdin, runtime.NormalizeHookName(name)+" input")
			if err != nil {
				return nil, "", err
			}
			return dto, name, nil
		},
		Encode: func(v any) runtime.Result {
			return internalqoder.EncodeContext(v, runtime.NormalizeHookName(name), name)
		},
	}, func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*T)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("qoder " + name)}
		}
		return runtime.Handled{Value: contextOutcomeFromResponse(fn(ev))}
	})
}
