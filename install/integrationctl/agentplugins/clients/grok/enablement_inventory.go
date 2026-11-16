package grok

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

type grokACPPlugin struct {
	id, name, root, scope, version string
	enabled, trusted               bool
	conflict                       bool
}

func parseEnablementPlugins(raw any) ([]grokACPPlugin, error) {
	rows, ok := raw.([]any)
	if !ok || len(rows) > 128 {
		return nil, fmt.Errorf("grok ACP plugin inventory is not bounded")
	}
	plugins := make([]grokACPPlugin, 0, len(rows))
	ids, roots, names := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		plugin, err := parseEnablementPlugin(row)
		if err != nil {
			return nil, err
		}
		id, root, name := shared.FoldJSONKey(plugin.id), shared.FoldJSONKey(plugin.root), shared.FoldJSONKey(plugin.name)
		if ids[id] || roots[root] || names[name] {
			return nil, fmt.Errorf("grok ACP plugin identity is duplicated or aliased")
		}
		ids[id], roots[root], names[name] = true, true, true
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

func parseEnablementPlugin(raw any) (grokACPPlugin, error) {
	object, ok := raw.(map[string]any)
	if !ok {
		return grokACPPlugin{}, fmt.Errorf("grok ACP plugin is not an object")
	}
	var row grokACPPlugin
	var valid bool
	stringsByField := map[string]*string{"id": &row.id, "name": &row.name, "root": &row.root, "scope": &row.scope}
	for key, field := range stringsByField {
		*field, valid = object[key].(string)
		if !valid || *field == "" || strings.IndexByte(*field, 0) >= 0 {
			return row, fmt.Errorf("grok ACP plugin %s is absent or invalid", key)
		}
	}
	if err := row.validateSource(); err != nil {
		return row, err
	}
	return parseEnablementPluginState(row, object)
}

func (row grokACPPlugin) validateSource() error {
	if !filepath.IsAbs(row.root) || filepath.Clean(row.root) != row.root || pathpolicy.ValidateLeafID(row.name) != nil {
		return fmt.Errorf("grok ACP plugin source identity is invalid")
	}
	if row.scope != "project" && row.scope != "user" && row.scope != "config" && row.scope != "cli" {
		return fmt.Errorf("grok ACP plugin scope is unknown")
	}
	if row.id != row.scope+"/"+enablementDigest([]byte(row.root))[7:15]+"/"+row.name {
		return fmt.Errorf("grok ACP full plugin ID differs from its source")
	}
	return nil
}

func parseEnablementPluginState(row grokACPPlugin, object map[string]any) (grokACPPlugin, error) {
	var valid bool
	if value, exists := object["version"]; exists && value != nil {
		row.version, valid = value.(string)
		if !valid || strings.IndexByte(row.version, 0) >= 0 {
			return row, fmt.Errorf("grok ACP plugin version is invalid")
		}
	}
	row.enabled, valid = object["enabled"].(bool)
	if !valid {
		return row, fmt.Errorf("grok ACP plugin enabled state is absent")
	}
	row.trusted, valid = object["trusted"].(bool)
	if !valid {
		return row, fmt.Errorf("grok ACP plugin trust state is absent")
	}
	if value, exists := object["conflict"]; exists && value != nil {
		conflict, ok := value.(string)
		if !ok || conflict == "" {
			return row, fmt.Errorf("grok ACP plugin conflict is invalid")
		}
		row.conflict = true
	}
	return row, nil
}

func selectEnablementPlugin(plugins []grokACPPlugin, path, name, version string) (grokACPPlugin, error) {
	for _, plugin := range plugins {
		if shared.FoldJSONKey(plugin.name) != shared.FoldJSONKey(name) && shared.FoldJSONKey(plugin.root) != shared.FoldJSONKey(path) {
			continue
		}
		if plugin.name != name || plugin.root != path || plugin.scope != "project" || plugin.version != version || plugin.conflict {
			return grokACPPlugin{}, fmt.Errorf("grok native project inventory differs from the exact managed plugin")
		}
		return plugin, nil
	}
	return grokACPPlugin{}, fmt.Errorf("grok native project inventory does not contain the exact managed plugin")
}
