package main

import (
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/spf13/cobra"
)

type doctorRunner interface {
	Doctor(app.PluginDoctorOptions) (app.PluginDoctorResult, error)
}

var doctorCmd = newDoctorCmd(pluginService)

func newDoctorCmd(runner doctorRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "doctor [path]",
		Short:         "Inspect repo-local runtime readiness without mutating files",
		Long:          "Read-only readiness check for package-standard projects. Reports lane, runtime, detected manager, status, and next commands.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			result, err := runner.Doctor(app.PluginDoctorOptions{Root: root})
			if err != nil {
				return err
			}
			for _, line := range result.Lines {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			if result.Ready {
				return nil
			}
			return exitx.Wrap(errors.New("doctor found issues"), 1)
		},
	}
	return cmd
}
