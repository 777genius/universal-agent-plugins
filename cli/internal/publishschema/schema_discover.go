package publishschema

import (
	"fmt"
	"slices"
)

func Discover(root string) (State, error) {
	return DiscoverInLayout(root, "")
}

func DiscoverInLayout(root, authoredRoot string) (State, error) {
	var out State
	if doc, ok, err := loadCodexMarketplace(root, authoredRoot); err != nil {
		return State{}, err
	} else if ok {
		out.Codex = doc
	}
	if doc, ok, err := loadClaudeMarketplace(root, authoredRoot); err != nil {
		return State{}, err
	} else if ok {
		out.Claude = doc
	}
	if doc, ok, err := loadGeminiGallery(root, authoredRoot); err != nil {
		return State{}, err
	} else if ok {
		out.Gemini = doc
	}
	return out, nil
}

func (s State) Paths() []string {
	var out []string
	if s.Codex != nil {
		out = append(out, s.Codex.Path)
	}
	if s.Claude != nil {
		out = append(out, s.Claude.Path)
	}
	if s.Gemini != nil {
		out = append(out, s.Gemini.Path)
	}
	slices.Sort(out)
	return out
}

func (s State) ValidateTargets(targets []string) error {
	enabled := setOf(targets)
	if s.Codex != nil && !enabled["codex-package"] {
		return fmt.Errorf("%s requires target %q in plugin.yaml", s.Codex.Path, "codex-package")
	}
	if s.Claude != nil && !enabled["claude"] {
		return fmt.Errorf("%s requires target %q in plugin.yaml", s.Claude.Path, "claude")
	}
	if s.Gemini != nil && !enabled["gemini"] {
		return fmt.Errorf("%s requires target %q in plugin.yaml", s.Gemini.Path, "gemini")
	}
	return nil
}
