package codex

import (
	"path/filepath"
	"strings"
)

type codexSkillSelector struct {
	Name    *string `toml:"name"`
	Path    *string `toml:"path"`
	Enabled *bool   `toml:"enabled"`
}

// The pinned client accepts skill selectors only from user/session layers, not
// project layers. This checks the visible user file's ordered rules only; it
// does not infer the selected native profile or session's effective policy.
func codexUserSkillDisabled(selectors []codexSkillSelector, name, document string) bool {
	// Native SkillsConfig decoding is all-or-nothing for the selected layer.
	// Missing enabled or a nonabsolute typed path invalidates the whole section.
	for _, selector := range selectors {
		if selector.Enabled == nil || selector.Path != nil && !filepath.IsAbs(*selector.Path) {
			return false
		}
	}
	disabled := false
	document = codexSkillDocumentIdentity(document)
	for _, selector := range selectors {
		matches := false
		switch {
		case selector.Name != nil && selector.Path == nil:
			matches = strings.TrimSpace(*selector.Name) == name
		case selector.Path != nil && selector.Name == nil:
			matches = codexSkillDocumentIdentity(*selector.Path) == document
		}
		if matches {
			disabled = !*selector.Enabled
		}
	}
	return disabled
}

func codexSkillDocumentIdentity(path string) string {
	if canonical, err := filepath.EvalSymlinks(path); err == nil {
		return canonical
	}
	return filepath.Clean(path)
}
