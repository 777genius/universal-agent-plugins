package platformmeta

func toolingProfiles() []PlatformProfile {
	return []PlatformProfile{
		geminiProfile(),
		cursorProfile(),
		cursorWorkspaceProfile(),
		opencodeProfile(),
		grokProfile(),
		kimiProfile(),
	}
}
