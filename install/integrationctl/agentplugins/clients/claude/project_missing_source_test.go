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
	for _, schema := range []string{"2.1.263", "2.1.325"} {
		for _, condition := range []string{"exact-missing", "source-present", "cache-modified", "wrong-error", "wrong-detail", "partial-project-enabled", "partial-error-details", "extra-error", "disabled", "foreign-index", "unowned"} {
			t.Run(schema+"/"+condition, func(t *testing.T) {
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
				if schema == "2.1.263" {
					delete(row, "projectEnabled")
					delete(row, "errorDetails")
				}
				if condition == "wrong-error" {
					row["errors"] = []string{"Marketplace " + market + " failed to load: policy-denied"}
				}
				if condition == "wrong-detail" {
					row["errorDetails"] = []any{map[string]any{"type": "marketplace-load-failed", "marketplace": "foreign"}}
				}
				if condition == "partial-project-enabled" {
					row["projectEnabled"] = true
					delete(row, "errorDetails")
				}
				if condition == "partial-error-details" {
					delete(row, "projectEnabled")
					row["errorDetails"] = []any{map[string]any{"type": "marketplace-load-failed", "marketplace": market}}
				}
				if condition == "extra-error" {
					row["errors"] = []string{"Marketplace " + market + " failed to load: cache-miss", "policy denied"}
				}
				if condition == "disabled" {
					row["enabled"] = false
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
}

// This selected row was captured from Claude 2.1.263 after repair-absent.
// Original listing SHA256: f2086b016622183ef3310abef319e10d2e0f9fcf07eaef07f13b67d18fe2111f.
// Tests relocate only its fixture identity; the missing metadata stays missing.
const claude263MissingSourceRow = `{
  "id": "uap-commands-fixture@agentplugins-8910f2e15743",
  "version": "1.0.0",
  "scope": "project",
  "enabled": true,
  "installPath": "D:\\a\\_temp\\uap-native-hosted-banu1s2d\\evidence\\uap-claude-native-evidence-3102240519\\claude-config\\plugins\\cache\\agentplugins-8910f2e15743\\uap-commands-fixture\\1.0.0",
  "installedAt": "2026-11-15T21:07:42.136Z",
  "lastUpdated": "2026-11-15T21:07:42.136Z",
  "projectPath": "D:\\a\\_temp\\uap-native-hosted-banu1s2d\\evidence\\uap-claude-native-evidence-3102240519\\project-A",
  "errors": [
    "Marketplace agentplugins-8910f2e15743 failed to load: cache-miss"
  ]
}`

func capturedClaude263MissingSourceRow(t *testing.T) map[string]any {
	t.Helper()
	var row map[string]any
	if err := json.Unmarshal([]byte(claude263MissingSourceRow), &row); err != nil {
		t.Fatal(err)
	}
	if len(row) != 9 {
		t.Fatalf("captured row schema changed: %v", row)
	}
	return row
}

func newClaude263MissingSourceFixture(t *testing.T) (domain.ActivationRequest, string, map[string]any) {
	t.Helper()
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
	writeClaude263MissingSourceIndex(t, r, cache, "project")
	row := capturedClaude263MissingSourceRow(t)
	row["id"], row["installPath"], row["projectPath"] = r.DeclaredName+"@"+market, cache, r.Plan.TargetContext.ProjectRoot
	row["errors"] = []any{"Marketplace " + market + " failed to load: cache-miss"}
	return r, cache, row
}

func writeClaude263MissingSourceIndex(t *testing.T, r domain.ActivationRequest, cache, scope string) {
	t.Helper()
	market := ProjectMarketplaceName(r.Plan.PhysicalArtifactID, r.Plan.TargetContext.ProjectRoot)
	native := map[string]any{"scope": scope, "projectPath": r.Plan.TargetContext.ProjectRoot, "installPath": cache, "version": "1.0.0"}
	index := map[string]any{"version": 2, "plugins": map[string]any{r.DeclaredName + "@" + market: []any{native}}}
	if err := shared.WriteJSON(filepath.Join(r.Client.ConfigRoot, "plugins", "installed_plugins.json"), index); err != nil {
		t.Fatal(err)
	}
}

func alterClaude263MissingSourceFixture(t *testing.T, condition string, r domain.ActivationRequest, cache string, row map[string]any, managed *domain.ClientBinding) *domain.ClientBinding {
	t.Helper()
	switch condition {
	case "cache-modified":
		claudeMarketplaceWrite(t, filepath.Join(cache, "foreign.txt"), "preserve")
	case "no-cache-receipt":
		managed.NativeObjects = nil
	case "unowned":
		return nil
	case "foreign-index":
		writeClaude263MissingSourceIndex(t, r, cache, "user")
	case "foreign-project":
		row["projectPath"] = filepath.Join(r.Plan.TargetContext.ProjectRoot, "foreign-project")
	case "foreign-cache":
		row["installPath"] = filepath.Join(r.Client.ConfigRoot, "plugins", "cache", "foreign", r.DeclaredName, "1.0.0")
	case "wrong-error":
		row["errors"] = []any{"Marketplace foreign failed to load: cache-miss"}
	case "disabled":
		row["enabled"] = false
	}
	return managed
}

func preserveClaude263MissingSourceFiles(t *testing.T, r domain.ActivationRequest, cache string) {
	t.Helper()
	for _, path := range []string{filepath.Join(r.Client.ConfigRoot, "plugins", "known_marketplaces.json"), filepath.Join(r.Client.ConfigRoot, "plugins", "installed_plugins.json"), filepath.Join(cache, ".claude-plugin", "plugin.json")} {
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatalf("read-only missing-source inspection changed %s: %v", path, err)
			}
		})
	}
}

func TestClaudeMissingMarketplace263CapturedRowRequiresOwnedAbsentSource(t *testing.T) {
	for _, condition := range []string{"owned-absent", "source-present", "cache-modified", "no-cache-receipt", "unowned", "foreign-index", "foreign-project", "foreign-cache", "wrong-error", "disabled", "duplicate-row"} {
		t.Run(condition, func(t *testing.T) {
			r, cache, row := newClaude263MissingSourceFixture(t)
			managed := &domain.ClientBinding{NativeObjects: r.Delivery.NativeObjects}
			managed = alterClaude263MissingSourceFixture(t, condition, r, cache, row, managed)
			if condition != "source-present" {
				if err := os.Rename(r.Plan.ActivePath, r.Plan.ActivePath+"-backup"); err != nil {
					t.Fatal(err)
				}
			}
			preserveClaude263MissingSourceFiles(t, r, cache)
			rows := []any{row}
			if condition == "duplicate-row" {
				rows = append(rows, row)
			}
			body, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			observed := inspectClaudeMarketplacePlugin(body, r.DeclaredName, r.Plan.PhysicalArtifactID, r.Plan.ActivePath, r.Plan.TargetContext.ProjectRoot, r.Client.ConfigRoot, "")
			if observed.state == projectListEnabled {
				t.Fatal("unavailable source manufactured native activation")
			}
			finding, err := inspectClaudeProjectRegistry(t.Context(), body, r.Client, r.Plan, managed)
			if condition == "owned-absent" {
				if err != nil || finding != clients.RegistryExpected || !observed.sourceMissing || observed.state != projectListAbsent {
					t.Fatalf("captured 2.1.263 unavailable source not repairable: finding=%v observation=%+v err=%v", finding, observed, err)
				}
			} else if finding == clients.RegistryExpected || finding == clients.RegistryClear {
				t.Fatalf("unsafe 2.1.263 missing-source repair admitted: %v %v", finding, err)
			}
		})
	}
}

func TestClaudeMissingMarketplace263RejectsPartialMetadata(t *testing.T) {
	const market = "agentplugins-8910f2e15743"
	detail := []any{map[string]any{"type": "marketplace-load-failed", "marketplace": market}}
	cases := map[string]map[string]any{
		"project-enabled-only": {"projectEnabled": true},
		"error-details-only":   {"errorDetails": detail},
		"false-flag":           {"projectEnabled": false, "errorDetails": detail},
		"null-flag":            {"projectEnabled": nil, "errorDetails": detail},
		"string-flag":          {"projectEnabled": "true", "errorDetails": detail},
		"null-details":         {"projectEnabled": true, "errorDetails": nil},
		"empty-details":        {"projectEnabled": true, "errorDetails": []any{}},
		"foreign-detail":       {"projectEnabled": true, "errorDetails": []any{map[string]any{"type": "marketplace-load-failed", "marketplace": "foreign"}}},
		"unknown-detail-kind":  {"projectEnabled": true, "errorDetails": []any{map[string]any{"type": "unknown", "marketplace": market}}},
		"extra-detail-member":  {"projectEnabled": true, "errorDetails": []any{map[string]any{"type": "marketplace-load-failed", "marketplace": market, "extra": true}}},
		"unknown-error":        {"errors": []any{"Marketplace " + market + " failed to load: policy-denied"}},
		"nonempty-notes":       {"notes": []any{"preserve another native diagnostic"}},
		"read-through-folder":  {"readFromFolder": "/foreign"},
		"read-through-version": {"folderVersion": "1.0.0"},
	}
	for name, changes := range cases {
		t.Run(name, func(t *testing.T) {
			row := capturedClaude263MissingSourceRow(t)
			for key, value := range changes {
				row[key] = value
			}
			if claudeMarketplaceSourceMissing(row, market) {
				t.Fatal("partial or unrecognized missing-source metadata accepted")
			}
		})
	}
}
