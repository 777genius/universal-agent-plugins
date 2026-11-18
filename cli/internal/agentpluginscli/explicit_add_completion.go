package agentpluginscli

import (
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func dependencyActionAliases(actions []dependencyAction) []string {
	var aliases []string
	for _, action := range actions {
		if !slices.Contains(aliases, action.Alias) {
			aliases = append(aliases, action.Alias)
		}
	}
	return aliases
}

func (adapter *dependencyInstaller) explicitAddTrustConsent(action dependencyAction) bool {
	return (action.Action == environment.ActionInstall || dependencyRepairOperation(action.Action)) && slices.Contains(adapter.trustAliases, action.Alias)
}

// A repair can reapply incomplete native projections before the prepared-setup
// classifier runs. Bound its new trust consent to the complete verified input,
// never the installer's selected or required component subset.
func completeEnvelopeTrustConsent(request installer.Request, action dependencyAction, envelope domain.PackageEnvelope) bool {
	if !request.ProjectTrustConsent || !dependencyRepairOperation(action.Action) {
		return request.ProjectTrustConsent
	}
	return request.Scope == domain.ScopeProject && pureSkillsEnvelope(envelope)
}

func pureSkillsEnvelope(envelope domain.PackageEnvelope) bool {
	inventory := envelope.Inventory
	if len(envelope.Skills) == 0 || len(envelope.Diagnostics) != 0 ||
		envelope.MCP.Present || envelope.MCP.Enabled || len(envelope.MCP.Servers)+len(envelope.MCP.InvalidServer) != 0 ||
		envelope.App.Present || envelope.App.Declared || envelope.App.Enabled || len(envelope.App.Bindings) != 0 ||
		len(envelope.Manifest.Extensions)+len(envelope.Commands)+len(envelope.InvalidCommands) != 0 {
		return false
	}
	if inventory.MCPPresent || inventory.MCPEnabled || inventory.AppPresent || inventory.InvalidSkillsRoot ||
		len(inventory.MCPServers)+len(inventory.InvalidMCPServer)+len(inventory.AppBindings)+len(inventory.InvalidSkills)+len(inventory.Extensions) != 0 ||
		len(inventory.Skills) != len(envelope.Skills) {
		return false
	}
	return true
}

// Plan actions are adapter-owned disclosure. Reuse existing review fields so
// native setup is visible before acceptance without another public DTO.
func (adapter *dependencyInstaller) explicitAddReviewReason(action dependencyAction, plan installer.Plan) string {
	if adapter.app.scope() != domain.ScopeProject || !slices.Contains(adapter.trustAliases, action.Alias) ||
		!domain.ClientTraitsFor(domain.ClientID(action.ClientID)).SavedProjectRepairOnAdd {
		return action.Reason
	}
	reason := action.Reason
	for _, text := range plan.Delivery.UserActions {
		text = prompt.SafeText(text)
		if text == "" {
			continue
		}
		if adapter.previewWarnings != nil {
			if !slices.Contains(*adapter.previewWarnings, text) {
				*adapter.previewWarnings = append(*adapter.previewWarnings, text)
			}
		} else if !strings.Contains(reason, text) {
			if reason != "" {
				reason += "; "
			}
			reason += text
		}
	}
	return reason
}

func skillsInitialReviewWarnings(lock environment.Lock, aliases, targets []string) []string {
	warnings := []string{}
	if notice := savedTargetNotice(lock, aliases, targets); notice != "" {
		warnings = append(warnings, notice)
	}
	return warnings
}

func skillsRetryDisclosureCovered(reviewed, retry []string) bool {
	for _, warning := range retry {
		if !slices.Contains(reviewed, warning) {
			return false
		}
	}
	return true
}

// A path-free warning cannot disclose a different native trust profile. Compare
// the selected authority across attempts, rather than only checking each new
// plan against itself. Reserved IDs without a receipt are not installed owners.
func skillsRetryTrustAuthorityCovered(original, next *PreparedSkillsBatch) bool {
	for _, action := range next.request.prepared {
		id := domain.ClientID(action.ClientID)
		if next.app.scope() != domain.ScopeProject || !domain.ClientTraitsFor(id).SavedProjectRepairOnAdd {
			continue
		}
		if !skillsRetryDisclosureCovered(original.Warnings(), next.Warnings()) {
			return false
		}
		before, found := original.request.detected[id]
		after, selected := next.request.detected[id]
		if !found || !selected || original.app.scope() != domain.ScopeProject || original.app.ProjectRoot != next.app.ProjectRoot ||
			before.ConfigRoot != after.ConfigRoot || before.ExecutablePath != after.ExecutablePath {
			return false
		}
		pin, found := original.request.lock.Package(action.Alias)
		if !found || action.Package == nil || !sameSavedIdentity(pin, *action.Package) || !skillsRetryBindingCovered(original.request.prepared, action) {
			return false
		}
	}
	return true
}

func skillsRetryBindingCovered(reviewed []dependencyAction, next dependencyAction) bool {
	for _, action := range reviewed {
		if action.Alias != next.Alias || action.ClientID != next.ClientID {
			continue
		}
		if action.Action != next.Action {
			return false
		}
		if action.Authority == nil || action.Authority.ReceiptID == "" {
			return true
		}
		return next.Authority != nil && action.Authority.ReceiptID == next.Authority.ReceiptID &&
			action.InstallationID == next.InstallationID && action.BindingID == next.BindingID
	}
	return false
}
