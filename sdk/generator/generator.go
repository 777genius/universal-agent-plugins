package generator

import (
	"os"
	"path/filepath"
)

type Artifact struct {
	Path    string
	Content []byte
}

func RenderArtifacts() ([]Artifact, error) {
	m, err := loadModel()
	if err != nil {
		return nil, err
	}
	return renderArtifacts(m), nil
}

func WriteAll(repoRoot string) error {
	arts, err := RenderArtifacts()
	if err != nil {
		return err
	}
	for _, art := range arts {
		full := filepath.Join(repoRoot, art.Path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, art.Content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
