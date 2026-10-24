package opencode

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Existing immutable projection bytes cannot silently retain a different data
// locator from the selected receipt. Reprojection reads and refuses; it does
// not rewrite the digest-covered package to adopt a new authority.
func validateOpenCodeProjectActiveProjection(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	layout, err := openCodePlanLayout(plan)
	if err != nil {
		return err
	}
	if root != plan.ActivePath {
		return fmt.Errorf("%w: OpenCode active root differs from selected package", domain.ErrProjectionRefreshRequired)
	}
	if err := pathpolicy.RequireContainedChild(layout.Root, root); err != nil {
		return err
	}
	if empty, err := emptyOpenCodeNativePlan(envelope, plan); err != nil || empty {
		return err
	}
	projection, err := ReadOpenCodeProjection(root)
	if err != nil {
		return err
	}
	if err := validateOpenCodeProjection(layout, root, projection); err != nil {
		return err
	}
	if projection.PackageRoot != plan.ActivePath {
		return fmt.Errorf("%w: OpenCode projection package root differs from selected package", domain.ErrProjectionRefreshRequired)
	}
	for _, server := range projection.MCPServers {
		if server.Type == "stdio" && (dataPath == "" || !filepath.IsAbs(dataPath) || filepath.Clean(dataPath) != dataPath || projection.DataRoot != dataPath) {
			return fmt.Errorf("%w: OpenCode projection data root differs from selected receipt", domain.ErrProjectionRefreshRequired)
		}
	}
	return nil
}
