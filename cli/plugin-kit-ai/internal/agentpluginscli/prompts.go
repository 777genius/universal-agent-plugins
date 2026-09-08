package agentpluginscli

import (
	"context"
	"fmt"
	"io"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

func withPrompter(cmd *cobra.Command, app App, opts *options) (App, error) {
	if app.Prompter != nil {
		return app, nil
	}
	if app.PromptFactory == nil {
		return app, prompt.ErrPromptUnavailable
	}
	var err error
	app.Prompter, app.reviewOutput, err = app.PromptFactory(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), opts.plain, !terminaltheme.For(visiblePromptWriter(cmd)).Enabled)
	return app, err
}
func reviewWriter(cmd *cobra.Command, app App) io.Writer {
	if app.reviewOutput != nil {
		return app.reviewOutput
	}
	return cmd.OutOrStdout()
}
func confirmInstall(ctx context.Context, cmd *cobra.Command, app App, loaded loadedPackage, results []usecase.AddResult) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if app.Prompter == nil {
		return false, prompt.ErrPromptUnavailable
	}
	writer := &planWriter{writer: reviewWriter(cmd, app)}
	if _, err := fmt.Fprintf(writer, "%s: %s\n%s: user\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Source"), prompt.SafeText(publicPackageSource(loaded.envelope.Source)), terminaltheme.For(writer).Text(terminaltheme.Label, "Scope")); err != nil {
		return false, err
	}
	if loaded.envelope.Source.ResolvedRevision != "" {
		if _, err := fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Label, "Revision")+": "+prompt.SafeText(loaded.envelope.Source.ResolvedRevision)); err != nil {
			return false, err
		}
	}
	if loaded.envelope.TreeDigest != "" {
		if _, err := fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Label, "Tree")+": "+prompt.SafeText(loaded.envelope.TreeDigest)); err != nil {
			return false, err
		}
	}
	for _, result := range results {
		if err := renderHumanPlan(writer, loaded.envelope, result); err != nil {
			return false, err
		}
	}
	answer, err := app.Prompter.Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply this plan?"})
	if err != nil {
		return false, err
	}
	if err = ctx.Err(); err != nil {
		return false, err
	}
	return answer.Accepted, nil
}

// planWriter tracks output errors; callers sanitize values before formatting.
type planWriter struct {
	writer io.Writer
	err    error
}

func (w *planWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	w.err = err
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func visiblePromptWriter(cmd *cobra.Command) io.Writer {
	if terminaltheme.IsTerminal(cmd.OutOrStdout()) {
		return cmd.OutOrStdout()
	}
	return cmd.ErrOrStderr()
}
func (w *planWriter) SemanticTheme() terminaltheme.Theme { return terminaltheme.For(w.writer) }
