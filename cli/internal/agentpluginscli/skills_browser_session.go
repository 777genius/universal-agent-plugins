package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
)

const skillsBrowserLaunchTimeout = 8 * time.Second

type skillsBrowserLauncher func(context.Context, string) error

func runSkillsBrowserSession(cmd *cobra.Command, app App, opts *options, server *skillspicker.Server, launch skillsBrowserLauncher) (bool, error) {
	// The opener is bounded independently. Its timeout cannot cancel work that
	// the browser may already have admitted through the session's parent context.
	launchCtx, cancel := context.WithTimeout(cmd.Context(), skillsBrowserLaunchTimeout)
	err := launch(launchCtx, server.URL())
	cancel()
	if err == nil {
		writer := &planWriter{writer: reviewWriter(cmd, app)}
		if _, err := fmt.Fprintln(writer, "Skills picker opened. Keep this command running while reviewing and installing."); err != nil {
			return true, skillsBrowserInteractionFailure(cmd.Context(), server, err)
		}
		return true, waitAndCloseSkillsPicker(cmd.Context(), server)
	}
	if cmd.Context().Err() != nil {
		server.Close()
		return true, cmd.Context().Err()
	}
	return offerSkillsTerminalFallback(cmd, app, opts, server)
}

func offerSkillsTerminalFallback(cmd *cobra.Command, app App, opts *options, server *skillspicker.Server) (bool, error) {
	writer := &planWriter{writer: reviewWriter(cmd, app)}
	// Display this capability only to the person opening the local tab, never
	// in structured results or technical provider logs.
	if _, err := fmt.Fprintf(writer, "Browser could not open. Picker URL: %s\n", server.URL()); err != nil {
		return true, skillsBrowserInteractionFailure(cmd.Context(), server, err)
	}
	prompted, err := withPrompter(cmd, app, opts)
	if err != nil {
		return true, skillsBrowserInteractionFailure(cmd.Context(), server, err)
	}
	answer, err := prompted.Prompter.Confirm(cmd.Context(), prompt.ConfirmationRequest{Title: "Continue with terminal selection?", Default: true})
	if err != nil {
		return true, skillsBrowserInteractionFailure(cmd.Context(), server, err)
	}
	if cmd.Context().Err() != nil {
		server.Close()
		return true, cmd.Context().Err()
	}
	if !answer.Accepted {
		return true, waitAndCloseSkillsPicker(cmd.Context(), server)
	}
	if server.CloseUnused() {
		return false, nil
	}
	if _, err := fmt.Fprintln(writer, "The browser is reviewing or has accepted an installation. Continue in that picker; terminal selection will not start another installation."); err != nil {
		return true, skillsBrowserInteractionFailure(cmd.Context(), server, err)
	}
	return true, waitAndCloseSkillsPicker(cmd.Context(), server)
}

// Output/prompt failures do not authorize cancelling browser-owned effects. If
// handoff has lost admission, keep that session alive until its owner ends it.
func skillsBrowserInteractionFailure(ctx context.Context, server *skillspicker.Server, cause error) error {
	if server.CloseUnused() {
		return cause
	}
	return errors.Join(cause, waitAndCloseSkillsPicker(ctx, server))
}
func waitAndCloseSkillsPicker(ctx context.Context, server *skillspicker.Server) error {
	defer server.Close()
	return waitForSkillsPicker(ctx, server)
}
