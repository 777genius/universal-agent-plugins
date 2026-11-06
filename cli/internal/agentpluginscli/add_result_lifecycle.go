package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

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
	Readiness      string                     `json:"readiness,omitempty"`
	NextActions    []AgentAction              `json:"next_actions,omitempty"`
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
	readiness, nextActions := deriveAddReadiness(result, dryRun)
	return addResultData{
		OperationID: result.Receipt.OperationID, Plugin: envelope.Manifest.Name,
		Version: envelope.Manifest.Version, Source: publicPackageSource(envelope.Source),
		Revision: envelope.Source.ResolvedRevision, TreeDigest: envelope.TreeDigest,
		ManifestDigest: envelope.ManifestDigest, NextAction: nextLifecycleAction(result),
		DryRun: dryRun, Readiness: readiness, NextActions: nextActions, Result: result, envelopeResult: addEnvelopeResult(result, nil),
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
