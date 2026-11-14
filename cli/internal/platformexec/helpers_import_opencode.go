package platformexec

type importedOpenCodeConfig struct {
	Plugins          []opencodePluginRef
	PluginsProvided  bool
	MCP              map[string]any
	MCPProvided      bool
	Commands         map[string]any
	CommandsProvided bool
	Agents           map[string]any
	AgentsProvided   bool
	DefaultAgent     string
	DefaultAgentSet  bool
	Instructions     []string
	InstructionsSet  bool
	Permission       any
	PermissionSet    bool
	Extra            map[string]any
}
