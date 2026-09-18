package cline

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

const (
	clineSkillObjectKind = "cline_global_skill_directory"
	clineMCPObjectKind   = "cline_global_mcp_server"
	clineProjectionFile  = ".agentplugins-cline-native.json"
)

type clineProjection struct {
	Servers map[string]nativeconfig.Server `json:"servers"`
}
