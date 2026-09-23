package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const OpenCodeNamespaceCheckedCode = "opencode_global_namespace_checked"

func (service Service) checkMCPNamespace(ctx context.Context, client domain.DetectedClient, plan *domain.DeliveryPlan, managed *domain.ClientBinding) error {
	if client.ClientID != domain.ClientOpenCode || len(domain.SelectedMCPNames(*plan)) == 0 {
		return nil
	}
	if service.NamespacePreflight == nil {
		return fmt.Errorf("OpenCode MCP namespace preflight is required")
	}
	if err := service.NamespacePreflight.CheckMCPNamespace(ctx, client, *plan, managed); err != nil {
		return fmt.Errorf("OpenCode MCP namespace preflight: %w", err)
	}
	for _, diagnostic := range plan.Diagnostics {
		if diagnostic.Code == OpenCodeNamespaceCheckedCode {
			return nil
		}
	}
	plan.Diagnostics = append(plan.Diagnostics, domain.Diagnostic{
		Severity: domain.SeverityInfo, Boundary: domain.BoundaryMCPServer,
		Code:    OpenCodeNamespaceCheckedCode,
		Message: "Initial global config server-name preflight found no potential overlap. Project settings, later config changes and live tool catalogs were not checked; verify tools in OpenCode before relying on them.",
	})
	return nil
}
