package agentpluginscli

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
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
