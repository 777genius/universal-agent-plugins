package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/cli/internal/outputjson"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
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
	a := commands.App{Projects: project.Service{Scratch: os.TempDir()}, Revision: commands.Revision, ClientRegistry: clientregistry.Default(), PublicContract: true, MCPRuntime: true, Bootstrap: true, JSONMaintenance: true,
		Release: &commands.ReleaseOptions{Product: "agentplugins", Version: version}}
	root, author, noEffect, err := a.ReleaseSelection(args, newReleaseRoot)
	if err != nil {
		return err
	}
	if author {
		return a.Execute(ctx, args, streams, newReleaseRoot)
	}
	if !noEffect {
		return runReleaseInstaller(streams, installer)
	}
	return executeReleaseUtility(ctx, args, streams, root)
}

func runReleaseInstaller(streams authoringcli.Streams, installer func() error) error {
	if err := installer(); err != nil {
		// Preserve the ordinary main's installer rendering and exit boundary.
		// Author and already-rendered utility failures never enter this branch.
		_, _ = fmt.Fprintln(streams.Err, "agentplugins:", err)
		return exitx.Wrap(err, 1)
	}
	return nil
}

func executeReleaseUtility(ctx context.Context, args []string, streams authoringcli.Streams, root *cobra.Command) error {
	var out bytes.Buffer
	root.SetOut(&out)
	stripReleaseCompletion(root)
	authoringcli.PrepareReleaseUtilities(root, true)
	help := attachReleaseHelp(root, &out)
	runErr := authoringcli.Factory(func() (*cobra.Command, error) { return root, nil }).Execute(ctx, args, authoringcli.Streams{In: streams.In, Out: &out, Err: io.Discard})
	if runErr != nil || help.err != nil {
		return writeReleaseUtilityFailure(streams, root, args)
	}
	if !help.rendered && commands.CompletionInvocation(root, args) && commands.OutputFormat(root, args) == "json" {
		if err := outputjson.Write(streams.Out, "completion", outputjson.Success, map[string]string{"script": out.String()}); err != nil {
			return exitx.Wrap(errors.New("output failed"), 1)
		}
		return nil
	}
	if _, err := streams.Out.Write(out.Bytes()); err != nil {
		return exitx.Wrap(errors.New("output failed"), 1)
	}
	return nil
}

func stripReleaseCompletion(root *cobra.Command) {
	for _, c := range root.Commands() {
		if c.Name() == "completion" {
			root.RemoveCommand(c)
		}
	}
}

type releaseHelpState struct {
	rendered bool
	err      error
}

func attachReleaseHelp(root *cobra.Command, out *bytes.Buffer) *releaseHelpState {
	state := &releaseHelpState{}
	root.SetHelpFunc(func(c *cobra.Command, _ []string) {
		state.rendered = true
		state.err = renderReleaseHelp(c, out)
	})
	return state
}

func renderReleaseHelp(c *cobra.Command, out *bytes.Buffer) error {
	format, _ := c.Flags().GetString("format")
	if format == "json" {
		var visible []string
		for _, child := range c.Commands() {
			if !child.Hidden {
				visible = append(visible, child.Name())
			}
		}
		return outputjson.Write(out, "help", outputjson.Success, map[string]any{"use": c.CommandPath(), "commands": visible})
	}
	return c.Usage()
}

func writeReleaseUtilityFailure(streams authoringcli.Streams, root *cobra.Command, args []string) error {
	format := commands.OutputFormat(root, args)
	var err error
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
