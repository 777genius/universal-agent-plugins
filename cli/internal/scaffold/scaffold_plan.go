package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Paths lists relative paths created by Write (for tests and docs).
func Paths(platform, name string, extras bool) []string {
	if platform == "gemini" || platform == "codex-package" || platform == "opencode" || platform == "cursor" || platform == "cursor-workspace" {
		return PathsForRuntime(platform, "", name, extras)
	}
	return PathsForRuntime(platform, RuntimeGo, name, extras)
}

func PathsForRuntime(platform, runtime, name string, extras bool) []string {
	return pathsForRuntime(platform, runtime, name, extras, false, false)
}

func PathsForRuntimeSharedPackage(platform, runtime, name string, extras bool) []string {
	return pathsForRuntime(platform, runtime, name, extras, false, true)
}

func PathsForRuntimeTypeScript(platform, name string, extras bool) []string {
	return pathsForRuntime(platform, RuntimeNode, name, extras, true, false)
}

func PathsForRuntimeTypeScriptSharedPackage(platform, name string, extras bool) []string {
	return pathsForRuntime(platform, RuntimeNode, name, extras, true, true)
}

func BuildPlan(d Data) (ProjectPlan, error) {
	d = applyPlanDefaults(d)
	if err := ValidateProjectName(d.ProjectName); err != nil {
		return ProjectPlan{}, err
	}
	if IsPackageOnlyJobTemplate(d.JobTemplate) {
		return buildJobTemplatePlan(d)
	}
	p, ok := LookupPlatform(d.Platform)
	if !ok {
		return ProjectPlan{}, fmt.Errorf("unknown platform %q", d.Platform)
	}
	d, err := normalizePlanPlatformData(d, p.Name)
	if err != nil {
		return ProjectPlan{}, err
	}
	return ProjectPlan{
		Platform: p.Name,
		Data:     d,
		Files:    expandTemplateFiles(planFilesFor(p.Name, d.Runtime, d.WithExtras, d.TypeScript, d.SharedRuntimePackage), d),
	}, nil
}

func applyPlanDefaults(d Data) Data {
	if strings.TrimSpace(d.ModulePath) == "" {
		d.ModulePath = DefaultModulePath(d.ProjectName)
	}
	if strings.TrimSpace(d.Description) == "" {
		d.Description = "plugin-kit-ai plugin"
	}
	if strings.TrimSpace(d.Version) == "" {
		d.Version = "0.1.0"
	}
	if strings.TrimSpace(d.GoSDKVersion) == "" {
		d.GoSDKVersion = DefaultGoSDKVersion
	}
	if strings.TrimSpace(d.AuthoredRoot) == "" {
		d.AuthoredRoot = "plugin"
	}
	if strings.TrimSpace(d.AuthoredReadmePath) == "" {
		d.AuthoredReadmePath = filepath.ToSlash(filepath.Join(d.AuthoredRoot, "README.md"))
	}
	return d
}
