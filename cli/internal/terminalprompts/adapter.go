package terminalprompts

import (
	"errors"
	"io"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/installerui"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func terminalUI(in io.Reader, out io.Writer, mode installerui.TerminalMode, noColor bool) (*installerui.Terminal, error) {
	input, inputOK := in.(*os.File)
	output, outputOK := terminaltheme.Unwrap(out).(*os.File)
	if !inputOK || !outputOK {
		return nil, prompt.ErrPromptUnavailable
	}
	t, err := installerui.NewTerminal(installerui.TerminalConfig{Input: input, Output: output, Mode: mode, NoColor: noColor})
	return t, adapterError(err)
}

func adapterError(err error) error {
	if errors.Is(err, installerui.ErrUnavailable) {
		return prompt.NormalizeIOError(errors.Join(promptio.ErrUnavailable, err))
	}
	if errors.Is(err, installerui.ErrCancelled) {
		return canceledPrompt(err)
	}
	return prompt.NormalizeIOError(err)
}

func selectionRequest(r prompt.TargetSelectionRequest, title string) (installerui.MultiSelectRequest, error) {
	// The typed legacy port permits repeated defaults; the opt-in UI is strict.
	seen := make(map[domain.ClientID]bool, len(r.DefaultIDs))
	defaults := make([]domain.ClientID, 0, len(r.DefaultIDs))
	for _, id := range r.DefaultIDs {
		if !seen[id] {
			seen[id] = true
			defaults = append(defaults, id)
		}
	}
	r.DefaultIDs = defaults
	if err := prompt.ValidateRequest(r); err != nil {
		return installerui.MultiSelectRequest{}, err
	}
	req := installerui.MultiSelectRequest{SelectRequest: installerui.SelectRequest{Title: title}, MinSelected: 1}
	for _, c := range r.Choices {
		req.Options = append(req.Options, installerui.Option{ID: string(c.ID), Label: prompt.SafeText(c.Label) + " (" + prompt.SafeText(string(c.ID)) + ")"})
	}
	for _, id := range defaults {
		req.Defaults = append(req.Defaults, string(id))
	}
	return req, nil
}

func writeSkipped(out io.Writer, labels []string, color bool) error {
	notice := prompt.SkippedClientsNotice(labels)
	if notice == "" {
		return nil
	}
	heading := (terminaltheme.Theme{Enabled: color}).Text(terminaltheme.Warning, prompt.SkippedClientsHeading)
	return promptio.WriteText(out, strings.Replace(notice, prompt.SkippedClientsHeading, heading, 1))
}

func targetResult(req prompt.TargetSelectionRequest, result installerui.Selection, err error) (prompt.TargetSelectionResult, error) {
	if err != nil {
		return prompt.TargetSelectionResult{}, adapterError(err)
	}
	if result.Cancelled { //nolint:misspell // Preserve the existing public cancellation API.
		return prompt.TargetSelectionResult{}, prompt.ErrPromptCanceled
	}
	ids := make([]domain.ClientID, len(result.IDs))
	for i, id := range result.IDs {
		ids[i] = domain.ClientID(id)
	}
	return prompt.ValidateSelection(req, ids)
}

func confirmationRequest(req prompt.ConfirmationRequest) installerui.ConfirmRequest {
	title := prompt.SafeText(req.Title)
	if title == "" {
		title = "Continue?"
	}
	return installerui.ConfirmRequest{Title: title, Summary: append([]string(nil), req.Summary...), Default: req.Default}
}

func confirmationResult(result installerui.Confirmation, err error) (prompt.ConfirmationResult, error) {
	if err != nil {
		return prompt.ConfirmationResult{}, adapterError(err)
	}
	if result.Cancelled { //nolint:misspell // Preserve the existing public cancellation API.
		return prompt.ConfirmationResult{}, prompt.ErrPromptCanceled
	}
	return prompt.ConfirmationResult{Accepted: result.Accepted}, nil
}
