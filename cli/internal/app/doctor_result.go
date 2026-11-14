package app

import "github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"

func buildDoctorResult(root string, project runtimecheck.Project) PluginDoctorResult {
	diagnosis := runtimecheck.Diagnose(project)
	return PluginDoctorResult{
		Ready: diagnosis.Status == runtimecheck.StatusReady,
		Lines: buildDoctorLines(root, project, diagnosis),
	}
}
