package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestClaudeMissingMarketplaceRepairRequiresIndependentNativeOwnership(t *testing.T) {
	for _, condition := range []string{"exact-missing", "source-present", "cache-modified", "wrong-error", "wrong-detail", "foreign-index", "unowned"} {
		t.Run(condition, func(t *testing.T) {
			r := newClaudeMarketplaceRequest(t)
			market := ProjectMarketplaceName(r.Plan.PhysicalArtifactID, r.Plan.TargetContext.ProjectRoot)
			cache := filepath.Join(r.Client.ConfigRoot, "plugins", "cache", market, r.DeclaredName, "1.0.0")
			if err := filetree.CopyDir(filepath.Join(r.Plan.ActivePath, claudeProjectBundleDirectory), cache); err != nil {
				t.Fatal(err)
			}
			known := map[string]any{market: map[string]any{"source": map[string]any{"source": "directory", "path": r.Plan.ActivePath}, "installLocation": r.Plan.ActivePath}}
			if err := shared.WriteJSON(filepath.Join(r.Client.ConfigRoot, "plugins", "known_marketplaces.json"), known); err != nil {
				t.Fatal(err)
			}
			native := map[string]any{"scope": "project", "projectPath": r.Plan.TargetContext.ProjectRoot, "installPath": cache, "version": "1.0.0"}
			if condition == "foreign-index" {
				native["scope"] = "user"
			}
			if err := shared.WriteJSON(filepath.Join(r.Client.ConfigRoot, "plugins", "installed_plugins.json"), map[string]any{"version": 2, "plugins": map[string]any{r.DeclaredName + "@" + market: []any{native}}}); err != nil {
				t.Fatal(err)
			}
			row := map[string]any{"id": r.DeclaredName + "@" + market, "version": "1.0.0", "scope": "project", "projectPath": r.Plan.TargetContext.ProjectRoot, "installPath": cache, "enabled": true, "projectEnabled": true, "errors": []string{"Marketplace " + market + " failed to load: cache-miss"}, "errorDetails": []any{map[string]any{"type": "marketplace-load-failed", "marketplace": market}}}
			if condition == "wrong-error" {
				row["errors"] = []string{"Marketplace " + market + " failed to load: policy-denied"}
			}
			if condition == "wrong-detail" {
				row["errorDetails"] = []any{map[string]any{"type": "marketplace-load-failed", "marketplace": "foreign"}}
			}
			if condition != "source-present" {
				if err := os.Rename(r.Plan.ActivePath, r.Plan.ActivePath+"-backup"); err != nil {
					t.Fatal(err)
				}
			}
			if condition == "cache-modified" {
				claudeMarketplaceWrite(t, filepath.Join(cache, "foreign.txt"), "preserve")
			}
			body, _ := json.Marshal([]any{row})
			observed := inspectClaudeMarketplacePlugin(body, r.DeclaredName, r.Plan.PhysicalArtifactID, r.Plan.ActivePath, r.Plan.TargetContext.ProjectRoot, r.Client.ConfigRoot, "")
			if observed.state == projectListEnabled {
				t.Fatal("unavailable source manufactured native activation")
			}
			managed := &domain.ClientBinding{NativeObjects: r.Delivery.NativeObjects}
			if condition == "unowned" {
				managed = nil
			}
			finding, err := inspectClaudeProjectRegistry(t.Context(), body, r.Client, r.Plan, managed)
			if condition == "exact-missing" {
				if err != nil || finding != clients.RegistryExpected {
					t.Fatalf("owned unavailable source not repairable: %v %v", finding, err)
				}
			} else if finding == clients.RegistryExpected || finding == clients.RegistryClear {
				t.Fatalf("unsafe missing-source repair admitted: %v %v", finding, err)
			}
		})
	}
}
