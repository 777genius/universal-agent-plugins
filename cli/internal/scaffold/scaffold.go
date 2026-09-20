// Package scaffold writes the package-standard plugin-kit-ai init project tree.
package scaffold

import (
	"embed"
	"regexp"
)

//go:embed templates/*.tmpl
var tmplFS embed.FS

const DefaultCodexModel = "gpt-5.4-mini"

const (
	InitTemplateOnlineService = "online-service"
	InitTemplateLocalTool     = "local-tool"
	InitTemplateCustomLogic   = "custom-logic"
)

// Data is passed to all templates.
type Data struct {
	ModulePath            string
	ProjectName           string
	Description           string
	Version               string
	GoSDKVersion          string
	GoSDKReplacePath      string
	Platform              string
	Runtime               string
	TypeScript            bool
	SharedRuntimePackage  bool
	RuntimePackageVersion string
	ExecutionMode         string
	Entrypoint            string
	CodexModel            string
	ClaudeExtendedHooks   bool
	HasSkills             bool
	HasCommands           bool
	WithExtras            bool
	JobTemplate           string
	Targets               []string
	AuthoredRoot          string
	AuthoredReadmePath    string
}

type TemplateFile struct {
	Path     string
	Template string
	Extra    bool
}

type PlatformDefinition struct {
	Name  string
	Files []TemplateFile
}

type PlannedFile struct {
	RelPath  string
	Template string
}

type ProjectPlan struct {
	Platform string
	Files    []PlannedFile
	Data     Data
}

var nameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)
