package agentpluginscli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/dependencymanifest"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func (app App) dependencyPaths() (manifest, lock string) {
	root := strings.TrimSpace(app.DependenciesRoot)
	if root == "" {
		root = filepath.Dir(app.ManagedRoot)
	}
	return filepath.Join(root, "uap.toml"), filepath.Join(root, "uap.lock")
}

func readDependencyManifest(path string) (environment.Requirements, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return environment.Requirements{}, err
	}
	return readDependencyManifestBytes(path, body)
}

func readDependencyManifestBytes(path string, body []byte) (environment.Requirements, error) {
	document, err := dependencymanifest.Decode(body, path)
	if err != nil {
		return environment.Requirements{}, err
	}
	return document.Root()
}

func writeInitialDependencyManifest(path string) error {
	body := initialDependencyManifest()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(body); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func initialDependencyManifest() []byte {
	return []byte(`# Agent Plugins dependency environment
schema_version = 1

[dependencies]

[targets]
`)
}
