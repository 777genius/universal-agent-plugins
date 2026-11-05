package codex

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// Official ChatGPT Desktop deep link: it opens a local plugin's detail page,
// not an installation or proof that the app loaded the plugin.
var desktopPluginName = regexp.MustCompile("^[a-z][a-z0-9-]*$")

// desktopPluginLink constructs a validated local plugin-detail URI without opening it.
func desktopPluginLink(marketplaceRoot, pluginName string) (string, error) {
	if !cleanAbsolute(marketplaceRoot) {
		return "", fmt.Errorf("ChatGPT Desktop marketplace root must be a clean absolute path")
	}
	if !desktopPluginName.MatchString(pluginName) {
		return "", fmt.Errorf("ChatGPT Desktop plugin name is not a canonical package identifier")
	}
	path := filepath.Join(marketplaceRoot, ".agents", "plugins", "marketplace.json")
	return "codex://plugins/" + url.PathEscape(pluginName) + "?marketplacePath=" + url.QueryEscape(path), nil
}

// desktopPluginHandoff is an optional manual next step. Only an existing
// manager-owned marketplace may be linked; this never launches the app.
func desktopPluginHandoff(marketplaceRoot, pluginName, physicalArtifactID string, cliAvailable bool) string {
	link, err := desktopPluginLink(marketplaceRoot, pluginName)
	if err != nil || physicalArtifactID == "" {
		return ""
	}
	path := filepath.Join(marketplaceRoot, ".agents", "plugins", "marketplace.json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > 64<<10 {
		return ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var document struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name   string `json:"name"`
			Source struct {
				Kind string `json:"source"`
				Path string `json:"path"`
			} `json:"source"`
		} `json:"plugins"`
	}
	if json.Unmarshal(body, &document) != nil ||
		document.Name != shared.ManagedMarketplaceName(physicalArtifactID) ||
		len(document.Plugins) != 1 ||
		document.Plugins[0].Name != pluginName ||
		document.Plugins[0].Source.Kind != "local" ||
		document.Plugins[0].Source.Path != "./" {
		return ""
	}
	launch := "Open ChatGPT Desktop directly"
	if cliAvailable {
		launch = "Run `codex app` on macOS/Windows or open Desktop directly"
	}
	constPrefix := "ChatGPT Desktop (optional, local): "
	action := constPrefix + link + ". " + launch + "; enable in Plugins > Personal and verify in a new local thread. Codex CLI verification does not prove ChatGPT runtime use."
	// The terminal sanitizes next actions to 512 runes. Refuse to print a
	// truncated deep link (including percent-encoded long profile paths).
	if utf8.RuneCountInString(action) > 390 {
		return constPrefix + "open ChatGPT Desktop manually, enable the plugin in Plugins > Personal and verify in a new local thread (deep link omitted for a long local path). Codex listing does not prove Desktop runtime use."
	}
	return action
}
