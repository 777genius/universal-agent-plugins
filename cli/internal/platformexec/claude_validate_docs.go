package platformexec

import (
	"fmt"
	"strings"
)

func validateClaudeSettings(root, rel string) []Diagnostic {
	doc, _, ok, err := loadClaudeJSONDoc(root, rel, "Claude settings")
	if err != nil {
		return claudeValidateDocFailure(rel, err)
	}
	if !ok {
		return nil
	}
	if value, exists := doc["agent"]; exists {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "claude",
				Message:  fmt.Sprintf(`Claude settings file %s must set "agent" as a non-empty string when present`, rel),
			}}
		}
	}
	return nil
}

func validateClaudeLSP(root, rel string) []Diagnostic {
	_, _, ok, err := loadClaudeJSONDoc(root, rel, "Claude LSP")
	if err != nil {
		return claudeValidateDocFailure(rel, err)
	}
	if !ok {
		return nil
	}
	return nil
}

func validateClaudeUserConfig(root, rel string) []Diagnostic {
	doc, _, ok, err := loadClaudeJSONDoc(root, rel, "Claude userConfig")
	if err != nil {
		return claudeValidateDocFailure(rel, err)
	}
	if !ok {
		return nil
	}
	for key, value := range doc {
		if _, ok := value.(map[string]any); !ok {
			return []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "claude",
				Message:  fmt.Sprintf("Claude userConfig entry %q in %s must be a JSON object", key, rel),
			}}
		}
	}
	return nil
}

func claudeValidateDocFailure(rel string, err error) []Diagnostic {
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeManifestInvalid,
		Path:     rel,
		Target:   "claude",
		Message:  err.Error(),
	}}
}
