package report

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// components projects inert package evidence without probing the host.
func (r *Report) components(p project.Result) {
	pkg := p.Facts.Package
	r.mcpComponents(pkg, p.Facts.Findings)
	r.skillComponents(pkg)
	for name := range pkg.Manifest.Extensions {
		r.Components = append(r.Components, Component{opaque("extension:" + name), "opaque_extension", NotEvaluated, []string{}})
	}
}

// mcpComponents preserves status precedence and redacted runtime requirements.
func (r *Report) mcpComponents(pkg *domain.PackageEnvelope, findings []conformance.Finding) {
	for name, s := range pkg.MCP.Servers {
		r.Components = append(r.Components, Component{opaque("mcp:" + name), "mcp_" + s.Type, mcpStatus(name, findings), mcpRequirements(s)})
	}
	for name := range pkg.MCP.InvalidServer {
		r.Components = append(r.Components, Component{opaque("mcp:" + name), "mcp_server", Fail, []string{}})
	}
}

// mcpRequirements reports declaration metadata without revealing commands, env or header values.
func mcpRequirements(s domain.MCPServer) []string {
	req := []string{}
	if s.Type == "stdio" {
		req = append(req, "executable_unresolved")
		if s.StdioRequirement != nil {
			req = append(req, "executable_"+string(s.StdioRequirement.Kind))
			if s.StdioRequirement.UsesPluginRoot {
				req = append(req, "plugin_root")
			}
			if s.StdioRequirement.UsesPluginData {
				req = append(req, "plugin_data")
			}
		}
		if env, ok := s.Decoded["env"].(map[string]any); ok && len(env) > 0 {
			req = append(req, "environment_values_redacted")
		}
	} else {
		req = append(req, "remote_endpoint_uncontacted")
		if h, ok := s.Decoded["headers"].(map[string]any); ok && len(h) > 0 {
			req = append(req, "header_values_redacted")
		}
	}
	return req
}

// mcpStatus retains normative failure precedence over incomplete host evidence.
func mcpStatus(name string, findings []conformance.Finding) State {
	status := Pass
	for _, finding := range findings {
		if finding.Code == "diagnostics_truncated" {
			status = combine(status, NotEvaluated)
		}
		if finding.Boundary != domain.BoundaryMCPServer || finding.Item != opaque(name) {
			continue
		}
		if finding.Layer == conformance.Normative {
			status = Fail
		} else {
			status = combine(status, NotEvaluated)
		}
	}
	return status
}

// skillComponents keeps valid and invalid Skills as distinct evidence entries.
func (r *Report) skillComponents(pkg *domain.PackageEnvelope) {
	for name, s := range pkg.Skills {
		req := []string{}
		if s.Compatibility != "" {
			req = append(req, "declared_compatibility_unresolved")
		}
		if s.AllowedTools != "" {
			req = append(req, "declared_tools_unresolved")
		}
		r.Components = append(r.Components, Component{opaque("skill:" + name), "skill", Pass, req})
	}
	for _, name := range pkg.Inventory.InvalidSkills {
		r.Components = append(r.Components, Component{opaque("skill:" + name), "skill", Fail, []string{}})
	}
}
