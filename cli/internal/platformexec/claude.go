package platformexec

type claudeAdapter struct{}

func (claudeAdapter) ID() string { return "claude" }
