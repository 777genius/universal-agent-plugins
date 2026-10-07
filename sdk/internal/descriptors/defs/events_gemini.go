package defs

func geminiEvents() []EventDescriptor {
	events := make([]EventDescriptor, 0, len(geminiSessionEvents())+len(geminiModelEvents())+len(geminiAgentToolEvents())+len(geminiNotificationEvents()))
	events = append(events, geminiSessionEvents()...)
	events = append(events, geminiModelEvents()...)
	events = append(events, geminiAgentToolEvents()...)
	events = append(events, geminiNotificationEvents()...)
	return events
}
