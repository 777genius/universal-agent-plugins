package windsurf

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

const windsurfMCPObjectKind = "windsurf_mcp_entry"
