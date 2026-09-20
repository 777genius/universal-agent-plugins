package validate

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type FailureKind string

const (
	FailureUnknownPlatform          FailureKind = "unknown_platform"
	FailureCannotInferPlatform      FailureKind = "cannot_infer_platform"
	FailureManifestMissing          FailureKind = "manifest_missing"
	FailureManifestInvalid          FailureKind = "manifest_invalid"
	FailureRequiredFileMissing      FailureKind = "required_file_missing"
	FailureForbiddenFilePresent     FailureKind = "forbidden_file_present"
	FailureBuildFailed              FailureKind = "build_failed"
	FailureRuntimeNotFound          FailureKind = "runtime_not_found"
	FailureEntrypointMismatch       FailureKind = "entrypoint_mismatch"
	FailureLauncherInvalid          FailureKind = "launcher_invalid"
	FailureRuntimeTargetMissing     FailureKind = "runtime_target_missing"
	FailureGeneratedContractInvalid FailureKind = "generated_contract_invalid"
	FailureSourceFileMissing        FailureKind = "source_file_missing"
	FailureUnsupportedTargetKind    FailureKind = "unsupported_target_kind"
)

type Failure struct {
	Kind    FailureKind `json:"kind"`
	Path    string      `json:"path,omitempty"`
	Target  string      `json:"target,omitempty"`
	Message string      `json:"message"`
}

type WarningKind string

const (
	WarningManifestUnknownField  WarningKind = "manifest_unknown_field"
	WarningGeminiDirNameMismatch WarningKind = "gemini_dir_name_mismatch"
	WarningGeminiMCPCommandStyle WarningKind = "gemini_mcp_command_style"
	WarningGeminiPolicyIgnored   WarningKind = "gemini_policy_ignored"
)

type Warning struct {
	Kind    WarningKind `json:"kind"`
	Path    string      `json:"path,omitempty"`
	Message string      `json:"message"`
}

type Report struct {
	Platform string    `json:"platform,omitempty"`
	Checks   []string  `json:"checks"`
	Warnings []Warning `json:"warnings"`
	Failures []Failure `json:"failures"`
}

type ReportError struct {
	Report Report
}

func (e *ReportError) Error() string {
	if len(e.Report.Failures) == 0 {
		return "validation failed"
	}
	f := e.Report.Failures[0]
	switch f.Kind {
	case FailureRequiredFileMissing:
		return "required file missing: " + f.Path
	case FailureForbiddenFilePresent:
		return fmt.Sprintf("forbidden file present for platform %s: %s", e.Report.Platform, f.Path)
	case FailureBuildFailed:
		return fmt.Sprintf("go build %s: %s", f.Target, f.Message)
	case FailureManifestMissing, FailureManifestInvalid, FailureRuntimeNotFound, FailureEntrypointMismatch, FailureLauncherInvalid, FailureRuntimeTargetMissing:
		return f.Message
	default:
		return f.Message
	}
}

type Rule struct {
	Platform       string
	RequiredFiles  []string
	ForbiddenFiles []string
	BuildTargets   []string
}

func Run(root, platform string) error {
	report, err := Validate(root, platform)
	if err != nil {
		return err
	}
	if len(report.Failures) > 0 {
		return &ReportError{Report: report}
	}
	return nil
}

func Validate(root, platform string) (Report, error) {
	if fileExists(filepath.Join(root, pluginmodel.SourceDirName, pluginmanifest.FileName)) ||
		fileExists(filepath.Join(root, pluginmodel.LegacySourceDirName, pluginmanifest.FileName)) {
		return validatePluginProject(root, platform)
	}
	if fileExists(filepath.Join(root, ".plugin-kit-ai", "project.toml")) {
		return Report{}, invalidProjectReport(
			FailureManifestInvalid,
			filepath.Join(".plugin-kit-ai", "project.toml"),
			"unsupported project format: .plugin-kit-ai/project.toml is not supported; use plugin/plugin.yaml and plugin/targets/<platform>/...",
		)
	}
	return Report{}, invalidProjectReport(
		FailureManifestMissing,
		filepath.Join(pluginmodel.SourceDirName, pluginmanifest.FileName),
		"required manifest missing: plugin/plugin.yaml",
	)
}

func invalidProjectReport(kind FailureKind, path string, message string) error {
	return &ReportError{Report: normalizeReport(Report{
		Failures: []Failure{{
			Kind:    kind,
			Path:    path,
			Message: message,
		}},
	})}
}
