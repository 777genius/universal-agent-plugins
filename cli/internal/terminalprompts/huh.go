package terminalprompts

import (
	"context"
	"io"

	"github.com/777genius/plugin-kit-ai/cli/installerui"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

type HuhPrompter struct {
	Input    io.Reader
	Output   io.Writer
	NoColor  bool
	terminal *installerui.Terminal
}

// RichReview preserves the host's structured install review policy.
func (HuhPrompter) RichReview() bool { return true }

func (p HuhPrompter) SelectTargets(ctx context.Context, r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
	if ctx == nil {
		return prompt.TargetSelectionResult{}, installerui.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	if p.Input == nil || p.Output == nil {
		return prompt.TargetSelectionResult{}, prompt.ErrPromptUnavailable
	}
	t, err := p.ui()
	if err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	req, err := selectionRequest(r, "Choose targets (all selected by default)")
	if err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	if err := writeSkipped(p.Output, r.SkippedLabels, !p.NoColor); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	result, err := t.SelectMany(ctx, req)
	return targetResult(r, result, err)
}

func (p HuhPrompter) Confirm(ctx context.Context, r prompt.ConfirmationRequest) (prompt.ConfirmationResult, error) {
	if ctx == nil {
		return prompt.ConfirmationResult{}, installerui.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return prompt.ConfirmationResult{}, err
	}
	if p.Input == nil || p.Output == nil {
		return prompt.ConfirmationResult{}, prompt.ErrPromptUnavailable
	}
	t, err := p.ui()
	if err != nil {
		return prompt.ConfirmationResult{}, err
	}
	result, err := t.Confirm(ctx, confirmationRequest(r))
	return confirmationResult(result, err)
}

// A factory-created adapter keeps the same UI mode across sequential prompts.
// Its renderer reflows after resize without repeating constructor eligibility.
func (p HuhPrompter) ui() (*installerui.Terminal, error) {
	if p.terminal != nil {
		return p.terminal, nil
	}
	return terminalUI(p.Input, p.Output, installerui.ModeRich, p.NoColor)
}
