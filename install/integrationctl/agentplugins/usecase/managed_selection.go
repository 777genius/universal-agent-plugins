package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type managedSelectionReader interface {
	ManagedMCPNames(context.Context, domain.ClientID, string, string) ([]string, error)
}

func installationIfExisting(state domain.StateFileV2, index int, existing bool) *domain.Installation {
	if !existing {
		return nil
	}
	return &state.Installations[index]
}

func (service Service) preflightTargetComponents(ctx context.Context, input AddInput, plan *domain.DeliveryPlan, installation *domain.Installation, repair, updating bool) error {
	prior := priorManagedBinding(installation, plan)
	if prior == nil {
		return service.preflightComponents(input.Envelope, plan, false)
	}
	names, unverified, err := service.recordedManagedMCPNames(ctx, input, prior, repair, updating)
	if err != nil {
		return err
	}
	if unverified {
		return service.preflightComponents(input.Envelope, plan, false)
	}
	if names == nil {
		return service.preflightComponents(input.Envelope, plan, true)
	}
	selected := map[string]bool{}
	for _, name := range names {
		selected[name] = true
	}
	if err := describeUnsupportedManagedSelection(plan, names, repair, updating); err != nil {
		return err
	}
	restrictRepairMCPSelection(plan, selected, repair)
	return service.preflightComponents(input.Envelope, plan, true, selected)
}

func priorManagedBinding(installation *domain.Installation, plan *domain.DeliveryPlan) *domain.ClientBinding {
	if installation == nil {
		return nil
	}
	for _, binding := range installation.Clients {
		if binding.Materialization != domain.MaterializationAbsent && binding.Scope == string(plan.Scope) && binding.PhysicalArtifact == plan.PhysicalArtifactID && sameNativeBackend(domain.ClientID(binding.ClientID), plan.ClientID) {
			cloned := binding
			return &cloned
		}
	}
	return nil
}

func (service Service) recordedManagedMCPNames(ctx context.Context, input AddInput, prior *domain.ClientBinding, repair, updating bool) ([]string, bool, error) {
	reader, ok := service.Stager.(managedSelectionReader)
	if !ok {
		return nil, false, nil
	}
	if service.Targets == nil {
		return nil, false, fmt.Errorf("target resolver required to verify prior MCP selection")
	}
	client := input.Client
	client.ClientID = domain.ClientID(prior.ClientID)
	target, err := service.Targets.ResolveTarget(ctx, client, input.Scope, prior.PhysicalArtifact)
	if err != nil {
		return nil, false, err
	}
	if err := service.Paths.RequireExactPath(target.ActivePath, prior.TargetLocator); err != nil {
		return nil, false, fmt.Errorf("untrusted persisted target while reading managed MCP selection: %w", err)
	}
	names, err := reader.ManagedMCPNames(ctx, client.ClientID, prior.TargetLocator, managedDigest(*prior))
	if err == nil {
		return names, false, nil
	}
	var verification *ports.VerificationError
	if repair && errors.As(err, &verification) && (verification.Kind == ports.VerificationAbsent || verification.Kind == ports.VerificationDigestMismatch) {
		// No names from damaged bytes are trusted. The existing exact rebuilt
		// artifact digest gate remains mandatory before any repair commit.
		return nil, true, nil
	}
	operation := "add"
	if updating {
		operation = "update"
	}
	return nil, false, fmt.Errorf("managed package was changed or is missing; refusing silent %s before previous MCP selection can be verified: %w", operation, err)
}

func describeUnsupportedManagedSelection(plan *domain.DeliveryPlan, names []string, repair, updating bool) error {
	supported := map[string]bool{}
	for _, c := range plan.Components {
		if c.Kind == domain.ComponentMCPServer && c.Support != domain.SupportUnsupported {
			supported[c.Name] = true
		}
	}
	staticRemoved := []string{}
	for _, name := range names {
		if !supported[name] {
			staticRemoved = append(staticRemoved, name)
		}
	}
	if len(staticRemoved) == 0 {
		return nil
	}
	plan.Warnings = append(plan.Warnings, "managed_component_removal_required")
	for _, name := range staticRemoved {
		plan.Diagnostics = append(plan.Diagnostics, domain.Diagnostic{Severity: domain.SeverityWarning, Boundary: domain.BoundaryMCPServer, Item: name, Code: "managed_component_removal_planned", Message: "current support requires controlled removal of this previously managed MCP entry"})
	}
	if repair || !updating {
		return fmt.Errorf("recorded MCP selection is no longer supported; use an explicit update for controlled removal: %s", strings.Join(staticRemoved, ", "))
	}
	return nil
}

func restrictRepairMCPSelection(plan *domain.DeliveryPlan, selected map[string]bool, repair bool) {
	if !repair {
		return
	}
	for i := range plan.Components {
		c := &plan.Components[i]
		if c.Kind == domain.ComponentMCPServer && !selected[c.Name] {
			c.Support = domain.SupportUnsupported
			c.Reason = "not_in_recorded_delivery"
		}
	}
}

func requiresComponentRemoval(plan domain.DeliveryPlan) bool {
	for _, warning := range plan.Warnings {
		if warning == "managed_component_removal_required" {
			return true
		}
	}
	return false
}
