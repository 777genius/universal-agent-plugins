package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func validateUnsupportedTargetSurfaces(root, target string, report *Report) {
	profile, ok := platformmeta.Lookup(target)
	if !ok {
		return
	}
	for _, surface := range profile.SurfaceTiers {
		if surface.Tier != platformmeta.SurfaceTierUnsupported {
			continue
		}
		for _, path := range unsupportedSurfacePaths(root, target, surface.Kind, profile) {
			report.Failures = append(report.Failures, Failure{
				Kind:    FailureUnsupportedTargetKind,
				Path:    path,
				Target:  target,
				Message: fmt.Sprintf("target %s does not support authored surface %s", target, surface.Kind),
			})
		}
	}
}

func unsupportedPortablePath(portable pluginmanifest.PortableComponents, kind string) string {
	switch kind {
	case "skills":
		if len(portable.Paths("skills")) > 0 {
			return canonicalAuthoredPath("skills")
		}
		return canonicalAuthoredPath("skills")
	case "mcp_servers":
		if portable.MCP != nil && strings.TrimSpace(portable.MCP.Path) != "" {
			return canonicalAuthoredPath(portable.MCP.Path)
		}
		return canonicalAuthoredPath("mcp")
	default:
		return kind
	}
}

func unsupportedTargetKindPath(target string, tc pluginmanifest.TargetComponents, kind string) string {
	if path := strings.TrimSpace(tc.DocPath(kind)); path != "" {
		return canonicalAuthoredPath(path)
	}
	if len(tc.ComponentPaths(kind)) > 0 {
		return canonicalAuthoredPath(filepath.ToSlash(filepath.Join("targets", target, kind)))
	}
	return canonicalAuthoredPath(filepath.ToSlash(filepath.Join("targets", target, kind)))
}

func unsupportedSurfacePaths(root, target, kind string, profile platformmeta.PlatformProfile) []string {
	seen := map[string]struct{}{}
	for _, doc := range profile.NativeDocs {
		if doc.Kind != kind {
			continue
		}
		authoredPath := pluginmodel.RebaseAuthoredPath(doc.Path, authoredProjectRoot(root))
		if fileExists(filepath.Join(root, authoredPath)) {
			seen[authoredPath] = struct{}{}
		}
	}
	dir := canonicalAuthoredPath(filepath.Join("targets", target, kind))
	if entries, err := os.ReadDir(filepath.Join(root, dir)); err == nil && len(entries) > 0 {
		seen[dir] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	slices.Sort(out)
	return out
}
