package agentpluginscli

import (
	"fmt"
	"io"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func renderHumanPlan(writer io.Writer, envelope domain.PackageEnvelope, result usecase.AddResult) error {
	checked := &planWriter{writer: writer}
	writer = checked
	_, _ = fmt.Fprintf(writer, "%s: %s %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Plugin"), prompt.SafeText(envelope.Manifest.Name), prompt.SafeText(envelope.Manifest.Version))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Target"), prompt.SafeText(string(result.Plan.ClientID)))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Package"), prompt.SafeText(string(result.Plan.PackageMode)))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Result"), prompt.SafeText(string(result.Plan.Status)))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Authentication"), prompt.SafeText(string(result.Plan.Authentication)))
	_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Verification"), prompt.SafeText(string(result.Plan.Verification)))
	for _, component := range result.Plan.Components {
		_, _ = fmt.Fprintf(writer, "  - %s %s: %s\n", prompt.SafeText(string(component.Kind)), prompt.SafeText(component.Name), prompt.SafeText(string(component.Support)))
	}
	for _, diagnostic := range result.Plan.Diagnostics {
		label := "Warning"
		if diagnostic.Severity == domain.SeverityInfo {
			label = "Info"
		}
		_, _ = fmt.Fprintf(writer, "  %s: %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, label), prompt.SafeText(diagnostic.Code), prompt.SafeText(diagnostic.Message))
	}
	for _, warning := range result.Plan.Warnings {
		_, _ = fmt.Fprintf(writer, "  %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Warning, "Warning"), prompt.SafeText(warning))
	}
	for _, action := range result.Plan.UserActions {
		_, _ = fmt.Fprintf(writer, "  %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Planned action"), prompt.SafeText(action))
	}
	for _, action := range result.Plan.LocalActions {
		_, _ = fmt.Fprintf(writer, "  %s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Planned action"), prompt.SafeText(action))
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
	if commandErr != nil {
		data.Readiness, data.NextActions = "failed", nil
	}
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

// renderHumanAddOutcome presents an add result without changing its verified lifecycle status.
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
	if result.NoChange && fullyInstalled(result.Activation) {
		if _, err := fmt.Fprintln(writer, terminaltheme.For(writer).Text(terminaltheme.Muted, "Already installed and lifecycle verification is complete. No changes made.")); err != nil {
			return err
		}
		return renderVerifiedLocalNextAction(writer, result)
	}
	if result.Mutated && fullyInstalled(result.Activation) {
		renderHumanAddInstalled(writer, result)
		return renderVerifiedLocalNextAction(writer, result)
	}
	if result.Mutated {
		renderHumanAddPending(writer, result)
	}
	if action := nextLocalLifecycleAction(result); action != "" && !fullyInstalled(result.Activation) {
		_, _ = fmt.Fprintf(writer, "%s: %s\n", terminaltheme.For(writer).Text(terminaltheme.Label, "Next"), action)
	}
	return nil
}

// renderVerifiedLocalNextAction displays only the verified Codex user's optional local step.
// It does not change the public JSON contract or turn Desktop activation into a required phase.
func renderVerifiedLocalNextAction(writer io.Writer, result usecase.AddResult) error {
	if !clientTraits(result.Plan.ClientID).ShowsVerifiedLocalNextAction || len(result.Activation.LocalActions) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(writer, "%s: %s\n",
		terminaltheme.For(writer).Text(terminaltheme.Label, "Next (optional)"),
		prompt.SafeText(result.Activation.LocalActions[0]))
	return err
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
