package app

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

type PluginDoctorOptions struct {
	Root string
}

type PluginDoctorResult struct {
	Ready bool
	Lines []string
}

func (PluginService) Doctor(opts PluginDoctorOptions) (PluginDoctorResult, error) {
	root := normalizeDoctorRoot(opts.Root)
	project, err := loadDoctorProject(root)
	if err != nil {
		return PluginDoctorResult{}, err
	}
	return buildDoctorResult(root, project), nil
}

func normalizeDoctorRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return "."
	}
	return root
}

func buildDoctorLines(root string, project runtimecheck.Project, diagnosis runtimecheck.Diagnosis) []string {
	lines := []string{
		project.ProjectLine(),
		fmt.Sprintf("Status: %s (%s)", diagnosis.Status, diagnosis.Reason),
	}
	if requirement := exportRuntimeRequirement(project.Runtime); strings.TrimSpace(requirement) != "" {
		lines = append(lines, "Runtime requirement: "+requirement)
	}
	if hint := exportRuntimeInstallHint(project.Runtime); strings.TrimSpace(hint) != "" {
		lines = append(lines, "Runtime install hint: "+hint)
	}
	lines = append(lines, doctorEnvironmentLines(root, project)...)
	if len(diagnosis.Next) > 0 {
		lines = append(lines, "Next:")
		for _, step := range diagnosis.Next {
			lines = append(lines, "  "+step)
		}
	}
	return lines
}
