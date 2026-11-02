package windsurf

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

// Active package bytes are immutable. A different selected data receipt needs
// a fresh projection; old launcher/environment locators cannot authorize it.
func validateDevinProjectStdio(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	for _, name := range shared.SupportedMCPNames(plan) {
		source := envelope.MCP.Servers[name]
		if source.Type != "stdio" {
			continue
		}
		server, err := devinProjectedServer(root, name)
		if err != nil {
			return err
		}
		launcher := filepath.Join(plan.ActivePath, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
		if !filepath.IsAbs(dataPath) || filepath.Clean(dataPath) != dataPath || server.Type != "stdio" || server.Command != launcher || server.CWD != "" || server.Env["PLUGIN_ROOT"] != plan.ActivePath || server.Env["PLUGIN_DATA"] != dataPath {
			return fmt.Errorf("%w: Devin stdio package/data binding differs from selected locators", domain.ErrProjectionRefreshRequired)
		}
		rawCWD, _ := source.Decoded["cwd"].(string)
		cwd, err := pathcontract.ExpandCWD(rawCWD, plan.ActivePath, dataPath)
		if err != nil {
			return err
		}
		_, resolvedCWD, err := shared.ResolveStdioPaths("", rawCWD, plan.ActivePath, dataPath, root)
		if err != nil {
			return err
		}
		prefix := managedstdio.Arguments(plan.ActivePath, dataPath, resolvedCWD, cwd.Anchor, "", nil)[:6]
		if len(server.Args) < 7 || !slices.Equal(server.Args[:6], prefix) {
			return fmt.Errorf("%w: Devin launcher package/data/cwd arguments differ from selected locators", domain.ErrProjectionRefreshRequired)
		}
	}
	return nil
}
