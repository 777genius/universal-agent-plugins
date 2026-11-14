package platformexec

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

func invalidGeminiCommandTOMLDiagnostics(rel string, body []byte) []Diagnostic {
	var discard map[string]any
	if err := toml.Unmarshal(body, &discard); err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini command file %s is invalid TOML: %v", rel, err),
		}}
	}
	return nil
}
