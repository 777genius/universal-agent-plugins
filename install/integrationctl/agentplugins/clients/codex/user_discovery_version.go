package codex

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Codex owns defaults for optional plugin versions. Read its exact installed
// identity instead of borrowing another client's fallback cache directory.
func codexNativeUserVersion(ctx context.Context, env clients.Env, request domain.ActivationRequest) (string, error) {
	listed, err := runCodex(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugin", "list", "--json")
	if err != nil {
		return "", shared.NativeProbeFailure(listed, err)
	}
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	if CodexRegistryFinding(listed.Stdout, request.DeclaredName, marketplace, true) != clients.RegistryExpected {
		return "", fmt.Errorf("codex user plugin registry is not exact")
	}
	document, err := shared.DecodeStrictJSONObject(listed.Stdout)
	if err != nil {
		return "", err
	}
	entries, _ := document["installed"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if entry["pluginId"] != request.DeclaredName+"@"+marketplace {
			continue
		}
		return codexInstalledUserVersion(entry, request)
	}
	return "", fmt.Errorf("codex user plugin version is unavailable")
}

func codexInstalledUserVersion(entry map[string]any, request domain.ActivationRequest) (string, error) {
	version, ok := entry["version"].(string)
	if !ok || version == "" || version != filepath.Base(version) || version == "." || version == ".." || strings.ContainsAny(version, "\\\x00\r\n") {
		return "", fmt.Errorf("codex user plugin cache version is unrecognized")
	}
	if request.Plan.DeclaredVersion != "" && version != request.Plan.DeclaredVersion {
		return "", fmt.Errorf("codex installed another authored plugin version")
	}
	source, _ := entry["source"].(map[string]any)
	if source["source"] != "local" || !codexDiscoveryPathMatches(source["path"], request.Delivery.ActivePath) {
		return "", fmt.Errorf("codex user plugin source differs from the managed package")
	}
	if entry["installed"] != true || entry["enabled"] != true {
		return "", fmt.Errorf("codex user plugin is not installed and enabled")
	}
	return version, nil
}
