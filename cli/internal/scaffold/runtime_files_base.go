package scaffold

func filesForGeminiGo(extras bool) []TemplateFile {
	files := []TemplateFile{
		{Path: "go.mod", Template: "go.mod.tmpl", Extra: false},
		{Path: "cmd/{{.ProjectName}}/main.go", Template: "gemini.main.go.tmpl", Extra: false},
		{Path: authoredPath("plugin.yaml"), Template: "plugin.yaml.tmpl", Extra: false},
		{Path: authoredPath("launcher.yaml"), Template: "launcher.yaml.tmpl", Extra: false},
		{Path: authoredPath("targets/gemini/package.yaml"), Template: "targets.gemini.package.yaml.tmpl", Extra: false},
		{Path: authoredPath("targets/gemini/contexts/GEMINI.md"), Template: "gemini.GEMINI.md.tmpl", Extra: false},
		{Path: authoredPath("targets/gemini/hooks/hooks.json"), Template: "targets.gemini.hooks.json.tmpl", Extra: false},
		{Path: authoredPath("README.md"), Template: "gemini.README.go.md.tmpl", Extra: false},
		{Path: "CLAUDE.md", Template: "ROOT.CLAUDE.md.tmpl", Extra: false},
		{Path: "AGENTS.md", Template: "ROOT.AGENTS.md.tmpl", Extra: false},
	}
	if !extras {
		return files
	}
	return append(files,
		TemplateFile{Path: authoredPath("mcp/servers.yaml"), Template: "mcp.servers.yaml.tmpl", Extra: true},
		TemplateFile{Path: "Makefile", Template: "Makefile.tmpl", Extra: true},
		TemplateFile{Path: ".goreleaser.yml", Template: "goreleaser.yml.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true},
	)
}

func baseRuntimeFiles(platform string) []TemplateFile {
	files := []TemplateFile{
		{Path: authoredPath("plugin.yaml"), Template: "plugin.yaml.tmpl", Extra: false},
		{Path: "CLAUDE.md", Template: "ROOT.CLAUDE.md.tmpl", Extra: false},
		{Path: "AGENTS.md", Template: "ROOT.AGENTS.md.tmpl", Extra: false},
	}
	if platformRequiresLauncher(platform) {
		files = append(files, TemplateFile{Path: authoredPath("launcher.yaml"), Template: "launcher.yaml.tmpl", Extra: false})
	}
	return files
}

func platformRequiresLauncher(platform string) bool {
	switch platform {
	case "gemini", "codex-package", "opencode", "cursor", "cursor-workspace":
		return false
	default:
		return true
	}
}

func includesSharedExtras(platform string, extras bool) bool {
	return extras && platform != "codex-runtime"
}

func includesBundleReleaseWorkflow(platform string, extras bool) bool {
	return extras && (platform == "claude" || platform == "codex-runtime")
}

func sharedExtraFiles(platform string, extras bool) []TemplateFile {
	if !includesSharedExtras(platform, extras) {
		return nil
	}
	return []TemplateFile{
		{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true},
	}
}
