package installer_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// This models only the observed official scoped commands. It is fixture
// evidence for installer transactions; the native executable test is separate.
func (r *claudeSDKRunner) marketplaceCommand(args []string) (legacyports.CommandResult, error) {
	if r.project == "" || len(args) < 4 || args[0] != "plugin" || !strings.Contains(strings.Join(args, " "), "--scope project") {
		r.t.Fatalf("unexpected native mutation: %v", args)
	}
	knownPath := filepath.Join(r.user, "plugins", "known_marketplaces.json")
	indexPath := filepath.Join(r.user, "plugins", "installed_plugins.json")
	known := r.readObject(knownPath)
	index := r.readObject(indexPath)
	plugins, ok := index["plugins"].(map[string]any)
	if !ok {
		plugins = map[string]any{}
	}
	index["plugins"], index["version"] = plugins, 2
	if args[1] == "marketplace" {
		switch args[2] {
		case "add":
			root := args[3]
			manifest := r.readObject(filepath.Join(root, ".claude-plugin", "marketplace.json"))
			name, ok := manifest["name"].(string)
			if !ok {
				r.t.Fatal("fixture marketplace lacks name")
			}
			known[name] = map[string]any{"source": map[string]any{"source": "directory", "path": root}, "installLocation": root}
		case "remove":
			delete(known, args[3])
		default:
			r.t.Fatalf("unknown marketplace command: %v", args)
		}
		r.writeObject(knownPath, known)
		return legacyports.CommandResult{}, nil
	}
	id := args[2]
	switch args[1] {
	case "install":
		_, market, _ := strings.Cut(id, "@")
		entry, ok := known[market].(map[string]any)
		if !ok {
			return legacyports.CommandResult{ExitCode: 1}, nil
		}
		root := entry["installLocation"].(string)
		bundle := filepath.Join(root, ".agentplugins-plugin")
		manifest := r.readObject(filepath.Join(bundle, ".claude-plugin", "plugin.json"))
		cache := filepath.Join(r.user, "plugins", "cache", market, manifest["name"].(string), manifest["version"].(string))
		if err := os.RemoveAll(cache); err != nil {
			r.t.Fatal(err)
		}
		if err := filetree.CopyDir(bundle, cache); err != nil {
			r.t.Fatal(err)
		}
		plugins[id] = []any{map[string]any{"scope": "project", "projectPath": r.project, "installPath": cache, "version": manifest["version"]}}
	case "uninstall":
		rows, _ := plugins[id].([]any)
		kept := []any{}
		for _, value := range rows {
			row := value.(map[string]any)
			if row["scope"] != "project" || row["projectPath"] != r.project {
				kept = append(kept, row)
			}
		}
		if len(kept) == 0 {
			delete(plugins, id)
		} else {
			plugins[id] = kept
		}
	default:
		r.t.Fatalf("unknown native operation: %v", args)
	}
	r.writeObject(indexPath, index)
	return legacyports.CommandResult{}, nil
}

func (r *claudeSDKRunner) marketplaceListing() []map[string]any {
	index := r.readObject(filepath.Join(r.user, "plugins", "installed_plugins.json"))
	plugins, _ := index["plugins"].(map[string]any)
	ids := make([]string, 0, len(plugins))
	for id := range plugins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := []map[string]any{}
	for _, id := range ids {
		rows, _ := plugins[id].([]any)
		for _, value := range rows {
			row := value.(map[string]any)
			listed := map[string]any{"id": id, "enabled": row["scope"] == "project" && row["projectPath"] == r.project}
			for key, value := range row {
				listed[key] = value
			}
			result = append(result, listed)
		}
	}
	return result
}

func (r *claudeSDKRunner) readObject(path string) map[string]any {
	r.t.Helper()
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}
	}
	if err != nil {
		r.t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil {
		r.t.Fatal(err)
	}
	return object
}
func (r *claudeSDKRunner) writeObject(path string, object map[string]any) {
	r.t.Helper()
	body, err := json.Marshal(object)
	if err != nil {
		r.t.Fatal(fmt.Errorf("fixture JSON: %w", err))
	}
	claudeSDKWrite(r.t, path, string(body))
}
