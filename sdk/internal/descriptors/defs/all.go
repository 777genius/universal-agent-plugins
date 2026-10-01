package defs

import "github.com/777genius/plugin-kit-ai/sdk/platformmeta"

func Profiles() []PlatformProfile {
	source := platformmeta.All()
	out := make([]PlatformProfile, 0, len(source))
	for _, profile := range source {
		if profile.ID == "codex-package" {
			continue
		}
		out = append(out, adaptProfile(profile))
	}
	return out
}

func Events() []EventDescriptor {
	events := make([]EventDescriptor, 0, len(claudeEvents())+len(geminiEvents())+len(codexEvents())+len(cursorEvents()))
	events = append(events, claudeEvents()...)
	events = append(events, geminiEvents()...)
	events = append(events, codexEvents()...)
	events = append(events, cursorEvents()...)
	return events
}
