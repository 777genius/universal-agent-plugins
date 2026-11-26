package agentpluginscli

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Browser and terminal project the same safe detection facts. Unavailable
// entries explain omissions without ever becoming selectable options.
func skillsTargetSelectionRequest(targets []skillspicker.Target) prompt.TargetSelectionRequest {
	request := prompt.TargetSelectionRequest{}
	for _, target := range targets {
		id := domain.ClientID(target.Id)
		if target.Available {
			request.Choices = append(request.Choices, prompt.TargetChoice{ID: id, Label: target.Label, Note: target.Scope, Info: target.DisplayPath})
			request.DefaultIDs = append(request.DefaultIDs, id)
			continue
		}
		skipped := prompt.SkippedTarget{ID: id, Label: target.Label, Target: publicCLITargetName(id), Kind: prompt.SkipOther}
		if target.Reason != nil {
			skipped.Reason = *target.Reason
		}
		request.Skipped = append(request.Skipped, skipped)
	}
	return request
}
