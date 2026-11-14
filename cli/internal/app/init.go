package app

import (
	"regexp"
)

// InitOptions is parsed CLI state for plugin-kit-ai init.
type InitOptions struct {
	ProjectName           string
	Template              string
	Platform              string
	PlatformExplicit      bool
	Runtime               string
	RuntimeExplicit       bool
	TypeScript            bool
	RuntimePackage        bool
	RuntimePackageVersion string
	OutputDir             string // empty → ./<project-name> under cwd
	Force                 bool
	Extras                bool
	ClaudeExtendedHooks   bool
}

// InitRunner runs plugin-kit-ai init.
type InitRunner struct{}

var stableRuntimePackageVersionRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)
