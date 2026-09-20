package scaffold

func filesForRuntime(platform, runtime string, extras, typescript, sharedRuntimePackage bool) []TemplateFile {
	switch runtime {
	case RuntimePython:
		return pythonRuntimeFiles(platform, extras, sharedRuntimePackage)
	case RuntimeNode:
		return nodeRuntimeFiles(platform, extras, typescript, sharedRuntimePackage)
	case RuntimeShell:
		return shellRuntimeFiles()
	default:
		return nil
	}
}

func pythonRuntimeFiles(platform string, extras, sharedRuntimePackage bool) []TemplateFile {
	files := []TemplateFile{
		{Path: "requirements.txt", Template: "python.requirements.txt.tmpl", Extra: false},
		{Path: authoredPath("main.py"), Template: "python.main.py.tmpl", Extra: false},
		{Path: "bin/{{.ProjectName}}", Template: "python.launcher.sh.tmpl", Extra: false},
		{Path: "bin/{{.ProjectName}}.cmd", Template: "python.launcher.cmd.tmpl", Extra: false},
	}
	if !sharedRuntimePackage {
		files = append(files, TemplateFile{Path: authoredPath("plugin_runtime.py"), Template: "python.plugin_runtime.py.tmpl", Extra: false})
	}
	if includesBundleReleaseWorkflow(platform, extras) {
		files = append(files, TemplateFile{Path: ".github/workflows/bundle-release.yml", Template: "bundle-release.workflow.yml.tmpl", Extra: true})
	}
	return files
}

func nodeRuntimeFiles(platform string, extras, typescript, sharedRuntimePackage bool) []TemplateFile {
	files := nodeEntryFiles(typescript)
	if !sharedRuntimePackage {
		files = append(files, nodeRuntimeHelperFile(typescript))
	}
	if includesBundleReleaseWorkflow(platform, extras) {
		files = append(files, TemplateFile{Path: ".github/workflows/bundle-release.yml", Template: "bundle-release.workflow.yml.tmpl", Extra: true})
	}
	return files
}

func nodeEntryFiles(typescript bool) []TemplateFile {
	if typescript {
		return []TemplateFile{
			{Path: authoredPath("main.ts"), Template: "node.main.ts.tmpl", Extra: false},
			{Path: "tsconfig.json", Template: "node.tsconfig.json.tmpl", Extra: false},
			{Path: "package.json", Template: "node.package.json.tmpl", Extra: false},
			{Path: "bin/{{.ProjectName}}", Template: "node.launcher.sh.tmpl", Extra: false},
			{Path: "bin/{{.ProjectName}}.cmd", Template: "node.launcher.cmd.tmpl", Extra: false},
		}
	}
	return []TemplateFile{
		{Path: authoredPath("main.mjs"), Template: "node.main.mjs.tmpl", Extra: false},
		{Path: "package.json", Template: "node.package.json.tmpl", Extra: false},
		{Path: "bin/{{.ProjectName}}", Template: "node.launcher.sh.tmpl", Extra: false},
		{Path: "bin/{{.ProjectName}}.cmd", Template: "node.launcher.cmd.tmpl", Extra: false},
	}
}

func nodeRuntimeHelperFile(typescript bool) TemplateFile {
	if typescript {
		return TemplateFile{Path: authoredPath("plugin-runtime.ts"), Template: "node.plugin-runtime.ts.tmpl", Extra: false}
	}
	return TemplateFile{Path: authoredPath("plugin-runtime.mjs"), Template: "node.plugin-runtime.mjs.tmpl", Extra: false}
}

func shellRuntimeFiles() []TemplateFile {
	return []TemplateFile{
		{Path: "scripts/main.sh", Template: "shell.main.sh.tmpl", Extra: false},
		{Path: "bin/{{.ProjectName}}", Template: "shell.launcher.sh.tmpl", Extra: false},
		{Path: "bin/{{.ProjectName}}.cmd", Template: "shell.launcher.cmd.tmpl", Extra: false},
	}
}
