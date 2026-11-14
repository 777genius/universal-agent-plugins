package codex

import (
	"context"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) buildAuthoredCodexManifest(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot, destRoot string) (map[string]any, error) {
	doc := newManagedCodexManifestDoc(manifest)
	if err := applyCodexPackageMeta(doc, authoredpath.Join(sourceRoot, "targets", "codex-package", "package.yaml")); err != nil {
		return nil, err
	}
	if err := copyCodexSkills(doc, sourceRoot, destRoot); err != nil {
		return nil, err
	}
	if err := addCodexInterfaceDoc(doc, authoredpath.Join(sourceRoot, "targets", "codex-package", "interface.json")); err != nil {
		return nil, err
	}
	if err := addCodexAppManifest(doc, sourceRoot, destRoot); err != nil {
		return nil, err
	}
	if err := a.addCodexMCPManifest(ctx, doc, sourceRoot, destRoot); err != nil {
		return nil, err
	}
	if err := mergeCodexManifestExtra(doc, authoredpath.Join(sourceRoot, "targets", "codex-package", "manifest.extra.json")); err != nil {
		return nil, err
	}
	return doc, nil
}

func newManagedCodexManifestDoc(manifest domain.IntegrationManifest) map[string]any {
	return map[string]any{
		"name":        manifest.IntegrationID,
		"version":     manifest.Version,
		"description": manifest.Description,
	}
}

func copyCodexSkills(doc map[string]any, sourceRoot, destRoot string) error {
	hasSkills, err := copyPathIfExists(authoredpath.Join(sourceRoot, "skills"), filepath.Join(destRoot, "skills"))
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Codex skills", err)
	}
	if hasSkills {
		doc["skills"] = "./skills/"
	}
	return nil
}

func addCodexInterfaceDoc(doc map[string]any, path string) error {
	if !fileExists(path) {
		return nil
	}
	value, err := readAnyJSON(path)
	if err != nil {
		return err
	}
	doc["interface"] = value
	return nil
}

func addCodexAppManifest(doc map[string]any, sourceRoot, destRoot string) error {
	src := authoredpath.Join(sourceRoot, "targets", "codex-package", "app.json")
	if !fileExists(src) {
		return nil
	}
	if err := copyFile(src, filepath.Join(destRoot, ".app.json")); err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Codex app manifest", err)
	}
	doc["apps"] = "./.app.json"
	return nil
}

func (a Adapter) addCodexMCPManifest(ctx context.Context, doc map[string]any, sourceRoot, destRoot string) error {
	mcp, err := a.renderCodexMCP(ctx, sourceRoot)
	if err != nil {
		return err
	}
	if len(mcp) == 0 {
		return nil
	}
	body, err := marshalJSON(mcp)
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "marshal Codex MCP config", err)
	}
	if err := os.WriteFile(filepath.Join(destRoot, ".mcp.json"), body, 0o644); err != nil {
		return domain.NewError(domain.ErrMutationApply, "write Codex MCP config", err)
	}
	doc["mcpServers"] = "./.mcp.json"
	return nil
}

func writeCodexPluginManifest(destRoot string, doc map[string]any) error {
	body, err := marshalJSON(doc)
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "marshal Codex plugin manifest", err)
	}
	if err := os.MkdirAll(filepath.Join(destRoot, ".codex-plugin"), 0o755); err != nil {
		return domain.NewError(domain.ErrMutationApply, "prepare Codex plugin manifest dir", err)
	}
	if err := os.WriteFile(filepath.Join(destRoot, ".codex-plugin", "plugin.json"), body, 0o644); err != nil {
		return domain.NewError(domain.ErrMutationApply, "write Codex plugin manifest", err)
	}
	return nil
}
