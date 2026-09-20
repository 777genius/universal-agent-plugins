package scaffold

import "strings"

const (
	RuntimeGo     = "go"
	RuntimePython = "python"
	RuntimeNode   = "node"
	RuntimeShell  = "shell"
)

var supportedRuntimes = map[string]struct{}{
	RuntimeGo:     {},
	RuntimePython: {},
	RuntimeNode:   {},
	RuntimeShell:  {},
}

func LookupRuntime(name string) (string, bool) {
	name = normalizeRuntime(name)
	_, ok := supportedRuntimes[name]
	return name, ok
}

func normalizeRuntime(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return RuntimeGo
	}
	return name
}

func defaultExecutionMode(runtime string) string {
	if runtime == RuntimeGo {
		return "direct"
	}
	return "launcher"
}
