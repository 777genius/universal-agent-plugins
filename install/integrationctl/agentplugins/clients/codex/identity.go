package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var _ clients.RegistryInspector = (*Adapter)(nil)

func (*Adapter) UsesNativeRegistryExecutable() bool { return true }

func (*Adapter) InspectNativeRegistry(ctx context.Context, env clients.Env, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if strings.TrimSpace(plan.NativeRegistryExecutable) != "" {
		return inspectCodexCLI(ctx, env, plan, managed)
	}
	return inspectCodexFiles(plan, managed)
}

func inspectCodexCLI(ctx context.Context, env clients.Env, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if env.Runner == nil {
		return clients.RegistryIndeterminate, nil
	}
	result, err := shared.RunNativeRegistry(ctx, env.Runner, legacyports.Command{Argv: []string{plan.NativeRegistryExecutable, "plugin", "list", "--json"}})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return clients.RegistryIndeterminate, ctxErr
		}
		return clients.RegistryIndeterminate, err
	}
	if result.ExitCode != 0 {
		if diagnostic := shared.BoundedNativeDiagnostic(result.Stdout, result.Stderr); diagnostic != "" {
			return clients.RegistryIndeterminate, fmt.Errorf("the Codex plugin registry command failed with exit code %d: %s", result.ExitCode, diagnostic)
		}
		return clients.RegistryIndeterminate, fmt.Errorf("the Codex plugin registry command failed with exit code %d", result.ExitCode)
	}
	return CodexRegistryFinding(result.Stdout, plan.DeclaredName, shared.ManagedMarketplaceName(plan.PhysicalArtifactID), managed != nil), nil
}

// CodexRegistryFinding classifies a `codex plugin list --json` document.
func CodexRegistryFinding(body []byte, name, expectedMarketplace string, owned bool) clients.RegistryFinding {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return clients.RegistryIndeterminate
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return clients.RegistryIndeterminate
	}
	document, ok := value.(map[string]any)
	if !ok {
		return clients.RegistryIndeterminate
	}
	entries, ok := document["installed"].([]any)
	if !ok {
		return clients.RegistryIndeterminate
	}
	finding := clients.RegistryClear
	seen := map[string]bool{}
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			return clients.RegistryIndeterminate
		}
		entryName, nameOK := entry["name"].(string)
		marketplace, marketplaceOK := entry["marketplaceName"].(string)
		pluginID, idOK := entry["pluginId"].(string)
		_, installedOK := entry["installed"].(bool)
		_, enabledOK := entry["enabled"].(bool)
		if !nameOK || !marketplaceOK || !idOK || !installedOK || !enabledOK || entryName == "" || marketplace == "" || pluginID != entryName+"@"+marketplace || seen[pluginID] {
			return clients.RegistryIndeterminate
		}
		seen[pluginID] = true
		if entryName != name {
			continue
		}
		if marketplace == expectedMarketplace {
			if !owned {
				return clients.RegistryCollision
			}
			finding = clients.RegistryExpected
		}
		// A different non-empty marketplace is positive namespace evidence and
		// can coexist with the managed marketplace.
	}
	return finding
}

func inspectCodexFiles(plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return clients.RegistryIndeterminate, nil
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return clients.RegistryClear, nil
	} else if err != nil {
		return clients.RegistryIndeterminate, err
	}
	expectedMarketplace := shared.ManagedMarketplaceName(plan.PhysicalArtifactID)
	finding := clients.RegistryClear
	configPath := filepath.Join(root, "config.toml")
	body, err := os.ReadFile(configPath)
	if err == nil {
		var config struct {
			Plugins map[string]map[string]any `toml:"plugins"`
		}
		if err := toml.Unmarshal(body, &config); err != nil {
			return clients.RegistryIndeterminate, err
		}
		for identity := range config.Plugins {
			parts := strings.Split(identity, "@")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return clients.RegistryIndeterminate, nil
			}
			if parts[0] == plan.DeclaredName && parts[1] == expectedMarketplace {
				if managed == nil {
					return clients.RegistryCollision, nil
				}
				finding = clients.RegistryExpected
			}
		}
	} else if !os.IsNotExist(err) {
		return clients.RegistryIndeterminate, err
	}
	cacheFinding, err := inspectCodexCache(filepath.Join(root, "plugins", "cache"), plan.DeclaredName, expectedMarketplace, managed != nil)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if cacheFinding == clients.RegistryCollision || cacheFinding == clients.RegistryIndeterminate {
		return cacheFinding, nil
	}
	if cacheFinding == clients.RegistryExpected {
		finding = clients.RegistryExpected
	}
	return finding, nil
}

func inspectCodexCache(root, name, expectedMarketplace string, owned bool) (clients.RegistryFinding, error) {
	markets, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return clients.RegistryClear, nil
	}
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	finding := clients.RegistryClear
	for _, market := range markets {
		next, err := inspectCodexMarket(root, market, name, expectedMarketplace, owned)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if next == clients.RegistryCollision || next == clients.RegistryIndeterminate {
			return next, nil
		}
		if next == clients.RegistryExpected {
			finding = clients.RegistryExpected
		}
	}
	return finding, nil
}

func inspectCodexMarket(root string, market os.DirEntry, name, expectedMarketplace string, owned bool) (clients.RegistryFinding, error) {
	if market.Type()&os.ModeSymlink != 0 || !market.IsDir() {
		return clients.RegistryIndeterminate, nil
	}
	plugins, err := os.ReadDir(filepath.Join(root, market.Name()))
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	finding := clients.RegistryClear
	for _, plugin := range plugins {
		next, err := inspectCodexCachedPlugin(root, market.Name(), plugin, name, expectedMarketplace, owned)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if next == clients.RegistryCollision || next == clients.RegistryIndeterminate {
			return next, nil
		}
		if next == clients.RegistryExpected {
			finding = clients.RegistryExpected
		}
	}
	return finding, nil
}

func inspectCodexCachedPlugin(root, market string, plugin os.DirEntry, name, expectedMarketplace string, owned bool) (clients.RegistryFinding, error) {
	if plugin.Type()&os.ModeSymlink != 0 || !plugin.IsDir() {
		return clients.RegistryIndeterminate, nil
	}
	manifest := filepath.Join(root, market, plugin.Name(), "local", ".codex-plugin", "plugin.json")
	manifestName, err := shared.ReadJSONManifestName(manifest)
	if os.IsNotExist(err) {
		return clients.RegistryIndeterminate, nil
	}
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if manifestName != name || market != expectedMarketplace {
		return clients.RegistryClear, nil
	}
	if !owned {
		return clients.RegistryCollision, nil
	}
	return clients.RegistryExpected, nil
}
