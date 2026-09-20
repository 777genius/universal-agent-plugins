package scaffold

func filesForPlatform(platform string, extras bool) ([]TemplateFile, bool) {
	switch platform {
	case "claude":
		return claudePlatformFiles(), false
	case "codex-runtime":
		return codexRuntimePlatformFiles(extras), false
	case "codex-package":
		return codexPackagePlatformFiles(extras), true
	case "gemini":
		return geminiPlatformFiles(extras), true
	case "opencode":
		return opencodePlatformFiles(extras), true
	case "cursor":
		return cursorPlatformFiles(extras), true
	case "cursor-workspace":
		return cursorWorkspacePlatformFiles(extras), true
	default:
		return nil, false
	}
}

func claudePlatformFiles() []TemplateFile {
	return []TemplateFile{
		{Path: authoredPath("targets/claude/hooks/hooks.json"), Template: "targets.claude.hooks.json.tmpl", Extra: false},
		{Path: authoredPath("targets/claude/settings.json"), Template: "empty.json.tmpl", Extra: true},
		{Path: authoredPath("targets/claude/lsp.json"), Template: "empty.json.tmpl", Extra: true},
		{Path: authoredPath("targets/claude/user-config.json"), Template: "empty.json.tmpl", Extra: true},
		{Path: authoredPath("targets/claude/manifest.extra.json"), Template: "empty.json.tmpl", Extra: true},
		{Path: authoredPath("README.md"), Template: "README.executable.md.tmpl", Extra: false},
	}
}

func codexRuntimePlatformFiles(extras bool) []TemplateFile {
	files := []TemplateFile{
		{Path: authoredPath("targets/codex-runtime/package.yaml"), Template: "targets.codex-runtime.package.yaml.tmpl", Extra: false},
		{Path: authoredPath("README.md"), Template: "codex-runtime.README.executable.md.tmpl", Extra: false},
	}
	if !extras {
		return files
	}
	return append(files,
		TemplateFile{Path: authoredPath("targets/codex-runtime/config.extra.toml"), Template: "empty.toml.tmpl", Extra: true},
	)
}

func codexPackagePlatformFiles(extras bool) []TemplateFile {
	files := []TemplateFile{
		{Path: authoredPath("README.md"), Template: "codex-package.README.md.tmpl", Extra: false},
	}
	if !extras {
		return files
	}
	return append(files,
		TemplateFile{Path: authoredPath("targets/codex-package/package.yaml"), Template: "targets.codex-package.package.yaml.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("targets/codex-package/interface.json"), Template: "codex-package.interface.json.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("targets/codex-package/manifest.extra.json"), Template: "empty.json.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("targets/codex-package/app.json"), Template: "empty.json.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("mcp/servers.yaml"), Template: "mcp.servers.yaml.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true},
	)
}

func geminiPlatformFiles(extras bool) []TemplateFile {
	files := []TemplateFile{
		{Path: authoredPath("targets/gemini/package.yaml"), Template: "targets.gemini.package.yaml.tmpl", Extra: false},
		{Path: authoredPath("targets/gemini/contexts/GEMINI.md"), Template: "gemini.GEMINI.md.tmpl", Extra: false},
		{Path: authoredPath("README.md"), Template: "gemini.README.md.tmpl", Extra: false},
	}
	if !extras {
		return files
	}
	return append(files,
		TemplateFile{Path: authoredPath("mcp/servers.yaml"), Template: "mcp.servers.yaml.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true},
	)
}

func opencodePlatformFiles(extras bool) []TemplateFile {
	files := []TemplateFile{
		{Path: authoredPath("targets/opencode/package.yaml"), Template: "targets.opencode.package.yaml.tmpl", Extra: false},
		{Path: authoredPath("README.md"), Template: "opencode.README.md.tmpl", Extra: false},
	}
	if !extras {
		return files
	}
	return append(files,
		TemplateFile{Path: authoredPath("mcp/servers.yaml"), Template: "mcp.servers.yaml.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "opencode.SKILL.md.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("targets/opencode/commands/{{.ProjectName}}.md"), Template: "opencode.command.md.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("targets/opencode/agents/{{.ProjectName}}.md"), Template: "opencode.agent.md.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("targets/opencode/themes/{{.ProjectName}}.json"), Template: "opencode.theme.json.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("targets/opencode/tools/{{.ProjectName}}.ts"), Template: "opencode.tool.ts.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("targets/opencode/plugins/example.js"), Template: "opencode.plugin.js.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("targets/opencode/package.json"), Template: "opencode.package.json.tmpl", Extra: true},
	)
}

func cursorPlatformFiles(extras bool) []TemplateFile {
	files := []TemplateFile{
		{Path: authoredPath("README.md"), Template: "cursor.README.md.tmpl", Extra: false},
	}
	if !extras {
		return files
	}
	return append(files,
		TemplateFile{Path: authoredPath("mcp/servers.yaml"), Template: "mcp.servers.yaml.tmpl", Extra: true},
		TemplateFile{Path: authoredPath("skills/{{.ProjectName}}/SKILL.md"), Template: "SKILL.md.tmpl", Extra: true},
	)
}

func cursorWorkspacePlatformFiles(extras bool) []TemplateFile {
	files := []TemplateFile{
		{Path: authoredPath("README.md"), Template: "cursor-workspace.README.md.tmpl", Extra: false},
		{Path: authoredPath("targets/cursor-workspace/rules/project.mdc"), Template: "cursor.rule.mdc.tmpl", Extra: false},
	}
	if !extras {
		return files
	}
	return append(files,
		TemplateFile{Path: authoredPath("mcp/servers.yaml"), Template: "mcp.servers.yaml.tmpl", Extra: true},
	)
}
