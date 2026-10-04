package vscode

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

func (a *LocalAdapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if _, err := a.ResolveProfileRoot(in.Client.ConfigRoot); err != nil {
		return err
	}
	if plan.NativeRegistryRoot != in.Client.ConfigRoot || plan.NativeRegistryExecutable != "" {
		return fmt.Errorf("local registry cannot use CLI authority")
	}
	if in.Envelope.LoaderKind != domain.LoaderKindAgentPlugins || in.Envelope.FormatID != domain.FormatIDAgentPluginsV1 || in.Envelope.Manifest.SchemaURI != domain.PluginSchemaV1 || !validLocalDigest(in.Envelope.TreeDigest) {
		return fmt.Errorf("local requires checked standard package schema/digest")
	}
	return a.verifyDeclaredHooks(in.Envelope)
}

func (*LocalAdapter) AuthorizeLocalPreparation(_ clients.PlanInput, plan *domain.DeliveryPlan) (bool, error) {
	// Source-qualified TEST admission is explicit construction, not a catalog or
	// production availability claim. Security assessment remains host-owned.
	plan.LocalPreparationAuthorized = true
	return true, nil
}

func (a *LocalAdapter) QualifyPlan(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	selected := make([]domain.ComponentDecision, 0, len(plan.Components))
	for _, component := range plan.Components {
		keep := component.Kind == domain.ComponentMCPServer && slices.Contains(a.config.MCPServers, component.Name) ||
			component.Kind == domain.ComponentSkill && slices.Contains(a.config.Skills, component.Name)
		if !keep {
			continue
		}
		if component.Support == domain.SupportUnsupported {
			return fmt.Errorf("local selected component unavailable")
		}
		if component.Kind == domain.ComponentMCPServer && in.Envelope.MCP.Servers[component.Name].Type != "stdio" {
			return fmt.Errorf("local selected MCP must use existing managed stdio")
		}
		selected = append(selected, component)
	}
	plan.Components = selected
	if len(domain.SelectedMCPNames(*plan)) != len(a.config.MCPServers) || countSkills(selected) != len(a.config.Skills) {
		return fmt.Errorf("local selected components missing from canonical package")
	}
	return a.selectPlan(in, plan)
}

func countSkills(components []domain.ComponentDecision) int {
	count := 0
	for _, c := range components {
		if c.Kind == domain.ComponentSkill {
			count++
		}
	}
	return count
}

func (a *LocalAdapter) selectPlan(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	enabled := true
	root, settings := filepath.Dir(a.config.ProfileSettingsPath), a.config.ProfileSettingsPath
	facts := domain.LocalDeliveryFacts{ProfileRoot: root, SettingsPath: settings, ProfileIdentity: root, SettingsIdentity: settings,
		Tuple: a.config.QualifiedTuple, NativeStop: a.config.NativeStop, MCPServers: a.config.MCPServers, Skills: a.config.Skills,
		CanonicalDigest: in.Envelope.TreeDigest, Registration: domain.OwnedProfileEntry{ObjectID: localObjectID(settings, plan.ActivePath), Selector: plan.ActivePath, DesiredValue: &enabled}}
	selected, err := domain.NewLocalDelivery(facts)
	if err != nil {
		return err
	}
	if _, err := localRecordedReceipt(selected, in.PreviousNativeObjects, in.LocalEntryObservation); err != nil {
		return err
	}
	owned, err := ownedLocalObjects(facts, in.PreviousNativeObjects)
	if err != nil {
		return err
	}
	if owned {
		_, err = inspectObservedRegistration(nativeconfig.New(), selected, in.PreviousNativeObjects, in.LocalEntryObservation)
		if errors.Is(err, vscodeprofile.ErrRecordedEntryAbsent) && in.LocalEntryObservation != nil {
			basis := in.LocalEntryObservation.Facts().RevisionBasis
			old, _ := basis.LocalFacts()
			if old.CanonicalDigest == facts.CanonicalDigest && selected.SameSelection(basis) {
				err = nil
			}
		} // Positive absence at an unchanged recorded selection; never a receipt.
	} else {
		_, err = inspectRegistration(nativeconfig.New(), facts, false)
	}
	if err != nil {
		return err
	}
	plan.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), in.PreviousNativeObjects...)
	plan.LocalEntryObservation = in.LocalEntryObservation.Clone()
	return clients.SelectLocalDelivery(plan, facts)
}

func (a *LocalAdapter) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := a.selectPlan(in, plan); err != nil {
		return err
	}
	plan.Status = domain.PlanReady
	plan.Activation = domain.ActivationPrepared
	plan.Verification = domain.VerificationPackageValid
	plan.Warnings = append(plan.Warnings, "Local TEST source contract; live activation and native qualification unknown")
	plan.UserActions = append(plan.UserActions, "reload the selected VS Code profile; registration does not prove Local hook execution")
	return nil
}
