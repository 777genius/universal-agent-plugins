package terminalprompts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/777genius/plugin-kit-ai/cli/installerui"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

type PlainPrompter struct {
	Input  io.Reader
	Output io.Writer
}

func (PlainPrompter) RichReview() bool { return false }

// Nonterminal injected fixtures retain the existing public line-oriented seam.
func (p PlainPrompter) ui() (*installerui.UI, error) {
	if p.Input == nil || p.Output == nil {
		return nil, prompt.ErrPromptUnavailable
	}
	return installerui.New(installerui.Config{Input: p.Input, Output: p.Output, ReadLine: promptio.ReadLine,
		Style: func(s string) string { return terminaltheme.For(p.Output).Text(terminaltheme.Label, s) }})
}

func (p PlainPrompter) SelectTargets(ctx context.Context, r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
	if ctx == nil {
		return prompt.TargetSelectionResult{}, installerui.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	if p.Input == nil || p.Output == nil {
		return prompt.TargetSelectionResult{}, prompt.ErrPromptUnavailable
	}
	req, err := selectionRequest(r, "Detected supported clients (all selected by default)")
	if err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	if err := writeSkipped(p.Output, r.SkippedLabels, terminaltheme.For(p.Output).Enabled); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	result, err := p.selectMany(ctx, req)
	if errors.Is(err, context.Canceled) {
		return prompt.TargetSelectionResult{}, canceledPrompt(err)
	}
	if err != nil {
		return prompt.TargetSelectionResult{}, fmt.Errorf("invalid client multiselect: %w", adapterError(err))
	}
	return targetResult(r, result, nil)
}

func (p PlainPrompter) Confirm(ctx context.Context, r prompt.ConfirmationRequest) (prompt.ConfirmationResult, error) {
	if ctx == nil {
		return prompt.ConfirmationResult{}, installerui.ErrInvalidRequest
	}
	if err := ctx.Err(); err != nil {
		return prompt.ConfirmationResult{}, err
	}
	if p.Input == nil || p.Output == nil {
		return prompt.ConfirmationResult{}, prompt.ErrPromptUnavailable
	}
	if _, ok := p.Input.(*os.File); ok {
		t, err := terminalUI(p.Input, p.Output, installerui.ModePlain, !terminaltheme.For(p.Output).Enabled)
		if err != nil {
			return prompt.ConfirmationResult{}, err
		}
		result, err := t.Confirm(ctx, confirmationRequest(r))
		return confirmationResult(result, err)
	}
	u, err := p.ui()
	if err != nil {
		return prompt.ConfirmationResult{}, err
	}
	result, err := u.Confirm(ctx, confirmationRequest(r))
	if errors.Is(err, context.Canceled) {
		return prompt.ConfirmationResult{}, canceledPrompt(err)
	}
	return confirmationResult(result, err)
}

func (p PlainPrompter) selectMany(ctx context.Context, req installerui.MultiSelectRequest) (installerui.Selection, error) {
	var err error
	var result installerui.Selection
	if _, ok := p.Input.(*os.File); ok {
		t, e := terminalUI(p.Input, p.Output, installerui.ModePlain, !terminaltheme.For(p.Output).Enabled)
		if e != nil {
			return installerui.Selection{}, e
		}
		result, err = t.SelectMany(ctx, req)
	} else {
		u, e := p.ui()
		if e != nil {
			return installerui.Selection{}, e
		}
		result, err = u.SelectMany(ctx, req.SelectRequest)
	}
	return result, err
}
