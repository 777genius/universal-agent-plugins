package gemini

import (
	"path/filepath"
	"strings"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) runner() ports.ProcessRunner {
	if a.Runner != nil {
		return a.Runner
	}
	return process.OS{}
}

func (a Adapter) fs() ports.FileSystem {
	if a.FS != nil {
		return a.FS
	}
	return fsadapter.OS{}
}

func (a Adapter) userHome() string {
	return pathpolicy.UserHome(a.UserHome)
}

func fileExists(path string) bool {
	return pathpolicy.FileExists(path)
}

func usesManagedLocalInstall(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "local_path", "github_repo_path":
		return true
	default:
		return false
	}
}

func installModeForKind(kind string) string {
	if usesManagedLocalInstall(kind) {
		return "local_projection"
	}
	return "install"
}

func installModeFromRecord(record domain.InstallationRecord) string {
	target, ok := record.Targets[domain.TargetGemini]
	if !ok {
		return ""
	}
	if value, ok := target.AdapterMetadata["install_mode"].(string); ok {
		return strings.TrimSpace(value)
	}
	if value, ok := target.AdapterMetadata["update_mode"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func materializedRootFromRecord(record domain.InstallationRecord) string {
	target, ok := record.Targets[domain.TargetGemini]
	if !ok {
		return ""
	}
	if value, ok := target.AdapterMetadata["materialized_source_root"].(string); ok && strings.TrimSpace(value) != "" {
		return filepath.Clean(value)
	}
	for _, obj := range target.OwnedNativeObjects {
		if obj.Kind == "managed_source_root" && strings.TrimSpace(obj.Path) != "" {
			return filepath.Clean(obj.Path)
		}
	}
	return ""
}

func ownedGeminiObjects(record domain.InstallationRecord, home string) []domain.NativeObjectRef {
	target, ok := record.Targets[domain.TargetGemini]
	if !ok {
		return nil
	}
	if len(target.OwnedNativeObjects) > 0 {
		return append([]domain.NativeObjectRef(nil), target.OwnedNativeObjects...)
	}
	owned := []domain.NativeObjectRef{{
		Kind:            "extension_dir",
		Path:            filepath.Join(home, ".gemini", "extensions", record.IntegrationID),
		ProtectionClass: domain.ProtectionUserMutable,
	}}
	if root := materializedRootFromRecord(record); root != "" {
		owned = append(owned, domain.NativeObjectRef{
			Kind:            "managed_source_root",
			Path:            root,
			ProtectionClass: domain.ProtectionUserMutable,
		})
	}
	return owned
}

func mergeSettingsMaps(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for key, value := range src {
		existing, hasExisting := dst[key]
		nextMap, nextIsMap := value.(map[string]any)
		prevMap, prevIsMap := existing.(map[string]any)
		if hasExisting && nextIsMap && prevIsMap {
			dst[key] = mergeSettingsMaps(prevMap, nextMap)
			continue
		}
		dst[key] = value
	}
	return dst
}

func isGitBackedGeminiSource(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "git_url", "github_repo_path":
		return true
	default:
		return false
	}
}

func stringSliceFromAny(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		if typed, ok := v.([]string); ok {
			return append([]string(nil), typed...)
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func truthyBool(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func dedupeStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
