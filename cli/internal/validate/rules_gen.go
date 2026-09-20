package validate

import "strings"

var generatedRules = map[string]Rule{
	"claude": {
		Platform: "claude",
		RequiredFiles: []string{
			"README.md",
			".claude-plugin/plugin.json",
		},
		ForbiddenFiles: []string{
			".codex/config.toml",
		},
		BuildTargets: []string{},
	},
	"codex-package": {
		Platform: "codex-package",
		RequiredFiles: []string{
			"README.md",
			".codex-plugin/plugin.json",
		},
		ForbiddenFiles: []string{
			"launcher.yaml",
			".codex/config.toml",
		},
		BuildTargets: []string{},
	},
	"codex-runtime": {
		Platform: "codex-runtime",
		RequiredFiles: []string{
			"go.mod",
			"README.md",
			"launcher.yaml",
			".codex/config.toml",
		},
		ForbiddenFiles: []string{
			".claude-plugin/plugin.json",
			"hooks/hooks.json",
		},
		BuildTargets: []string{
			"./...",
		},
	},
	"cursor": {
		Platform: "cursor",
		RequiredFiles: []string{
			"README.md",
			".cursor-plugin/plugin.json",
		},
		ForbiddenFiles: []string{
			"launcher.yaml",
		},
		BuildTargets: []string{},
	},
	"cursor-workspace": {
		Platform: "cursor-workspace",
		RequiredFiles: []string{
			"README.md",
		},
		ForbiddenFiles: []string{
			"launcher.yaml",
		},
		BuildTargets: []string{},
	},
	"gemini": {
		Platform: "gemini",
		RequiredFiles: []string{
			"README.md",
			"gemini-extension.json",
		},
		ForbiddenFiles: []string{},
		BuildTargets:   []string{},
	},
	"opencode": {
		Platform: "opencode",
		RequiredFiles: []string{
			"README.md",
			"opencode.json",
		},
		ForbiddenFiles: []string{
			"launcher.yaml",
		},
		BuildTargets: []string{},
	},
}

func LookupRule(name string) (Rule, bool) {
	r, ok := generatedRules[normalizePlatform(name)]
	return r, ok
}

func normalizePlatform(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "codex-runtime"
	}
	return name
}
