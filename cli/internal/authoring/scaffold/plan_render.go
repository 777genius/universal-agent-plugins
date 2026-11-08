package scaffold

import (
	"encoding/json"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// renderPlan orders and validates the complete, in-memory tree before exposure.
func renderPlan(o Options, manifest map[string]any, registry *specregistry.Registry, lane Template, hasSkill bool, license string) (Plan, error) {
	p := Plan{}
	add := func(path string, b []byte) { p.files = append(p.files, File{path, b, 0644}) }
	if err := registry.Validate(domain.PluginSchemaV1, manifest); err != nil {
		return Plan{}, err
	}
	appendPlanCore(o, manifest, lane, hasSkill, license, add)
	if lane == MCPRemote || lane == MCPStdio {
		mcp, err := planMCP(o, lane, registry)
		if err != nil {
			return Plan{}, err
		}
		add("mcp.json", mcp)
	}
	if lane == MCPStdio {
		pkg, lock, err := nodePackages(o.Name)
		if err != nil {
			return Plan{}, err
		}
		add("package.json", pkg)
		add("package-lock.json", lock)
		add("src/server.mjs", nodeServerSource(o.Name))
	}
	sort.Slice(p.files, func(i, j int) bool { return p.files[i].Path < p.files[j].Path })
	if err := validateFiles(p.files); err != nil {
		return Plan{}, err
	}
	return p, nil

}

func appendPlanCore(o Options, manifest map[string]any, lane Template, hasSkill bool, license string, add func(string, []byte)) {
	add("plugin.json", jsonBytes(manifest))
	add(".gitignore", []byte("node_modules/\n.DS_Store\n"))
	readme := "# " + o.Name + "\n\n" + o.Description + "\n\nThis package uses Agent Plugins 1.0: `plugin.json`, with portable components in `skills/` and/or `mcp.json`.\n"
	if lane == MCPStdio {
		readme += "\nThe stdio server requires Node >=22 and the official MCP SDK pinned in package-lock.json. Dependency installation and runtime execution are separate, explicit author actions. Creation performs neither; runtime behavior has not been tested.\n"
	}
	if lane == MCPRemote {
		readme += "\nThe remote MCP URL is configuration only. Creation does not contact the endpoint or verify authentication or runtime behavior.\n"
	}
	add("README.md", []byte(readme))
	if license != "" {
		add("LICENSE", []byte(license))
	}
	if hasSkill {
		desc, _ := json.Marshal(o.Description)
		add("skills/"+o.SkillName+"/SKILL.md", []byte("---\nname: "+o.SkillName+"\ndescription: "+string(desc)+"\n---\n\n# "+o.SkillName+"\n\n"+o.Description+"\n\nUse this skill when the request matches its description. Clarify missing requirements before taking action and report the result.\n"))
	}
}

func planMCP(o Options, lane Template, registry *specregistry.Registry) ([]byte, error) {
	server := map[string]any{"type": "streamable-http", "url": o.RemoteURL}
	if lane == MCPStdio {
		server = map[string]any{"type": "stdio", "command": "node", "args": []any{"${PLUGIN_ROOT}/src/server.mjs"}}
	}
	mcp := map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{o.Name: server}}
	if err := registry.Validate(domain.MCPSchemaV1, mcp); err != nil {
		return nil, err
	}
	return jsonBytes(mcp), nil
}
