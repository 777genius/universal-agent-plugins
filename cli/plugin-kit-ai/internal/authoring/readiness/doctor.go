package readiness

import (
	"crypto/sha256"
	"fmt"
	"runtime"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
)

// Check is an allowlisted static observation. Pass applies only to the named
// metadata check, never execution, dependency validity, or toolchain readiness.
type Check struct {
	ID     string `json:"id"`
	ItemID string `json:"item_id,omitempty"`
	Status string `json:"status"`
	Action string `json:"action"`
}

func itemID(value string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(value))) }

func Doctor(p project.Result) []Check {
	checks := []Check{}
	add := func(id, item, status, action string) { checks = append(checks, Check{id, item, status, action}) }
	if p.Facts.Package == nil {
		return checks
	}
	entries := map[string]packageview.Observation{}
	for _, o := range p.Input.Inventory {
		entries[o.Path] = o
	}
	// The reader captures these files into the scope digest, but exports metadata
	// rather than their bytes. Do not reopen a mutable source or parse scripts.
	for _, lane := range []struct {
		file, language string
		locks          []string
	}{
		{"package.json", "node", []string{"package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"}},
		{"pyproject.toml", "python", []string{"uv.lock", "poetry.lock", "Pipfile.lock"}},
		{"go.mod", "go", []string{"go.sum"}},
	} {
		o, exists := entries[lane.file]
		if !exists {
			continue
		}
		status := "not_evaluated"
		if o.Captured && o.State == packageview.Present && o.Kind == "file" {
			status = "pass"
		}
		add(lane.language+"_project_file", "", status, "Inspect the language-native project file; contents and build scripts are not interpreted.")
		lockStatus := "not_evaluated"
		count := 0
		for _, file := range lane.locks {
			if o, ok := entries[file]; ok && o.Captured && o.State == packageview.Present && o.Kind == "file" {
				count++
			}
		}
		if count == 1 {
			lockStatus = "pass"
		}
		action := "Review native dependency inputs and supply the selected manager's lock/checksum file where required; no dependencies were installed."
		if count > 1 {
			action = "Multiple manager lockfiles are present; explicitly select and reconcile dependency inputs."
		}
		add(lane.language+"_lockfile_presence", "", lockStatus, action)
		add(lane.language+"_dependency_consistency", "", "not_evaluated", "Verify dependency versions, lockfile consistency and build outputs with the selected native toolchain separately.")
	}
	for name, skill := range p.Facts.Package.Skills {
		if skill.Compatibility != "" || skill.AllowedTools != "" {
			add("skill_host_requirements", itemID("skill:"+name), "not_evaluated", "Review declared Skill compatibility and tool requirements in the intended host; declarations do not prove availability.")
		}
	}
	names := make([]string, 0, len(p.Facts.Package.MCP.Servers))
	for name := range p.Facts.Package.MCP.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := p.Facts.Package.MCP.Servers[name]
		id := itemID("mcp:" + name)
		if s.Type != "stdio" {
			add("remote_connection", id, "not_evaluated", "Remote endpoint and authentication requirements have not been tested.")
			add("oauth", id, "not_evaluated", "Determine authentication requirements separately; no credential stores were inspected.")
			continue
		}
		command, _ := s.Decoded["command"].(string)
		if strings.ContainsAny(command, `/\`) {
			o, status := resolve(entries, command, p.Input.Coverage.InventoryComplete)
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
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].ID != checks[j].ID {
			return checks[i].ID < checks[j].ID
		}
		return checks[i].ItemID < checks[j].ItemID
	})
	return checks
}

// resolve traverses retained observations, including contained links, without
// lexical cleaning away intermediates or opening any source/host path.
func resolve(entries map[string]packageview.Observation, value string, complete bool) (packageview.Observation, string) {
	unknown := func() (packageview.Observation, string) { return packageview.Observation{}, "not_evaluated" }
	if strings.HasPrefix(value, "${PLUGIN_DATA}") || strings.Contains(value, `\`) {
		return unknown()
	}
	value = strings.TrimPrefix(value, "${PLUGIN_ROOT}/")
	if strings.HasPrefix(value, "/") || strings.Contains(value, "${") {
		return unknown()
	}
	todo := strings.Split(value, "/")
	var stack []string
	var last packageview.Observation
	links := 0
	for len(todo) > 0 {
		part := todo[0]
		todo = todo[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(stack) == 0 {
				return packageview.Observation{}, "fail"
			}
			stack = stack[:len(stack)-1]
			continue
		}
		key := strings.Join(append(append([]string{}, stack...), part), "/")
		o, ok := entries[key]
		if !ok {
			if complete {
				return o, "fail"
			}
			return unknown()
		}
		if !o.Captured || o.State != packageview.Present {
			return unknown()
		}
		if o.Kind == "symlink" {
			links++
			if links > 40 || strings.HasPrefix(o.Target, "/") || strings.Contains(o.Target, `\`) {
				return unknown()
			}
			todo = append(strings.Split(o.Target, "/"), todo...)
			continue
		}
		if len(todo) > 0 && o.Kind != "directory" {
			return o, "fail"
		}
		stack = append(stack, part)
		last = o
	}
	if last.Kind != "file" {
		return last, "fail"
	}
	return last, "pass"
}
