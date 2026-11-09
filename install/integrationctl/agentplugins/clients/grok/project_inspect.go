package grok

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type projectPlugin struct {
	name, scope, path string
	enabled           bool
	mcpServers        int64
}
type projectInspection struct {
	version string
	plugins []projectPlugin
}

func parseProjectInspect(body []byte, project string) (projectInspection, error) {
	fail := func() (projectInspection, error) {
		return projectInspection{}, fmt.Errorf("grok project inspect contract (1.0.67 or 1.0.70) is not recognized")
	}
	value, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return fail()
	}
	if !exactProjectFields(value, []string{"grokVersion", "cwd", "projectRoot", "projectTrusted", "plugins", "skills"}) {
		return fail()
	}
	if (value["grokVersion"] != "1.0.67" && value["grokVersion"] != "1.0.70") || value["cwd"] != project || (value["projectRoot"] != project && value["projectRoot"] != project+string(filepath.Separator)) {
		return fail()
	}
	if _, ok := value["projectTrusted"].(bool); !ok {
		return fail()
	}
	if _, ok := value["skills"].([]any); !ok {
		return fail()
	}
	entries, ok := value["plugins"].([]any)
	if !ok {
		return fail()
	}
	version, _ := value["grokVersion"].(string)
	out := projectInspection{version: version}
	seen := map[string]bool{}
	for _, entry := range entries {
		parsed, err := parseProjectPlugin(entry)
		if err != nil {
			return fail()
		}
		// A case-folded alias cannot represent an independent native plugin
		// identity, including on case-insensitive project filesystems.
		identity := shared.FoldJSONKey(parsed.name)
		if seen[identity] {
			return fail()
		}
		seen[identity] = true
		out.plugins = append(out.plugins, parsed)
	}
	return out, nil
}
func exactProjectFields(object map[string]any, fields []string) bool {
	for key := range object {
		for _, field := range fields {
			if shared.FoldJSONKey(key) == shared.FoldJSONKey(field) && key != field {
				return false
			}
		}
	}
	return true
}
func parseProjectPlugin(entry any) (projectPlugin, error) {
	object, ok := entry.(map[string]any)
	if !ok || !exactProjectFields(object, []string{"name", "scope", "path", "enabled", "provides"}) {
		return projectPlugin{}, fmt.Errorf("grok plugin inspect identity is not recognized")
	}
	name, n := object["name"].(string)
	scope, s := object["scope"].(string)
	path, p := object["path"].(string)
	enabled, e := object["enabled"].(bool)
	if !n || !s || !p || !e || strings.TrimSpace(name) == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || (scope != "project" && scope != "user") {
		return projectPlugin{}, fmt.Errorf("grok plugin inspect identity is not recognized")
	}
	// Directory discovery and MCP discovery are separate native observations.
	// Unreported MCP servers must never be treated as loaded or available.
	var mcpServers int64
	if raw, present := object["provides"]; present {
		provides, valid := raw.(map[string]any)
		if !valid || !exactProjectFields(provides, []string{"mcpServers"}) {
			return projectPlugin{}, fmt.Errorf("grok plugin provides contract is not recognized")
		}
		if count, present := provides["mcpServers"]; present {
			number, valid := count.(json.Number)
			if !valid {
				return projectPlugin{}, fmt.Errorf("grok plugin MCP count is not numeric")
			}
			parsed, err := number.Int64()
			if err != nil || parsed < 0 {
				return projectPlugin{}, fmt.Errorf("grok plugin MCP count is invalid")
			}
			mcpServers = parsed
		}
	}
	return projectPlugin{name: name, scope: scope, path: path, enabled: enabled, mcpServers: mcpServers}, nil
}
func inspectProject(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, owned, requireMCPDiscovery bool) (clients.RegistryFinding, error) {
	root, err := grokPlanRoot(client, plan)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if env.Runner == nil || plan.NativeRegistryExecutable == "" || plan.NativeRegistryExecutable != client.ExecutablePath {
		return clients.RegistryIndeterminate, fmt.Errorf("pinned Grok project inspect CLI unavailable or differs from detected client")
	}
	finding, err := inspectProjectFiles(plan, owned)
	if err != nil || finding == clients.RegistryCollision {
		return finding, err
	}
	bounded, cancel := context.WithTimeout(ctx, grokListTimeout)
	defer cancel()
	result, err := shared.RunNativeRegistry(bounded, env.Runner, legacyports.Command{Argv: []string{plan.NativeRegistryExecutable, "inspect", "--json"}, Dir: filepath.Dir(root), StdoutLimitBytes: grokListStdoutLimitBytes})
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		return clients.RegistryIndeterminate, fmt.Errorf("grok project inspect failed or emitted stderr")
	}
	native, err := parseProjectInspect(result.Stdout, filepath.Dir(root))
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if shared.HasSupportedMCP(plan.Components) && native.version != "1.0.70" {
		return clients.RegistryIndeterminate, fmt.Errorf("grok project MCP preparation requires qualified Grok 1.0.70; 1.0.67 remains qualified for Skills preparation only")
	}
	for _, entry := range native.plugins {
		if entry.name == plan.DeclaredName {
			if owned && entry.scope == "project" && entry.path == plan.ActivePath {
				if finding != clients.RegistryExpected {
					return clients.RegistryIndeterminate, fmt.Errorf("grok project source lacks its exact prepared manifest")
				}
				if requireMCPDiscovery && shared.HasSupportedMCP(plan.Components) && entry.mcpServers != int64(len(shared.SupportedMCPNames(plan))) {
					return clients.RegistryIndeterminate, fmt.Errorf("grok native inspect did not report the selected project MCP servers")
				}
				return clients.RegistryExpected, nil
			}
			return clients.RegistryCollision, nil
		}
	}
	if requireMCPDiscovery && shared.HasSupportedMCP(plan.Components) {
		return clients.RegistryIndeterminate, fmt.Errorf("grok native inspect did not report the selected project MCP plugin")
	}
	return finding, nil
}
