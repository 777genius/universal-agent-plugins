package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func formatInitSuccess(outDir string, opts app.InitOptions) string {
	templateName := strings.TrimSpace(opts.Template)
	platform := strings.TrimSpace(opts.Platform)
	if platform == "" {
		platform = "codex-runtime"
	}
	runtime := strings.TrimSpace(opts.Runtime)
	if runtime == "" {
		runtime = "go"
	}

	lines := []string{
		fmt.Sprintf("Created plugin %q at %s", opts.ProjectName, outDir),
		"Next:",
		fmt.Sprintf("  cd %s", strconv.Quote(outDir)),
	}

	if templateName == scaffold.InitTemplateOnlineService || templateName == scaffold.InitTemplateLocalTool {
		lines = append(lines,
			"  plugin-kit-ai inspect . --authoring",
			"  plugin-kit-ai generate .",
			"  plugin-kit-ai generate --check .",
		)
		if opts.PlatformExplicit && strings.TrimSpace(opts.Platform) != "" {
			lines = append(lines, fmt.Sprintf("  plugin-kit-ai validate . --platform %s --strict", strings.TrimSpace(opts.Platform)))
		} else {
			lines = append(lines, "  plugin-kit-ai validate . --platform claude --strict")
		}
		lines = append(lines, "  See plugin/README.md for the first run")
		return strings.Join(lines, "\n") + "\n"
	}

	if templateName == scaffold.InitTemplateCustomLogic {
		lines = append(lines,
			"  plugin-kit-ai inspect . --authoring",
		)
	}

	if platform == "gemini" && strings.TrimSpace(opts.Runtime) == "go" {
		if opts.Extras {
			lines = append(lines, "  Portable MCP starter: plugin/mcp/servers.yaml")
		}
		lines = append(lines,
			"  go mod tidy",
			"  go test ./...",
			"  plugin-kit-ai generate .",
			"  plugin-kit-ai generate --check .",
			fmt.Sprintf("  plugin-kit-ai validate . --platform %s --strict", platform),
			"  plugin-kit-ai inspect . --target gemini",
			"  plugin-kit-ai capabilities --mode runtime --platform gemini",
			"  make test-gemini-runtime",
			"  gemini extensions link .",
			"  make test-gemini-runtime-live",
			"  See README.md for Gemini runtime steps",
		)
		return strings.Join(lines, "\n") + "\n"
	}

	if platform == "gemini" || platform == "codex-package" || platform == "opencode" || platform == "cursor" || platform == "cursor-workspace" {
		if opts.Extras {
			lines = append(lines, "  Portable MCP starter: plugin/mcp/servers.yaml")
		}
		lines = append(lines,
			"  plugin-kit-ai generate .",
			"  plugin-kit-ai generate --check .",
			fmt.Sprintf("  plugin-kit-ai validate . --platform %s --strict", platform),
			"  See plugin/README.md for the full first run",
		)
		return strings.Join(lines, "\n") + "\n"
	}

	switch runtime {
	case "python":
		if opts.RuntimePackage && strings.TrimSpace(opts.RuntimePackageVersion) != "" {
			lines = append(lines, fmt.Sprintf("  Shared helper dependency: plugin-kit-ai-runtime@%s", opts.RuntimePackageVersion))
		}
		lines = append(lines,
			"  plugin-kit-ai doctor .",
			"  plugin-kit-ai bootstrap .",
			fmt.Sprintf("  plugin-kit-ai validate . --platform %s --strict", platform),
			fmt.Sprintf("  %s", initTestCommand(platform)),
			fmt.Sprintf("  %s", initDevCommand(platform)),
			"  See plugin/README.md for the full first run",
		)
	case "node":
		if opts.RuntimePackage && strings.TrimSpace(opts.RuntimePackageVersion) != "" {
			lines = append(lines, fmt.Sprintf("  Shared helper dependency: plugin-kit-ai-runtime@%s", opts.RuntimePackageVersion))
		}
		lines = append(lines,
			"  plugin-kit-ai doctor .",
			"  plugin-kit-ai bootstrap .",
			fmt.Sprintf("  plugin-kit-ai validate . --platform %s --strict", platform),
			fmt.Sprintf("  %s", initTestCommand(platform)),
			fmt.Sprintf("  %s", initDevCommand(platform)),
			"  See plugin/README.md for the full first run",
		)
	case "shell":
		lines = append(lines,
			"  plugin-kit-ai doctor .",
			"  plugin-kit-ai bootstrap .",
			fmt.Sprintf("  plugin-kit-ai validate . --platform %s --strict", platform),
			fmt.Sprintf("  %s", initTestCommand(platform)),
			fmt.Sprintf("  %s", initDevCommand(platform)),
			"  See plugin/README.md for the full first run",
		)
	default:
		lines = append(lines,
			"  go mod tidy",
			fmt.Sprintf("  go build -o bin/%s ./cmd/%s", opts.ProjectName, opts.ProjectName),
			fmt.Sprintf("  plugin-kit-ai validate . --platform %s --strict", platform),
			fmt.Sprintf("  %s", initTestCommand(platform)),
			fmt.Sprintf("  %s", initDevCommand(platform)),
			"  See plugin/README.md for SDK setup and first-run steps",
		)
	}

	if templateName == scaffold.InitTemplateCustomLogic {
		lines = append(lines, "  Advanced path: start with plugin/README.md, then grow into deeper runtime and hook details only when you need them.")
	} else if templateName == "" {
		lines = append(lines, "  Legacy compatibility path. For a new online service or local tool repo, start with --template online-service or --template local-tool instead.")
	}

	return strings.Join(lines, "\n") + "\n"
}

func initTestCommand(platform string) string {
	switch strings.TrimSpace(platform) {
	case "claude":
		return "plugin-kit-ai test . --platform claude --all"
	case "codex-runtime":
		return "plugin-kit-ai test . --platform codex-runtime --event Notify"
	default:
		return "plugin-kit-ai test ."
	}
}

func initDevCommand(platform string) string {
	switch strings.TrimSpace(platform) {
	case "claude":
		return "plugin-kit-ai dev . --platform claude --event Stop"
	case "codex-runtime":
		return "plugin-kit-ai dev . --platform codex-runtime --event Notify"
	default:
		return "plugin-kit-ai dev ."
	}
}
