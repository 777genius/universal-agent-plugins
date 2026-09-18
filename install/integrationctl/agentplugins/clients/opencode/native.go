package opencode

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

const (
	openCodeProjectionFile = ".agentplugins-opencode.json"
	openCodeMCPObjectKind  = "opencode_global_mcp_server"
	openCodeSkillKind      = "opencode_global_skill_directory"
)

type openCodeProjection struct {
	ResolvedCWD map[string]bool                `json:"resolved_cwd,omitempty"`
	Version     int                            `json:"version"`
	ConfigPath  string                         `json:"config_path"`
	ConfigJSON  string                         `json:"config_json"`
	ConfigJSONC string                         `json:"config_jsonc"`
	PackageRoot string                         `json:"package_root"`
	DataRoot    string                         `json:"data_root,omitempty"`
	MCPServers  map[string]nativeconfig.Server `json:"mcp_servers,omitempty"`
}
