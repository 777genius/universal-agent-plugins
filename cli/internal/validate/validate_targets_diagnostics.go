package validate

import (
	"os"

	"github.com/777genius/plugin-kit-ai/cli/internal/platformexec"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func applyAdapterDiagnostics(report *Report, diagnostics []platformexec.Diagnostic) {
	for _, diagnostic := range diagnostics {
		switch diagnostic.Severity {
		case platformexec.SeverityWarning:
			report.Warnings = append(report.Warnings, Warning{
				Kind:    mapAdapterWarningKind(diagnostic.Code),
				Path:    diagnostic.Path,
				Message: diagnostic.Message,
			})
		default:
			report.Failures = append(report.Failures, Failure{
				Kind:    mapAdapterFailureKind(diagnostic.Code),
				Path:    diagnostic.Path,
				Target:  diagnostic.Target,
				Message: diagnostic.Message,
			})
		}
	}
}

func mapAdapterFailureKind(code string) FailureKind {
	switch code {
	case platformexec.CodeGeneratedContractInvalid:
		return FailureGeneratedContractInvalid
	case platformexec.CodeEntrypointMismatch:
		return FailureEntrypointMismatch
	default:
		return FailureManifestInvalid
	}
}

func mapAdapterWarningKind(code string) WarningKind {
	switch code {
	case platformexec.CodeGeminiDirNameMismatch:
		return WarningGeminiDirNameMismatch
	case platformexec.CodeGeminiMCPCommandStyle:
		return WarningGeminiMCPCommandStyle
	case platformexec.CodeGeminiPolicyIgnored:
		return WarningGeminiPolicyIgnored
	default:
		return WarningManifestUnknownField
	}
}

func mapManifestWarningKind(kind pluginmanifest.WarningKind) WarningKind {
	switch kind {
	case pluginmanifest.WarningUnknownField:
		return WarningManifestUnknownField
	default:
		return WarningManifestUnknownField
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func setOf(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}
