package pluginmodel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func LoadNativeExtraDoc(root, rel string, format NativeDocFormat) (NativeExtraDoc, error) {
	if strings.TrimSpace(rel) == "" {
		return NativeExtraDoc{}, nil
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return NativeExtraDoc{}, err
	}
	fields, err := ParseNativeExtraDocFields(body, format)
	if err != nil {
		return NativeExtraDoc{}, fmt.Errorf("parse %s: %w", rel, err)
	}
	return NativeExtraDoc{
		Path:   rel,
		Format: format,
		Raw:    body,
		Fields: fields,
	}, nil
}

func ParseNativeExtraDocFields(body []byte, format NativeDocFormat) (map[string]any, error) {
	fields := map[string]any{}
	switch format {
	case NativeDocFormatJSON:
		if err := json.Unmarshal(body, &fields); err != nil {
			return nil, err
		}
	case NativeDocFormatTOML:
		if err := toml.Unmarshal(body, &fields); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported native doc format %q", format)
	}
	if fields == nil {
		fields = map[string]any{}
	}
	return fields, nil
}

func ValidateNativeExtraDocConflicts(doc NativeExtraDoc, label string, managedPaths []string) error {
	if len(doc.Fields) == 0 {
		return nil
	}
	if conflict, ok := findManagedPathConflict(doc.Fields, "", setOf(managedPaths)); ok {
		return fmt.Errorf("%s may not override canonical field %q", label, conflict)
	}
	return nil
}

func MergeNativeExtraObject(base map[string]any, doc NativeExtraDoc, label string, managedPaths []string) error {
	if len(doc.Fields) == 0 {
		return nil
	}
	if err := ValidateNativeExtraDocConflicts(doc, label, managedPaths); err != nil {
		return err
	}
	mergeExtraObject(base, doc.Fields)
	return nil
}

func TrimmedExtraBody(doc NativeExtraDoc) []byte {
	return bytes.TrimSpace(doc.Raw)
}

func IsCanonicalCodexNotify(notify []string) bool {
	return len(notify) == 2 && strings.TrimSpace(notify[0]) != "" && strings.TrimSpace(notify[1]) == "notify"
}

func setOf(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func findManagedPathConflict(values map[string]any, prefix string, managed map[string]bool) (string, bool) {
	for key, value := range values {
		path := joinPath(prefix, key)
		if _, blocked := managed[path]; blocked {
			return path, true
		}
		if nested, ok := asStringMap(value); ok {
			if conflict, found := findManagedPathConflict(nested, path, managed); found {
				return conflict, true
			}
			continue
		}
		for managedPath := range managed {
			if strings.HasPrefix(managedPath, path+".") {
				return managedPath, true
			}
		}
	}
	return "", false
}

func mergeExtraObject(base, extra map[string]any) {
	for key, value := range extra {
		existing, hasExisting := asStringMap(base[key])
		incoming, incomingIsMap := asStringMap(value)
		if hasExisting && incomingIsMap {
			mergeExtraObject(existing, incoming)
			base[key] = existing
			continue
		}
		base[key] = value
	}
}

func asStringMap(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return typed, true
}

func joinPath(prefix, key string) string {
	key = strings.TrimSpace(key)
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}
