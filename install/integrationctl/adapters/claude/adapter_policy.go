package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type managedSettings struct {
	StrictKnownMarketplaces json.RawMessage `json:"strictKnownMarketplaces"`
}

func hasRestriction(items []domain.EnvironmentRestrictionCode, want domain.EnvironmentRestrictionCode) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func dedupeRestrictions(items []domain.EnvironmentRestrictionCode) []domain.EnvironmentRestrictionCode {
	seen := map[domain.EnvironmentRestrictionCode]struct{}{}
	out := make([]domain.EnvironmentRestrictionCode, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func blockingSteps(inspect ports.InspectResult) ([]string, bool) {
	if len(inspect.EnvironmentRestrictions) == 0 {
		return nil, false
	}
	for _, restriction := range inspect.EnvironmentRestrictions {
		if restriction == domain.RestrictionReadOnlyNativeLayer {
			return []string{"this Claude marketplace is seed-managed and read-only; ask an administrator to update the seed image instead of mutating it locally"}, true
		}
		if restriction == domain.RestrictionManagedPolicyBlock {
			return []string{"managed settings block adding this Claude marketplace; ask an administrator to update the allowlist or seed configuration"}, true
		}
		if restriction == domain.RestrictionSourceToolMissing {
			return []string{"Claude Code CLI is not available on PATH; install/configure Claude Code and rerun from a shell where `claude` works"}, true
		}
	}
	return []string{"install or configure Claude Code before applying this integration"}, true
}

func (a Adapter) readManagedSettings(scope, workspaceRoot string) (string, managedSettings, bool) {
	for _, candidate := range a.managedSettingsCandidates(scope, workspaceRoot) {
		body, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		var doc managedSettings
		if err := json.Unmarshal(body, &doc); err != nil {
			continue
		}
		return candidate, doc, true
	}
	return "", managedSettings{}, false
}

func (a Adapter) managedSettingsCandidates(scope, workspaceRoot string) []string {
	candidates := []string{}
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "project" {
		root := effectiveWorkspaceRoot(workspaceRoot, a.ProjectRoot)
		if root != "" {
			candidates = append(candidates, filepath.Join(root, ".claude", "managed-settings.json"))
		}
	}
	candidates = append(candidates,
		filepath.Join(a.userHome(), ".claude", "managed-settings.json"),
		"/etc/claude-code/managed-settings.json",
	)
	return candidates
}

func (m managedSettings) blocksAllMarketplaceAdds() bool {
	raw := strings.TrimSpace(string(m.StrictKnownMarketplaces))
	return raw == "[]"
}

func (a Adapter) marketplaceAddBlocked(scope, workspaceRoot, integrationID string) (bool, string) {
	_, managed, ok := a.readManagedSettings(scope, workspaceRoot)
	if !ok {
		return false, ""
	}
	raw := strings.TrimSpace(string(managed.StrictKnownMarketplaces))
	if raw == "" || raw == "null" {
		return false, ""
	}
	if raw == "[]" {
		return true, "managed settings set strictKnownMarketplaces to an empty allowlist, so no new Claude marketplaces can be added"
	}
	var allowlist []map[string]any
	if err := json.Unmarshal(managed.StrictKnownMarketplaces, &allowlist); err != nil {
		return false, ""
	}
	managedRoot := managedMarketplaceRoot(a.userHome(), integrationID)
	for _, entry := range allowlist {
		source, _ := entry["source"].(string)
		if source != "pathPattern" {
			continue
		}
		pattern, _ := entry["pathPattern"].(string)
		if pattern == "" {
			continue
		}
		if re, err := regexp.Compile(pattern); err == nil && re.MatchString(managedRoot) {
			return false, ""
		}
	}
	return true, "managed strictKnownMarketplaces does not allow the integrationctl-managed Claude marketplace path; ask an administrator to allow this path pattern or pre-seed the marketplace"
}

func (a Adapter) seedManagedMarketplacePath(integrationID string, record *domain.InstallationRecord) (string, bool) {
	marketplaceName := managedMarketplaceName(integrationID)
	if record != nil {
		if value := marketplaceNameFromRecord(*record); value != "" {
			marketplaceName = value
		}
	}
	seedDirs := strings.TrimSpace(os.Getenv("CLAUDE_CODE_PLUGIN_SEED_DIR"))
	if seedDirs == "" {
		return "", false
	}
	for _, root := range strings.Split(seedDirs, string(os.PathListSeparator)) {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		path := filepath.Join(root, "marketplaces", marketplaceName)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}
