package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
	plan, planErr := prepareClientSession(ctx, app, targets[0], planner, inspect)
	if planErr == nil && !opts.dryRun {
		planErr = planner.VerifySession(ctx, app.SessionEnv, plan)
	}
	releaseErr := unlock()
	if err := errors.Join(planErr, releaseErr); err != nil {
		return err
	}
	if opts.dryRun {
		if opts.format == "json" {
			return writeJSONOutput(cmd.OutOrStdout(), "run", map[string]any{"client": plan.ClientID, "scope": "project", "servers": plan.Servers, "starts_process": false, "requires_client_start_consent": true})
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Client: %s\nProject: %s\nOwned MCP servers: %s\nConfirmed startup first verifies the native inventory, then runs the selected client command.\nThese commands select the whole Project MCP source; User MCP entries are not merged. No trust, activation or login state is changed by UAP.\n",
			prompt.SafeText(string(plan.ClientID)), prompt.SafeText(plan.Directory), prompt.SafeText(strings.Join(plan.Servers, ", ")))
		return err
	}
	// Recheck files after releasing the installer lock; the interactive client
	// must not hold that lock for its session lifetime. No receipt is promoted.
	check, err := prepareClientSession(ctx, app, targets[0], planner, inspect)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(plan, check) {
		return fmt.Errorf("client startup plan changed; review the new plan")
	}
	return app.StartNativeSession(ctx, plan)
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
