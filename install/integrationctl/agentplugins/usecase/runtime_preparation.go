package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// prepareExistingRuntime is only called after confirmation. It does not touch
// the managed package or client configuration, but may fill its owned cache.
func (service Service) prepareExistingRuntime(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, installation domain.Installation, client domain.ClientBinding) error {
	if !packageNeedsPluginData(envelope, plan) {
		return nil
	}
	if service.PluginData == nil {
		return fmt.Errorf("PLUGIN_DATA manager is required for stdio MCP packages")
	}
	receipt, ok := installation.DataReceipts[client.DataReceiptID]
	if !ok || receipt.Locator == "" {
		return fmt.Errorf("installed stdio MCP package is missing its owned PLUGIN_DATA receipt")
	}
	if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
		return fmt.Errorf("validate PLUGIN_DATA before runtime preparation: %w", err)
	}
	return service.PluginData.PrepareRuntime(ctx, envelope, plan, receipt.Locator)
}
