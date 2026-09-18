package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// Aliases keep in-package parser tests compiling after the bodies moved into
// client packages. Production code uses the adapters through the registry.
type (
	codexStatus   = codex.ListStatus
	claudeStatus  = claude.ListStatus
	copilotStatus = shared.CopilotStatus
)

const (
	copilotLiveHeader     = shared.CopilotLiveHeader
	codexStatusUnknown    = codex.StatusUnknown
	codexStatusInstalled  = codex.StatusInstalled
	codexStatusAbsent     = codex.StatusAbsent
	claudeStatusUnknown   = claude.StatusUnknown
	claudeStatusInstalled = claude.StatusInstalled
	claudeStatusAbsent    = claude.StatusAbsent
	claudeStatusCollision = claude.StatusCollision
)

func codexPluginStatus(body []byte, name, marketplace string) codexStatus {
	return codex.PluginStatusFromList(body, name, marketplace)
}

func claudePluginStatus(body []byte, name, activePath string) claudeStatus {
	return claude.PluginStatusFromList(body, name, activePath)
}
