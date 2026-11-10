package readiness

import (
	"runtime"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// hostChecks reports inert Skill and MCP declarations. Live host availability,
// credentials, process execution and version checks are deliberately excluded.
func hostChecks(pkg *domain.PackageEnvelope, entries map[string]packageview.Observation, complete bool) []Check {
	checks := []Check{}
	add := func(id, item, status, action string) { checks = append(checks, Check{id, item, status, action}) }
	for name, skill := range pkg.Skills {
		if skill.Compatibility != "" || skill.AllowedTools != "" {
			add("skill_host_requirements", itemID("skill:"+name), "not_evaluated", "Review declared Skill compatibility and tool requirements in the intended host; declarations do not prove availability.")
		}
	}
	names := make([]string, 0, len(pkg.MCP.Servers))
	for name := range pkg.MCP.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := pkg.MCP.Servers[name]
		id := itemID("mcp:" + name)
		if s.Type != "stdio" {
			add("remote_connection", id, "not_evaluated", "Remote endpoint and authentication requirements have not been tested.")
			add("oauth", id, "not_evaluated", "Determine authentication requirements separately; no credential stores were inspected.")
			continue
		}
		command, _ := s.Decoded["command"].(string)
		if strings.ContainsAny(command, `/\`) {
			o, status := resolve(entries, command, complete)
			add("bundled_executable_presence", id, status, "Provide the referenced bundled file inside the package; no command was executed.")
			mode := "not_evaluated"
			if status == "pass" && runtime.GOOS != "windows" {
				mode = "fail"
				if o.Executable {
					mode = "pass"
				}
			}
			add("bundled_executable_mode", id, mode, "Ensure the bundled executable has appropriate executable permissions for the target OS.")
		} else {
			add("path_executable_availability", id, "not_evaluated", "Confirm the declared executable is available in the intended host PATH; this command performs no PATH lookup.")
		}
		add("executable_version", id, "not_evaluated", "Verify the required executable version separately; version probes are not run.")
		add("runtime", id, "not_evaluated", "Static evidence does not prove process startup, MCP handshake or runtime dependencies.")
	}
	return checks
}
