package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateOpenCodeAgentMarkdownFiles(root string, rels []string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, rel := range rels {
		body, failure := readOpenCodeAgentMarkdown(root, rel)
		if failure != nil {
			diagnostics = append(diagnostics, *failure)
			continue
		}
		frontmatter, _, err := parseMarkdownFrontmatterDocument(body, "OpenCode agent file "+rel)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  err.Error(),
			})
			continue
		}
		diagnostics = append(diagnostics, validateOpenCodeAgentFrontmatterFields(rel, frontmatter)...)
	}
	return diagnostics
}

func readOpenCodeAgentMarkdown(root, rel string) ([]byte, *Diagnostic) {
	if filepath.Ext(rel) != ".md" {
		diag := openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s must use the .md extension", rel))
		return nil, &diag
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		diag := openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode agent file %s is not readable: %v", rel, err))
		return nil, &diag
	}
	return body, nil
}

func validateOpenCodeDefaultAgentFile(root string, rel string) []Diagnostic {
	if strings.TrimSpace(rel) == "" {
		return nil
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return []Diagnostic{openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode default agent file %s is not readable: %v", rel, err))}
	}
	if strings.TrimSpace(string(body)) == "" {
		return []Diagnostic{openCodeAgentDiagnostic(rel, fmt.Sprintf("OpenCode default agent file %s must contain a non-empty agent name", rel))}
	}
	return nil
}

func openCodeAgentDiagnostic(rel, message string) Diagnostic {
	return Diagnostic{
		Severity: SeverityFailure,
		Code:     CodeManifestInvalid,
		Path:     rel,
		Target:   "opencode",
		Message:  message,
	}
}
