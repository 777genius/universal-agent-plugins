package gemini

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

type registryFinding = clients.RegistryFinding

const (
	registryClear         = clients.RegistryClear
	registryExpected      = clients.RegistryExpected
	registryCollision     = clients.RegistryCollision
	registryIndeterminate = clients.RegistryIndeterminate
)

const (
	geminiSkillObjectKind = "gemini_global_skill_directory"
	geminiMCPObjectKind   = "gemini_global_mcp_server"
	geminiDescriptorName  = ".agentplugins-gemini.json"
)

type geminiDescriptor struct {
	DataRoot string `json:"data_root"`
}
