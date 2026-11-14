package platformexec

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type openCodeWorkspaceDirectoryImport struct {
	source  opencodeImportSource
	dstRoot string
	keep    func(string) bool
}

func openCodeThemeDirectoryImport(cfg opencodeScopeConfig) openCodeWorkspaceDirectoryImport {
	return openCodeWorkspaceDirectoryImport{
		source: opencodeImportSource{
			dir:     filepath.Join(cfg.workspaceRoot, "themes"),
			display: filepath.ToSlash(filepath.Join(cfg.workspaceDisplay, "themes")),
		},
		dstRoot: filepath.Join("targets", "opencode", "themes"),
		keep:    func(rel string) bool { return filepath.Ext(rel) == ".json" },
	}
}

func openCodeCommandDirectoryImport(cfg opencodeScopeConfig) openCodeWorkspaceDirectoryImport {
	return openCodeWorkspaceDirectoryImport{
		source: opencodeImportSource{
			dir:     filepath.Join(cfg.workspaceRoot, "commands"),
			display: filepath.ToSlash(filepath.Join(cfg.workspaceDisplay, "commands")),
		},
		dstRoot: filepath.Join("targets", "opencode", "commands"),
		keep:    func(rel string) bool { return filepath.Ext(rel) == ".md" },
	}
}

func openCodeAgentDirectoryImport(cfg opencodeScopeConfig) openCodeWorkspaceDirectoryImport {
	return openCodeWorkspaceDirectoryImport{
		source: opencodeImportSource{
			dir:     filepath.Join(cfg.workspaceRoot, "agents"),
			display: filepath.ToSlash(filepath.Join(cfg.workspaceDisplay, "agents")),
		},
		dstRoot: filepath.Join("targets", "opencode", "agents"),
		keep:    func(rel string) bool { return filepath.Ext(rel) == ".md" },
	}
}

func openCodeSkillDirectoryImport(cfg opencodeScopeConfig) openCodeWorkspaceDirectoryImport {
	return openCodeWorkspaceDirectoryImport{
		source: opencodeImportSource{
			dir:     filepath.Join(cfg.workspaceRoot, "skills"),
			display: filepath.ToSlash(filepath.Join(cfg.workspaceDisplay, "skills")),
		},
		dstRoot: "skills",
		keep:    func(rel string) bool { return strings.HasSuffix(rel, "SKILL.md") },
	}
}

func openCodePluginDirectoryImport(cfg opencodeScopeConfig) openCodeWorkspaceDirectoryImport {
	return openCodeWorkspaceDirectoryImport{
		source: opencodeImportSource{
			dir:     filepath.Join(cfg.workspaceRoot, "plugins"),
			display: filepath.ToSlash(filepath.Join(cfg.workspaceDisplay, "plugins")),
		},
		dstRoot: filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "plugins"),
	}
}
