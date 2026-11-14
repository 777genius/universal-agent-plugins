package main

import (
	"github.com/spf13/cobra"
)

func newPublicationCmd(runner inspectRunner) *cobra.Command {
	var target string
	var format string
	cmd := &cobra.Command{
		Use:   "publication [path]",
		Short: "Show the publication-oriented package and channel view",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublication(cmd, runner, target, format, args)
		},
	}
	cmd.Flags().StringVar(&target, "target", "all", `publication target ("all", "claude", "codex-package", or "gemini")`)
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	cmd.AddCommand(newPublicationDoctorCmd(runner))
	if materializer, ok := any(runner).(publicationMaterializeRunner); ok {
		cmd.AddCommand(newPublicationMaterializeCmd(materializer))
		cmd.AddCommand(newPublicationRemoveCmd(materializer))
	}
	return cmd
}
