package providers

import (
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"os"
	"path/filepath"
)

// PreflightDataPath observes the same locator EnsureData owns, without creating it.
func (manager PluginDataManager) PreflightDataPath(physicalBackend string) (string, bool, error) {
	if manager.Base == "" {
		return "", false, fmt.Errorf("plugin data base is required")
	}
	locator := filepath.Join(manager.Base, physicalBackend)
	if err := pathpolicy.RequireContainedChild(manager.Base, locator); err != nil {
		return "", false, err
	}
	if _, err := os.Lstat(locator); os.IsNotExist(err) {
		return locator, false, nil
	} else if err != nil {
		return "", false, err
	}
	receipt, err := manager.readOwned(locator)
	if err != nil {
		return "", false, err
	}
	if receipt.PhysicalBackend != physicalBackend {
		return "", false, fmt.Errorf("PLUGIN_DATA physical ownership mismatch")
	}
	return locator, true, nil
}
