package targetcontracts

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func nativeDocs(items []platformmeta.NativeDocSpec) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		out = append(out, item.Kind+"="+authoringDocPath(item.Path))
	}
	return out
}

func nativeDocPaths(items []platformmeta.NativeDocSpec) map[string]string {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]string, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Kind) == "" || strings.TrimSpace(item.Path) == "" {
			continue
		}
		out[item.Kind] = authoringDocPath(item.Path)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func authoringDocPath(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	return pluginmodel.CanonicalAuthoredPath(path)
}

func fromSurfaceSupport(items []platformmeta.SurfaceSupport) []Surface {
	out := make([]Surface, 0, len(items))
	for _, item := range items {
		out = append(out, Surface{
			Kind: item.Kind,
			Tier: string(item.Tier),
		})
	}
	return out
}

func nativeSurfaceTiers(items []platformmeta.SurfaceSupport) map[string]string {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]string, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Kind) == "" || strings.TrimSpace(string(item.Tier)) == "" {
			continue
		}
		out[item.Kind] = string(item.Tier)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
