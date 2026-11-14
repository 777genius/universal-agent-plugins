package codex

import (
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func readPluginConfigState(path, pluginRef string) (pluginConfigState, string) {
	if strings.TrimSpace(path) == "" || strings.TrimSpace(pluginRef) == "" {
		return pluginConfigState{}, ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return pluginConfigState{}, ""
		}
		return pluginConfigState{}, "read Codex config.toml: " + err.Error()
	}
	var doc pluginConfigDoc
	if err := toml.Unmarshal(body, &doc); err != nil {
		return pluginConfigState{}, "parse Codex config.toml: " + err.Error()
	}
	entry, ok := doc.Plugins[pluginRef]
	if !ok {
		return pluginConfigState{}, ""
	}
	state := pluginConfigState{Present: true}
	if entry.Enabled != nil && !*entry.Enabled {
		state.Disabled = true
	}
	return state, ""
}
