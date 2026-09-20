package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
)

func loadReadyExportProject(root, platform string, launcher *pluginmanifest.Launcher) (runtimecheck.Project, error) {
	report, err := loadExportServiceValidateReport(root, platform)
	if err != nil {
		return runtimecheck.Project{}, err
	}
	if err := validateExportServiceReport(report, platform); err != nil {
		return runtimecheck.Project{}, err
	}
	project, err := inspectExportProject(root, platform, launcher)
	if err != nil {
		return runtimecheck.Project{}, err
	}
	return requireReadyExportProject(project)
}

func validateExportServiceReadiness(root, platform string) error {
	report, err := loadExportServiceValidateReport(root, platform)
	if err != nil {
		return err
	}
	return validateExportServiceReport(report, platform)
}

func loadExportServiceValidateReport(root, platform string) (validate.Report, error) {
	report, err := validate.Validate(root, platform)
	if err != nil {
		if re, ok := err.(*validate.ReportError); ok {
			return re.Report, nil
		} else {
			return validate.Report{}, err
		}
	}
	return report, nil
}

func validateExportServiceReport(report validate.Report, platform string) error {
	if failures := exportBlockingFailures(report.Failures); len(failures) > 0 {
		return fmt.Errorf("export requires validate --strict to pass for %s", platform)
	}
	if len(report.Warnings) > 0 {
		return fmt.Errorf("export requires validate --strict to pass for %s: %d warning(s) present", platform, len(report.Warnings))
	}
	return nil
}

func inspectReadyExportProject(root, platform string, launcher *pluginmanifest.Launcher) (runtimecheck.Project, error) {
	project, err := inspectExportProject(root, platform, launcher)
	if err != nil {
		return runtimecheck.Project{}, err
	}
	return requireReadyExportProject(project)
}

func inspectExportProject(root, platform string, launcher *pluginmanifest.Launcher) (runtimecheck.Project, error) {
	return runtimecheck.Inspect(runtimecheck.Inputs{
		Root:     root,
		Targets:  []string{platform},
		Launcher: launcher,
	})
}

func requireReadyExportProject(project runtimecheck.Project) (runtimecheck.Project, error) {
	diagnosis := runtimecheck.Diagnose(project)
	if diagnosis.Status != runtimecheck.StatusReady {
		return runtimecheck.Project{}, fmt.Errorf("export requires runtime readiness: %s", diagnosis.Reason)
	}
	return project, nil
}
