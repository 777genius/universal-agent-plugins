package agentpluginscli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

func renderAddMultiResult(cmd *cobra.Command, opts *options, result addMultiResult, envelope domain.PackageEnvelope) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 || batchStatusIndicatesFailure(result.Status) {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "add", overall, struct {
			addMultiResult
			Environment map[string]any `json:"environment"`
		}{result, map[string]any{"saved": false, "reason": opts.environmentReason}})
	}
	if result.Status == "preflight_failed" {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Nothing was installed: a selected client could not pass the checks before installation. See the error below; there is no new configuration to activate."); err != nil {
			return err
		}
		for _, target := range result.Targets {
			if target.Output.Result.Plan.Status == domain.PlanUnsupported {
				if err := renderHumanPlan(cmd.OutOrStdout(), envelope, target.Output.Result); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if result.Status == "preparation_failed" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No selected client was changed: package preparation failed. See the error below, then retry the same command.")
		return err
	}
	if result.DryRun {
		if len(result.Targets) == 1 {
			return renderAddResult(cmd.OutOrStdout(), "human", envelope, result.Targets[0].Output.Result, true)
		}
		return renderAddMultiDryRun(cmd.OutOrStdout(), result)
	}
	// Failed single-target applies use the same summary so error detail and retry
	// guidance stay visible instead of the legacy "Add: external_failed" line.
	if len(result.Targets) == 1 && result.Failed == 0 && !batchStatusIndicatesFailure(result.Status) {
		return renderAddResult(cmd.OutOrStdout(), "human", envelope, result.Targets[0].Output.Result, false)
	}
	return renderAddMultiApplySummary(cmd.OutOrStdout(), result, envelope)
}

func renderAddMultiDryRun(writer io.Writer, result addMultiResult) error {
	if _, err := fmt.Fprintf(writer, "Plugin: %s %s\n", result.Plugin, result.Version); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Targets: %s\n", addResultTargets(result.Targets)); err != nil {
		return err
	}
	for _, target := range result.Targets {
		if target.Status == "action_required" {
			if _, err := fmt.Fprintf(writer, "  %s: setup required\n    Next: %s\n", target.Target, target.NextAction); err != nil {
				return err
			}
			continue
		}
		if err := renderOpenCodeRuntimeNotice(writer, target.Output.Result); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "  %s: %s\n", target.Target, target.Status); err != nil {
			return err
		}
		if target.NextAction != "" && !fullyInstalled(target.Output.Result.Activation) {
			if _, err := fmt.Fprintf(writer, "    Next: %s\n", localTargetLifecycleAction(target.Output.Result, target.NextAction)); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(writer, "No changes made (dry run)."); err != nil {
		return err
	}
	return nil
}

func renderAddMultiApplySummary(writer io.Writer, result addMultiResult, envelope domain.PackageEnvelope) error {
	theme := terminaltheme.For(writer)
	title := batchApplyHeadline(result, envelope)
	if _, err := fmt.Fprintln(writer, theme.Text(terminaltheme.Label, title)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	width := utf8.RuneCountInString("Client")
	for _, target := range result.Targets {
		if name := batchTargetLabel(target); utf8.RuneCountInString(name) > width {
			width = utf8.RuneCountInString(name)
		}
	}
	if _, err := fmt.Fprintf(writer, "%-*s  %s\n", width, "Client", "Result"); err != nil {
		return err
	}
	for _, target := range result.Targets {
		label := batchTargetLabel(target)
		kind := classifyBatchPresentation(target)
		if _, err := fmt.Fprintf(writer, "%-*s  %s\n", width, prompt.SafeText(label), theme.Text(batchResultTone(kind), string(kind))); err != nil {
			return err
		}
	}
	attention := make([]addTargetResult, 0, len(result.Targets))
	for _, target := range result.Targets {
		switch classifyBatchPresentation(target) {
		case batchPresentationAttested, batchPresentationSetupRequired, batchPresentationSignInRequired, batchPresentationFailed, batchPresentationNotCompleted, batchPresentationRolledBack:
			attention = append(attention, target)
		}
	}
	if len(attention) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, theme.Text(terminaltheme.Warning, "Needs attention")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	for index, target := range attention {
		if index > 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(writer, prompt.SafeText(batchTargetLabel(target))); err != nil {
			return err
		}
		if err := renderOpenCodeRuntimeNotice(writer, target.Output.Result); err != nil {
			return err
		}
		for _, line := range batchAttentionLines(target) {
			for _, part := range strings.Split(line, "\n") {
				if strings.TrimSpace(part) == "" {
					continue
				}
				if _, err := fmt.Fprintf(writer, "  %s\n", prompt.SafeText(part)); err != nil {
					return err
				}
			}
		}
		if target.RetryCommand != "" {
			if _, err := fmt.Fprintln(writer, "  Retry:"); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "    %s\n", prompt.SafeText(target.RetryCommand)); err != nil {
				return err
			}
		}
	}
	return nil
}

func batchApplyHeadline(result addMultiResult, envelope domain.PackageEnvelope) string {
	name := strings.TrimSpace(envelope.Manifest.Name)
	if name == "" {
		name = result.Plugin
	}
	switch {
	case result.Failed > 0 && result.Succeeded > 0:
		return name + " installation finished with issues"
	case result.Failed > 0:
		return name + " installation failed"
	case result.ActionRequired > 0:
		return name + " installation prepared"
	default:
		return name + " installation completed"
	}
}

func batchTargetLabel(target addTargetResult) string {
	if strings.TrimSpace(target.displayName) != "" {
		return target.displayName
	}
	if definition, ok := domain.ClientDefinitionFor(domain.ClientID(target.Target)); ok && definition.DisplayName != "" {
		return definition.DisplayName
	}
	return target.Target
}

func classifyBatchPresentation(target addTargetResult) batchPresentationKind {
	if target.Status == "action_required" {
		return batchPresentationSetupRequired
	}
	phase := target.Output.Result.GroupPhase
	switch phase {
	case usecase.GroupTargetManagedRolledBack:
		return batchPresentationRolledBack
	case usecase.GroupTargetPlanned, usecase.GroupTargetManagedCommitted, usecase.GroupTargetExternalNotAttempted:
		return batchPresentationNotCompleted
	case usecase.GroupTargetExternalFailed, usecase.GroupTargetExternalPartial, usecase.GroupTargetManagedUnknown:
		return batchPresentationFailed
	}
	activation := target.Output.Result.Activation
	if target.Error != nil || activation.Activation == domain.ActivationFailed || activation.Authentication == domain.AuthenticationFailed || activation.Verification == domain.VerificationFailed {
		return batchPresentationFailed
	}
	if activation.Authentication == domain.AuthenticationPending {
		return batchPresentationSignInRequired
	}
	if fullyInstalled(activation) {
		if activation.ActivationAttested || activation.AuthenticationAttested {
			return batchPresentationAttested
		}
		return batchPresentationInstalled
	}
	if activation.Authentication == domain.AuthenticationNotChecked {
		return batchPresentationSetupRequired
	}
	if activation.Activation == domain.ActivationPrepared || activation.Activation == domain.ActivationManual || target.Status == string(usecase.GroupTargetExternalCompleted) {
		return batchPresentationSetupRequired
	}
	if phase == usecase.GroupTargetExternalCompleted {
		return batchPresentationSetupRequired
	}
	return batchPresentationNotCompleted
}

func batchResultTone(kind batchPresentationKind) terminaltheme.Role {
	switch kind {
	case batchPresentationInstalled:
		return terminaltheme.Success
	case batchPresentationAttested, batchPresentationSetupRequired, batchPresentationSignInRequired:
		return terminaltheme.Warning
	case batchPresentationFailed, batchPresentationNotCompleted, batchPresentationRolledBack:
		return terminaltheme.Error
	default:
		return terminaltheme.Muted
	}
}

func batchAttentionLines(target addTargetResult) []string {
	kind := classifyBatchPresentation(target)
	phase := target.Output.Result.GroupPhase
	switch kind {
	case batchPresentationAttested:
		return []string{"Lifecycle was explicitly attested by the user; it was not observed from the client."}
	case batchPresentationRolledBack:
		return []string{"Managed installation was rolled back; no client changes were kept."}
	case batchPresentationFailed, batchPresentationNotCompleted:
		if phase == usecase.GroupTargetManagedUnknown {
			return []string{"Managed commit state is unknown; run npx universal-agent-plugins doctor before retrying."}
		}
		if batchFailureNeedsDoctor(target) {
			return []string{"Installation state could not be saved safely; run npx universal-agent-plugins doctor before retrying."}
		}
		if target.Error != nil && strings.TrimSpace(target.Error.Message) != "" {
			return []string{target.Error.Message}
		}
		if action := localTargetLifecycleAction(target.Output.Result, target.NextAction); action != "" {
			return []string{action}
		}
		if kind == batchPresentationNotCompleted {
			return []string{"Client installation was not attempted."}
		}
		return []string{"Client installation failed."}
	default:
		action := localTargetLifecycleAction(target.Output.Result, target.NextAction)
		if action == "" {
			action = nextLifecycleAction(target.Output.Result)
		}
		if action == "" {
			return []string{"Finish setup in the selected client."}
		}
		return []string{action}
	}
}

func countBatchTarget(result *addMultiResult, target addTargetResult) {
	switch classifyBatchPresentation(target) {
	case batchPresentationInstalled, batchPresentationAttested:
		result.Succeeded++
	case batchPresentationSetupRequired, batchPresentationSignInRequired:
		// Deferred ChatGPT (status action_required, no group phase) is reported
		// separately and must not inflate succeeded beyond prepared peers.
		if target.Output.Result.GroupPhase == usecase.GroupTargetExternalCompleted || fullyInstalled(target.Output.Result.Activation) {
			result.Succeeded++
		}
		result.ActionRequired++
	case batchPresentationFailed, batchPresentationNotCompleted, batchPresentationRolledBack:
		result.Failed++
	default:
		result.Failed++
	}
}

func batchRetrySource(envelope domain.PackageEnvelope, origin domain.OriginMode, public string) string {
	// Directory retries must keep the safe product selector (for example context7),
	// not a reconstructed repository//path that changes provenance.
	if origin == domain.OriginModeDirectory {
		if source := strings.TrimSpace(envelope.Source.RequestedSource); isBatchRetryableSource(source) {
			return source
		}
		return ""
	}
	// Direct GitHub retries require an immutable owner/repo@FULL_SHA[//path] selector.
	if repo := strings.TrimSpace(envelope.Source.Repository); repo != "" {
		return batchRetryGitHubSource(envelope.Source)
	}
	if source := strings.TrimSpace(envelope.Source.RequestedSource); isBatchRetryableSource(source) {
		return source
	}
	if source := strings.TrimSpace(public); isBatchRetryableSource(source) {
		return source
	}
	return ""
}

func batchRetryGitHubSource(source domain.SourceIdentity) string {
	repo := strings.TrimSpace(source.Repository)
	rev := publicImmutableRevision(strings.TrimSpace(source.ResolvedRevision))
	if repo == "" || rev == "" {
		return ""
	}
	value := repo + "@" + rev
	if sub := strings.TrimSpace(source.PackageSubpath); sub != "" {
		value += "//" + sub
	}
	if !isBatchRetrySafeArg(value) {
		return ""
	}
	return value
}

func isBatchPresentationSource(source string) bool {
	switch strings.TrimSpace(source) {
	case "", "direct local source", "direct immutable source":
		return true
	default:
		return false
	}
}

func isBatchRetryableSource(source string) bool {
	source = strings.TrimSpace(source)
	if source == "" || isBatchPresentationSource(source) || isLocalFilesystemSource(source) {
		return false
	}
	return true
}

func isLocalFilesystemSource(source string) bool {
	source = strings.TrimSpace(source)
	if source == "" || strings.Contains(source, "://") {
		return false
	}
	// Match acquisition's portable local-path detector so Windows spellings such
	// as d:/plugin are suppressed on every host, not only on Windows.
	if strings.HasPrefix(source, "~") {
		return true
	}
	return explicitLocalPath(source)
}
