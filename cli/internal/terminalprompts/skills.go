package terminalprompts

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/installerui"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

func (p HuhPrompter) SelectSkills(ctx context.Context, request prompt.SkillsSelectionRequest) (prompt.SkillsSelectionResult, error) {
	if ctx == nil {
		return prompt.SkillsSelectionResult{}, installerui.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return prompt.SkillsSelectionResult{}, err
	}
	terminal, err := p.ui()
	if err != nil {
		return prompt.SkillsSelectionResult{}, err
	}
	req := installerui.MultiSelectRequest{SelectRequest: installerui.SelectRequest{Title: "Choose Skills to install"}, MinSelected: 1, Filterable: true}
	ids := make(map[string]string, len(request.Choices))
	for i, choice := range request.Choices {
		id := fmt.Sprintf("skill-%d", i+1)
		ids[id] = choice.ID
		label := []rune(prompt.SafeText(choice.Label))
		if len(label) > 128 {
			label = append(label[:127], '…')
		}
		req.Options = append(req.Options, installerui.Option{ID: id, Label: string(label)})
	}
	selected, err := terminal.SelectMany(ctx, req)
	if err != nil {
		return prompt.SkillsSelectionResult{}, adapterError(err)
	}
	if selected.Cancelled { //nolint:misspell // Preserve the public selection field.
		return prompt.SkillsSelectionResult{}, prompt.ErrPromptCanceled
	}
	result := prompt.SkillsSelectionResult{}
	for _, id := range selected.IDs {
		original, ok := ids[id]
		if !ok {
			return prompt.SkillsSelectionResult{}, installerui.ErrInvalidSelection
		}
		result.IDs = append(result.IDs, original)
	}
	return result, nil
}
