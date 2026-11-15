// A disposable native CLI fixture, not Codex or authentication evidence.
package main

import (
	"bufio"
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
	if len(args) == 1 && args[0] == "app-server" {
		nativeDiscovery(root, cwd, state)
		return
	}
	if len(args) < 2 || args[0] != "plugin" {
		must(fmt.Errorf("unsupported fixture command"))
	}
	switch args[1] {
	case "list":
		entries := []any{}
		for spec, enabled := range state.Plugins {
			name, market, ok := strings.Cut(spec, "@")
			if !ok {
				must(fmt.Errorf("invalid fixture identity"))
			}
			entry := map[string]any{"pluginId": spec, "name": name, "marketplaceName": market, "installed": true, "enabled": enabled}
			if body, err := os.ReadFile(filepath.Join(state.Markets[market], "plugin.json")); err == nil {
				var manifest struct{ Name, Version string }
				must(json.Unmarshal(body, &manifest))
				if manifest.Name != name {
					must(fmt.Errorf("fixture manifest identity differs"))
				}
				if manifest.Version == "" {
					manifest.Version = "0.0.0"
				}
				entry["version"] = manifest.Version
				entry["source"] = map[string]string{"source": "local", "path": state.Markets[market]}
			}
			entries = append(entries, entry)
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
			var manifest struct {
				Name string `json:"name"`
			}
			must(json.Unmarshal(body, &manifest))
			if manifest.Name == "" {
				must(fmt.Errorf("empty fixture marketplace"))
			}
			state.Markets[manifest.Name] = args[3]
		case "update":
			if state.Markets[args[3]] == "" {
				must(fmt.Errorf("marketplace not configured or installed"))
			}
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
	for name := range state.Markets {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		fmt.Fprintf(&config, "[marketplaces.%q]\nsource = %q\nsource_type = \"local\"\n", name, state.Markets[name])
	}
	keys = nil
	for spec := range state.Plugins {
		keys = append(keys, spec)
	}
	sort.Strings(keys)
	for _, spec := range keys {
		fmt.Fprintf(&config, "[plugins.%q]\nenabled = %v\n", spec, state.Plugins[spec])
	}
	must(os.WriteFile(filepath.Join(root, "config.toml"), []byte(config.String()), 0600))
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true}))
}

// Inert native inventory is bound to the registered local source and its actual
// delivered files; it never starts an MCP server or performs a model call.
func nativeDiscovery(profile, cwd string, state registry) {
	body, err := os.ReadFile(filepath.Join(cwd, "plugin.json"))
	must(err)
	var manifest struct{ Name, Version string }
	must(json.Unmarshal(body, &manifest))
	if manifest.Version == "" {
		manifest.Version = "0.0.0"
	}
	pluginID, market := "", ""
	for id, enabled := range state.Plugins {
		name, m, _ := strings.Cut(id, "@")
		if enabled && name == manifest.Name && state.Markets[m] == cwd {
			if pluginID != "" {
				must(fmt.Errorf("ambiguous fixture source"))
			}
			pluginID, market = id, m
		}
	}
	if pluginID == "" {
		must(fmt.Errorf("unregistered fixture discovery source"))
	}
	skills := []any{}
	dirs, err := os.ReadDir(filepath.Join(cwd, "skills"))
	if err != nil && !os.IsNotExist(err) {
		must(err)
	}
	for _, dir := range dirs {
		if dir.IsDir() {
			if _, err := os.Stat(filepath.Join(cwd, "skills", dir.Name(), "SKILL.md")); err != nil {
				must(err)
			}
			skills = append(skills, map[string]any{"name": manifest.Name + ":" + dir.Name(), "scope": "user", "enabled": true, "pluginId": pluginID, "path": filepath.Join(profile, "plugins", "cache", market, manifest.Name, manifest.Version, "skills", dir.Name(), "SKILL.md")})
		}
	}
	var mcp struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	body, err = os.ReadFile(filepath.Join(cwd, ".mcp.json"))
	if err == nil {
		must(json.Unmarshal(body, &mcp))
	} else if !os.IsNotExist(err) {
		must(err)
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	initialized := false
	for scanner.Scan() {
		var request struct {
			ID     any
			Method string
			Params struct {
				ServerName string
				Cwds       []string
			}
		}
		must(json.Unmarshal(scanner.Bytes(), &request))
		var result any
		switch request.Method {
		case "initialize":
			if initialized {
				must(fmt.Errorf("duplicate initialization"))
			}
			initialized = true
			result = map[string]any{"userAgent": "inert-installer-fixture", "codexHome": profile}
		case "initialized":
			if !initialized {
				must(fmt.Errorf("uninitialized fixture"))
			}
			continue
		case "skills/list":
			if !initialized || len(request.Params.Cwds) != 1 || request.Params.Cwds[0] != cwd {
				must(fmt.Errorf("foreign fixture skills context"))
			}
			result = map[string]any{"data": []any{map[string]any{"cwd": cwd, "errors": []any{}, "skills": skills}}}
		case "mcpServerStatus/list":
			server, ok := mcp.Servers[request.Params.ServerName]
			if !initialized || !ok || server["command"] == nil {
				must(fmt.Errorf("unknown fixture MCP server"))
			}
			if kind, _ := server["type"].(string); kind != "" && kind != "stdio" {
				must(fmt.Errorf("fixture MCP is not local stdio"))
			}
			result = map[string]any{"data": []any{map[string]any{"name": request.Params.ServerName, "pluginId": pluginID, "authStatus": "unsupported", "runtimeStatus": "connected", "serverInfo": map[string]string{"name": "inert-" + request.Params.ServerName, "version": "1"}, "tools": map[string]any{}}}, "nextCursor": nil}
		default:
			must(fmt.Errorf("unsupported fixture discovery method %q", request.Method))
		}
		must(encoder.Encode(map[string]any{"id": request.ID, "result": result}))
	}
	must(scanner.Err())
}
