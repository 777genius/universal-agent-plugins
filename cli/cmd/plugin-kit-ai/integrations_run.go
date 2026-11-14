package main

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"strings"
	"syscall"

	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/install/integrationctl"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/spf13/cobra"
)

func newIntegrationSignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGTERM)
}

func runIntegrationReportAction(cmd *cobra.Command, action func(context.Context) (integrationctl.Report, error)) error {
	ctx, stop := newIntegrationSignalContext()
	defer stop()
	report, err := action(ctx)
	if err != nil {
		return exitx.Wrap(err, integrationctl.ExitCodeFromErr(err))
	}
	printIntegrationReport(cmd, report)
	return nil
}

type integrationFailureContext struct {
	Action string
	Name   string
	Target string
}

func runIntegrationResultAction(cmd *cobra.Command, startLine string, failure integrationFailureContext, blockedPlan func(context.Context) (integrationctl.Report, error), action func(context.Context) (integrationctl.Result, error)) error {
	ctx, stop := newIntegrationSignalContext()
	defer stop()
	if strings.TrimSpace(startLine) != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "⏳ %s\n\n", startLine)
	}
	result, err := action(ctx)
	if err != nil {
		if isBlockedMutationError(err) && blockedPlan != nil {
			if report, reportErr := blockedPlan(ctx); reportErr == nil {
				printBlockedIntegrationPlan(cmd, failure, report)
				return exitx.Wrap(err, integrationctl.ExitCodeFromErr(err))
			}
		}
		printIntegrationError(cmd, failure, err)
		return exitx.Wrap(err, integrationctl.ExitCodeFromErr(err))
	}
	printIntegrationReport(cmd, result.Report)
	return nil
}

func integrationResultPreview(action func(context.Context) (integrationctl.Result, error)) func(context.Context) (integrationctl.Report, error) {
	return func(ctx context.Context) (integrationctl.Report, error) {
		result, err := action(ctx)
		if err != nil {
			return integrationctl.Report{}, err
		}
		return result.Report, nil
	}
}

func executeIntegrationsAdd(cmd *cobra.Command, params integrationctl.AddParams) error {
	return runIntegrationResultAction(cmd, integrationStartLineForAdd(params), integrationFailureContext{
		Action: "add",
		Name:   params.Source,
	}, integrationResultPreview(func(ctx context.Context) (integrationctl.Result, error) {
		preview := params
		preview.DryRun = true
		return integrationsRunner.Controller.Add(ctx, preview)
	}), func(ctx context.Context) (integrationctl.Result, error) {
		return integrationsRunner.Controller.Add(ctx, params)
	})
}

func executeIntegrationsUpdate(cmd *cobra.Command, params integrationctl.UpdateParams) error {
	return runIntegrationResultAction(cmd, integrationStartLineForUpdate(params), integrationFailureContext{
		Action: "update",
		Name:   params.Name,
	}, integrationResultPreview(func(ctx context.Context) (integrationctl.Result, error) {
		preview := params
		preview.DryRun = true
		return integrationsRunner.Controller.Update(ctx, preview)
	}), func(ctx context.Context) (integrationctl.Result, error) {
		return integrationsRunner.Controller.Update(ctx, params)
	})
}

func executeIntegrationsRemove(cmd *cobra.Command, params integrationctl.RemoveParams) error {
	return runIntegrationResultAction(cmd, integrationStartLineForRemove(params), integrationFailureContext{
		Action: "remove",
		Name:   params.Name,
	}, integrationResultPreview(func(ctx context.Context) (integrationctl.Result, error) {
		preview := params
		preview.DryRun = true
		return integrationsRunner.Controller.Remove(ctx, preview)
	}), func(ctx context.Context) (integrationctl.Result, error) {
		return integrationsRunner.Controller.Remove(ctx, params)
	})
}

func executeIntegrationsRepair(cmd *cobra.Command, params integrationctl.RepairParams) error {
	return runIntegrationResultAction(cmd, integrationStartLineForRepair(params), integrationFailureContext{
		Action: "repair",
		Name:   params.Name,
		Target: params.Target,
	}, integrationResultPreview(func(ctx context.Context) (integrationctl.Result, error) {
		preview := params
		preview.DryRun = true
		return integrationsRunner.Controller.Repair(ctx, preview)
	}), func(ctx context.Context) (integrationctl.Result, error) {
		return integrationsRunner.Controller.Repair(ctx, params)
	})
}

func integrationStartLineForAdd(params integrationctl.AddParams) string {
	if params.DryRun {
		return ""
	}
	return fmt.Sprintf("Installing integration %q across managed targets...", params.Source)
}

func integrationStartLineForUpdate(params integrationctl.UpdateParams) string {
	if params.DryRun {
		return ""
	}
	if params.All {
		return "Updating all managed integrations..."
	}
	return fmt.Sprintf("Updating managed integration %q...", params.Name)
}

func integrationStartLineForRemove(params integrationctl.RemoveParams) string {
	if params.DryRun {
		return ""
	}
	return fmt.Sprintf("Removing managed integration %q...", params.Name)
}

func integrationStartLineForRepair(params integrationctl.RepairParams) string {
	if params.DryRun {
		return ""
	}
	if strings.TrimSpace(params.Target) != "" {
		return fmt.Sprintf("Repairing managed integration %q for target %q...", params.Name, params.Target)
	}
	return fmt.Sprintf("Repairing managed integration %q...", params.Name)
}

func integrationStartLineForToggle(verb, name, target string, dryRun bool) string {
	if dryRun {
		return ""
	}
	if strings.TrimSpace(target) != "" {
		return fmt.Sprintf("%s managed integration %q for target %q...", verb, name, target)
	}
	return fmt.Sprintf("%s managed integration %q...", verb, name)
}

func printIntegrationError(cmd *cobra.Command, failure integrationFailureContext, err error) {
	for _, line := range integrationErrorLines(failure, err) {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), line)
	}
}

func printBlockedIntegrationPlan(cmd *cobra.Command, failure integrationFailureContext, report integrationctl.Report) {
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "❌ "+integrationFailureSummary(failure)+" is blocked before changes.")
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "🔎 Review the blocked plan below.")
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	printIntegrationReport(cmd, report)
}

func integrationErrorLines(failure integrationFailureContext, err error) []string {
	var de *domain.Error
	if errors.As(err, &de) {
		switch {
		case de.Code == domain.ErrUnsupportedTarget && strings.Contains(de.Message, "manifest does not expose target "):
			target := unsupportedTargetFromMessage(de.Message)
			name := strings.TrimSpace(failure.Name)
			if name != "" && target != "" {
				return []string{
					fmt.Sprintf("❌ %q does not support target %q.", name, target),
					fmt.Sprintf("💡 Run `plugin-kit-ai add %s --dry-run` without `--target` to inspect the targets it supports.", name),
				}
			}
		case de.Code == domain.ErrStateConflict && strings.Contains(de.Message, "integration not found in state: "):
			name := strings.TrimSpace(failure.Name)
			if name == "" {
				name = missingIntegrationFromMessage(de.Message)
			}
			if name != "" {
				return []string{
					fmt.Sprintf("❌ Integration %q is not managed yet.", name),
					"💡 Run `plugin-kit-ai integrations list` to inspect managed integrations before updating, repairing, or removing one.",
				}
			}
		case de.Code == domain.ErrStateConflict && strings.Contains(de.Message, "already exists in state"):
			name := strings.TrimSpace(failure.Name)
			if name == "" {
				name = integrationIDFromMessage(de.Message)
			}
			if name != "" {
				return []string{
					fmt.Sprintf("❌ Integration %q is already managed.", name),
					fmt.Sprintf("💡 Try `plugin-kit-ai update %s` to refresh it, or `plugin-kit-ai integrations list` to inspect current state.", name),
				}
			}
		case (de.Code == domain.ErrMutationApply || de.Code == domain.ErrRepairApply) && strings.Contains(de.Message, "degraded state persisted"):
			lines := []string{
				"❌ " + integrationFailureSummary(failure) + " failed after partial progress.",
				"💡 Run `plugin-kit-ai integrations doctor` to inspect degraded targets and open operations.",
			}
			if failure.Action != "repair" && strings.TrimSpace(failure.Name) != "" {
				lines = append(lines, fmt.Sprintf("💡 Then run `plugin-kit-ai repair %s`.", failure.Name))
			}
			return lines
		case (de.Code == domain.ErrMutationApply || de.Code == domain.ErrRepairApply) && strings.Contains(de.Message, "planned mutation is blocked for target "):
			target := blockedTargetFromMessage(de.Message)
			lines := []string{
				"❌ " + integrationBlockedSummary(failure, target) + ".",
			}
			return lines
		}
		if strings.TrimSpace(de.Message) != "" {
			return []string{"❌ " + de.Message}
		}
	}
	return []string{"❌ " + err.Error()}
}

func integrationFailureSummary(failure integrationFailureContext) string {
	switch failure.Action {
	case "add":
		if strings.TrimSpace(failure.Name) != "" {
			return fmt.Sprintf("Install for %q", failure.Name)
		}
		return "Install"
	case "update":
		if strings.TrimSpace(failure.Name) != "" {
			return fmt.Sprintf("Update for %q", failure.Name)
		}
		return "Update"
	case "remove":
		if strings.TrimSpace(failure.Name) != "" {
			return fmt.Sprintf("Remove for %q", failure.Name)
		}
		return "Remove"
	case "repair":
		if strings.TrimSpace(failure.Name) != "" && strings.TrimSpace(failure.Target) != "" {
			return fmt.Sprintf("Repair for %q on target %q", failure.Name, failure.Target)
		}
		if strings.TrimSpace(failure.Name) != "" {
			return fmt.Sprintf("Repair for %q", failure.Name)
		}
		return "Repair"
	default:
		return "Operation"
	}
}

func isBlockedMutationError(err error) bool {
	var de *domain.Error
	return errors.As(err, &de) && (de.Code == domain.ErrMutationApply || de.Code == domain.ErrRepairApply) && strings.Contains(de.Message, "planned mutation is blocked for target ")
}

func integrationBlockedSummary(failure integrationFailureContext, target string) string {
	summary := integrationFailureSummary(failure) + " is blocked"
	if strings.TrimSpace(target) != "" {
		summary += fmt.Sprintf(" for target %q", target)
	}
	return summary
}

func integrationIDFromMessage(message string) string {
	const prefix = "integration already exists in state: "
	if strings.HasPrefix(message, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(message, prefix))
	}
	return ""
}

func blockedTargetFromMessage(message string) string {
	const prefix = "planned mutation is blocked for target "
	if !strings.HasPrefix(message, prefix) {
		return ""
	}
	rest := strings.TrimSpace(strings.TrimPrefix(message, prefix))
	if idx := strings.Index(rest, ";"); idx >= 0 {
		rest = rest[:idx]
	}
	return strings.TrimSpace(rest)
}

func unsupportedTargetFromMessage(message string) string {
	const prefix = "manifest does not expose target "
	if !strings.HasPrefix(message, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(message, prefix))
}

func missingIntegrationFromMessage(message string) string {
	const prefix = "integration not found in state: "
	if !strings.HasPrefix(message, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(message, prefix))
}
