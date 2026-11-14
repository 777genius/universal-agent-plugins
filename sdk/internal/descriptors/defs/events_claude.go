package defs

func claudeEvents() []EventDescriptor {
	events := make([]EventDescriptor, 0,
		len(claudeCoreEvents())+
			len(claudeSessionEvents())+
			len(claudeToolEvents())+
			len(claudeAgentEvents())+
			len(claudeWorkspaceEvents()),
	)
	events = append(events, claudeCoreEvents()...)
	events = append(events, claudeSessionEvents()...)
	events = append(events, claudeToolEvents()...)
	events = append(events, claudeAgentEvents()...)
	events = append(events, claudeWorkspaceEvents()...)
	return events
}
