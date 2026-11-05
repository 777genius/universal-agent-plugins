package codex

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

// Official ChatGPT Desktop deep link: it opens a local plugin's detail page,
// not an installation or proof that the app loaded the plugin.
var desktopPluginName = regexp.MustCompile("^[a-z][a-z0-9-]*$")

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
// manager-prepared marketplace may be linked, and this never launches the app.
func desktopPluginHandoff(marketplaceRoot, pluginName string) string {
	link, err := desktopPluginLink(marketplaceRoot, pluginName)
	if err != nil {
		return ""
	}
	path := filepath.Join(marketplaceRoot, ".agents", "plugins", "marketplace.json")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return "ChatGPT Desktop local plugin details: " + link + ". Restart ChatGPT Desktop if the marketplace is not visible; finish installation or enablement in Plugins > Personal, then verify in a new local thread. Codex CLI verification does not prove ChatGPT runtime use."
}
