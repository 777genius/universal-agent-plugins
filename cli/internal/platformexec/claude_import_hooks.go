package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func importClaudeHooks(root string, manifest importedClaudePluginManifest) ([]pluginmodel.Artifact, []byte, []pluginmodel.Warning, error) {
	const dst = "targets/claude/hooks/hooks.json"
	if manifest.HooksOverride {
		switch {
		case manifest.InlineHooks != nil:
			body := mustJSON(manifest.InlineHooks)
			return []pluginmodel.Artifact{{RelPath: dst, Content: body}}, body, []pluginmodel.Warning{{
				Kind:    pluginmodel.WarningFidelity,
				Path:    filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json")),
				Message: "custom Claude hooks were normalized into targets/claude/hooks/hooks.json",
			}}, nil
		case len(manifest.HookRefs) == 1:
			ref := cleanRelativeRef(manifest.HookRefs[0])
			body, err := os.ReadFile(filepath.Join(root, ref))
			if err != nil {
				return nil, nil, nil, err
			}
			return []pluginmodel.Artifact{{RelPath: dst, Content: body}}, body, []pluginmodel.Warning{{
				Kind:    pluginmodel.WarningFidelity,
				Path:    filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json")),
				Message: "custom Claude hooks path was normalized into targets/claude/hooks/hooks.json",
			}}, nil
		case len(manifest.HookRefs) > 1:
			body, err := mergeClaudeHookRefs(root, manifest.HookRefs)
			if err != nil {
				return nil, nil, nil, err
			}
			return []pluginmodel.Artifact{{RelPath: dst, Content: body}}, body, []pluginmodel.Warning{{
				Kind:    pluginmodel.WarningFidelity,
				Path:    filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json")),
				Message: "custom Claude hooks path array was normalized into canonical package-standard layout",
			}}, nil
		default:
			return nil, nil, nil, nil
		}
	}
	body, err := os.ReadFile(filepath.Join(root, "hooks", "hooks.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, err
	}
	return []pluginmodel.Artifact{{RelPath: dst, Content: body}}, body, nil, nil
}

func mergeClaudeHookRefs(root string, refs []string) ([]byte, error) {
	merged := map[string]any{}
	for _, ref := range refs {
		ref = cleanRelativeRef(ref)
		body, err := os.ReadFile(filepath.Join(root, ref))
		if err != nil {
			return nil, err
		}
		doc, err := decodeJSONObject(body, fmt.Sprintf("Claude hooks file %s", ref))
		if err != nil {
			return nil, err
		}
		value, ok := doc["hooks"]
		if !ok {
			return nil, fmt.Errorf("claude hooks file %s is incompatible with package-standard normalization: top-level \"hooks\" object required", ref)
		}
		hooksMap, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("claude hooks file %s is incompatible with package-standard normalization: top-level \"hooks\" must be a JSON object", ref)
		}
		if err := mergeClaudeHookTree(merged, hooksMap, ref, "hooks"); err != nil {
			return nil, err
		}
	}
	return marshalJSON(map[string]any{"hooks": merged})
}

func mergeClaudeHookTree(dst, src map[string]any, ref, path string) error {
	for key, srcValue := range src {
		nextPath := key
		if strings.TrimSpace(path) != "" {
			nextPath = path + "." + key
		}
		dstValue, exists := dst[key]
		if !exists {
			dst[key] = srcValue
			continue
		}
		switch typed := srcValue.(type) {
		case []any:
			dstSlice, ok := dstValue.([]any)
			if !ok {
				return fmt.Errorf("claude hooks file %s is incompatible with package-standard normalization: %s mixes array and non-array shapes", ref, nextPath)
			}
			dst[key] = append(dstSlice, typed...)
		case map[string]any:
			dstMap, ok := dstValue.(map[string]any)
			if !ok {
				return fmt.Errorf("claude hooks file %s is incompatible with package-standard normalization: %s mixes object and non-object shapes", ref, nextPath)
			}
			if err := mergeClaudeHookTree(dstMap, typed, ref, nextPath); err != nil {
				return err
			}
		default:
			if !reflect.DeepEqual(dstValue, srcValue) {
				return fmt.Errorf("claude hooks file %s is incompatible with package-standard normalization: %s has conflicting scalar values", ref, nextPath)
			}
		}
	}
	return nil
}

func mergeClaudeObjectRefs(root string, refs []string, label string) ([]byte, error) {
	merged := map[string]any{}
	for _, ref := range refs {
		ref = cleanRelativeRef(ref)
		body, err := os.ReadFile(filepath.Join(root, ref))
		if err != nil {
			return nil, err
		}
		doc, err := decodeJSONObject(body, fmt.Sprintf("%s file %s", label, ref))
		if err != nil {
			return nil, err
		}
		for key, value := range doc {
			if existing, ok := merged[key]; ok {
				if !reflect.DeepEqual(existing, value) {
					return nil, fmt.Errorf("%s path array cannot be normalized safely: duplicate key %q conflicts in %s", label, key, ref)
				}
				continue
			}
			merged[key] = value
		}
	}
	return marshalJSON(merged)
}
