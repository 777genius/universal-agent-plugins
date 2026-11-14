package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func newRunClientCommand(app App, opts *options) *cobra.Command {
	var confirmed, inspect bool
	command := &cobra.Command{
		Use: "run", Short: "Start a qualified client with its owned project MCP source (explicit consent)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			selected, err := app.forOptions(opts)
			if err != nil {
				return err
			}
			if !opts.dryRun && !confirmed {
				return fmt.Errorf("client startup is separate from installation; review --dry-run, then pass --confirm-client-start")
			}
			return runClientSession(cmd.Context(), cmd, selected, opts, inspect, confirmed)
		},
	}
	command.Flags().BoolVar(&confirmed, "confirm-client-start", false, "allow native inventory verification and the selected client command with the reviewed Project MCP source; does not grant workspace trust")
	command.Flags().BoolVar(&inspect, "inspect", false, "run the native configuration inventory after verification instead of an interactive session; no model call")
	return command
}

func runClientSession(ctx context.Context, cmd *cobra.Command, app App, opts *options, inspect, confirmed bool) error {
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	if len(targets) != 1 || app.scope() != domain.ScopeProject {
		return fmt.Errorf("client startup requires one explicit --target and --scope project")
	}
	planner, ok := clients.As[clients.SessionPlanner](app.ClientRegistry, targets[0])
	if !ok {
		return fmt.Errorf("selected client does not support managed session startup")
	}
	if app.Lifecycle.Lock == nil || app.StateStore == nil || app.Detector == nil {
		return fmt.Errorf("client startup services unavailable")
	}
	if !opts.dryRun && (opts.format != "human" || app.StartNativeSession == nil || !confirmed) {
		return fmt.Errorf("native session requires human output, explicit consent and a process runner")
	}
	unlock, err := app.Lifecycle.Lock.Acquire(ctx)
	if err != nil {
		return err
	}
	app.SessionEnv.SessionEnvironment = append([]string{}, os.Environ()...)
	plan, planErr := prepareClientSession(ctx, app, targets[0], planner, inspect)
	if planErr != nil {
		return errors.Join(planErr, unlock())
	}
	if !opts.dryRun {
		return executeClientSession(ctx, cmd, app, targets[0], planner, inspect, plan, unlock)
	}
	if err := unlock(); err != nil {
		return err
	}
	if opts.format == "json" {
		return writeJSONOutput(cmd.OutOrStdout(), "run", map[string]any{"client": plan.ClientID, "scope": "project", "servers": plan.Servers, "starts_process": false, "requires_client_start_consent": true})
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Client: %s\nProject: %s\nOwned MCP servers: %s\nConfirmed startup first verifies the native inventory, then runs the selected client command.\nThese commands select the whole Project MCP source; User MCP entries are not merged. No trust, activation or login state is changed by UAP.\n",
		prompt.SafeText(string(plan.ClientID)), prompt.SafeText(plan.Directory), prompt.SafeText(strings.Join(plan.Servers, ", ")))
	return err
}

func executeClientSession(ctx context.Context, cmd *cobra.Command, app App, target domain.ClientID, planner clients.SessionPlanner, inspect bool, plan clients.SessionPlan, unlock func() error) (resultErr error) {
	env := app.SessionEnv
	var execution clients.SessionExecution
	var planErr error
	if owner, ok := planner.(clients.SessionExecutionPlanner); ok {
		execution, planErr = owner.BeginSessionExecution(ctx, env, plan)
	}
	if execution != nil {
		if execution.Context() == nil || execution.Runner() == nil || !inspect {
			planErr = errors.Join(planErr, fmt.Errorf("client execution authority is incomplete or not an inspection"), execution.Close())
			execution = nil
		} else {
			defer func() {
				if execution != nil {
					resultErr = errors.Join(resultErr, execution.Revalidate(), execution.Close())
				}
			}()
			ctx, env.Runner = execution.Context(), execution.Runner()
		}
	}
	if planErr == nil {
		planErr = planner.VerifySession(ctx, env, plan)
	}
	if err := errors.Join(planErr, unlock()); err != nil {
		return err
	}
	// The interactive child never holds the installer lock. A retained optional
	// execution still spans this original replan and the final process boundary.
	base := append([]string{}, os.Environ()...)
	app.SessionEnv.SessionEnvironment = append([]string{}, base...)
	check, err := prepareClientSession(ctx, app, target, planner, inspect)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(plan, check) {
		return fmt.Errorf("client startup plan changed; review the new plan")
	}
	if execution != nil {
		if err := execution.Revalidate(); err != nil {
			return err
		}
		retained := execution
		execution = nil
		return runRetainedClientInspection(ctx, cmd, retained, plan, base)
	}
	return app.StartNativeSession(ctx, plan, base)
}

func runRetainedClientInspection(ctx context.Context, cmd *cobra.Command, execution clients.SessionExecution, plan clients.SessionPlan, base []string) error {
	// A retained inspection must return to its original authority. Unix exec
	// would discard these handles and bypass post-child validation and cleanup.
	const limit = 1 << 20
	result, err := execution.Runner().Run(ctx, legacyports.Command{
		Argv: append([]string{plan.Executable}, plan.Arguments...), Dir: plan.Directory,
		Env: plan.ChildEnvironment(base), StdoutLimitBytes: limit,
	})
	if result.ExitCode != 0 || len(result.Stdout) >= limit || len(result.Stderr) > 32768 || !result.CleanupComplete || result.NoChildStarted {
		err = errors.Join(err, fmt.Errorf("native inspection failed, exceeded its bound or lacked process cleanup"))
	}
	// Close can intentionally cancel the execution context. Check that original
	// context first; settle all retained resources before publishing any output.
	if err = errors.Join(err, ctx.Err(), execution.Revalidate(), execution.Close()); err != nil {
		return err
	}
	_, outErr := io.Copy(cmd.OutOrStdout(), bytes.NewReader(result.Stdout))
	_, errErr := io.Copy(cmd.ErrOrStderr(), bytes.NewReader(result.Stderr))
	return errors.Join(outErr, errErr)
}

func prepareClientSession(ctx context.Context, app App, target domain.ClientID, planner clients.SessionPlanner, inspect bool) (clients.SessionPlan, error) {
	detected, err := app.Detector.Detect(ctx)
	if err != nil {
		return clients.SessionPlan{}, err
	}
	selected := []domain.DetectedClient{}
	for _, client := range detected {
		if client.ClientID == target {
			selected = append(selected, client)
		}
	}
	if len(selected) != 1 {
		return clients.SessionPlan{}, fmt.Errorf("client detection is absent or ambiguous")
	}
	bindings, err := sessionBindings(ctx, app, selected[0])
	if err != nil {
		return clients.SessionPlan{}, err
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].ClientBindingID < bindings[j].ClientBindingID })
	return planner.PlanSession(ctx, app.SessionEnv, clients.SessionRequest{Client: selected[0], ProjectRoot: app.ProjectRoot, Bindings: bindings, Inspect: inspect})
}
