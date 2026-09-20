package geminimanifest

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type PackageMeta struct {
	ContextFileName string   `yaml:"context_file_name,omitempty"`
	ExcludeTools    []string `yaml:"exclude_tools,omitempty"`
	MigratedTo      string   `yaml:"migrated_to,omitempty"`
	PlanDirectory   string   `yaml:"plan_directory,omitempty"`
}

type ImportedExtension struct {
	Name        string
	Version     string
	Description string
	Meta        PackageMeta
	MCPServers  map[string]any
	Settings    []any
	Themes      []any
	Extra       map[string]any
}

func DecodeImportedExtension(body []byte) (ImportedExtension, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return ImportedExtension{}, err
	}
	return decodeImportedExtensionObject(raw)
}

func ReadImportedExtension(root string) (ImportedExtension, bool, error) {
	body, err := os.ReadFile(filepath.Join(root, "gemini-extension.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return ImportedExtension{}, false, nil
		}
		return ImportedExtension{}, false, err
	}
	data, err := DecodeImportedExtension(body)
	if err != nil {
		return ImportedExtension{}, false, err
	}
	return data, true, nil
}
