package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func inspectAuthoringPath(report pluginmanifest.Inspection) string {
	if report.Launcher != nil {
		return "build custom plugin logic"
	}
	if report.Portable.MCP != nil && report.Portable.MCP.File != nil {
		hasRemote, hasLocal := inspectAuthoringMCPTransportKinds(report)
		switch {
		case hasRemote && !hasLocal:
			return "connect an online service"
		case hasLocal && !hasRemote:
			return "connect a local tool"
		case hasLocal && hasRemote:
			return "connect online services and local tools"
		}
	}
	return "manage generated plugin outputs from one authored source"
}

func inspectAuthoringMCPTransportKinds(report pluginmanifest.Inspection) (hasRemote bool, hasLocal bool) {
	for _, server := range report.Portable.MCP.File.Servers {
		switch strings.TrimSpace(server.Type) {
		case "remote":
			hasRemote = true
		case "stdio":
			hasLocal = true
		}
	}
	return hasRemote, hasLocal
}

func inspectAuthoringNextCommands(report pluginmanifest.Inspection) []string {
	commands := []string{"plugin-kit-ai generate ."}
	if report.Launcher == nil {
		commands = append(commands, "plugin-kit-ai generate --check .")
	}
	commands = append(commands, fmt.Sprintf("plugin-kit-ai validate . --platform %s --strict", inspectAuthoringPrimaryTarget(report)))
	if len(report.Manifest.Targets) > 1 {
		commands = append(commands, "Then validate any other outputs you plan to ship.")
	}
	return commands
}

func inspectAuthoringPrimaryTarget(report pluginmanifest.Inspection) string {
	if len(report.Manifest.Targets) == 0 {
		return "claude"
	}
	return report.Manifest.Targets[0]
}

func inspectAuthoringTargetLabel(target string) string {
	switch strings.TrimSpace(target) {
	case "claude":
		return "Claude (claude)"
	case "codex-package":
		return "Codex package (codex-package)"
	case "codex-runtime":
		return "Codex runtime (codex-runtime)"
	case "gemini":
		return "Gemini extension (gemini)"
	case "opencode":
		return "OpenCode (opencode)"
	case "cursor":
		return "Cursor plugin (cursor)"
	case "cursor-workspace":
		return "Cursor workspace (cursor-workspace)"
	default:
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			return target
		}
		return strings.ToUpper(trimmed[:1]) + trimmed[1:] + " (" + trimmed + ")"
	}
}
