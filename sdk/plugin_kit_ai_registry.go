package pluginkitai

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/gen"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type registrarBackend struct {
	app *App
}

func (b registrarBackend) Register(platform runtime.PlatformID, event runtime.EventID, handler runtime.TypedHandler) {
	b.app.mu.Lock()
	defer b.app.mu.Unlock()
	if b.app.runDone {
		panic("plugin-kit-ai: register after Run")
	}
	b.app.handlers.Register(platform, event, handler)
}

func (b registrarBackend) RegisterCustom(rawName string, desc runtime.Descriptor, handler runtime.TypedHandler) error {
	b.app.mu.Lock()
	defer b.app.mu.Unlock()
	if b.app.runDone {
		panic("plugin-kit-ai: register after Run")
	}
	if err := b.app.custom.Register(rawName, desc); err != nil {
		return err
	}
	b.app.handlers.Register(desc.Platform, desc.Event, handler)
	return nil
}

type customRegistry struct {
	byRaw  map[string]runtime.Invocation
	byDesc map[customKey]runtime.Descriptor
}

type customKey struct {
	platform runtime.PlatformID
	event    runtime.EventID
}

func newCustomRegistry() *customRegistry {
	return &customRegistry{
		byRaw:  make(map[string]runtime.Invocation),
		byDesc: make(map[customKey]runtime.Descriptor),
	}
}

func (r *customRegistry) Register(rawName string, desc runtime.Descriptor) error {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return fmt.Errorf("custom hook name required")
	}
	if desc.Platform == "" || desc.Event == "" {
		return fmt.Errorf("custom hook descriptor requires platform and event")
	}
	if desc.Decode == nil || desc.Encode == nil {
		return fmt.Errorf("custom hook descriptor requires decode and encode")
	}
	if _, ok := gen.Lookup(desc.Platform, desc.Event); ok {
		return fmt.Errorf("custom hook %s/%s conflicts with built-in descriptor", desc.Platform, desc.Event)
	}
	if _, err := gen.ResolveInvocation([]string{"plugin-kit-ai", name}, nil); err == nil {
		return fmt.Errorf("custom hook name %q conflicts with built-in invocation", name)
	}
	rawKey := strings.ToLower(name)
	if inv, ok := r.byRaw[rawKey]; ok {
		return fmt.Errorf("custom hook name %q already registered for %s/%s", name, inv.Platform, inv.Event)
	}
	key := customKey{platform: desc.Platform, event: desc.Event}
	if _, ok := r.byDesc[key]; ok {
		return fmt.Errorf("custom hook descriptor already registered for %s/%s", desc.Platform, desc.Event)
	}
	r.byRaw[rawKey] = runtime.Invocation{Platform: desc.Platform, Event: desc.Event, RawName: name}
	r.byDesc[key] = desc
	return nil
}
