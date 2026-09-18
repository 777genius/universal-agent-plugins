package contracttest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const managedPackageDirectoryKind = "managed_package_directory"

// RunProjector asserts the staging half of the contract. A projector writes
// only inside the staging tree it was given, never claims the generic
// managed_package_directory object, and is deterministic: a second Project on
// a clean copy of the same tree returns the same objects.
func RunProjector(t *testing.T, adapter clients.Adapter) {
	t.Helper()
	RunAdapter(t, adapter)
	for _, violation := range projectorViolations(t, adapter) {
		t.Error(violation)
	}
}

func projectorViolations(t *testing.T, adapter clients.Adapter) []string {
	t.Helper()
	projector, ok := adapter.(clients.Projector)
	if !ok {
		return []string{"adapter does not implement clients.Projector"}
	}
	root := t.TempDir()
	staging := filepath.Join(root, "staging")
	if err := seedProjectorStaging(staging); err != nil {
		return []string{fmt.Sprintf("seed staging tree: %v", err)}
	}
	outside := filepath.Join(root, "outside-marker")
	if err := os.WriteFile(outside, []byte("keep\n"), 0o644); err != nil {
		return []string{fmt.Sprintf("write outside marker: %v", err)}
	}
	input := projectorInput(adapter.ID(), root, staging)
	before := listPaths(root)
	first, err := projector.Project(context.Background(), input)
	if err != nil {
		return []string{fmt.Sprintf("project: %v", err)}
	}
	violations := projectorObjectViolations(first)
	violations = append(violations, projectorWriteViolations(root, staging, outside, before)...)

	reset := filepath.Join(root, "staging-reset")
	if err := seedProjectorStaging(reset); err != nil {
		return append(violations, fmt.Sprintf("seed reset staging tree: %v", err))
	}
	input.StagingPath = reset
	second, err := projector.Project(context.Background(), input)
	if err != nil {
		return append(violations, fmt.Sprintf("project on a clean staging tree: %v", err))
	}
	if !reflect.DeepEqual(first, second) {
		violations = append(violations, "projecting twice on a clean staging tree returned different objects")
	}
	return violations
}

func projectorObjectViolations(objects []domain.NativeObjectOwnership) []string {
	violations := []string{}
	for _, object := range objects {
		if object.Kind == managedPackageDirectoryKind {
			violations = append(violations, "projector returned a managed_package_directory object; that object belongs to the generic stager")
		}
	}
	return violations
}

func projectorWriteViolations(root, staging, outside string, before map[string]struct{}) []string {
	violations := []string{}
	after := listPaths(root)
	for path := range after {
		if _, known := before[path]; known {
			continue
		}
		if containedBy(staging, path) {
			continue
		}
		violations = append(violations, fmt.Sprintf("projector wrote %q outside StagingPath", path))
	}
	body, err := os.ReadFile(outside)
	if err != nil || string(body) != "keep\n" {
		violations = append(violations, "projector mutated a file outside StagingPath")
	}
	return violations
}

func containedBy(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func listPaths(root string) map[string]struct{} {
	paths := map[string]struct{}{}
	_ = filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths[path] = struct{}{}
		return nil
	})
	return paths
}

func projectorInput(id domain.ClientID, root, staging string) clients.ProjectionInput {
	anchor := filepath.Join(root, "anchor")
	target := filepath.Join(anchor, "root")
	active := filepath.Join(target, "demo-0123456789ab")
	config := filepath.Join(root, "config")
	data := filepath.Join(root, "plugin-data")
	_ = os.MkdirAll(target, 0o700)
	_ = os.MkdirAll(config, 0o700)
	_ = os.MkdirAll(data, 0o700)
	return clients.ProjectionInput{
		StagingPath: staging,
		Envelope:    projectorEnvelope(),
		Plan: domain.DeliveryPlan{
			ClientID: id, Scope: domain.ScopeUser, Status: domain.PlanReady,
			PackageMode: domain.PackageProjection, Activation: domain.ActivationPrepared,
			PhysicalArtifactID: "demo-0123456789ab",
			DeclaredName:       "demo", DeclaredVersion: "1.0.0",
			NativeRegistryRoot: config,
			TargetAnchor:       anchor, TargetRoot: target, ActivePath: active,
			Components: []domain.ComponentDecision{
				{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportProjected},
				{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportProjected},
			},
		},
		PluginDataPath: data,
	}
}

func projectorEnvelope() domain.PackageEnvelope {
	raw := []byte(`{"type":"streamable-http","url":"https://example.test/mcp"}`)
	return domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0"},
		Skills:   map[string]domain.Skill{"docs": {Name: "docs", RelativePath: "skills/docs/SKILL.md"}},
		MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"local": {
				Name: "local", Type: "streamable-http", Raw: raw,
				Decoded: map[string]any{"type": "streamable-http", "url": "https://example.test/mcp"},
			},
		}},
	}
}

func seedProjectorStaging(staging string) error {
	files := map[string]string{
		"plugin.json":          `{"name":"demo","version":"1.0.0"}`,
		"mcp.json":             `{"mcpServers":{"local":{"type":"streamable-http","url":"https://example.test/mcp"}}}`,
		"skills/docs/SKILL.md": "---\nname: docs\ndescription: Docs\n---\nUse it.\n",
	}
	for rel, body := range files {
		path := filepath.Join(staging, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}
