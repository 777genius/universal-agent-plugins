package agentpluginscli

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Retain existing stage text for automation/plain output, and animate only an
// actual human terminal. The command activity owns no prompts or consent.
func startAddProgress(cmd *cobra.Command, app App, opts *options, message string) func() {
	if !app.Terminal || opts.format != "human" || opts.plain || os.Getenv("TERM") == "dumb" ||
		!terminaltheme.IsTerminal(terminaltheme.Unwrap(cmd.ErrOrStderr())) {
		writeProgress(app, opts.format, message)
		return func() {}
	}
	return startCommandActivity(cmd, app, opts, strings.TrimSuffix(message, "..."))
}

// Acquisition may emit revocation/security warnings. Clear and join the
// renderer before the first write, preserving warning bytes and write errors.
type pluginActivityWriter struct {
	io.Writer
	stop func()
}

func (writer pluginActivityWriter) Write(value []byte) (int, error) {
	writer.stop()
	return writer.Writer.Write(value)
}

func (writer pluginActivityWriter) SemanticTheme() terminaltheme.Theme {
	return terminaltheme.For(writer.Writer)
}

func stopPluginActivityOnWrite(app App, stop func()) App {
	app.ErrorOutput = pluginActivityWriter{Writer: app.errorOutput(), stop: stop}
	return app
}

func loadAddPackageFor(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, request packageResolutionRequest, message string) (loadedPackage, error) {
	stop := startAddProgress(cmd, app, opts, message)
	defer stop()
	return stopPluginActivityOnWrite(app, stop).loadPackageFor(ctx, source, request)
}

// Keep discovery and acquisition activity together, ending each before review
// or mutation starts in the existing grouped install flow.
func runAddManyWithClients(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, activationComplete, authComplete bool, clients []domain.DetectedClient) error {
	stop := startCommandActivity(cmd, app, opts, "Checking selected agents")
	_, detected, err := preflightAddTargets(ctx, app, opts, source, targets, clients)
	stop()
	if err != nil {
		return err
	}
	loaded, err := loadAddPackageFor(ctx, cmd, app, opts, source, withDetectedClients(app.addResolutionRequest(source, targets), detected), "Resolving and validating one Agent Plugin package for every selected target...")
	if err != nil {
		return err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	return runAddManyLoaded(ctx, cmd, app, opts, loaded, targets, activationComplete, authComplete, detectedClientValues(detected), false)
}
