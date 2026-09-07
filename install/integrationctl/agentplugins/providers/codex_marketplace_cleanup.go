package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// managedCodexMarketplaceRegistered proves that the exact manager-owned
// marketplace still points at the exact managed package before native cleanup.
// A same-name user replacement is never removed.
func managedCodexMarketplaceRegistered(configRoot, marketplace, managedArtifactPath string) (bool, error) {
	if strings.TrimSpace(configRoot) == "" || strings.TrimSpace(managedArtifactPath) == "" {
		return false, fmt.Errorf("managed Codex marketplace ownership evidence is incomplete")
	}
	body, err := os.ReadFile(filepath.Join(configRoot, "config.toml"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect managed Codex marketplace config: %w", err)
	}
	var document map[string]any
	if err := toml.Unmarshal(body, &document); err != nil {
		return false, fmt.Errorf("inspect managed Codex marketplace config: %w", err)
	}
	marketplaces, ok := document["marketplaces"].(map[string]any)
	if !ok {
		if _, present := document["marketplaces"]; present {
			return false, fmt.Errorf("refuse managed Codex marketplace cleanup because the registry shape is not recognized")
		}
		return false, nil
	}
	entryValue, present := marketplaces[marketplace]
	if !present {
		return false, nil
	}
	entry, ok := entryValue.(map[string]any)
	if !ok {
		return false, fmt.Errorf("refuse managed Codex marketplace cleanup because %s has an unrecognized entry", marketplace)
	}
	source, ok := entry["source"].(string)
	if !ok || strings.TrimSpace(source) == "" {
		return false, fmt.Errorf("refuse managed Codex marketplace cleanup because %s has no local source", marketplace)
	}
	if sourceType, present := entry["source_type"]; present && sourceType != "local" {
		return false, fmt.Errorf("refuse managed Codex marketplace cleanup because %s is not a local source", marketplace)
	}
	if !equivalentLocalPath(source, managedArtifactPath) {
		return false, fmt.Errorf("refuse managed Codex marketplace cleanup because %s no longer points at the managed artifact", marketplace)
	}
	return true, nil
}

// managedCodexPluginEntryPresent reports whether Codex's config.toml still
// carries a per-plugin `[plugins."<declaredName>@<marketplace>"]` enablement
// entry, independent of whether its marketplace source is still registered.
// The two records are cleared by separate CLI commands; a stale plugin entry
// left behind is what lets a freshly started Codex app-server silently
// re-materialize an already-removed plugin. Presence alone is the ownership
// signal here (unlike managedCodexMarketplaceRegistered's source-path check):
// the key embeds this installation's own generated marketplace name
// (managedMarketplaceName), so an unrelated plugin can only collide by
// coincidentally sharing both that generated name and the declared name.
func managedCodexPluginEntryPresent(configRoot, declaredName, marketplace string) (bool, error) {
	if strings.TrimSpace(configRoot) == "" || strings.TrimSpace(declaredName) == "" || strings.TrimSpace(marketplace) == "" {
		return false, fmt.Errorf("managed Codex plugin ownership evidence is incomplete")
	}
	body, err := os.ReadFile(filepath.Join(configRoot, "config.toml"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect managed Codex plugin config: %w", err)
	}
	var document struct {
		Plugins map[string]map[string]any `toml:"plugins"`
	}
	if err := toml.Unmarshal(body, &document); err != nil {
		return false, fmt.Errorf("inspect managed Codex plugin config: %w", err)
	}
	_, present := document.Plugins[declaredName+"@"+marketplace]
	return present, nil
}

// equivalentLocalPath compares the filesystem identity rather than only the
// spelling of a path. macOS commonly exposes /tmp through /private/tmp, and
// Windows can expose the same directory with an extended-length path prefix.
// Identical cleaned paths are accepted directly; differing paths must both
// stat successfully and identify the same file before an alias is accepted.
func equivalentLocalPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if left == right {
		return true
	}
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}
