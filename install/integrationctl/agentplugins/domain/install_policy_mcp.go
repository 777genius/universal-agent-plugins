package domain

import (
	"net/url"
	"slices"
	"strings"
)

func evaluatePolicyMCP(p InstallPolicyMCP, e PackageEnvelope, add policyDenial) {
	if !p.AllowedTransports.Present && !p.AllowHosts.Present && len(p.DenyHosts) == 0 {
		return
	}
	if len(e.MCP.InvalidServer) > 0 || len(e.Inventory.InvalidMCPServer) > 0 || (e.MCP.Present && !e.MCP.Enabled) {
		add("mcp.definition", "MCP", "MCP definitions cannot be completely evaluated under this policy")
	}
	names := make([]string, 0, len(e.MCP.Servers))
	for name := range e.MCP.Servers {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		server := e.MCP.Servers[name]
		if p.AllowedTransports.Present && !slices.Contains(p.AllowedTransports.Values, server.Type) {
			add("mcp.allowed_transports", "MCP "+name, "MCP transport is not allowed")
		}
		if server.Type != "stdio" && (p.AllowHosts.Present || len(p.DenyHosts) > 0) {
			evaluatePolicyMCPHost(p, name, server, add)
		}
	}
}

func evaluatePolicyMCPHost(p InstallPolicyMCP, name string, server MCPServer, add policyDenial) {
	raw, ok := server.Decoded["url"].(string)
	u, err := url.Parse(raw)
	if !ok || err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || !ValidPolicyHost(strings.ToLower(u.Hostname())) {
		add("mcp.endpoint", "MCP "+name, "remote MCP endpoint has no unambiguous policy hostname")
		return
	}
	host := strings.ToLower(u.Hostname())
	if slices.Contains(p.DenyHosts, host) {
		add("mcp.deny_hosts", "MCP "+name, "remote MCP endpoint uses a denied host")
	}
	if p.AllowHosts.Present && !slices.Contains(p.AllowHosts.Values, host) {
		add("mcp.allow_hosts", "MCP "+name, "remote MCP endpoint does not use an approved host")
	}
}
