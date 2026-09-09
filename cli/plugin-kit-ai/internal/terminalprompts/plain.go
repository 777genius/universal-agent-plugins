package terminalprompts

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type PlainPrompter struct {
	Input  io.Reader
	Output io.Writer
}

func (p PlainPrompter) SelectTargets(ctx context.Context, r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
	if p.Input == nil || p.Output == nil {
		return prompt.TargetSelectionResult{}, prompt.ErrPromptUnavailable
	}
	if err := ctx.Err(); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	if err := prompt.ValidateRequest(r); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	var b strings.Builder
	if len(r.SkippedLabels) > 0 {
		b.WriteString(terminaltheme.For(p.Output).Text(terminaltheme.Warning, "Skipped installed clients that this package cannot install together") + ": ")
		for i, s := range r.SkippedLabels {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(prompt.SafeText(s))
		}
		b.WriteByte('\n')
	}
	b.WriteString(terminaltheme.For(p.Output).Text(terminaltheme.Label, "Detected supported clients") + terminaltheme.For(p.Output).Text(terminaltheme.Muted, " (all selected by default)") + ":\n")
	for i, c := range r.Choices {
		fmt.Fprintf(&b, "  %d. [x] %s (%s)\n", i+1, prompt.SafeText(c.Label), prompt.SafeText(string(c.ID)))
	}
	b.WriteString(terminaltheme.For(p.Output).Text(terminaltheme.Label, "Choose targets by number, comma-separated [all]") + ": ")
	if err := promptio.WriteText(p.Output, b.String()); err != nil {
		return prompt.TargetSelectionResult{}, fmt.Errorf("write prompt: %w", err)
	}
	line, err := promptio.ReadLine(ctx, p.Input)
	if err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	ids := r.DefaultIDs
	if strings.TrimSpace(line) != "" {
		ids = nil
		for _, raw := range strings.Split(line, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil || n < 1 || n > len(r.Choices) {
				return prompt.TargetSelectionResult{}, fmt.Errorf("invalid client multiselect")
			}
			ids = append(ids, domain.ClientID(r.Choices[n-1].ID))
		}
	}
	if err := ctx.Err(); err != nil {
		return prompt.TargetSelectionResult{}, err
	}
	return prompt.ValidateSelection(r, ids)
}
func (p PlainPrompter) Confirm(ctx context.Context, r prompt.ConfirmationRequest) (prompt.ConfirmationResult, error) {
	if p.Input == nil || p.Output == nil {
		return prompt.ConfirmationResult{}, prompt.ErrPromptUnavailable
	}
	if err := ctx.Err(); err != nil {
		return prompt.ConfirmationResult{}, err
	}
	for _, s := range r.Summary {
		if err := promptio.WriteText(p.Output, prompt.SafeText(s)+"\n"); err != nil {
			return prompt.ConfirmationResult{}, fmt.Errorf("write prompt: %w", err)
		}
	}
	if err := promptio.WriteText(p.Output, fmt.Sprintf("%s [y/N] ", terminaltheme.For(p.Output).Text(terminaltheme.Label, prompt.SafeText(r.Title)))); err != nil {
		return prompt.ConfirmationResult{}, fmt.Errorf("write prompt: %w", err)
	}
	line, err := promptio.ReadLine(ctx, p.Input)
	if err != nil {
		return prompt.ConfirmationResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return prompt.ConfirmationResult{}, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return prompt.ConfirmationResult{Accepted: line == "y" || line == "yes"}, nil
}
