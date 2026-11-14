package main

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

type publishFlags struct {
	Channel     string
	Dest        string
	PackageRoot string
	DryRun      bool
	All         bool
	Format      string
}

func runPublishCommand(cmd *cobra.Command, runner publishRunner, flags publishFlags, args []string) error {
	options, err := newPublishOptions(flags, args)
	if err != nil {
		return err
	}
	result, err := runner.Publish(options)
	if err != nil {
		return err
	}
	return renderPublishResult(cmd, result, flags.Format)
}

func newPublishOptions(flags publishFlags, args []string) (app.PluginPublishOptions, error) {
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	if err := validatePublishFlags(flags); err != nil {
		return app.PluginPublishOptions{}, err
	}
	return app.PluginPublishOptions{
		Root:        root,
		Channel:     flags.Channel,
		Dest:        flags.Dest,
		PackageRoot: flags.PackageRoot,
		DryRun:      flags.DryRun,
		All:         flags.All,
	}, nil
}

func validatePublishFlags(flags publishFlags) error {
	if flags.All && flags.Channel != "" {
		return fmt.Errorf("publish --all cannot be combined with --channel")
	}
	if !flags.All && flags.Channel == "" {
		return fmt.Errorf("publish requires --channel unless --all is set")
	}
	if flags.All && !flags.DryRun {
		return fmt.Errorf("publish --all currently supports only --dry-run planning")
	}
	return nil
}
