package pluginmanifest

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func detectAuthoredLayout(root string) (authoredLayout, error) {
	canonical := authoredLayout{RootRel: pluginmodel.SourceDirName}
	legacy := authoredLayout{RootRel: pluginmodel.LegacySourceDirName}
	if legacyRel := rootLegacyPortableMCPPath(root); legacyRel != "" {
		return authoredLayout{}, fmt.Errorf("unsupported portable MCP authored path %s: use %s/mcp/servers.yaml", legacyRel, pluginmodel.SourceDirName)
	}
	canonicalPresent := authoredLayoutPresent(root, canonical)
	legacyPresent := authoredLayoutPresent(root, legacy)
	rootPresent := rootAuthoredLayoutPresent(root)

	switch {
	case legacyPresent:
		return authoredLayout{}, fmt.Errorf("unsupported authored layout: legacy %s/ is no longer supported; move manual plugin sources into %s/", pluginmodel.LegacySourceDirName, pluginmodel.SourceDirName)
	case canonicalPresent && rootPresent:
		return authoredLayout{}, fmt.Errorf("mixed authored layout: keep manual plugin sources only under %s/ and remove root-authored plugin files", pluginmodel.SourceDirName)
	case canonicalPresent:
		return canonical, nil
	case rootPresent:
		return authoredLayout{}, fmt.Errorf("unsupported authored layout: move manual plugin sources into %s/", pluginmodel.SourceDirName)
	default:
		return canonical, nil
	}
}

func rootLegacyPortableMCPPath(root string) string {
	for _, rel := range []string{
		filepath.ToSlash(filepath.Join("mcp", "servers.json")),
		filepath.ToSlash(filepath.Join("mcp", "servers.yml")),
	} {
		if authoredInputExists(root, rel) {
			return rel
		}
	}
	return ""
}

func authoredLayoutPresent(root string, layout authoredLayout) bool {
	for _, rel := range authoredSentinelPaths() {
		if authoredInputExists(root, layout.Path(rel)) {
			return true
		}
	}
	return false
}

func rootAuthoredLayoutPresent(root string) bool {
	for _, rel := range rootAuthoredSentinelPaths() {
		if authoredInputExists(root, rel) {
			return true
		}
	}
	return false
}

func authoredSentinelPaths() []string {
	return []string{
		FileName,
		LauncherFileName,
		filepath.ToSlash(filepath.Join("mcp", "servers.yaml")),
		"skills",
		"targets",
		"publish",
	}
}

func rootAuthoredSentinelPaths() []string {
	return []string{
		FileName,
		LauncherFileName,
		filepath.ToSlash(filepath.Join("mcp", "servers.yaml")),
		filepath.ToSlash(filepath.Join("mcp", "servers.yml")),
		filepath.ToSlash(filepath.Join("mcp", "servers.json")),
		"targets",
		"publish",
	}
}
