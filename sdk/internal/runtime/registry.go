package runtime

import "sync"

type handlerKey struct {
	platform PlatformID
	event    EventID
}

type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[handlerKey]TypedHandler
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{handlers: make(map[handlerKey]TypedHandler)}
}

func (r *HandlerRegistry) Register(platform PlatformID, event EventID, handler TypedHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[handlerKey{platform: platform, event: event}] = handler
}

func (r *HandlerRegistry) Lookup(platform PlatformID, event EventID) (TypedHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[handlerKey{platform: platform, event: event}]
	return h, ok
}

// HasPlatform reports whether any handler is registered for the platform.
func (r *HandlerRegistry) HasPlatform(platform PlatformID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for key := range r.handlers {
		if key.platform == platform {
			return true
		}
	}
	return false
}
