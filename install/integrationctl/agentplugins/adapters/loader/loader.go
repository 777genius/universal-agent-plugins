package loader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type Loader struct {
	Registry ports.SchemaRegistry
}

// LoadSnapshot is the standard acquisition-to-loading boundary. It carries
// the exact sealed root, digest, executable modes, and source identity as one
// value so later planning never rereads the mutable source.
func (loader Loader) LoadSnapshot(ctx context.Context, snapshot domain.PackageSnapshot) (domain.PackageEnvelope, error) {
	if snapshot.DigestAlgorithm != domain.TreeDigestAlgorithm || !validSHA256Digest(snapshot.TreeDigest) {
		return domain.PackageEnvelope{}, domain.FatalLoad("snapshot_digest_invalid", "", "package snapshot does not use the supported versioned tree digest", nil)
	}
	return loader.Load(ctx, domain.LoadInput{SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: snapshot.Source})
}

func validSHA256Digest(value string) bool {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+sha256.Size*2 {
		return false
	}
	encoded := strings.TrimPrefix(value, prefix)
	if encoded != strings.ToLower(encoded) {
		return false
	}
	_, err := hex.DecodeString(encoded)
	return err == nil
}

func (loader Loader) Load(ctx context.Context, input domain.LoadInput) (domain.PackageEnvelope, error) {
	if err := ctx.Err(); err != nil {
		return domain.PackageEnvelope{}, err
	}
	if loader.Registry == nil {
		return domain.PackageEnvelope{}, domain.FatalLoad("schema_registry_missing", "plugin.json", "Agent Plugins schema registry is required", nil)
	}
	root, err := filepath.Abs(input.SnapshotRoot)
	if err != nil {
		return domain.PackageEnvelope{}, domain.FatalLoad("snapshot_invalid", "plugin.json", "resolve package snapshot root", err)
	}
	root = filepath.Clean(root)
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return domain.PackageEnvelope{}, domain.FatalLoad("snapshot_invalid", "plugin.json", "package snapshot root must be a real directory", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return domain.PackageEnvelope{}, domain.FatalLoad("snapshot_invalid", "plugin.json", "resolve package snapshot root", err)
	}
	root = filepath.Clean(resolvedRoot)
	manifestPath := filepath.Join(root, "plugin.json")
	formatID := domain.FormatIDAgentPluginsV1
	schemaVersion := "1.0.0"
	mcpPath := filepath.Join(root, "mcp.json")
	skillsPath := filepath.Join(root, "skills")
	manifest, diagnostics, manifestDigest, err := loader.loadPluginManifest(manifestPath)
	if err != nil {
		return domain.PackageEnvelope{}, err
	}
	mcp, mcpDiagnostics := loader.loadMCP(mcpPath, manifest.SchemaURI)
	diagnostics = append(diagnostics, mcpDiagnostics...)
	var skills map[string]domain.Skill
	var invalidSkills []string
	var invalidSkillsRoot bool
	var skillDiagnostics []domain.Diagnostic
	if skillsPath != "" {
		skills, invalidSkills, invalidSkillsRoot, skillDiagnostics = loadSkills(skillsPath)
	}
	diagnostics = append(diagnostics, skillDiagnostics...)

	inventory := domain.ComponentInventory{
		MCPPresent:        mcp.Present,
		MCPEnabled:        mcp.Enabled,
		MCPServers:        sortedMCPServerNames(mcp.Servers),
		Skills:            sortedSkillNames(skills),
		InvalidSkills:     invalidSkills,
		InvalidSkillsRoot: invalidSkillsRoot,
		Extensions:        sortedRawKeys(manifest.Extensions),
	}
	for name := range mcp.InvalidServer {
		inventory.InvalidMCPServer = append(inventory.InvalidMCPServer, name)
	}
	sort.Strings(inventory.InvalidMCPServer)

	executableFiles := append([]string(nil), input.ExecutableFiles...)
	sort.Strings(executableFiles)
	return domain.PackageEnvelope{
		LoaderKind:      domain.LoaderKindAgentPlugins,
		FormatID:        formatID,
		SchemaURI:       manifest.SchemaURI,
		SchemaVersion:   schemaVersion,
		ManifestSchema:  domain.SchemaIdentity{URI: manifest.SchemaURI, Version: schemaVersion},
		Manifest:        manifest,
		MCP:             mcp,
		Skills:          skills,
		Inventory:       inventory,
		Diagnostics:     diagnostics,
		Source:          input.Source,
		TreeDigest:      strings.TrimSpace(input.TreeDigest),
		ManifestDigest:  manifestDigest,
		ExecutableFiles: executableFiles,
		SnapshotRoot:    root,
	}, nil
}

func rejectDiscoverableHooks(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("inspect package root for auto-discovered hooks: %w", err)
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), "hooks") {
			return fmt.Errorf("package root contains auto-discoverable hooks path %q", entry.Name())
		}
	}
	return nil
}

func readRegularFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, true, fmt.Errorf("path is not a regular file")
	}
	body, err := os.ReadFile(path)
	return body, true, err
}

func sha256Digest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func sortedRawKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedMCPServerNames(values map[string]domain.MCPServer) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedAppBindingNames(values map[string]domain.AppBinding) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSkillNames(values map[string]domain.Skill) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
