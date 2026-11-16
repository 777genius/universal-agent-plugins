package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// This finite fixture models native CLI cleanup of only this request's exact
// namespace. The final inventory is read from the resulting selected profile.
func codexRemovalRunner(t *testing.T, request domain.DeactivationRequest, alreadyAbsent bool) *recordingRunner {
	t.Helper()
	marketplace := shared.ManagedMarketplaceName(request.PhysicalArtifactID)
	pluginID := request.DeclaredName + "@" + marketplace
	return &recordingRunner{run: func(command legacyports.Command) legacyports.CommandResult {
		t.Helper()
		fail := func(err error) legacyports.CommandResult {
			return legacyports.CommandResult{ExitCode: 1, Stderr: []byte(err.Error())}
		}
		profile := ""
		for _, entry := range command.Env {
			if strings.HasPrefix(entry, "CODEX_HOME=") {
				profile = strings.TrimPrefix(entry, "CODEX_HOME=")
			}
		}
		if profile != request.Client.ConfigRoot {
			return fail(fmt.Errorf("fixture cleanup lost selected profile"))
		}
		path := filepath.Join(profile, "config.toml")
		document := map[string]any{}
		body, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return fail(err)
		}
		if len(body) > 0 {
			if err := toml.Unmarshal(body, &document); err != nil {
				return fail(err)
			}
		}
		args := command.Argv[1:]
		var group, key string
		switch {
		case len(args) == 4 && args[0] == "plugin" && args[1] == "remove" && args[2] == pluginID && args[3] == "--json":
			group, key = "plugins", pluginID
		case len(args) == 5 && args[0] == "plugin" && args[1] == "marketplace" && args[2] == "remove" && args[3] == marketplace && args[4] == "--json":
			group, key = "marketplaces", marketplace
		case len(args) == 3 && args[0] == "plugin" && args[1] == "list" && args[2] == "--json":
			rows := []map[string]any{}
			if plugins, ok := document["plugins"].(map[string]any); ok {
				for id, value := range plugins {
					name, market, qualified := strings.Cut(id, "@")
					if !qualified {
						return fail(fmt.Errorf("fixture plugin namespace is unqualified"))
					}
					entry, ok := value.(map[string]any)
					if !ok {
						return fail(fmt.Errorf("fixture plugin entry is invalid"))
					}
					rows = append(rows, map[string]any{"pluginId": id, "name": name, "marketplaceName": market, "installed": true, "enabled": entry["enabled"]})
				}
			}
			out, err := json.Marshal(map[string]any{"installed": rows})
			if err != nil {
				return fail(err)
			}
			return legacyports.CommandResult{Stdout: out}
		default:
			return fail(fmt.Errorf("unexpected fixture Codex cleanup command"))
		}
		if alreadyAbsent {
			return legacyports.CommandResult{ExitCode: 1, Stderr: []byte("exact fixture plugin is not configured or installed")}
		}
		if entries, ok := document[group].(map[string]any); ok {
			delete(entries, key)
		}
		next, err := toml.Marshal(document)
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(path, next, 0600); err != nil {
			return fail(err)
		}
		return legacyports.CommandResult{Stdout: []byte(`{"ok":true}`)}
	}}
}
