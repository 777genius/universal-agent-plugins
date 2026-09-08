package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
)

func main() {
	if commands.IsEnabled() {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		app := commands.App{Projects: project.Service{Scratch: os.TempDir()}, Revision: commands.Revision}
		if err := app.Execute(ctx, os.Args[1:], authoringcli.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}, authoringcli.NewPluginKitRoot); err != nil {
			os.Exit(exitx.Code(err))
		}
		return
	}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(exitx.Code(err))
	}
}
