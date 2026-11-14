package platformexec

import (
	"fmt"
	"strings"
)

type geminiValidationTracker map[string]string

func (tracker geminiValidationTracker) duplicateDiagnostic(rel, value, kind, field string) []Diagnostic {
	key := strings.ToLower(strings.TrimSpace(value))
	if prev, ok := tracker[key]; ok {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini %s file %s duplicates %s %q already declared in %s", kind, rel, field, value, prev),
		}}
	}
	tracker[key] = rel
	return nil
}
