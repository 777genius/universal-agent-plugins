package agentpluginscli

import (
	"fmt"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func renderHumanPlan(writer io.Writer, envelope domain.PackageEnvelope, result usecase.AddResult) error {
	result = withOpenCodeRuntimeNotice(result)
	checked := &planWriter{writer: writer}
	writer = checked
	_, _ = fmt.Fprintf(writer, "%s: %s %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Plugin"), prompt.SafeText(string(envelope.Manifest.Name)), prompt.SafeText(string(envelope.Manifest.Version)))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Target"), prompt.SafeText(string(result.Plan.ClientID)))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Package"), prompt.SafeText(string(result.Plan.PackageMode)))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Result"), prompt.SafeText(string(result.Plan.Status)))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Authentication"), prompt.SafeText(string(result.Plan.Authentication)))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Verification"), prompt.SafeText(string(result.Plan.Verification)))
	for _, component := range result.Plan.Components {
		_, _ = fmt.Fprintf(writer, "  - %s %s: %s\n", prompt.SafeText(string(component.Kind)), prompt.SafeText(string(component.Name)), prompt.SafeText(string(component.Support)))
	}
	for _, diagnostic := range result.Plan.Diagnostics {
		_, _ = fmt.Fprintf(writer, "  %s: %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Warning, "Warning"), prompt.SafeText(string(diagnostic.Code)), prompt.SafeText(string(diagnostic.Message)))
	}
	for _, warning := range result.Plan.Warnings {
		_, _ = fmt.Fprintf(writer, "  %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Warning, "Warning"), prompt.SafeText(string(warning)))
	}
	for _, action := range result.Plan.UserActions {
		_, _ = fmt.Fprintf(writer, "  %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Planned action"), prompt.SafeText(string(action)))
	}
	for _, action := range result.Plan.LocalActions {
		_, _ = fmt.Fprintf(writer, "  %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Planned action"), prompt.SafeText(string(action)))
	}
	return checked.err
}

func renderAddResult(writer io.Writer, format string, envelope domain.PackageEnvelope, result usecase.AddResult, dryRun bool) error {
	return renderAddResultError(writer, format, envelope, result, dryRun, nil)
}

func renderAddResultWithSecurity(writer io.Writer, format string, envelope domain.PackageEnvelope, security *domain.SecurityAssessment, result usecase.AddResult, dryRun bool) error {
	return renderAddResultErrorWithSecurity(writer, format, envelope, security, result, dryRun, nil)
}

func renderAddResultError(writer io.Writer, format string, envelope domain.PackageEnvelope, result usecase.AddResult, dryRun bool, commandErr error) error {
	return renderAddResultErrorWithSecurity(writer, format, envelope, nil, result, dryRun, commandErr)
}

func renderAddResultErrorWithSecurity(writer io.Writer, format string, envelope domain.PackageEnvelope, security *domain.SecurityAssessment, result usecase.AddResult, dryRun bool, commandErr error) error {
	data := newAddResultData(envelope, result, dryRun)
	data.Security = security
	data.envelopeResult = addEnvelopeResult(result, commandErr)
	if format == "json" {
		return writeJSONOutput(writer, "add", data)
	}
	done, err := renderHumanAddFailure(writer, envelope, result, commandErr)
	if err != nil || done {
		return err
	}
	if dryRun {
		return renderHumanPlan(writer, envelope, result)
	}
	return renderHumanAddOutcome(writer, result)
}

func renderHumanAddFailure(writer io.Writer, envelope domain.PackageEnvelope, result usecase.AddResult, commandErr error) (bool, error) {
	failure := addFailureStatus(result, commandErr)
	if failure == "" {
		return false, nil
	}
	if _, err := fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Error, "Add"), failure); err != nil {
		return true, err
	}
	if result.Plan.Status == domain.PlanUnsupported {
		return true, renderHumanPlan(writer, envelope, result)
	}
	if failure == "activation_failed" || failure == "authentication_failed" || failure == "verification_failed" {
		_, err := fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Next"), nextLocalLifecycleAction(result))
		return true, err
	}
	return true, nil
}

func renderHumanAddOutcome(writer io.Writer, result usecase.AddResult) error {
	if err := renderOpenCodeRuntimeNotice(writer, result); err != nil {
		return err
	}
	if result.NoChange && result.Plan.InstallIntent == domain.InstallIntentPrepare {
		message := "Owned " + domain.ClientDisplayName(result.Plan.ClientID) + " configuration remains prepared. No changes made."
		if requiresPersonalMapping(result.Plan.ClientID) {
			message = "Owned ChatGPT package remains prepared. Remote connection and tool calls have not been verified."
		}
		_, err := fmt.Fprintln(writer, message+"\nNext: "+nextLocalLifecycleAction(result))
		return err
	}
	if result.NoChange {
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Muted, "Already installed and lifecycle verification is complete. No changes made."))
		return nil
	}
	if result.Mutated && fullyInstalled(result.Activation) {
		renderHumanAddInstalled(writer, result)
		return nil
	}
	if result.Mutated {
		renderHumanAddPending(writer, result)
	}
	if action := nextLocalLifecycleAction(result); action != "" && !fullyInstalled(result.Activation) {
		_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Next"), action)
	}
	return nil
}

func renderHumanAddInstalled(writer io.Writer, result usecase.AddResult) {
	if result.Activation.ActivationAttested || result.Activation.AuthenticationAttested {
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Warning, "Lifecycle is user-attested for the explicitly confirmed phase; it was not observed from the client."))
		return
	}
	if reportsMCPToolNamespaceCollision(result.Plan.ClientID) && len(domain.SelectedMCPNames(result.Plan)) > 0 {
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Success, "OpenCode MCP configuration installed and verified."))
		return
	}
	_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Success, "Installed and verified for the selected client."))
}

func renderHumanAddPending(writer io.Writer, result usecase.AddResult) {
	switch {
	case result.Activation.Authentication == domain.AuthenticationPending && result.Activation.Activation == domain.ActivationActive:
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Warning, "Package materialized and client activation completed. Authentication is pending."))
	case result.Activation.Authentication == domain.AuthenticationPending:
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Warning, "Package prepared. Authentication and client activation are pending."))
	case result.Activation.Authentication == domain.AuthenticationNotChecked && result.Activation.Activation == domain.ActivationActive:
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Warning, "Package materialized and client activation verified. Authentication requirements have not been checked."))
	default:
		_, _ = fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Warning, "Package prepared. Activation is not complete yet."))
	}
}

type addResultData struct {
	OperationID    string                     `json:"operation_id,omitempty"`
	Plugin         string                     `json:"plugin"`
	Version        string                     `json:"version,omitempty"`
	Source         string                     `json:"source"`
	Revision       string                     `json:"revision,omitempty"`
	TreeDigest     string                     `json:"tree_digest"`
	ManifestDigest string                     `json:"manifest_digest"`
	Security       *domain.SecurityAssessment `json:"security,omitempty"`
	NextAction     string                     `json:"next_action,omitempty"`
	DryRun         bool                       `json:"dry_run"`
	Result         usecase.AddResult          `json:"result"`
	envelopeResult string
}

func (data addResultData) outputResult() string {
	return data.envelopeResult
}

func addEnvelopeResult(result usecase.AddResult, commandErr error) string {
	if addFailureStatus(result, commandErr) != "" {
		return outputResultFailure
	}
	return outputResultSuccess
}

func addFailureStatus(result usecase.AddResult, commandErr error) string {
	if result.GroupPhase == usecase.GroupTargetManagedRolledBack ||
		result.GroupPhase == usecase.GroupTargetManagedUnknown ||
		result.GroupPhase == usecase.GroupTargetExternalFailed ||
		result.GroupPhase == usecase.GroupTargetExternalPartial {
		return string(result.GroupPhase)
	}
	if result.Plan.Status == domain.PlanUnsupported {
		return string(domain.PlanUnsupported)
	}
	if result.Activation.Activation == domain.ActivationFailed {
		return "activation_failed"
	}
	if result.Activation.Authentication == domain.AuthenticationFailed {
		return "authentication_failed"
	}
	if result.Activation.Verification == domain.VerificationFailed {
		return "verification_failed"
	}
	if commandErr != nil {
		return "failed"
	}
	return ""
}

func newAddResultData(envelope domain.PackageEnvelope, result usecase.AddResult, dryRun bool) addResultData {
	result = withOpenCodeRuntimeNotice(result)
	return addResultData{
		OperationID: result.Receipt.OperationID, Plugin: envelope.Manifest.Name,
		Version: envelope.Manifest.Version, Source: publicPackageSource(envelope.Source),
		Revision: envelope.Source.ResolvedRevision, TreeDigest: envelope.TreeDigest,
		ManifestDigest: envelope.ManifestDigest, NextAction: nextLifecycleAction(result),
		DryRun: dryRun, Result: result, envelopeResult: addEnvelopeResult(result, nil),
	}
}

func nextLifecycleAction(result usecase.AddResult) string {
	return lifecycleAction(result, false)
}

// nextLocalLifecycleAction is private terminal guidance. Provider LocalActions
// may contain host paths, so public JSON must use nextLifecycleAction instead.
func nextLocalLifecycleAction(result usecase.AddResult) string {
	return lifecycleAction(result, true)
}

func lifecycleAction(result usecase.AddResult, includePrivate bool) string {
	if result.Plan.InstallIntent == domain.InstallIntentPrepare {
		return prepareLifecycleAction(result, includePrivate)
	}
	action := firstLifecycleAction(result, includePrivate)
	return decorateLifecycleAction(result, action)
}

func prepareLifecycleAction(result usecase.AddResult, includePrivate bool) string {
	if !requiresPersonalMapping(result.Plan.ClientID) {
		return clientplanner.KiroPrepareAction
	}
	if includePrivate && result.Activation.Activation == domain.ActivationPrepared {
		if len(result.Activation.LocalActions) > 0 {
			return result.Activation.LocalActions[0]
		}
		if result.Plan.ActivePath != "" {
			return domain.ChatGPTPreparedAction(result.Plan.ActivePath, result.Plan.DeclaredName)
		}
	}
	return domain.ChatGPTMappedPreparationAction
}

func firstLifecycleAction(result usecase.AddResult, includePrivate bool) string {
	if includePrivate && len(result.Activation.LocalActions) > 0 {
		return result.Activation.LocalActions[0]
	}
	if len(result.Activation.UserActions) > 0 {
		return result.Activation.UserActions[0]
	}
	if includePrivate && len(result.Plan.LocalActions) > 0 {
		return result.Plan.LocalActions[0]
	}
	if len(result.Plan.UserActions) > 0 {
		return result.Plan.UserActions[0]
	}
	return ""
}

func decorateLifecycleAction(result usecase.AddResult, action string) string {
	if result.Activation.Authentication == domain.AuthenticationPending {
		if action != "" {
			return fmt.Sprintf("%s; complete authentication, then rerun add to verify activation and authentication", action)
		}
		return "complete authentication for the prepared plugin in the selected client, then rerun add to verify activation and authentication"
	}
	if result.Activation.Authentication == domain.AuthenticationNotChecked {
		if action != "" {
			if strings.Contains(strings.ToLower(action), "verify this plugin's authentication requirements") {
				return action
			}
			return action + "; verify this plugin's authentication requirements before using it"
		}
		return "verify this plugin's authentication requirements before using it"
	}
	if action != "" {
		return action
	}
	return "start a new client session and verify the plugin is available"
}

func fullyInstalled(outcome domain.ActivationOutcome) bool {
	authComplete := outcome.Authentication == domain.AuthenticationNotRequired || outcome.Authentication == domain.AuthenticationComplete
	return outcome.Activation == domain.ActivationActive && outcome.Verification == domain.VerificationInstalled && authComplete
}

// Preserve group preflight guidance unless a prepared personal marketplace exists.
func localTargetLifecycleAction(result usecase.AddResult, publicAction string) string {
	if requiresPersonalMapping(result.Plan.ClientID) && result.Plan.InstallIntent == domain.InstallIntentPrepare && result.Activation.Activation == domain.ActivationPrepared {
		return nextLocalLifecycleAction(result)
	}
	return publicAction
}
