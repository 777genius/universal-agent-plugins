package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/spf13/cobra"
)

type publicationDoctorFlags struct {
	target      string
	format      string
	dest        string
	packageRoot string
}

type publicationDoctorInputData struct {
	root        string
	target      string
	format      string
	dest        string
	packageRoot string
}

type publicationDoctorInspectionResult struct {
	report    pluginmanifest.Inspection
	warnings  []pluginmanifest.Warning
	diagnosis publicationDiagnosis
	localRoot *app.PluginPublicationVerifyRootResult
}

func publicationDoctorInput(flags publicationDoctorFlags, args []string) publicationDoctorInputData {
	return publicationDoctorInputData{
		root:        publicationDoctorRoot(args),
		target:      flags.target,
		format:      flags.format,
		dest:        flags.dest,
		packageRoot: flags.packageRoot,
	}
}

func publicationDoctorRoot(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}

func runPublicationDoctor(cmd *cobra.Command, runner inspectRunner, in publicationDoctorInputData) error {
	inspected, err := inspectPublicationDoctor(runner, in)
	if err != nil {
		return err
	}
	return renderPublicationDoctor(cmd, newPublicationDoctorRenderInput(in, inspected))
}

func newPublicationDoctorRenderInput(in publicationDoctorInputData, inspected publicationDoctorInspectionResult) publicationDoctorRenderInput {
	return publicationDoctorRenderInput{
		format:    in.format,
		target:    in.target,
		report:    inspected.report,
		warnings:  inspected.warnings,
		diagnosis: inspected.diagnosis,
		localRoot: inspected.localRoot,
	}
}

func inspectPublicationDoctor(runner inspectRunner, in publicationDoctorInputData) (publicationDoctorInspectionResult, error) {
	report, warnings, err := inspectPublicationDoctorReport(runner, in)
	if err != nil {
		return publicationDoctorInspectionResult{}, err
	}
	diagnosis, localRoot, err := inspectPublicationDoctorDiagnosis(runner, in, report)
	if err != nil {
		return publicationDoctorInspectionResult{}, err
	}
	return publicationDoctorInspectionResult{
		report:    report,
		warnings:  warnings,
		diagnosis: diagnosis,
		localRoot: localRoot,
	}, nil
}

func inspectPublicationDoctorReport(runner inspectRunner, in publicationDoctorInputData) (pluginmanifest.Inspection, []pluginmanifest.Warning, error) {
	report, warnings, err := runner.Inspect(app.PluginInspectOptions{
		Root:   in.root,
		Target: in.target,
	})
	if err != nil {
		return pluginmanifest.Inspection{}, nil, err
	}
	return report, warnings, nil
}

func inspectPublicationDoctorDiagnosis(runner inspectRunner, in publicationDoctorInputData, report pluginmanifest.Inspection) (publicationDiagnosis, *app.PluginPublicationVerifyRootResult, error) {
	diagnosis := diagnosePublication(in.root, in.target, report)
	localRoot, err := maybeVerifyPublicationLocalRoot(runner, in.root, in.target, in.dest, in.packageRoot, diagnosis.Status)
	if err != nil {
		return publicationDiagnosis{}, nil, err
	}
	diagnosis = mergePublicationDiagnosisWithLocalRoot(diagnosis, in.target, localRoot)
	return diagnosis, localRoot, nil
}
