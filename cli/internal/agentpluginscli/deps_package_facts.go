package agentpluginscli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/dependencymanifest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Requirements come only from the exact, verified package root. Author targets
// and nested lock files never participate in the consumer environment.
func packageDependencyFacts(envelope domain.PackageEnvelope) (map[string]environment.Requirement, error) {
	if err := dependencymanifest.RejectLegacyDependencies(envelope.Manifest.Extensions); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(envelope.SnapshotRoot)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	_, err = root.Lstat("uap.toml")
	if os.IsNotExist(err) {
		return map[string]environment.Requirement{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("package uap.toml: %w", err)
	}
	info, err := root.Stat("uap.toml")
	if err != nil {
		return nil, fmt.Errorf("package uap.toml: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("package uap.toml must be a regular file")
	}
	body, err := root.ReadFile("uap.toml")
	if err != nil {
		return nil, fmt.Errorf("package uap.toml: %w", err)
	}
	document, err := dependencymanifest.Decode(body, filepath.Join(envelope.SnapshotRoot, "uap.toml"))
	if err != nil {
		return nil, err
	}
	return document.Package()
}

func rejectImperativePackageRequirements(loaded loadedPackage) error {
	requirements, err := packageDependencyFacts(loaded.envelope)
	if err != nil {
		return err
	}
	if len(requirements) != 0 {
		return fmt.Errorf("package %s declares UAP requirements; use a managed saved add/install workflow instead of --no-save or local/manual installation", loaded.envelope.Manifest.Name)
	}
	return nil
}
