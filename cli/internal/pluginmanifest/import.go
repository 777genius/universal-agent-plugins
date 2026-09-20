package pluginmanifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/platformexec"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type preparedImport struct {
	Manifest      Manifest
	Launcher      *Launcher
	Artifacts     []Artifact
	Warnings      []Warning
	ImportSource  string
	DetectedKinds []string
	DroppedKinds  []string
}

func importPackage(root string, from string, force bool, includeUserScope bool) (Manifest, []Warning, error) {
	if _, err := detectAuthoredLayout(root); err != nil && !os.IsNotExist(err) {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			return Manifest{}, nil, err
		}
	}
	if fileExists(filepath.Join(root, ".plugin-kit-ai", "project.toml")) {
		return Manifest{}, nil, fmt.Errorf("unsupported project format for import: .plugin-kit-ai/project.toml is not supported; rewrite the project into the package standard layout")
	}
	prepared, err := prepareImportFromRoot(root, from, includeUserScope)
	if err != nil {
		return Manifest{}, prepared.Warnings, err
	}
	if err := writePreparedImport(root, prepared, force); err != nil {
		return prepared.Manifest, prepared.Warnings, err
	}
	return prepared.Manifest, prepared.Warnings, nil
}

func prepareImportFromRoot(root string, from string, includeUserScope bool) (preparedImport, error) {
	explicitFrom := strings.TrimSpace(from) != ""
	from = normalizeTarget(from)
	matches := platformexec.DetectImport(root)
	detectedKinds := make([]string, 0, len(matches))
	for _, match := range matches {
		detectedKinds = append(detectedKinds, match.ID())
	}
	if from == "" {
		switch {
		case len(matches) == 0:
			from = ""
		case len(matches) == 1:
			from = matches[0].ID()
		default:
			var ids []string
			for _, match := range matches {
				ids = append(ids, match.ID())
			}
			return preparedImport{}, fmt.Errorf("ambiguous import source: detected multiple native layouts (%s); pass --from explicitly", strings.Join(ids, ", "))
		}
	}
	if explicitFrom && from == "codex" {
		return preparedImport{}, fmt.Errorf("unsupported import source %q", from)
	}
	if !isSupportedImportSource(from) {
		return preparedImport{}, fmt.Errorf("unsupported import source %q", from)
	}
	adapter, ok := platformexec.Lookup(from)
	if !ok {
		return preparedImport{}, fmt.Errorf("unsupported import source %q", from)
	}
	seed := platformexec.ImportSeed{
		Manifest:         defaultManifest(defaultName(root), from, inferRuntime(root), "plugin-kit-ai plugin"),
		Explicit:         explicitFrom,
		IncludeUserScope: includeUserScope,
	}
	if requiresLauncherForTarget(from) {
		launcher := defaultLauncher(defaultName(root), inferRuntime(root))
		seed.Launcher = &launcher
	}
	imported, err := adapter.Import(root, seed)
	if err != nil {
		return preparedImport{}, err
	}
	artifacts := append([]Artifact{}, imported.Artifacts...)
	if mcpArtifacts, err := importedPortableMCPArtifacts(root); err != nil {
		return preparedImport{Warnings: imported.Warnings}, err
	} else {
		artifacts = append(artifacts, mcpArtifacts...)
	}
	if fileExists(filepath.Join(root, ".mcp.json")) {
		imported.Warnings = append(imported.Warnings, Warning{
			Kind:    WarningFidelity,
			Path:    ".mcp.json",
			Message: "portable MCP will be preserved under plugin/mcp/servers.yaml",
		})
	}
	return preparedImport{
		Manifest:      imported.Manifest,
		Launcher:      imported.Launcher,
		Artifacts:     artifacts,
		Warnings:      imported.Warnings,
		ImportSource:  from,
		DetectedKinds: detectedKinds,
		DroppedKinds:  uniqueSortedKinds(imported.DroppedKinds),
	}, nil
}

func writePreparedImport(root string, prepared preparedImport, force bool) error {
	layout := authoredLayout{RootRel: pluginmodel.SourceDirName}
	if err := saveManifestWithLayout(root, layout, prepared.Manifest, force); err != nil {
		return err
	}
	if prepared.Launcher != nil {
		if err := saveLauncherWithLayout(root, layout, *prepared.Launcher, force); err != nil {
			return err
		}
	}
	artifacts := prefixAuthoredArtifacts(prepared.Artifacts, layout)
	return writeArtifacts(root, artifacts)
}
