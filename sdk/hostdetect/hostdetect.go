package hostdetect

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// Platform identifies a supported host product.
type Platform string

const (
	// PlatformUnknown is the fail-closed result when no signal matches.
	PlatformUnknown Platform = ""
	// PlatformClaude identifies Claude Code.
	PlatformClaude Platform = "claude"
	// PlatformCodex identifies the Codex CLI.
	PlatformCodex Platform = "codex"
)

// Env abstracts environment lookups so callers can inject fakes in tests.
type Env interface {
	LookupEnv(string) (string, bool)
}

// Signal describes how one platform is recognized. Earlier registry entries
// win when more than one signal matches.
type Signal struct {
	// Platform is the detection result when this signal matches.
	Platform Platform
	// EnvMarkers are environment variable names whose presence identifies
	// the platform. A marker must be set (present) to match; its value is
	// not inspected.
	EnvMarkers []string
	// PayloadSniff inspects only top-level keys of the decoded JSON payload.
	// It must not perform typed platform decoding.
	PayloadSniff func(map[string]any) bool
}

// Registry is an ordered signal list evaluated tier by tier.
type Registry []Signal

// Detect resolves the invoking platform.
//
// A non-empty override must name a platform present in the registry;
// an unknown override is an error rather than an arbitrary Platform.
// Without an override, env markers are checked in registry order, then
// payload sniffing runs on bounded top-level JSON. When nothing matches,
// Detect returns PlatformUnknown with a nil error.
func Detect(registry Registry, override string, env Env, payload []byte) (Platform, error) {
	if p := strings.TrimSpace(override); p != "" {
		for _, sig := range registry {
			if sig.Platform != PlatformUnknown && strings.EqualFold(string(sig.Platform), p) {
				return sig.Platform, nil
			}
		}
		return PlatformUnknown, fmt.Errorf("unknown product override %q", override)
	}

	if env != nil {
		for _, sig := range registry {
			for _, marker := range sig.EnvMarkers {
				if _, ok := env.LookupEnv(marker); ok {
					return sig.Platform, nil
				}
			}
		}
	}

	if top, ok := decodeTopLevel(payload); ok {
		for _, sig := range registry {
			if sig.PayloadSniff != nil && sig.PayloadSniff(top) {
				return sig.Platform, nil
			}
		}
	}

	return PlatformUnknown, nil
}

// decodeTopLevel decodes a bounded JSON object into its top-level keys.
// Oversized, empty, malformed, or non-object payloads yield no sniff input.
func decodeTopLevel(payload []byte) (map[string]any, bool) {
	if len(payload) == 0 || len(payload) > runtime.MaxPayloadBytes {
		return nil, false
	}
	var top map[string]any
	if err := json.Unmarshal(payload, &top); err != nil {
		return nil, false
	}
	return top, true
}
