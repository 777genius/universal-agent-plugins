package platformexec

import (
	"path/filepath"
	"slices"
	"strings"
)

func geminiPackageMetaEqual(left, right geminiPackageMeta) bool {
	return strings.TrimSpace(left.ContextFileName) == strings.TrimSpace(right.ContextFileName) &&
		slices.Equal(normalizeGeminiExcludeTools(left.ExcludeTools), normalizeGeminiExcludeTools(right.ExcludeTools)) &&
		strings.TrimSpace(left.MigratedTo) == strings.TrimSpace(right.MigratedTo) &&
		strings.TrimSpace(left.PlanDirectory) == strings.TrimSpace(right.PlanDirectory)
}

func geminiExtensionDirBase(root string) string {
	abs, err := filepath.Abs(root)
	if err == nil {
		return filepath.Base(filepath.Clean(abs))
	}
	return filepath.Base(filepath.Clean(root))
}

func normalizeGeminiExcludeTools(values []string) []string {
	var out []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func validateGeminiExcludeTools(path string, values []string) []Diagnostic {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     path,
				Target:   "gemini",
				Message:  "Gemini exclude_tools entries must be non-empty strings naming built-in tools",
			}}
		}
	}
	return nil
}
