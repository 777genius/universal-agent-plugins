package usecase

import (
	"context"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"strings"
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
	var prior *domain.ClientBinding
	if installation != nil {
		for _, binding := range installation.Clients {
			if binding.Materialization != domain.MaterializationAbsent && binding.Scope == string(plan.Scope) && binding.PhysicalArtifact == plan.PhysicalArtifactID && sameNativeBackend(domain.ClientID(binding.ClientID), plan.ClientID) {
				copy := binding
				prior = &copy
				break
			}
		}
	}
	if prior == nil {
		return service.preflightComponents(input.Envelope, plan, false)
	}
	reader, ok := service.Stager.(managedSelectionReader)
	if !ok {
		return service.preflightComponents(input.Envelope, plan, true)
	}
	if service.Targets == nil {
		return fmt.Errorf("target resolver required to verify prior MCP selection")
	}
	client := input.Client
	client.ClientID = domain.ClientID(prior.ClientID)
	target, err := service.Targets.ResolveTarget(ctx, client, input.Scope, prior.PhysicalArtifact)
	if err != nil {
		return err
	}
	if err := pathpolicy.RequireExactPath(target.ActivePath, prior.TargetLocator); err != nil {
		return fmt.Errorf("untrusted persisted target while reading managed MCP selection: %w", err)
	}
	names, err := reader.ManagedMCPNames(ctx, client.ClientID, prior.TargetLocator, managedDigest(*prior))
	if err != nil {
		var verification *ports.VerificationError
		if repair && errors.As(err, &verification) && (verification.Kind == ports.VerificationAbsent || verification.Kind == ports.VerificationDigestMismatch) {
			// No names from damaged bytes are trusted. The existing exact rebuilt
			// artifact digest gate remains mandatory before any repair commit.
			return service.preflightComponents(input.Envelope, plan, false)
		}
		operation := "add"
		if updating {
			operation = "update"
		}
		return fmt.Errorf("managed package was changed or is missing; refusing silent %s before previous MCP selection can be verified: %w", operation, err)
	}
	selected := map[string]bool{}
	for _, name := range names {
		selected[name] = true
	}
	staticRemoved := []string{}
	supported := map[string]bool{}
	for _, c := range plan.Components {
		if c.Kind == domain.ComponentMCPServer && c.Support != domain.SupportUnsupported {
			supported[c.Name] = true
		}
	}
	for _, name := range names {
		if !supported[name] {
			staticRemoved = append(staticRemoved, name)
		}
	}
	if len(staticRemoved) > 0 {
		plan.Warnings = append(plan.Warnings, "managed_component_removal_required")
		for _, name := range staticRemoved {
			plan.Diagnostics = append(plan.Diagnostics, domain.Diagnostic{Severity: domain.SeverityWarning, Boundary: domain.BoundaryMCPServer, Item: name, Code: "managed_component_removal_planned", Message: "current support requires controlled removal of this previously managed MCP entry"})
		}
		if repair || !updating {
			return fmt.Errorf("recorded MCP selection is no longer supported; use an explicit update for controlled removal: %s", strings.Join(staticRemoved, ", "))
		}
	}
	if repair {
		for i := range plan.Components {
			c := &plan.Components[i]
			if c.Kind == domain.ComponentMCPServer && !selected[c.Name] {
				c.Support = domain.SupportUnsupported
				c.Reason = "not_in_recorded_delivery"
			}
		}
	}
	return service.preflightComponents(input.Envelope, plan, true, selected)
}

func requiresComponentRemoval(plan domain.DeliveryPlan) bool {
	for _, warning := range plan.Warnings {
		if warning == "managed_component_removal_required" {
			return true
		}
	}
	return false
}
