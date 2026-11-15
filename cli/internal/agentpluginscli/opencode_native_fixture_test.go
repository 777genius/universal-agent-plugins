package agentpluginscli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tailscale/hujson"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// fixtureOpenCodeNative is an inert native-CLI observer for disposable user
// profiles. Real OpenCode discovery is covered by separate native E2E; this
// test seam must derive its catalog from the exact installed native files.
func fixtureOpenCodeNative(command legacyports.Command) (legacyports.CommandResult, error) {
	if len(command.Argv) != 3 || !filepath.IsAbs(command.Dir) {
		return legacyports.CommandResult{}, fmt.Errorf("unexpected OpenCode fixture argv: %v", command.Argv)
	}
	selected := false
	for _, value := range command.Env {
		if value == "OPENCODE_CONFIG_DIR="+command.Dir {
			selected = true
		}
	}
	if !selected {
		return legacyports.CommandResult{}, errors.New("OpenCode fixture lost its selected native profile")
	}

	config := []byte("{}")
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		body, err := os.ReadFile(filepath.Join(command.Dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		config, err = hujson.Standardize(body)
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		break
	}
	args := command.Argv[1:]
	if args[0] == "debug" && args[1] == "config" {
		return legacyports.CommandResult{Stdout: config}, nil
	}
	if args[0] == "debug" && args[1] == "skill" {
		entries, err := os.ReadDir(filepath.Join(command.Dir, "skills"))
		if os.IsNotExist(err) {
			return legacyports.CommandResult{Stdout: []byte("[]")}, nil
		}
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		skills := []map[string]string{}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(command.Dir, "skills", entry.Name(), "SKILL.md")
			body, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return legacyports.CommandResult{}, err
			}
			document := strings.ReplaceAll(string(body), "\r\n", "\n")
			if !strings.HasPrefix(document, "---\n") {
				return legacyports.CommandResult{}, fmt.Errorf("invalid OpenCode fixture Skill: %s", entry.Name())
			}
			_, content, ok := strings.Cut(document[4:], "\n---\n")
			if !ok {
				return legacyports.CommandResult{}, fmt.Errorf("missing OpenCode fixture Skill body: %s", entry.Name())
			}
			skills = append(skills, map[string]string{"name": entry.Name(), "location": path, "content": content})
		}
		body, err := json.Marshal(skills)
		return legacyports.CommandResult{Stdout: body}, err
	}
	if args[0] == "mcp" && args[1] == "list" {
		var root map[string]json.RawMessage
		if err := json.Unmarshal(config, &root); err != nil {
			return legacyports.CommandResult{}, err
		}
		var servers map[string]json.RawMessage
		if raw := root["mcp"]; len(raw) != 0 {
			if err := json.Unmarshal(raw, &servers); err != nil {
				return legacyports.CommandResult{}, err
			}
		}
		if wrapped, present := servers["servers"]; present {
			var inside map[string]json.RawMessage
			if err := json.Unmarshal(wrapped, &inside); err != nil {
				return legacyports.CommandResult{}, err
			}
			if _, server := inside["type"]; !server {
				servers = inside
			}
		}
		names := make([]string, 0, len(servers))
		for name, raw := range servers {
			var entry map[string]json.RawMessage
			if err := json.Unmarshal(raw, &entry); err != nil {
				return legacyports.CommandResult{}, err
			}
			var kind string
			var argv []string
			if err := json.Unmarshal(entry["type"], &kind); err != nil || kind != "local" {
				return legacyports.CommandResult{}, fmt.Errorf("fixture MCP %s is not local", name)
			}
			if err := json.Unmarshal(entry["command"], &argv); err != nil || len(argv) == 0 {
				return legacyports.CommandResult{}, fmt.Errorf("fixture MCP %s has no local command", name)
			}
			names = append(names, name)
		}
		sort.Strings(names)
		var listed strings.Builder
		listed.WriteString("┌  MCP Servers\n")
		for _, name := range names {
			fmt.Fprintf(&listed, "●  ✓ %s connected\n", name)
		}
		fmt.Fprintf(&listed, "└  %d server(s)\n", len(names))
		return legacyports.CommandResult{Stdout: []byte(listed.String())}, nil
	}
	return legacyports.CommandResult{}, fmt.Errorf("unexpected OpenCode fixture command: %v", command.Argv)
}
