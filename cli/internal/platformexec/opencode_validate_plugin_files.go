package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateOpenCodePluginFiles(root string, rels []string, packageDoc map[string]any) []Diagnostic {
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
		hasEntry         bool
		usesPluginHelper bool
	)
	for _, rel := range rels {
		fullPath := filepath.Join(root, rel)
		info, err := os.Stat(fullPath)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode local plugin file %s is not readable: %v", rel, err),
			})
			continue
		}
		if info.IsDir() {
			continue
		}
		if isOpenCodePluginEntryFile(rel) {
			hasEntry = true
		}
		body, err := os.ReadFile(fullPath)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  fmt.Sprintf("OpenCode local plugin file %s is not readable: %v", rel, err),
			})
			continue
		}
		src := string(body)
		if strings.Contains(src, `export default`) && strings.Contains(src, `setup(`) {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     rel,
				Target:   "opencode",
				Message:  "OpenCode local plugin file uses the old scaffold shape `export default { setup() { ... } }`; use official named async plugin exports instead",
			})
		}
		if strings.Contains(src, `@opencode-ai/plugin`) {
			usesPluginHelper = true
		}
	}
	if !hasEntry {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     filepath.ToSlash(filepath.Join(authoredRoot, "targets", "opencode", "plugins")),
			Target:   "opencode",
			Message:  fmt.Sprintf("OpenCode local plugin code requires at least one JS/TS plugin entry file under %s/targets/opencode/plugins (for example .js, .mjs, .cjs, .ts, .mts, or .cts)", authoredRoot),
		})
	}
	if usesPluginHelper && !openCodePackageDeclaresDependency(packageDoc, "@opencode-ai/plugin") {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     filepath.ToSlash(filepath.Join(authoredRoot, "targets", "opencode", "package.json")),
			Target:   "opencode",
			Message:  fmt.Sprintf(`OpenCode plugin files that import "@opencode-ai/plugin" must declare that dependency in %s/targets/opencode/package.json`, authoredRoot),
		})
	}
	return diagnostics
}

func isOpenCodePluginEntryFile(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".js", ".mjs", ".cjs", ".ts", ".mts", ".cts":
		return true
	default:
		return false
	}
}
