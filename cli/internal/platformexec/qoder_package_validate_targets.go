package platformexec

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

// validateQoderHooksContract checks the authored hooks document shape and that
// the generated hooks mirror is an exact projection of it. Hooks are mirrored
// verbatim, so any drift means generate output was hand-edited or stale.
func validateQoderHooksContract(root string, state pluginmodel.TargetState) []Diagnostic {
	authored := state.ComponentPaths("hooks")
	if len(authored) == 0 {
		return validateQoderMirrorAbsent(root, "hooks")
	}
	source := authored[0]
	body, err := os.ReadFile(filepath.Join(root, source))
	if err != nil {
		return []Diagnostic{qoderSourceDiagnostic(source, fmt.Sprintf("Qoder hooks document is not readable: %v", err))}
	}
	if diagnostics := validateQoderHooksDocumentShape(source, body); len(diagnostics) > 0 {
		return diagnostics
	}
	return validateQoderMirrorProjection(root, source, filepath.ToSlash(filepath.Join("hooks", "hooks.json")))
}

func validateQoderHooksDocumentShape(source string, body []byte) []Diagnostic {
	doc, err := parseQoderHooksDocument(body)
	if err != nil {
		return []Diagnostic{qoderSourceDiagnostic(source, fmt.Sprintf("Qoder hooks document is invalid: %v", err))}
	}
	for _, event := range sortedHookEvents(doc.Hooks) {
		if !qoderHookNameRecognized(event) {
			return []Diagnostic{qoderSourceDiagnostic(source, fmt.Sprintf("Qoder hooks document uses unrecognized hook event %q", event))}
		}
		for groupIndex, group := range doc.Hooks[event] {
			if len(group.Hooks) == 0 {
				return []Diagnostic{qoderSourceDiagnostic(source, fmt.Sprintf("Qoder hook event %q group %d has no hook commands", event, groupIndex))}
			}
			for hookIndex, hook := range group.Hooks {
				where := fmt.Sprintf("Qoder hook event %q group %d hook %d", event, groupIndex, hookIndex)
				if hook.Type != "command" {
					return []Diagnostic{qoderSourceDiagnostic(source, where+` must use type "command"`)}
				}
				if !qoderHookCommandBundled(hook.Command) {
					return []Diagnostic{qoderSourceDiagnostic(source, where+" command must be a bare runtime or bundled under "+qoderHookRootToken)}
				}
			}
		}
	}
	return nil
}

// validateQoderCommandsContract mirrors the hooks contract for target-native
// command documents: authored frontmatter must carry a description and the
// generated commands/ tree must be an exact projection.
func validateQoderCommandsContract(root string, state pluginmodel.TargetState) []Diagnostic {
	authored := state.ComponentPaths("commands")
	if len(authored) == 0 {
		return validateQoderMirrorAbsent(root, "commands")
	}
	sourceDir := authoredComponentDir(state, "commands", filepath.Join("targets", "qoder", "commands"))
	for _, source := range authored {
		body, err := os.ReadFile(filepath.Join(root, source))
		if err != nil {
			return []Diagnostic{qoderSourceDiagnostic(source, fmt.Sprintf("Qoder command document is not readable: %v", err))}
		}
		if !qoderCommandHasDescription(body) {
			return []Diagnostic{qoderSourceDiagnostic(source, "Qoder command document must begin with frontmatter defining a non-empty description")}
		}
		rel, err := filepath.Rel(sourceDir, source)
		if err != nil {
			return []Diagnostic{qoderSourceDiagnostic(source, "Qoder command document is outside targets/qoder/commands")}
		}
		if diagnostics := validateQoderMirrorProjection(root, source, filepath.ToSlash(filepath.Join("commands", rel))); len(diagnostics) > 0 {
			return diagnostics
		}
	}
	return nil
}

func validateQoderMirrorProjection(root, source, generatedRel string) []Diagnostic {
	authoredBody, err := os.ReadFile(filepath.Join(root, source))
	if err != nil {
		return []Diagnostic{qoderSourceDiagnostic(source, fmt.Sprintf("Qoder authored document is not readable: %v", err))}
	}
	generatedBody, err := os.ReadFile(filepath.Join(root, generatedRel))
	if err != nil {
		return []Diagnostic{qoderGeneratedDiagnostic(generatedRel, fmt.Sprintf("Qoder generated mirror %s is missing; run generate", generatedRel))}
	}
	if bytes.Equal(authoredBody, generatedBody) {
		return nil
	}
	return []Diagnostic{qoderGeneratedDiagnostic(generatedRel, fmt.Sprintf("Qoder generated mirror %s does not match authored %s", generatedRel, source))}
}

func validateQoderMirrorAbsent(root, dir string) []Diagnostic {
	if !fileExists(filepath.Join(root, dir)) {
		return nil
	}
	return []Diagnostic{qoderGeneratedDiagnostic(dir, fmt.Sprintf("Qoder generated %s/ exists without authored targets/qoder/%s sources", dir, dir))}
}

// qoderCommandHasDescription accepts the observed Qoder command document:
// YAML frontmatter with a non-empty description, then a markdown body.
func qoderCommandHasDescription(body []byte) bool {
	normalized := bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(normalized, []byte("---\n")) {
		return false
	}
	remainder := normalized[len("---\n"):]
	end := bytes.Index(remainder, []byte("\n---"))
	if end < 0 {
		return false
	}
	for _, line := range strings.Split(string(remainder[:end]), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "description" {
			continue
		}
		return strings.TrimSpace(value) != ""
	}
	return false
}

func sortedHookEvents(hooks map[string][]qoderHookGroup) []string {
	events := make([]string, 0, len(hooks))
	for event := range hooks {
		events = append(events, event)
	}
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j] < events[j-1]; j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}
	return events
}

func qoderSourceDiagnostic(path, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeManifestInvalid,
		Path:     filepath.ToSlash(path),
		Target:   "qoder",
		Message:  message,
	}
}

func qoderGeneratedDiagnostic(path, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     filepath.ToSlash(path),
		Target:   "qoder",
		Message:  message,
	}
}
