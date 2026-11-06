package usecase

import (
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *groupSession) preflightCompatibleBindings() error {
	if !session.replace || !session.existing || session.input.Repair {
		return nil
	}
	compatibleBindings := map[string]bool{}
	for _, target := range session.planned {
		if target.managed != nil {
			compatibleBindings[target.managed.ClientBindingID] = true
		}
	}
	checks := session.input.CompatibilityChecks
	if session.input.Switch {
		checks = nil
	}
	for _, check := range checks {
		if err := session.preflightOneCompatibleBinding(check, compatibleBindings); err != nil {
			return err
		}
	}
	for _, binding := range session.state.Installations[session.installationIndex].Clients {
		if binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		if !compatibleBindings[binding.ClientBindingID] {
			return fmt.Errorf("update group must preflight every installed physical binding; %s is missing", binding.ClientID)
		}
	}
	return nil
}

// requireObservedCompatibilityBinding fences the exact original sibling ID
// against the state reloaded under the usecase mutation lock. Client/backend
// equivalence alone cannot authorize a new unselected binding after preflight.
func (session *groupSession) requireObservedCompatibilityBinding(check AddInput) error {
	if check.CompatibilityBindingID == "" {
		return nil
	}
	binding, ok := session.state.Installations[session.installationIndex].Clients[check.CompatibilityBindingID]
	if !ok || binding.ClientBindingID != check.CompatibilityBindingID ||
		binding.ClientID != string(check.Client.ClientID) ||
		binding.Scope != string(check.Scope) ||
		binding.Materialization == domain.MaterializationAbsent ||
		binding.ProfileNamespace != check.Client.ProfileNamespace ||
		!domain.SameProfileAuthority(binding.ProfileAuthority, check.Client.ProfileAuthority) {
		return fmt.Errorf("unselected compatibility binding changed for %s", check.Client.ClientID)
	}
	return nil
}

func (session *groupSession) preflightOneCompatibleBinding(check AddInput, compatibleBindings map[string]bool) error {
	if err := session.requireObservedCompatibilityBinding(check); err != nil {
		return err
	}
	check.DryRun = check.DryRun || session.input.DryRun
	check.OnlinePreview = check.OnlinePreview || session.input.OnlinePreview
	// A selected binding already has this operation's reviewed host. A second
	// compatibility pass must validate that snapshot, never select a new baseline.
	for _, prepared := range session.result.PreparedClients {
		if prepared.ClientID == check.Client.ClientID && prepared.ExecutablePath == check.Client.ExecutablePath {
			check.Client.OpenCodeHost = prepared.OpenCodeHost
			check.Client.ProfileAuthority = domain.CloneProfileAuthority(prepared.ProfileAuthority)
			check.Client.ProfileNamespace = prepared.ProfileNamespace
			break
		}
	}
	if check.Envelope.TreeDigest != session.first.Envelope.TreeDigest || check.Envelope.ManifestDigest != session.first.Envelope.ManifestDigest {
		return fmt.Errorf("compatibility preflight must use the update candidate bytes")
	}
	plan, err := session.service.planInstall(session.ctx, &check, domain.ComputePhysicalArtifactID(check.Envelope.Manifest.Name, session.installationID), installationIfExisting(session.state, session.installationIndex, session.existing))
	if err != nil {
		return err
	}
	session.result.PreparedClients = append(session.result.PreparedClients, check.Client)
	if plan.Status == domain.PlanUnsupported {
		return fmt.Errorf("update candidate is incompatible with installed binding %s", check.Client.ClientID)
	}
	if err := session.service.preflightActivation(check, plan); err != nil {
		return err
	}
	if err := session.service.preflightTargetComponents(session.ctx, check, &plan, &session.state.Installations[session.installationIndex], false, true); err != nil {
		return err
	}
	for _, binding := range session.state.Installations[session.installationIndex].Clients {
		if sameNativeBackend(domain.ClientID(binding.ClientID), check.Client.ClientID, check.Scope) && binding.Scope == string(check.Scope) {
			compatibleBindings[binding.ClientBindingID] = true
		}
	}
	return nil
}

// preflightUnchangedProjections stops before any effect when the installed
// tree of an unchanged target no longer matches the command rules, instead of
// recording a failed activation for it. An update preview is exempt: the
// confirmed update refreshes such a tree before the group runs.
func (session *groupSession) preflightUnchangedProjections() error {
	if session.replace && (session.input.DryRun || !session.input.Confirmed) {
		return nil
	}
	for _, target := range session.planned {
		if !target.noChange || target.managed == nil {
			continue
		}
		if err := session.service.requireCurrentCommandProjection(session.ctx, target.input, target.plan, session.state.Installations[session.installationIndex], *target.managed); err != nil {
			return fmt.Errorf("target %s: %w", target.input.Client.ClientID, err)
		}
	}
	return nil
}

func (session *groupSession) reviewGroupDirectoryAbsence(plan domain.DeliveryPlan, managed *domain.ClientBinding) (bool, error) {
	if managed == nil {
		return true, nil
	}
	if !session.input.Repair {
		return false, nil
	}
	_, err := os.Lstat(plan.ActivePath)
	if os.IsNotExist(err) {
		return true, nil
	}
	return false, err
}
