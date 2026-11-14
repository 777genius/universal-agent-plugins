package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateOpenCodeToolFiles(root string, rels []string, packageDoc map[string]any) []Diagnostic {
	if len(rels) == 0 {
		return nil
	}
	authoredRoot := pluginmodel.SourceDirName
	for _, rel := range rels {
		if candidate := authoredRootFromPath(rel); candidate != "" {
			authoredRoot = candidate
			break
		}
	}
	var (
		diagnostics      []Diagnostic
		hasDefinition    bool
		usesPluginHelper bool
		seenCaseFolded   = map[string]string{}
	)
	for _, rel := range rels {
		clean := filepath.ToSlash(filepath.Clean(rel))
		if clean != rel || strings.Contains(clean, "..") {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode tool file %s must stay within targets/opencode/tools without path traversal", rel),
			})
			continue
		}
		lower := strings.ToLower(clean)
		if prior, ok := seenCaseFolded[lower]; ok && prior != clean {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode tool files %s and %s collide on case-insensitive filesystems", prior, rel),
			})
		} else {
			seenCaseFolded[lower] = clean
		}
		fullPath := filepath.Join(root, rel)
		info, err := os.Lstat(fullPath)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode tool file %s is not readable: %v", rel, err),
			})
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode tool file %s must not be a symlink", rel),
			})
			continue
		}
		if info.IsDir() {
			continue
		}
		body, err := os.ReadFile(fullPath)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode tool file %s is not readable: %v", rel, err),
			})
			continue
		}
		if isOpenCodePluginEntryFile(rel) {
			hasDefinition = true
		}
		if strings.Contains(string(body), `@opencode-ai/plugin`) {
			usesPluginHelper = true
		}
	}
	if !hasDefinition {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     filepath.ToSlash(filepath.Join(authoredRoot, "targets", "opencode", "tools")),
			Target:   "opencode",
			Message:  fmt.Sprintf("OpenCode standalone tools require at least one JS/TS tool definition file under %s/targets/opencode/tools", authoredRoot),
		})
	}
	if usesPluginHelper && !openCodePackageDeclaresDependency(packageDoc, "@opencode-ai/plugin") {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     filepath.ToSlash(filepath.Join(authoredRoot, "targets", "opencode", "package.json")),
			Target:   "opencode",
			Message:  fmt.Sprintf(`OpenCode standalone tool files that import "@opencode-ai/plugin" must declare that dependency in %s/targets/opencode/package.json`, authoredRoot),
		})
	}
	return diagnostics
}
