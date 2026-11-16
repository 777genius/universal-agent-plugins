package agentpluginscli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tailscale/hujson"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Native outputs describe only the selected fixture's installed files. The
// fixture never executes a client, a model or the configured MCP commands.
func dependencyOpenCodeObservation(command legacyports.Command) (legacyports.CommandResult, error) {
	body, err := dependencyOpenCodeOutput(command)
	if err != nil {
		return legacyports.CommandResult{NoChildStarted: true}, err
	}
	return legacyports.CommandResult{Stdout: body, CleanupComplete: true}, nil
}
func dependencyOpenCodeOutput(command legacyports.Command) ([]byte, error) {
	// The selected OpenCode runner receives the prepared layout as its working
	// directory. Its installed files are the fixture's observation authority.
	selected := command.Dir
	if !filepath.IsAbs(selected) || command.StdoutLimitBytes == 0 || len(command.Argv) != 3 {
		return nil, fmt.Errorf("unexpected selected OpenCode fixture context: %v", command.Argv)
	}
	config, err := dependencyOpenCodeConfig(selected)
	if err != nil {
		return nil, err
	}
	switch command.Argv[1] + " " + command.Argv[2] {
	case "debug config":
		return config, nil
	case "debug skill":
		return dependencyOpenCodeSkills(selected)
	case "mcp list":
		return dependencyOpenCodeMCP(config)
	default:
		return nil, fmt.Errorf("unsupported OpenCode fixture observation")
	}
}
func dependencyOpenCodeConfig(root string) ([]byte, error) {
	var selected []byte
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if selected != nil {
			return nil, fmt.Errorf("fixture has ambiguous native config files")
		}
		if len(body) > 4<<20 {
			return nil, fmt.Errorf("fixture native config exceeds bound")
		}
		selected, err = hujson.Standardize(body)
		if err != nil {
			return nil, err
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("selected fixture native config is missing")
	}
	return selected, nil
}
func dependencyOpenCodeSkills(root string) ([]byte, error) {
	directories, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		return nil, err
	}
	rows := []map[string]string{}
	for _, directory := range directories {
		if !directory.IsDir() {
			continue
		}
		path := filepath.Join(root, "skills", directory.Name(), "SKILL.md")
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		document := strings.ReplaceAll(string(body), "\r\n", "\n")
		if !strings.HasPrefix(document, "---\n") {
			return nil, fmt.Errorf("fixture skill has no metadata")
		}
		header, content, ok := strings.Cut(document[4:], "\n---\n")
		if !ok {
			return nil, fmt.Errorf("fixture skill metadata is incomplete")
		}
		name := ""
		for _, line := range strings.Split(header, "\n") {
			if value, ok := strings.CutPrefix(line, "name:"); ok {
				name = strings.TrimSpace(value)
			}
		}
		if name == "" || name != directory.Name() {
			return nil, fmt.Errorf("fixture skill name differs from its installed directory")
		}
		rows = append(rows, map[string]string{"name": name, "location": path, "content": content})
	}
	return json.Marshal(rows)
}
func dependencyOpenCodeMCP(config []byte) ([]byte, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(config, &document); err != nil {
		return nil, err
	}
	var servers map[string]map[string]any
	if err := json.Unmarshal(document["mcp"], &servers); err != nil {
		return nil, err
	}
	if nested, ok := servers["servers"]; ok && nested["type"] == nil {
		body, err := json.Marshal(nested)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &servers); err != nil {
			return nil, err
		}
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	var output strings.Builder
	output.WriteString("┌  MCP Servers\n")
	for _, name := range names {
		server := servers[name]
		commands, local := server["command"].([]any)
		status := "✗ " + name + " unobserved"
		if server["type"] == "local" && local && len(commands) > 0 && server["enabled"] != false && server["disabled"] != true {
			status = "✓ " + name + " connected"
		}
		output.WriteString("●  " + status + "\n")
	}
	fmt.Fprintf(&output, "└  %d server(s)\n", len(names))
	return []byte(output.String()), nil
}
