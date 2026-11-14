package agentpluginscli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
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
	body, err := readPackageDependencyManifest(root)
	if err != nil {
		return nil, err
	}
	document, err := dependencymanifest.Decode(body, filepath.Join(envelope.SnapshotRoot, "uap.toml"))
	if err != nil {
		return nil, err
	}
	return document.Package()
}

// Sidecars may follow links inside the captured package. The rooted open keeps
// that policy distinct from authority files, whose leaf links are forbidden.
func readPackageDependencyManifest(root *os.Root) ([]byte, error) {
	before, err := root.Stat("uap.toml")
	if err != nil {
		return nil, fmt.Errorf("package uap.toml: %w", err)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("package uap.toml must be a regular file")
	}
	if before.Size() > dependencymanifest.MaxBytes {
		return nil, fmt.Errorf("package uap.toml exceeds %d bytes", dependencymanifest.MaxBytes)
	}
	file, err := atomicfile.OpenRootReadFile(root, "uap.toml")
	if err != nil {
		return nil, fmt.Errorf("package uap.toml: %w", err)
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("package uap.toml: %w", err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) || before.Size() != opened.Size() || !before.ModTime().Equal(opened.ModTime()) {
		return nil, fmt.Errorf("package uap.toml changed while opening")
	}
	body, err := io.ReadAll(io.LimitReader(file, dependencymanifest.MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("package uap.toml: %w", err)
	}
	if int64(len(body)) > dependencymanifest.MaxBytes {
		return nil, fmt.Errorf("package uap.toml exceeds %d bytes", dependencymanifest.MaxBytes)
	}
	after, err := root.Stat("uap.toml")
	if err != nil {
		return nil, fmt.Errorf("package uap.toml: %w", err)
	}
	if !os.SameFile(opened, after) || opened.Size() != after.Size() || int64(len(body)) != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return nil, fmt.Errorf("package uap.toml changed during read")
	}
	return body, nil
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
