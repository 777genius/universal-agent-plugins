package scaffold

import (
	"fmt"
	"strings"
)

// ValidateProjectName returns an error if name is not a safe directory/binary segment.
func ValidateProjectName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name is empty")
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid project name %q: use letters, digits, underscore, hyphen; start with a letter; max 64 characters", name)
	}
	return nil
}

// DefaultModulePath returns example.com/<name> for generated go.mod.
func DefaultModulePath(name string) string {
	return "example.com/" + name
}

func NormalizeTemplate(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func IsKnownTemplate(name string) bool {
	switch NormalizeTemplate(name) {
	case "", InitTemplateOnlineService, InitTemplateLocalTool, InitTemplateCustomLogic:
		return true
	default:
		return false
	}
}

func DefaultJobTemplateTargets(template string) []string {
	switch NormalizeTemplate(template) {
	case InitTemplateOnlineService, InitTemplateLocalTool:
		return []string{"claude", "codex-package", "opencode", "cursor"}
	default:
		return nil
	}
}

func IsPackageOnlyJobTemplate(template string) bool {
	switch NormalizeTemplate(template) {
	case InitTemplateOnlineService, InitTemplateLocalTool:
		return true
	default:
		return false
	}
}

func (d Data) EffectiveTargets() []string {
	if len(d.Targets) > 0 {
		return append([]string(nil), d.Targets...)
	}
	if strings.TrimSpace(d.Platform) == "" {
		return nil
	}
	return []string{d.Platform}
}

func (d Data) PrimaryTarget() string {
	targets := d.EffectiveTargets()
	if len(targets) == 1 {
		return targets[0]
	}
	return ""
}

func (d Data) IsOnlineServiceTemplate() bool {
	return NormalizeTemplate(d.JobTemplate) == InitTemplateOnlineService
}

func (d Data) IsLocalToolTemplate() bool {
	return NormalizeTemplate(d.JobTemplate) == InitTemplateLocalTool
}

func (d Data) IsCustomLogicTemplate() bool {
	return NormalizeTemplate(d.JobTemplate) == InitTemplateCustomLogic
}

func normalizePackageVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	return version
}
