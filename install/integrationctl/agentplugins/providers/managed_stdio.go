package providers

import (
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"os"
)

// PreflightManagedStdio is read-only and called before any installation writes.
func (stager Stager) PreflightManagedStdio(snapshotRoot string) error {
	if !managedstdio.Supported() {
		return &domain.ComponentReadinessError{Code: "managed_stdio_platform_unsupported", Message: "managed stdio lacks native process lifecycle support on this platform"}
	}
	if err := stager.LauncherSource.Available(); err != nil {
		var pathError *os.PathError
		if errors.As(err, &pathError) && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return &domain.ComponentReadinessError{Code: "managed_stdio_helper_unavailable", Message: err.Error()}
	}
	collision, err := managedstdio.CheckCollision(snapshotRoot)
	if err != nil {
		return err
	}
	if collision {
		return &domain.ComponentReadinessError{Code: "managed_stdio_path_collision", Message: "authored package occupies the reserved managed stdio path"}
	}
	return nil
}
func (stager Stager) deliverManagedStdio(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	for _, name := range supportedMCPNames(plan) {
		if envelope.MCP.Servers[name].Type == "stdio" {
			if !managedstdio.Supported() {
				return fmt.Errorf("managed stdio platform unsupported")
			}
			return stager.LauncherSource.Deliver(root)
		}
	}
	return nil
}
