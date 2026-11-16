package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Inert discovery describes only the exact package registered by this fixture.
// It never executes a client, MCP server, network request or model call.
// The initialize version models the qualified 0.162.0 selected-server protocol.
func fixtureCodexDiscovery(command legacyports.Command, registeredIDs map[string]bool, exchange func(io.Writer, io.Reader) error) error {
	if len(command.Argv) != 2 || command.Argv[1] != "app-server" {
		return fmt.Errorf("unexpected fixture discovery command")
	}
	profile := ""
	for _, entry := range command.Env {
		if strings.HasPrefix(entry, "CODEX_HOME=") {
			profile = strings.TrimPrefix(entry, "CODEX_HOME=")
		}
	}
	if profile == "" {
		return errors.New("discovery lost selected profile")
	}
	manifestBytes, err := os.ReadFile(filepath.Join(command.Dir, "plugin.json"))
	if err != nil {
		return err
	}
	var manifest struct{ Name, Version string }
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return err
	}
	version := manifest.Version
	if version == "" {
		version = "1.0.0"
	}
	marketplaceBody, err := os.ReadFile(filepath.Join(command.Dir, ".agents", "plugins", "marketplace.json"))
	if err != nil {
		return err
	}
	var source struct {
		Name    string
		Plugins []struct {
			Name   string
			Source struct{ Source, Path string }
		}
	}
	if err := json.Unmarshal(marketplaceBody, &source); err != nil {
		return err
	}
	if source.Name == "" || len(source.Plugins) != 1 || source.Plugins[0].Name != manifest.Name || source.Plugins[0].Source.Source != "local" || source.Plugins[0].Source.Path != "./" {
		return errors.New("fixture marketplace does not identify the selected local package")
	}
	marketplace := source.Name
	pluginID := manifest.Name + "@" + marketplace
	if !registeredIDs[pluginID] {
		return errors.New("exact native fixture plugin is not registered")
	}

	dirs, err := os.ReadDir(filepath.Join(command.Dir, "skills"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	skills := []map[string]any{}
	for _, dir := range dirs {
		if dir.IsDir() {
			skills = append(skills, map[string]any{"name": manifest.Name + ":" + dir.Name(), "scope": "user", "enabled": true, "pluginId": pluginID, "path": filepath.Join(profile, "plugins", "cache", marketplace, manifest.Name, version, "skills", dir.Name(), "SKILL.md")})
		}
	}
	var responses bytes.Buffer
	encoder := json.NewEncoder(&responses)
	if err := encoder.Encode(map[string]any{"id": 1, "result": map[string]any{"userAgent": "agentplugins/0.162.0", "codexHome": profile}}); err != nil {
		return err
	}
	if err := encoder.Encode(map[string]any{"id": 3, "result": map[string]any{"data": []any{map[string]any{"cwd": command.Dir, "errors": []any{}, "skills": skills}}}}); err != nil {
		return err
	}

	servers, err := fixtureProjectedMCP(command.Dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(servers))
	for name, server := range servers {
		if kind, _ := server["type"].(string); kind != "" && kind != "stdio" {
			return fmt.Errorf("fixture Codex discovery requires explicit local stdio server %q", name)
		}
		if executable, _ := server["command"].(string); executable == "" {
			return fmt.Errorf("fixture MCP server %q has no command", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for index, name := range names {
		if err := encoder.Encode(map[string]any{"id": index + 4, "result": map[string]any{"data": []any{map[string]any{"name": name, "pluginId": pluginID, "authStatus": "unsupported", "runtimeStatus": "connected", "serverInfo": map[string]string{"name": "inert-" + name, "version": "1"}, "tools": map[string]any{}}}, "nextCursor": nil}}); err != nil {
			return err
		}
	}
	var requests bytes.Buffer
	if err := exchange(&requests, &responses); err != nil {
		return err
	}
	decoder := json.NewDecoder(&requests)
	for _, method := range []string{"initialize", "initialized", "skills/list"} {
		var request struct{ Method string }
		if err := decoder.Decode(&request); err != nil {
			return err
		}
		if request.Method != method {
			return fmt.Errorf("unexpected fixture discovery method %q", request.Method)
		}
	}

	for _, name := range names {
		var request struct {
			Method string
			Params struct{ ServerName string }
		}
		if err := decoder.Decode(&request); err != nil {
			return err
		}
		if request.Method != "mcpServerStatus/list" || request.Params.ServerName != name {
			return fmt.Errorf("unexpected fixture MCP discovery request for %q", request.Params.ServerName)
		}
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected extra discovery request")
	}
	return nil
}

func fixtureProjectedMCP(root string) (map[string]map[string]any, error) {
	body, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if os.IsNotExist(err) {
		return map[string]map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var document struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if document.MCPServers == nil {
		return nil, errors.New("fixture projection has no MCP server collection")
	}
	return document.MCPServers, nil
}

func (r *recordingCodexRunner) RunDuplexToCompletion(_ context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	r.calls = append(r.calls, append([]string(nil), command.Argv...))
	return fixtureCodexDiscovery(command, map[string]bool{r.pluginID: r.installed}, exchange)
}

func fixtureCodexEntry(id, root string, enabled bool) (map[string]any, error) {
	name, market, ok := strings.Cut(id, "@")
	if !ok || root == "" {
		return nil, errors.New("fixture plugin has no registered source")
	}
	body, err := os.ReadFile(filepath.Join(root, "plugin.json"))
	if err != nil {
		return nil, err
	}
	var manifest struct{ Name, Version string }
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, err
	}
	if manifest.Name != name {
		return nil, errors.New("fixture plugin manifest differs from registration")
	}
	if manifest.Version == "" {
		manifest.Version = "1.0.0"
	}
	return map[string]any{"pluginId": id, "name": name, "marketplaceName": market, "installed": true, "enabled": enabled, "version": manifest.Version, "source": map[string]string{"source": "local", "path": root}}, nil
}

func fixtureClaudeHealth(selector string, listings []map[string]any) ([]byte, error) {
	parts := strings.Split(selector, ":")
	if len(parts) != 3 || parts[0] != "plugin" || parts[1] == "" || parts[2] == "" {
		return nil, fmt.Errorf("invalid fixture MCP selector")
	}
	matches := 0
	var selected map[string]any
	for _, row := range listings {
		id, _ := row["id"].(string)
		name, _, _ := strings.Cut(id, "@")
		if name != parts[1] || row["enabled"] != true {
			continue
		}
		root, _ := row["installPath"].(string)
		body, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
		if err != nil {
			return nil, err
		}
		var servers map[string]map[string]any
		if err := json.Unmarshal(body, &servers); err != nil {
			return nil, err
		}
		server, exists := servers[parts[2]]
		if !exists {
			return nil, fmt.Errorf("selected fixture plugin has no MCP server %q", parts[2])
		}
		selected = server
		matches++
	}
	if matches != 1 {
		return nil, fmt.Errorf("fixture MCP selector must resolve exactly once, got %d", matches)
	}
	kind, _ := selected["type"].(string)
	if kind == "" {
		if command, _ := selected["command"].(string); command != "" {
			kind = "stdio"
		}
	}
	if kind != "stdio" && kind != "http" && kind != "sse" {
		return nil, fmt.Errorf("unsupported delivered fixture MCP transport %q", kind)
	}
	return []byte(fmt.Sprintf("%s:\n  Scope: Dynamic config (from command line)\n  Status: ✔ Connected\n  Type: %s\n", selector, kind)), nil
}

// Each explicit CODEX_HOME has an independent inert native registry. Value
// listingRunner copies share only that profile's fixture client, as real CLI
// subprocesses would; no real native client is launched.
type fixtureCodexState struct {
	mu    sync.Mutex
	codex recordingCodexRunner
}

var fixtureCodexStates sync.Map

func fixtureCodexStateForCommand(command legacyports.Command) (*fixtureCodexState, bool) {
	for _, entry := range command.Env {
		if profile, ok := strings.CutPrefix(entry, "CODEX_HOME="); ok && profile != "" {
			state, _ := fixtureCodexStates.LoadOrStore(profile, &fixtureCodexState{})
			return state.(*fixtureCodexState), true
		}
	}
	return nil, false
}
func (r listingRunner) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	state, ok := fixtureCodexStateForCommand(command)
	if !ok {
		return errors.New("fixture discovery has no selected Codex profile")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.codex.RunDuplexToCompletion(ctx, command, exchange)
}
func (r *groupHostReconciliationRunner) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	r.calls = append(r.calls, append([]string(nil), command.Argv...))
	return r.codex.RunDuplexToCompletion(ctx, command, exchange)
}
func (r *activationStopListingRunner) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	return r.inner.RunDuplexToCompletion(ctx, command, exchange)
}
func (r *capturingRunner) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	r.calls = append(r.calls, append([]string(nil), command.Argv...))
	return r.inner.RunDuplexToCompletion(ctx, command, exchange)
}

func seedCodexProjectTrust(t *testing.T, profile, project string) {
	t.Helper()
	path := filepath.Join(profile, "config.toml")
	deathWrite(t, path, "projects."+strconv.Quote(project)+".trust_level = 'trusted'\n")
	// Seed the normal exact-file guard too, so the untouched-user assertion
	// remains byte-for-byte strict throughout later confirmed operations.
	handle, err := nativeconfig.New().BeginExactFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
}
