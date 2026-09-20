package codexconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type ImportedConfig struct {
	Model  string
	Notify []string
	Extra  map[string]any
}

func DecodeImportedConfig(body []byte) (ImportedConfig, error) {
	var raw map[string]any
	if err := toml.Unmarshal(body, &raw); err != nil {
		return ImportedConfig{}, err
	}
	out := ImportedConfig{}
	if value, ok := raw["model"]; ok {
		text, ok := value.(string)
		if !ok {
			return ImportedConfig{}, fmt.Errorf("Codex config field %q must be a string", "model")
		}
		out.Model = strings.TrimSpace(text)
	}
	if value, ok := raw["notify"]; ok {
		items, ok := value.([]any)
		if !ok {
			return ImportedConfig{}, fmt.Errorf("Codex config field %q must be an array of non-empty strings", "notify")
		}
		out.Notify = make([]string, 0, len(items))
		for i, item := range items {
			text, ok := item.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return ImportedConfig{}, fmt.Errorf("Codex config field %q must contain non-empty strings (invalid entry at index %d)", "notify", i)
			}
			out.Notify = append(out.Notify, strings.TrimSpace(text))
		}
	}
	delete(raw, "model")
	delete(raw, "notify")
	if len(raw) > 0 {
		out.Extra = raw
	}
	return out, nil
}

func ReadImportedConfig(root string) (ImportedConfig, []byte, error) {
	body, err := os.ReadFile(filepath.Join(root, ".codex", "config.toml"))
	if err != nil {
		return ImportedConfig{}, nil, err
	}
	out, err := DecodeImportedConfig(body)
	if err != nil {
		return ImportedConfig{}, nil, err
	}
	return out, body, nil
}
