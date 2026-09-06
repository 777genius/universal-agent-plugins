package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/cli/internal/outputjson"
	"github.com/spf13/cobra"
)

func newReleaseRoot(factories ...authoringcli.Factory) (*cobra.Command, error) {
	root := agentpluginscli.NewRoot(agentpluginscli.App{Version: version})
	author, err := authoringcli.NewReleaseAuthorCommand(factories...)
	if err != nil {
		return nil, err
	}
	root.AddCommand(author)
	return root, nil
}

// The installer callback is the sole entry to home, feeds, security and client
// setup. Tests replace it with a trap; selection only constructs fresh commands.
func executeRelease(ctx context.Context, args []string, streams authoringcli.Streams, installer func() error) error {
	a := commands.App{Projects: project.Service{Scratch: os.TempDir()}, Revision: commands.Revision, PublicContract: true,
		Release: &commands.ReleaseOptions{Product: "agentplugins", Version: version}}
	root, author, noEffect, err := a.ReleaseSelection(args, newReleaseRoot)
	if err != nil {
		return err
	}
	if author {
		return a.Execute(ctx, args, streams, newReleaseRoot)
	}
	if !noEffect {
		return installer()
	}
	var out bytes.Buffer
	root.SetOut(&out)
	for _, c := range root.Commands() {
		if c.Name() == "completion" {
			root.RemoveCommand(c)
		}
	}
	authoringcli.PrepareReleaseUtilities(root, true)
	helpRendered := false
	root.SetHelpFunc(func(c *cobra.Command, _ []string) {
		helpRendered = true
		format, _ := c.Flags().GetString("format")
		if format == "json" {
			var visible []string
			for _, child := range c.Commands() {
				if !child.Hidden {
					visible = append(visible, child.Name())
				}
			}
			err = outputjson.Write(&out, "help", outputjson.Success, map[string]any{"use": c.CommandPath(), "commands": visible})
		} else {
			err = c.Usage()
		}
	})
	runErr := authoringcli.Factory(func() (*cobra.Command, error) { return root, nil }).Execute(ctx, args, authoringcli.Streams{In: streams.In, Out: &out, Err: io.Discard})
	if runErr != nil || err != nil {
		format := commands.OutputFormat(root, args)
		if format == "json" {
			err = outputjson.Write(streams.Out, "arguments", outputjson.Failure, map[string]string{"code": "arguments_invalid"})
		} else {
			_, err = io.WriteString(streams.Out, "Use agentplugins --help for implemented commands.\n")
		}
		if err != nil {
			return exitx.Wrap(errors.New("output failed"), 1)
		}
		return exitx.Wrap(errors.New("arguments invalid"), 2)
	}
	if !helpRendered && commands.CompletionInvocation(root, args) && commands.OutputFormat(root, args) == "json" {
		if err = outputjson.Write(streams.Out, "completion", outputjson.Success, map[string]string{"script": out.String()}); err != nil {
			return exitx.Wrap(errors.New("output failed"), 1)
		}
		return nil
	}
	if _, err = streams.Out.Write(out.Bytes()); err != nil {
		return exitx.Wrap(errors.New("output failed"), 1)
	}
	return nil
}
