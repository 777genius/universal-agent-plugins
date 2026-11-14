package gemini

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func materializeAuthoredContexts(sourceRoot, destRoot string, meta packageMeta) (string, error) {
	contextsRoot := authoredpath.Join(sourceRoot, "targets", "gemini", "contexts")
	candidates, err := discoverFiles(contextsRoot)
	if err != nil {
		return "", domain.NewError(domain.ErrMutationApply, "discover Gemini contexts", err)
	}
	if len(candidates) == 0 {
		return "", nil
	}
	selected, ok, err := selectPrimaryContext(candidates, meta.ContextFileName)
	if err != nil {
		return "", domain.NewError(domain.ErrMutationApply, "select Gemini primary context", err)
	}
	if !ok {
		return "", nil
	}
	for _, rel := range candidates {
		src := filepath.Join(contextsRoot, rel)
		dest := authoredContextDestPath(destRoot, rel, selected)
		if err := copyFile(src, dest); err != nil {
			return "", err
		}
	}
	return filepath.Base(selected), nil
}

func authoredContextDestPath(destRoot, rel, selected string) string {
	name := filepath.Base(rel)
	if rel == selected {
		return filepath.Join(destRoot, name)
	}
	return filepath.Join(destRoot, "contexts", name)
}

func selectPrimaryContext(candidates []string, configured string) (string, bool, error) {
	configured = strings.TrimSpace(filepath.Base(configured))
	if configured != "" {
		return selectConfiguredPrimaryContext(candidates, configured)
	}
	return selectDefaultPrimaryContext(candidates)
}

func selectConfiguredPrimaryContext(candidates []string, configured string) (string, bool, error) {
	var matches []string
	for _, candidate := range candidates {
		if filepath.Base(candidate) == configured {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 0:
		return "", false, fmt.Errorf("context_file_name %q does not resolve to a Gemini-native context source", configured)
	case 1:
		return matches[0], true, nil
	default:
		return "", false, fmt.Errorf("context_file_name %q is ambiguous across multiple context sources", configured)
	}
}

func selectDefaultPrimaryContext(candidates []string) (string, bool, error) {
	var gemini []string
	for _, candidate := range candidates {
		if filepath.Base(candidate) == "GEMINI.md" {
			gemini = append(gemini, candidate)
		}
	}
	switch len(gemini) {
	case 1:
		return gemini[0], true, nil
	case 0:
		if len(candidates) == 1 {
			return candidates[0], true, nil
		}
		if len(candidates) == 0 {
			return "", false, nil
		}
		return "", false, fmt.Errorf("primary context selection is ambiguous; set targets/gemini/package.yaml context_file_name explicitly")
	default:
		return "", false, fmt.Errorf("primary context selection is ambiguous for GEMINI.md; keep one root context or set context_file_name explicitly")
	}
}
