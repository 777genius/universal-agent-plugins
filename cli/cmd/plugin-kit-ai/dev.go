package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/spf13/cobra"
)

type devRunner interface {
	Dev(context.Context, app.PluginDevOptions, func(app.PluginDevUpdate)) (app.PluginDevSummary, error)
}

type devFlagState struct {
	platform  string
	event     string
	fixture   string
	goldenDir string
	all       bool
	once      bool
	interval  time.Duration
}

var devCmd = newDevCmd(pluginService)

func newDevCmd(runner devRunner) *cobra.Command {
	flags := devFlagState{
		interval: 750 * time.Millisecond,
	}

	cmd := &cobra.Command{
		Use:           "dev [path]",
		Short:         "Watch the project, re-generate, re-validate, rebuild when needed, and rerun fixtures",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `Watch launcher-based runtime targets in a fast inner loop.

Each cycle re-generates the selected target, performs runtime-aware rebuilds when needed,
runs strict validation, and reruns the configured stable Claude or Codex fixture smoke tests.

Gemini has a production-ready 9-hook Go runtime with dedicated runtime gates and stays outside this stable watch loop.
For Gemini use generate, generate --check, validate --strict, inspect, capabilities --mode runtime,
make test-gemini-runtime, then gemini extensions link . and optionally rerun
make test-gemini-runtime-live after changes.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			var lastPassed = true
			summary, err := runner.Dev(ctx, app.PluginDevOptions{
				Root:      root,
				Platform:  flags.platform,
				Event:     flags.event,
				Fixture:   flags.fixture,
				GoldenDir: flags.goldenDir,
				All:       flags.all,
				Once:      flags.once,
				Interval:  flags.interval,
			}, func(update app.PluginDevUpdate) {
				lastPassed = update.Passed
				for _, line := range update.Lines {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			})
			if err != nil {
				return err
			}
			if flags.once && !summary.LastPassed {
				return exitx.Wrap(errors.New("dev cycle failed"), 1)
			}
			if flags.once && !lastPassed {
				return exitx.Wrap(errors.New("dev cycle failed"), 1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.platform, "platform", "", `target override ("claude" or "codex-runtime")`)
	cmd.Flags().StringVar(&flags.event, "event", "", "stable event to execute (for example Stop, PreToolUse, UserPromptSubmit, or Notify)")
	cmd.Flags().BoolVar(&flags.all, "all", false, "run every stable event for the selected platform on each cycle")
	cmd.Flags().StringVar(&flags.fixture, "fixture", "", "fixture JSON path for single-event runs (default: fixtures/<platform>/<event>.json)")
	cmd.Flags().StringVar(&flags.goldenDir, "golden-dir", "", "golden output directory (default: goldens/<platform>)")
	cmd.Flags().BoolVar(&flags.once, "once", false, "run a single generate/validate/test cycle and exit")
	cmd.Flags().DurationVar(&flags.interval, "interval", 750*time.Millisecond, "poll interval for watch mode")
	return cmd
}
