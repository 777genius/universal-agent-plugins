package platformexec

type codexPackageAdapter struct{}
type codexRuntimeAdapter struct{}

func (codexPackageAdapter) ID() string { return "codex-package" }
func (codexRuntimeAdapter) ID() string { return "codex-runtime" }
