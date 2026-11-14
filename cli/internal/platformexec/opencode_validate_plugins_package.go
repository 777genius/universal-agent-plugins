package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateOpenCodePluginPackageJSON(root string, rel string) (map[string]any, []Diagnostic) {
	if strings.TrimSpace(rel) == "" {
		return nil, nil
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "opencode",
			Message:  fmt.Sprintf("OpenCode plugin dependency metadata %s is not readable: %v", rel, err),
		}}
	}
	doc, err := decodeJSONObject(body, "OpenCode plugin dependency metadata "+rel)
	if err != nil {
		return nil, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     rel,
			Target:   "opencode",
			Message:  err.Error(),
		}}
	}
	return doc, nil
}

func openCodePackageDeclaresDependency(doc map[string]any, name string) bool {
	if len(doc) == 0 {
		return false
	}
	for _, field := range []string{"dependencies", "devDependencies", "peerDependencies"} {
		raw, ok := doc[field]
		if !ok {
			continue
		}
		deps, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if value, ok := deps[name]; ok {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				return true
			}
		}
	}
	return false
}
