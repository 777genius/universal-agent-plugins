package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateOpenCodeCommandFiles(root string, rels []string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, rel := range rels {
		if filepath.Ext(rel) != ".md" {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode command file %s must use the .md extension", rel),
			})
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode command file %s is not readable: %v", rel, err),
			})
			continue
		}
		frontmatter, markdown, err := parseMarkdownFrontmatterDocument(body, "OpenCode command file "+rel)
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
		if strings.TrimSpace(markdown) == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode command file %s must define a markdown command template body", rel),
			})
		}
		if description, ok := frontmatter["description"]; ok {
			text, ok := description.(string)
			if !ok || strings.TrimSpace(text) == "" {
				diagnostics = append(diagnostics, Diagnostic{
					Severity: SeverityFailure,
					Code:     CodeManifestInvalid,
					Path:     rel,
					Target:   "opencode",
					Message:  fmt.Sprintf("OpenCode command file %s frontmatter field %q must be a non-empty string", rel, "description"),
				})
			}
		}
	}
	return diagnostics
}
