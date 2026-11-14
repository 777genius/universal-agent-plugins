package scaffold

func runtimeTestScaffoldFiles(platform string) []TemplateFile {
	switch normalizePlatform(platform) {
	case "claude":
		return []TemplateFile{
			{Path: "fixtures/claude/Stop.json", Template: "fixtures.claude.Stop.json.tmpl", Extra: false},
			{Path: "fixtures/claude/PreToolUse.json", Template: "fixtures.claude.PreToolUse.json.tmpl", Extra: false},
			{Path: "fixtures/claude/UserPromptSubmit.json", Template: "fixtures.claude.UserPromptSubmit.json.tmpl", Extra: false},
			{Path: "goldens/claude/Stop.stdout", Template: "goldens.claude.stdout.tmpl", Extra: false},
			{Path: "goldens/claude/Stop.stderr", Template: "goldens.empty.tmpl", Extra: false},
			{Path: "goldens/claude/Stop.exitcode", Template: "goldens.exitcode.tmpl", Extra: false},
			{Path: "goldens/claude/PreToolUse.stdout", Template: "goldens.claude.stdout.tmpl", Extra: false},
			{Path: "goldens/claude/PreToolUse.stderr", Template: "goldens.empty.tmpl", Extra: false},
			{Path: "goldens/claude/PreToolUse.exitcode", Template: "goldens.exitcode.tmpl", Extra: false},
			{Path: "goldens/claude/UserPromptSubmit.stdout", Template: "goldens.claude.stdout.tmpl", Extra: false},
			{Path: "goldens/claude/UserPromptSubmit.stderr", Template: "goldens.empty.tmpl", Extra: false},
			{Path: "goldens/claude/UserPromptSubmit.exitcode", Template: "goldens.exitcode.tmpl", Extra: false},
		}
	case "codex-runtime":
		return []TemplateFile{
			{Path: "fixtures/codex-runtime/Notify.json", Template: "fixtures.codex-runtime.Notify.json.tmpl", Extra: false},
			{Path: "goldens/codex-runtime/Notify.stdout", Template: "goldens.empty.tmpl", Extra: false},
			{Path: "goldens/codex-runtime/Notify.stderr", Template: "goldens.empty.tmpl", Extra: false},
			{Path: "goldens/codex-runtime/Notify.exitcode", Template: "goldens.exitcode.tmpl", Extra: false},
		}
	default:
		return nil
	}
}
