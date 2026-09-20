package platformexec

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func readGeminiYAMLMap(root, rel string) ([]byte, map[string]any, error) {
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, nil, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(body, &raw); err != nil {
		return nil, nil, err
	}
	return body, raw, nil
}
