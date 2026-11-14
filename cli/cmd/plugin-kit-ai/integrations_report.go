package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/spf13/cobra"
)

func validateUpdateArgs(all bool, args []string) error {
	if all {
		if len(args) != 0 {
			return fmt.Errorf("update --all does not accept a name")
		}
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("update requires exactly one integration name unless --all is set")
	}
	return nil
}

func printIntegrationReport(cmd *cobra.Command, report integrationctl.Report) {
	mode := integrationReportMode(report.Summary)
	if strings.TrimSpace(report.OperationID) != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🆔 Operation: %s\n", report.OperationID)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", integrationSummaryEmoji(mode, report.Summary), report.Summary)
	if overview := integrationProgressOverview(mode, report); overview != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "📊 %s\n", overview)
	}
	if note := integrationProgressNote(mode, report.Targets); note != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", note)
	}
	for _, line := range integrationQuickSummaryLines(mode, report) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", line)
	}
	for _, warning := range report.Warnings {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "⚠️ Warning: %s\n", warning)
	}
	if len(report.Targets) == 0 {
		return
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	if shouldGroupIntegrationTargets(report.Targets) {
		printGroupedIntegrationTargets(cmd, mode, report.Targets)
		return
	}
	for i, target := range report.Targets {
		if i > 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}
		printIntegrationTargetWithIndent(cmd, mode, target, "")
	}
}

type integrationReportRenderMode int

const (
	integrationReportNeutral integrationReportRenderMode = iota
	integrationReportPlan
	integrationReportApplied
)

func integrationReportMode(summary string) integrationReportRenderMode {
	switch {
	case strings.HasPrefix(summary, "Dry-run "), strings.Contains(summary, " plan for "):
		return integrationReportPlan
	case strings.HasPrefix(summary, "Installed integration "),
		strings.HasPrefix(summary, "Updated integration "),
		strings.HasPrefix(summary, "Removed managed targets "),
		strings.HasPrefix(summary, "Repaired managed targets "),
		strings.HasPrefix(summary, "Enabled managed targets "),
		strings.HasPrefix(summary, "Disabled managed targets "),
		strings.HasPrefix(summary, "Applied "):
		return integrationReportApplied
	default:
		return integrationReportNeutral
	}
}

func printIntegrationTarget(cmd *cobra.Command, mode integrationReportRenderMode, target domain.TargetReport) {
	printIntegrationTargetWithIndent(cmd, mode, target, "")
}

func printIntegrationTargetWithIndent(cmd *cobra.Command, mode integrationReportRenderMode, target domain.TargetReport, indent string) {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%s %s - %s\n", indent, integrationTargetEmoji(mode, target), target.TargetID, integrationTargetHeadline(mode, target.ActionClass, target.State, target.ActivationState, target.DeliveryKind))
	switch mode {
	case integrationReportApplied:
		printAppliedIntegrationTarget(cmd, target, indent+"  ")
	default:
		printPlannedOrNeutralIntegrationTarget(cmd, target, indent+"  ")
	}
}

func printPlannedOrNeutralIntegrationTarget(cmd *cobra.Command, target domain.TargetReport, indent string) {
	if current := integrationCurrentStatus(target.State, target.ActivationState); current != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%scurrent - %s\n", indent, current)
	}
	if activation := integrationActivationStatus(target.ActivationState, target.EnvironmentRestrictions); activation != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sactivation - %s\n", indent, activation)
	}
	for _, step := range target.ManualSteps {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%snext - %s\n", indent, step)
	}
	for _, restriction := range target.EnvironmentRestrictions {
		if note := integrationRestrictionNote(restriction, target.ActivationState); note != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%snote - %s\n", indent, note)
		}
	}
}

func printAppliedIntegrationTarget(cmd *cobra.Command, target domain.TargetReport, indent string) {
	followUpLabel := integrationAppliedFollowUpLabel(target)
	followUpLines := integrationAppliedFollowUpLines(target)
	requiredSteps := integrationRequiredManualSteps(target)
	if ready := integrationAppliedReadyLine(target.State); ready != "" && !integrationTargetIsPartial(target) && !integrationTargetNeedsFollowUp(target) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sready - %s\n", indent, ready)
	}
	if len(requiredSteps) > 0 && followUpLabel != "" {
		for _, step := range requiredSteps {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%s - %s\n", indent, followUpLabel, step)
		}
	} else if len(followUpLines) > 0 && followUpLabel != "" {
		for _, line := range followUpLines {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%s - %s\n", indent, followUpLabel, line)
		}
	} else if ready := integrationAppliedReadyLine(target.State); ready != "" && integrationTargetIsPartial(target) && !integrationTargetNeedsFollowUp(target) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%sready - %s\n", indent, ready)
	}
	if tip := integrationTargetOptionalTip(target); tip != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%stip - %s\n", indent, tip)
	}
	for _, restriction := range target.EnvironmentRestrictions {
		if note := integrationRestrictionNote(restriction, target.ActivationState); note != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%snote - %s\n", indent, note)
		}
	}
}

func shouldGroupIntegrationTargets(targets []domain.TargetReport) bool {
	seen := map[string]struct{}{}
	for _, target := range targets {
		if strings.TrimSpace(target.IntegrationID) == "" {
			continue
		}
		seen[target.IntegrationID] = struct{}{}
		if len(seen) > 1 {
			return true
		}
	}
	return false
}

func printGroupedIntegrationTargets(cmd *cobra.Command, mode integrationReportRenderMode, targets []domain.TargetReport) {
	order, groups := integrationTargetGroups(targets)
	for groupIndex, integrationID := range order {
		if groupIndex > 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout())
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "📦 %s\n", integrationID)
		for i, target := range groups[integrationID] {
			if i > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
			printIntegrationTargetWithIndent(cmd, mode, target, "  ")
		}
	}
}

func integrationTargetGroups(targets []domain.TargetReport) ([]string, map[string][]domain.TargetReport) {
	order := make([]string, 0, len(targets))
	groups := map[string][]domain.TargetReport{}
	for _, target := range targets {
		key := strings.TrimSpace(target.IntegrationID)
		if key == "" {
			key = "other"
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], target)
	}
	return order, groups
}

func integrationTargetHeadline(mode integrationReportRenderMode, actionClass, state, activationState, deliveryKind string) string {
	if mode == integrationReportApplied {
		return integrationAppliedHeadline(actionClass, state, activationState, deliveryKind)
	}
	delivery := integrationDeliveryLabel(deliveryKind)
	if action := integrationActionLabel(actionClass, state); action != "" {
		return action + " " + delivery
	}
	return delivery
}

func integrationAppliedHeadline(actionClass, state, activationState, deliveryKind string) string {
	delivery := integrationDeliveryLabel(deliveryKind)
	switch actionClass {
	case "remove_orphaned_target":
		switch state {
		case "removed":
			return "removed " + delivery
		case "degraded":
			return "partially removed " + delivery
		}
	case "disable_target":
		return "disabled " + delivery
	case "enable_target":
		if state == "installed" {
			return "enabled " + delivery
		}
	}
	switch state {
	case "installed":
		return "installed " + delivery
	case "prepared", "activation_pending":
		if deliveryKind == "codex-marketplace-plugin" || activationState == "native_activation_pending" {
			return "prepared " + delivery + " for in-app install"
		}
		return "prepared " + delivery
	case "disabled":
		return "installed " + delivery + " but it is currently disabled"
	case "degraded":
		return "partially installed " + delivery
	case "removed":
		return "removed " + delivery
	default:
		return delivery
	}
}

func integrationActionLabel(actionClass, state string) string {
	switch actionClass {
	case "install_missing":
		switch state {
		case "installed", "disabled":
			return "will adopt existing"
		case "prepared", "activation_pending", "degraded":
			return "will finish preparing"
		default:
			return "will install"
		}
	case "update_version":
		return "will update"
	case "adopt_new_target":
		return "will add target support for"
	case "migrate_source":
		return "will migrate"
	case "repair_drift":
		return "will repair"
	case "remove_orphaned_target":
		return "will remove"
	case "await_activation":
		return "is waiting for activation for"
	case "await_auth_completion":
		return "is waiting for authentication for"
	case "noop":
		return "no changes for"
	default:
		return ""
	}
}

func integrationDeliveryLabel(deliveryKind string) string {
	switch deliveryKind {
	case "claude-marketplace-plugin":
		return "Claude plugin"
	case "codex-marketplace-plugin":
		return "Codex plugin"
	case "gemini-extension":
		return "Gemini extension"
	case "cursor-mcp":
		return "Cursor MCP setup"
	case "opencode-plugin":
		return "OpenCode plugin"
	default:
		if strings.TrimSpace(deliveryKind) == "" {
			return "integration target"
		}
		return deliveryKind
	}
}

func integrationCurrentStatus(state, activationState string) string {
	switch state {
	case "installed":
		return "already installed"
	case "removed":
		return "not currently installed"
	case "prepared":
		return "prepared locally but not yet installed"
	case "activation_pending":
		return "prepared, but the target app has not activated it yet"
	case "auth_pending":
		return "waiting for interactive authentication"
	case "disabled":
		return "installed but currently disabled"
	case "degraded":
		if activationState == "native_activation_pending" {
			return "partially prepared and still needs activation in the target app"
		}
		return "partially installed and may need repair"
	default:
		return ""
	}
}

func integrationActivationStatus(activationState string, restrictions []string) string {
	switch activationState {
	case "reload_pending":
		return "reload the target app or plugin list after applying changes"
	case "restart_pending":
		return "restart the target app after applying changes"
	case "new_thread_pending":
		return "start a new thread or session after applying changes"
	case "native_activation_pending":
		if containsString(restrictions, "new_thread_required") {
			return "complete the native install in the target app, then start a new thread or session"
		}
		return "complete the native install in the target app"
	default:
		return ""
	}
}

func integrationRestrictionNote(restriction, activationState string) string {
	switch restriction {
	case "native_activation_required":
		if activationState == "native_activation_pending" {
			return ""
		}
		return "the target app still requires a native activation step"
	case "reload_required":
		if activationState == "reload_pending" {
			return ""
		}
		return "reload the target app or plugin list after applying changes"
	case "restart_required":
		if activationState == "restart_pending" {
			return ""
		}
		return "restart the target app after applying changes"
	case "new_thread_required":
		if activationState == "new_thread_pending" || activationState == "native_activation_pending" {
			return ""
		}
		return "start a new thread or session after applying changes"
	case "managed_policy_block":
		return "a managed policy currently blocks automatic changes here"
	case "trust_required":
		return "the target app requires trust or allowlist approval before this can work"
	case "source_auth_required":
		return "the source needs authentication before the integration can be fetched"
	case "native_auth_required":
		return "the target app needs an interactive sign-in step before this can be used"
	case "source_tool_missing":
		return "the target app or CLI does not appear to be installed or on PATH"
	case "source_shape_unsupported":
		return "the source shape is not supported for this target"
	case "read_only_native_layer":
		return "the target config layer is read-only here"
	case "volatile_override_layer":
		return "another higher-priority config layer may override this change"
	default:
		return restriction
	}
}

func integrationAppliedReadyLine(state string) string {
	switch state {
	case "installed":
		return "available now"
	case "removed":
		return "fully removed"
	default:
		return ""
	}
}

func integrationSummaryEmoji(mode integrationReportRenderMode, summary string) string {
	switch mode {
	case integrationReportPlan:
		return "🧭"
	case integrationReportApplied:
		switch {
		case strings.HasPrefix(summary, "Removed "):
			return "🗑️"
		case strings.HasPrefix(summary, "Disabled "):
			return "⏸️"
		default:
			return "✅"
		}
	default:
		return "ℹ️"
	}
}

func integrationTargetEmoji(mode integrationReportRenderMode, target domain.TargetReport) string {
	if integrationTargetBlocked(target) {
		return "🚫"
	}
	if mode == integrationReportApplied {
		if integrationTargetNeedsFollowUp(target) || integrationTargetIsPartial(target) {
			return "🟡"
		}
		return "✅"
	}
	switch target.State {
	case "installed", "disabled":
		return "✅"
	case "removed":
		return "⬇️"
	case "prepared", "activation_pending", "degraded", "auth_pending":
		return "🟡"
	default:
		return "🔹"
	}
}

func integrationProgressOverview(mode integrationReportRenderMode, report integrationctl.Report) string {
	targets := report.Targets
	if len(targets) == 0 {
		return ""
	}
	switch mode {
	case integrationReportApplied:
		requested := report.RequestedTargetCount
		if requested <= 0 {
			requested = len(targets)
		}
		if requested < len(targets) {
			requested = len(targets)
		}
		if requested > len(targets) {
			return fmt.Sprintf("Applied changes: %s %d/%d requested target(s)", integrationProgressBar(len(targets), requested), len(targets), requested)
		}
		return fmt.Sprintf("Progress: %s %d/%d target(s) changed successfully", integrationProgressBar(len(targets), len(targets)), len(targets), len(targets))
	case integrationReportPlan:
		alreadyPresent := 0
		needsAttention := 0
		newInstall := 0
		blocked := 0
		for _, target := range targets {
			if integrationTargetBlocked(target) {
				blocked++
			}
			switch target.State {
			case "installed", "disabled":
				alreadyPresent++
			case "prepared", "activation_pending", "degraded", "auth_pending":
				needsAttention++
			default:
				newInstall++
			}
		}
		summary := fmt.Sprintf("Preview: %d target(s) already present, %d need attention, %d would be newly installed", alreadyPresent, needsAttention, newInstall)
		if blocked > 0 {
			summary += fmt.Sprintf(", %d blocked", blocked)
		}
		return summary
	default:
		return fmt.Sprintf("%d target(s)", len(targets))
	}
}

func integrationProgressNote(mode integrationReportRenderMode, targets []domain.TargetReport) string {
	if mode != integrationReportApplied {
		return ""
	}
	if len(integrationTargetIDsByState(targets, integrationTargetNeedsCompletion)) > 0 {
		return "📝 Changes were written successfully, but some targets are only prepared until you finish the steps below."
	}
	if len(integrationTargetIDsByState(targets, integrationTargetNeedsActivation)) > 0 {
		return "📝 Changes were written successfully. A few targets still need a restart, reload, or one last activation step."
	}
	return ""
}

func integrationQuickSummaryLines(mode integrationReportRenderMode, report integrationctl.Report) []string {
	targets := report.Targets
	if len(targets) == 0 {
		return nil
	}
	lines := []string{}
	switch mode {
	case integrationReportPlan:
		alreadyPresent := integrationTargetIDsByState(targets, func(target domain.TargetReport) bool {
			return target.State == "installed" || target.State == "disabled"
		})
		needsAttention := integrationTargetIDsByState(targets, func(target domain.TargetReport) bool {
			return target.State == "prepared" || target.State == "activation_pending" || target.State == "degraded" || target.State == "auth_pending"
		})
		newInstall := integrationTargetIDsByState(targets, func(target domain.TargetReport) bool {
			return target.State != "installed" && target.State != "disabled" && target.State != "prepared" && target.State != "activation_pending" && target.State != "degraded" && target.State != "auth_pending"
		})
		blocked := integrationTargetIDsByState(targets, integrationTargetBlocked)
		if len(alreadyPresent) > 0 {
			lines = append(lines, "✅ Already present: "+strings.Join(alreadyPresent, ", "))
		}
		if len(needsAttention) > 0 {
			lines = append(lines, "🟡 Needs attention: "+strings.Join(needsAttention, ", "))
		}
		if len(newInstall) > 0 {
			lines = append(lines, "⬇️ New install: "+strings.Join(newInstall, ", "))
		}
		if len(blocked) > 0 {
			lines = append(lines, "🚫 Blocked: "+strings.Join(blocked, ", "))
		}
	case integrationReportApplied:
		if len(report.SkippedTargets) > 0 {
			lines = append(lines, "⏭️ Skipped: "+strings.Join(report.SkippedTargets, ", "))
		}
		readyNow := integrationTargetIDsByState(targets, integrationTargetReadyNow)
		restartOrReload := integrationTargetIDsByState(targets, integrationTargetNeedsRestartOrReload)
		reopenSession := integrationTargetIDsByState(targets, integrationTargetNeedsNewSession)
		activateInApp := integrationTargetIDsByState(targets, integrationTargetNeedsNativeActivation)
		signIn := integrationTargetIDsByState(targets, integrationTargetNeedsSignIn)
		finishSetup := integrationTargetIDsByState(targets, integrationTargetNeedsFinishSetup)
		if len(readyNow) > 0 {
			lines = append(lines, "🚀 Ready now: "+strings.Join(readyNow, ", "))
		}
		if len(restartOrReload) > 0 {
			lines = append(lines, "🔄 Restart or reload: "+strings.Join(restartOrReload, ", "))
		}
		if len(reopenSession) > 0 {
			lines = append(lines, "🪟 Open a new session: "+strings.Join(reopenSession, ", "))
		}
		if len(activateInApp) > 0 {
			lines = append(lines, "⚡ Activate in the app: "+strings.Join(activateInApp, ", "))
		}
		if len(signIn) > 0 {
			lines = append(lines, "🔐 Sign in: "+strings.Join(signIn, ", "))
		}
		if len(finishSetup) > 0 {
			lines = append(lines, "🧩 Finish setup: "+strings.Join(finishSetup, ", "))
		}
	}
	return lines
}

func integrationTargetIDsByState(targets []domain.TargetReport, keep func(domain.TargetReport) bool) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		if keep(target) {
			out = append(out, target.TargetID)
		}
	}
	return out
}

func integrationTargetNeedsFollowUp(target domain.TargetReport) bool {
	return integrationTargetNeedsCompletion(target) || integrationTargetNeedsActivation(target)
}

func integrationRequiredActivationStatus(target domain.TargetReport) string {
	if integrationTargetOptionalTip(target) != "" {
		return ""
	}
	return integrationActivationStatus(target.ActivationState, target.EnvironmentRestrictions)
}

func integrationRequiredManualSteps(target domain.TargetReport) []string {
	if len(target.ManualSteps) == 0 {
		return nil
	}
	tip := integrationTargetOptionalTip(target)
	out := make([]string, 0, len(target.ManualSteps))
	for _, step := range target.ManualSteps {
		if step == tip {
			continue
		}
		out = append(out, step)
	}
	return out
}

func integrationTargetOptionalTip(target domain.TargetReport) string {
	if target.TargetID != "claude" || target.ActivationState != "reload_pending" {
		return ""
	}
	for _, step := range target.ManualSteps {
		if strings.Contains(step, "current session") || strings.Contains(step, "pick up the new plugin immediately") {
			return step
		}
	}
	return ""
}

func integrationAppliedFollowUpLabel(target domain.TargetReport) string {
	if integrationTargetNeedsCompletion(target) {
		return "finish"
	}
	if target.ActionClass == "remove_orphaned_target" && len(integrationRequiredManualSteps(target)) > 0 {
		switch target.ActivationState {
		case "restart_pending":
			return "restart"
		case "reload_pending":
			return "reload"
		case "new_thread_pending":
			return "reopen"
		default:
			return "finish"
		}
	}
	switch target.ActivationState {
	case "reload_pending":
		return "reload"
	case "restart_pending":
		return "restart"
	case "new_thread_pending":
		return "reopen"
	case "native_activation_pending":
		return "activate"
	default:
		if len(integrationRequiredManualSteps(target)) > 0 {
			return "activate"
		}
		return ""
	}
}

func integrationAppliedFollowUpLines(target domain.TargetReport) []string {
	if steps := integrationRequiredManualSteps(target); len(steps) > 0 {
		return steps
	}
	if status := integrationRequiredActivationStatus(target); status != "" {
		return []string{status}
	}
	return nil
}

func integrationTargetReadyNow(target domain.TargetReport) bool {
	return !integrationTargetNeedsCompletion(target) && !integrationTargetNeedsActivation(target)
}

func integrationTargetNeedsCompletion(target domain.TargetReport) bool {
	return integrationTargetIsPartial(target)
}

func integrationTargetNeedsActivation(target domain.TargetReport) bool {
	if integrationTargetNeedsCompletion(target) {
		return false
	}
	return len(integrationAppliedFollowUpLines(target)) > 0
}

func integrationTargetNeedsRestartOrReload(target domain.TargetReport) bool {
	if !integrationTargetNeedsActivation(target) {
		return false
	}
	return target.ActivationState == "reload_pending" || target.ActivationState == "restart_pending"
}

func integrationTargetNeedsNewSession(target domain.TargetReport) bool {
	return integrationTargetNeedsActivation(target) && target.ActivationState == "new_thread_pending"
}

func integrationTargetNeedsNativeActivation(target domain.TargetReport) bool {
	return integrationTargetNeedsActivation(target) && target.ActivationState == "native_activation_pending"
}

func integrationTargetNeedsSignIn(target domain.TargetReport) bool {
	if !integrationTargetNeedsCompletion(target) {
		return false
	}
	if target.State == "auth_pending" {
		return true
	}
	for _, restriction := range target.EnvironmentRestrictions {
		if restriction == "native_auth_required" || restriction == "source_auth_required" {
			return true
		}
	}
	return false
}

func integrationTargetNeedsFinishSetup(target domain.TargetReport) bool {
	return integrationTargetNeedsCompletion(target) && !integrationTargetNeedsSignIn(target)
}

func integrationTargetIsPartial(target domain.TargetReport) bool {
	switch target.State {
	case "prepared", "activation_pending", "degraded", "auth_pending":
		return true
	default:
		return false
	}
}

func integrationTargetBlocked(target domain.TargetReport) bool {
	for _, restriction := range target.EnvironmentRestrictions {
		switch restriction {
		case "managed_policy_block", "trust_required", "source_auth_required", "native_auth_required", "source_tool_missing", "source_shape_unsupported", "read_only_native_layer":
			return true
		}
	}
	return false
}

func integrationProgressBar(done, total int) string {
	if total <= 0 {
		return ""
	}
	const width = 8
	filled := done * width / total
	if done > 0 && filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func boolPtr(v bool) *bool { return &v }

func firstNormalizedTarget(values []string) string {
	normalized := integrationctl.NormalizeTargets(values)
	if len(normalized) == 0 {
		return ""
	}
	return normalized[0]
}
