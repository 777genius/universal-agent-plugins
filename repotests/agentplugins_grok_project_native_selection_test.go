//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// This is fixture authorship in the original disposable HOME, before any child
// or protected baseline. It does not import or renew a native trust decision.
func grokProjectSelectionTrust(f *projectNativeCLIAttempt) ([]byte, error) {
	if f.spec.client != "grok" || f.base == nil {
		return nil, fmt.Errorf("Grok selection requires its original fixture")
	}
	root, home, project := f.base.Root, f.base.Home, f.base.Project
	if filepath.Dir(root) == root || project != filepath.Join(root, "project") || filepath.Dir(home) != root || home == project || home == root || project == f.other {
		return nil, fmt.Errorf("Grok trust must select only the original project A")
	}
	for _, path := range []string{root, home, project} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return nil, fmt.Errorf("Grok trust requires canonical absolute roots")
		}
		if err := projectNativeRuntimeParents(filepath.Join(path, "leaf")); err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.IsDir() || stat.Uid != uint32(os.Getuid()) {
			return nil, fmt.Errorf("Grok trust requires original ordinary owned roots")
		}
	}
	for _, name := range []string{"trusted_folders.toml", "config.toml"} {
		path := filepath.Join(home, ".grok", name)
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return nil, fmt.Errorf("Grok selection refuses an existing or unreadable config entry")
		}
	}
	quoted, err := json.Marshal(project)
	if err != nil {
		return nil, err
	}
	return []byte("[folders." + string(quoted) + "]\ntrusted = true\n"), nil
}

// The publisher's plugin enabled bit means trust. Only the separate loaded
// skill record can join the selected source to the enabled plugin registry.
func grokProjectSelectedSkill(skills []any, name, path string) error {
	wantSource := map[string]any{"type": "plugin", "plugin_name": name, "path": path}
	found := false
	for _, value := range skills {
		entry, ok := value.(map[string]any)
		if !ok || !projectGrokIdentityKeys(entry, "name description source userInvocable disabled") {
			return fmt.Errorf("Grok loaded skill identity is ambiguous")
		}
		entryName, named := entry["name"].(string)
		source, sourced := entry["source"].(map[string]any)
		if !named || entryName == "" || !sourced || !projectGrokIdentityKeys(source, "type plugin_name path") {
			return fmt.Errorf("Grok loaded skill name/source differs")
		}
		pluginName, _ := source["plugin_name"].(string)
		if shared.FoldJSONKey(entryName) != shared.FoldJSONKey(name) && shared.FoldJSONKey(pluginName) != shared.FoldJSONKey(name) {
			continue
		}
		disabled, present := entry["disabled"]
		if found || entryName != name || entry["description"] != "Harmless installed startup marker" || entry["userInvocable"] != true ||
			present && disabled != false || !reflect.DeepEqual(source, wantSource) {
			return fmt.Errorf("Grok selected loaded skill differs from its owned source")
		}
		found = true
	}
	if !found {
		return fmt.Errorf("Grok selected loaded skill is absent")
	}
	return nil
}
