package vscode

import (
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Reuse validates the complete immutable stdio projection against the selected
// package and data receipt. It never rewrites old locators into fresh authority.
func validateWorkspaceStdio(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	for _, name := range shared.SupportedMCPNames(plan) {
		source := envelope.MCP.Servers[name]
		if source.Type != "stdio" {
			continue
		}
		if !filepath.IsAbs(dataPath) || filepath.Clean(dataPath) != dataPath {
			return fmt.Errorf("%w: VS Code workspace selected stdio data path is not absolute and clean", domain.ErrProjectionRefreshRequired)
		}
		entry := shared.CloneObject(source.Decoded)
		delete(entry, "type")
		if err := shared.ApplyStdioDataContract(entry, plan.ActivePath, dataPath, root); err != nil {
			return err
		}
		expected, err := decodeWorkspaceServer(entry)
		if err != nil {
			return err
		}
		expected, err = bindWorkspaceStdio(expected, source, plan.ActivePath, dataPath)
		if err != nil {
			return err
		}
		actual, err := readWorkspaceServer(root, name)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("%w: VS Code workspace immutable stdio package/data/launcher projection differs", domain.ErrProjectionRefreshRequired)
		}
	}
	return nil
}
