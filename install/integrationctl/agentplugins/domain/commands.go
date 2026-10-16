package domain

// ExtensionNamespace is the package directory reserved for agentplugins'
// own content. Authors write CommandsDir and LockedRuntimeDir below it; the
// installer writes ManagedStdioDir into the staged copy.
const (
	ExtensionNamespace = "io.github.777genius.agentplugins"
	CommandsDir        = "commands"
	LockedRuntimeDir   = "runtime"
	ManagedStdioDir    = "managed-stdio-v1"
)
