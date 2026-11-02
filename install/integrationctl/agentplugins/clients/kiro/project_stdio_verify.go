package kiro

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

func validateKiroProjectStdio(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	if root != plan.ActivePath {
		return fmt.Errorf("kiro active reprojection differs from selected package")
	}
	if err := validateKiroProjectionRoot(root, plan); err != nil {
		return err
	}
	for _, name := range shared.SupportedMCPNames(plan) {
		source := envelope.MCP.Servers[name]
		if source.Type != "stdio" {
			continue
		}
		if !filepath.IsAbs(dataPath) || filepath.Clean(dataPath) != dataPath {
			return fmt.Errorf("%w: Kiro selected data locator must be clean and absolute", domain.ErrProjectionRefreshRequired)
		}
		entry, err := projectedKiroMCPServer(root, name)
		if err != nil {
			return err
		}
		if err := verifyKiroStdioBinding(root, entry, source, plan.ActivePath, dataPath); err != nil {
			return err
		}
	}
	return nil
}

func verifyKiroStdioBinding(observed string, entry map[string]any, source domain.MCPServer, packageRoot, dataRoot string) error {
	expected := filepath.Join(packageRoot, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	env, ok := entry["env"].(map[string]any)
	if !ok || env["PLUGIN_ROOT"] != packageRoot || env["PLUGIN_DATA"] != dataRoot || entry["command"] != expected {
		return fmt.Errorf("%w: Kiro stdio package/data binding differs", domain.ErrProjectionRefreshRequired)
	}
	if _, present := entry["cwd"]; present {
		return fmt.Errorf("%w: Kiro launcher cannot depend on a native cwd field", domain.ErrProjectionRefreshRequired)
	}
	rawCWD, _ := source.Decoded["cwd"].(string)
	cwd, err := pathcontract.ExpandCWD(rawCWD, packageRoot, dataRoot)
	if err != nil {
		return err
	}
	_, resolved, err := shared.ResolveStdioPaths("", rawCWD, packageRoot, dataRoot, observed)
	if err != nil {
		return err
	}
	args, err := kiroProjectArguments(entry)
	if err != nil {
		return err
	}
	prefix := managedstdio.Arguments(packageRoot, dataRoot, resolved, cwd.Anchor, "", nil)[:6]
	if len(args) < 7 || !slices.Equal(args[:6], prefix) {
		return fmt.Errorf("%w: Kiro launcher roots or cwd differ", domain.ErrProjectionRefreshRequired)
	}
	return nil
}
