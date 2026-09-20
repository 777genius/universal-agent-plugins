package platformexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateCodexHooksDiagnostics(root string, state pluginmodel.TargetState) []Diagnostic {
	if len(state.ComponentPaths("hooks")) == 0 {
		return nil
	}
	hookPath := filepath.ToSlash(filepath.Join(authoredComponentDir(state, "hooks", filepath.Join("targets", "codex-package", "hooks")), "hooks.json"))
	body, err := os.ReadFile(filepath.Join(root, hookPath))
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     hookPath,
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex package hooks require %s when hooks are authored: %v", hookPath, err),
		}}
	}
	doc, err := decodeJSONObject(body, fmt.Sprintf("Codex hooks file %s", hookPath))
	if err != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     hookPath,
			Target:   "codex-package",
			Message:  err.Error(),
		}}
	}
	hooks, ok := doc["hooks"]
	if !ok {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     hookPath,
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex hooks file %s must contain a top-level \"hooks\" object", hookPath),
		}}
	}
	if _, ok := hooks.(map[string]any); !ok {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     hookPath,
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex hooks file %s top-level \"hooks\" must be a JSON object", hookPath),
		}}
	}
	return nil
}
