package readiness

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"

// languageChecks reports only captured project and lockfile metadata. It neither
// opens files nor infers dependency consistency from their mere presence.
func languageChecks(entries map[string]packageview.Observation) []Check {
	checks := []Check{}
	add := func(id, item, status, action string) { checks = append(checks, Check{id, item, status, action}) }
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
	return checks
}
