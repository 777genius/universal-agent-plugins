package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/spf13/cobra"
)

type testRunner interface {
	Test(context.Context, app.PluginTestOptions) (app.PluginTestResult, error)
}

type testFlagState struct {
	platform     string
	event        string
	fixture      string
	goldenDir    string
	format       string
	updateGolden bool
	all          bool
}

var testCmd = newTestCmd(pluginService)

func newTestCmd(runner testRunner) *cobra.Command {
	flags := testFlagState{
		format: "text",
	}

	cmd := &cobra.Command{
		Use:           "test [path]",
		Short:         "Run stable fixture-driven smoke tests against the launcher entrypoint",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `Run stable Claude or Codex runtime smoke tests from JSON fixtures.

The command loads a fixture, invokes the configured launcher entrypoint with the correct carrier
(stdin JSON for Claude stable hooks, argv JSON for Codex notify), and optionally compares or updates
golden stdout/stderr/exitcode files for CI-grade regression checks.

Gemini has a production-ready 9-hook Go runtime with dedicated runtime gates and stays outside this stable fixture surface.
For Gemini use go test, generate --check, validate --strict, inspect, capabilities --mode runtime,
make test-gemini-runtime, then gemini extensions link . and optionally make test-gemini-runtime-live.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			if flags.format != "text" && flags.format != "json" {
				return fmt.Errorf("unsupported format %q (use text or json)", flags.format)
			}
			result, err := runner.Test(cmd.Context(), app.PluginTestOptions{
				Root:         root,
				Platform:     flags.platform,
				Event:        flags.event,
				Fixture:      flags.fixture,
				GoldenDir:    flags.goldenDir,
				UpdateGolden: flags.updateGolden,
				All:          flags.all,
			})
			if err != nil {
				return err
			}
			if flags.format == "json" {
				body, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(body))
			} else {
				for _, line := range result.Lines {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			}
			if result.Passed {
				return nil
			}
			return exitx.Wrap(errors.New("test failures"), 1)
		},
	}
	cmd.Flags().StringVar(&flags.platform, "platform", "", `target override ("claude" or "codex-runtime")`)
	cmd.Flags().StringVar(&flags.event, "event", "", "stable event to execute (for example Stop, PreToolUse, UserPromptSubmit, or Notify)")
	cmd.Flags().BoolVar(&flags.all, "all", false, "run every stable event for the selected platform")
	cmd.Flags().StringVar(&flags.fixture, "fixture", "", "fixture JSON path for single-event runs (default: fixtures/<platform>/<event>.json)")
	cmd.Flags().StringVar(&flags.goldenDir, "golden-dir", "", "golden output directory (default: goldens/<platform>)")
	cmd.Flags().BoolVar(&flags.updateGolden, "update-golden", false, "write current stdout/stderr/exitcode outputs into the golden files")
	cmd.Flags().StringVar(&flags.format, "format", "text", "output format: text or json")
	return cmd
}
