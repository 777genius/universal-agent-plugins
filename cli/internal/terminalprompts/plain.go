package terminalprompts

import (
	"context"
	"fmt"
	"io"

	"github.com/777genius/plugin-kit-ai/cli/installerui"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type PlainPrompter struct {
	Input  io.Reader
	Output io.Writer
}

func (p PlainPrompter) ui() (*installerui.UI, error) {
	if p.Input == nil || p.Output == nil {
		return nil, prompt.ErrPromptUnavailable
	}
	return installerui.New(installerui.Config{
		Input: p.Input, Output: p.Output, ReadLine: promptio.ReadLine,
		Style: func(text string) string { return terminaltheme.For(p.Output).Text(terminaltheme.Label, text) },
	})
}

func (p PlainPrompter) SelectTargets(ctx context.Context, r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
	u, err := p.ui()
	if err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	if err := prompt.ValidateRequest(r); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	for _, label := range r.SkippedLabels {
		if err := promptio.WriteText(p.Output, terminaltheme.For(p.Output).Text(terminaltheme.Warning, "Skipped (not installed in this attempt)")+": "+prompt.SafeText(label)+"\n"); err != nil {
			return prompt.TargetSelectionResult{}, err
		}
	}
	req := installerui.SelectRequest{Title: "Detected supported clients (all selected by default)"}
	for _, c := range r.Choices {
		req.Options = append(req.Options, installerui.Option{ID: string(c.ID), Label: prompt.SafeText(c.Label) + " (" + prompt.SafeText(string(c.ID)) + ")"})
	}
	for _, id := range r.DefaultIDs {
		req.Defaults = append(req.Defaults, string(id))
	}
	result, err := u.SelectMany(ctx, req)
	if err != nil {
		return prompt.TargetSelectionResult{}, fmt.Errorf("invalid client multiselect: %w", err)
	}
	if result.Cancelled {
		return prompt.TargetSelectionResult{}, prompt.ErrPromptCanceled
	}
	ids := make([]domain.ClientID, len(result.IDs))
	for i, id := range result.IDs {
		ids[i] = domain.ClientID(id)
	}
	return prompt.ValidateSelection(r, ids)
}

func (p PlainPrompter) Confirm(ctx context.Context, r prompt.ConfirmationRequest) (prompt.ConfirmationResult, error) {
	u, err := p.ui()
	if err != nil {
		return prompt.ConfirmationResult{}, err
	}
	title := prompt.SafeText(r.Title)
	if title == "" {
		title = "Continue?"
	}
	summary := make([]string, len(r.Summary))
	for i, s := range r.Summary {
		summary[i] = prompt.SafeText(s)
	}
	result, err := u.Confirm(ctx, installerui.ConfirmRequest{Title: title, Summary: summary})
	if err != nil {
		return prompt.ConfirmationResult{}, err
	}
	if result.Cancelled {
		return prompt.ConfirmationResult{}, prompt.ErrPromptCanceled
	}
	return prompt.ConfirmationResult{Accepted: result.Accepted}, nil
}
