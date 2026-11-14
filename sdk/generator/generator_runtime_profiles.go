package generator

import (
	"sort"

	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/defs"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func runtimeProfiles(m model) []defs.PlatformProfile {
	var out []defs.PlatformProfile
	for _, p := range sortedProfiles(m) {
		if p.Status == runtime.StatusRuntimeSupported {
			out = append(out, p)
		}
	}
	return out
}

func sortedProfiles(m model) []defs.PlatformProfile {
	var out []defs.PlatformProfile
	for _, p := range m.profiles {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Platform < out[j].Platform
	})
	return out
}

func eventsForPlatform(m model, platform runtime.PlatformID) []defs.EventDescriptor {
	var out []defs.EventDescriptor
	for _, e := range m.events {
		if e.Platform == platform {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Event < out[j].Event
	})
	return out
}
