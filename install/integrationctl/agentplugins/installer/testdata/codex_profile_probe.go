// A disposable native CLI fixture, not Codex or authentication evidence.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type registry struct {
	Markets map[string]string `json:"markets"`
	Plugins map[string]bool   `json:"plugins"`
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	root := os.Getenv("CODEX_HOME")
	if !filepath.IsAbs(root) {
		must(fmt.Errorf("fixture requires explicit CODEX_HOME"))
	}
	info, err := os.Stat(root)
	must(err)
	if !info.IsDir() {
		must(fmt.Errorf("fixture requires an existing profile"))
	}
	args := os.Args[1:]
	log, err := os.OpenFile(filepath.Join(root, "fixture-commands.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	must(err)
	cwd, err := os.Getwd()
	must(err)
	must(json.NewEncoder(log).Encode(map[string]any{"root": root, "cwd": cwd, "args": args}))
	must(log.Close())
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println("codex-cli 1.2.3")
		return
	}
	statePath := filepath.Join(root, "fixture-registry.json")
	state := registry{Markets: map[string]string{}, Plugins: map[string]bool{}}
	body, err := os.ReadFile(statePath)
	if err == nil {
		must(json.Unmarshal(body, &state))
	} else if !os.IsNotExist(err) {
		must(err)
	}
	if len(args) < 2 || args[0] != "plugin" {
		must(fmt.Errorf("unsupported fixture command"))
	}
	switch args[1] {
	case "list":
		entries := []any{}
		for spec, enabled := range state.Plugins {
			name, market, ok := strings.Cut(spec, "@")
			if !ok { must(fmt.Errorf("invalid fixture identity")) }
			entries = append(entries, map[string]any{"pluginId": spec, "name": name, "marketplaceName": market, "installed": true, "enabled": enabled})
		}
		must(json.NewEncoder(os.Stdout).Encode(map[string]any{"installed": entries}))
		return
	case "add":
		state.Plugins[args[2]] = true
	case "remove":
		delete(state.Plugins, args[2])
	case "marketplace":
		switch args[2] {
		case "add":
			body, err := os.ReadFile(filepath.Join(args[3], ".agents", "plugins", "marketplace.json"))
			must(err)
			var manifest struct { Name string `json:"name"` }
			must(json.Unmarshal(body, &manifest))
			if manifest.Name == "" { must(fmt.Errorf("empty fixture marketplace")) }
			state.Markets[manifest.Name] = args[3]
		case "update":
			if state.Markets[args[3]] == "" { must(fmt.Errorf("marketplace not configured or installed")) }
		case "remove":
			delete(state.Markets, args[3])
		default:
			must(fmt.Errorf("unsupported fixture marketplace command"))
		}
	default:
		must(fmt.Errorf("unsupported fixture plugin command"))
	}
	body, err = json.Marshal(state)
	must(err)
	must(os.WriteFile(statePath, body, 0600))
	var config strings.Builder
	keys := []string{}
	for name := range state.Markets { keys = append(keys, name) }
	sort.Strings(keys)
	for _, name := range keys {
		fmt.Fprintf(&config, "[marketplaces.%q]\nsource = %q\nsource_type = \"local\"\n", name, state.Markets[name])
	}
	keys = nil
	for spec := range state.Plugins { keys = append(keys, spec) }
	sort.Strings(keys)
	for _, spec := range keys { fmt.Fprintf(&config, "[plugins.%q]\nenabled = %v\n", spec, state.Plugins[spec]) }
	must(os.WriteFile(filepath.Join(root, "config.toml"), []byte(config.String()), 0600))
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true}))
}
