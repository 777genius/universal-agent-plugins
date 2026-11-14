package cursor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

const (
	cursorPluginManifestRelPath = ".cursor-plugin/plugin.json"
	cursorPluginRootKind        = "cursor_plugin_root"
)

func (a Adapter) targetPluginRoot(integrationID string) string {
	return filepath.Join(a.userHome(), ".cursor", "plugins", "local", integrationID)
}

func (a Adapter) validatePluginRoot(integrationID, pluginRoot string) error {
	if err := pathpolicy.ValidateLeafID(integrationID); err != nil {
		return fmt.Errorf("invalid Cursor integration id: %w", err)
	}
	if err := pathpolicy.RequireExactPath(a.targetPluginRoot(integrationID), pluginRoot); err != nil {
		return fmt.Errorf("unsafe Cursor plugin root: %w", err)
	}
	return nil
}

func shouldUsePluginPackage(manifest domain.IntegrationManifest, sourceRoot string) bool {
	if cursorPluginPackageExists(sourceRoot) {
		return true
	}
	for _, delivery := range manifest.Deliveries {
		if delivery.TargetID == domain.TargetCursor && delivery.DeliveryKind == domain.DeliveryCursorPlugin {
			return true
		}
	}
	return false
}

func cursorPluginPackageExists(sourceRoot string) bool {
	if strings.TrimSpace(sourceRoot) == "" {
		return false
	}
	return pathpolicy.FileExists(filepath.Join(sourceRoot, cursorPluginManifestRelPath))
}

func (a Adapter) syncManagedPlugin(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot, pluginRoot string) error {
	_ = ctx
	_ = manifest
	if err := a.validatePluginRoot(manifest.IntegrationID, pluginRoot); err != nil {
		return domain.NewError(domain.ErrMutationApply, "validate Cursor managed plugin root", err)
	}
	root := filepath.Clean(sourceRoot)
	if !cursorPluginPackageExists(root) {
		return domain.NewError(domain.ErrMutationApply, "Cursor plugin package requires .cursor-plugin/plugin.json", nil)
	}
	parent := filepath.Dir(pluginRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return domain.NewError(domain.ErrMutationApply, "prepare Cursor plugin parent", err)
	}
	tmpRoot, err := os.MkdirTemp(parent, filepath.Base(pluginRoot)+".tmp-*")
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "create Cursor materialization temp root", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpRoot)
		}
	}()
	if err := copyNativeCursorPackage(root, tmpRoot); err != nil {
		return err
	}
	if err := copyCursorManifestRefs(root, tmpRoot); err != nil {
		return err
	}
	if err := rewriteCursorPackageMCP(filepath.Join(tmpRoot, ".mcp.json"), root); err != nil {
		return err
	}
	if !cursorPluginPackageExists(tmpRoot) {
		return domain.NewError(domain.ErrMutationApply, "Cursor package copy did not produce .cursor-plugin/plugin.json", nil)
	}
	if err := os.RemoveAll(pluginRoot); err != nil && !os.IsNotExist(err) {
		return domain.NewError(domain.ErrMutationApply, "replace Cursor managed plugin root", err)
	}
	if err := os.Rename(tmpRoot, pluginRoot); err != nil {
		return domain.NewError(domain.ErrMutationApply, "activate Cursor managed plugin root", err)
	}
	cleanup = false
	return nil
}

func copyCursorManifestRefs(sourceRoot, destRoot string) error {
	body, err := os.ReadFile(filepath.Join(sourceRoot, cursorPluginManifestRelPath))
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "read Cursor plugin manifest", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		return domain.NewError(domain.ErrMutationApply, "parse Cursor plugin manifest", err)
	}
	keys := []string{"logo", "commands", "agents", "skills", "rules", "hooks", "mcpServers"}
	for _, key := range keys {
		for _, ref := range cursorManifestStringRefs(manifest[key]) {
			if err := copyCursorRelativeRef(sourceRoot, destRoot, ref); err != nil {
				return err
			}
		}
	}
	return nil
}

func cursorManifestStringRefs(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if ref, ok := item.(string); ok {
				out = append(out, ref)
			}
		}
		return out
	default:
		return nil
	}
}

func copyCursorRelativeRef(sourceRoot, destRoot, ref string) error {
	rel, ok, err := safeCursorRelativeRef(ref)
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "validate Cursor manifest path", err)
	}
	if !ok {
		return nil
	}
	if strings.ContainsAny(rel, "*?[") {
		matches, err := filepath.Glob(filepath.Join(sourceRoot, rel))
		if err != nil {
			return domain.NewError(domain.ErrMutationApply, "expand Cursor manifest glob", err)
		}
		if len(matches) == 0 {
			return domain.NewError(domain.ErrMutationApply, fmt.Sprintf("Cursor manifest glob %q matched no files", ref), nil)
		}
		for _, match := range matches {
			if err := copyCursorMatchedPath(sourceRoot, destRoot, match); err != nil {
				return err
			}
		}
		return nil
	}
	copied, err := copyPathIfExists(filepath.Join(sourceRoot, rel), filepath.Join(destRoot, rel))
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Cursor manifest path", err)
	}
	if !copied {
		return domain.NewError(domain.ErrMutationApply, fmt.Sprintf("Cursor manifest path %q does not exist", ref), nil)
	}
	return nil
}

func copyCursorMatchedPath(sourceRoot, destRoot, match string) error {
	rel, err := filepath.Rel(sourceRoot, match)
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "resolve Cursor manifest glob match", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return domain.NewError(domain.ErrMutationApply, "Cursor manifest glob escaped plugin root", nil)
	}
	if _, err := copyPathIfExists(match, filepath.Join(destRoot, rel)); err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Cursor manifest glob match", err)
	}
	return nil
}

func safeCursorRelativeRef(ref string) (string, bool, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "://") || strings.HasPrefix(ref, "data:") || filepath.IsAbs(ref) {
		return "", false, nil
	}
	ref = strings.TrimPrefix(ref, "./")
	rel := filepath.Clean(filepath.FromSlash(ref))
	if rel == "." {
		return "", false, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false, fmt.Errorf("path %q escapes plugin root", ref)
	}
	return rel, true, nil
}

func copyNativeCursorPackage(sourceRoot, destRoot string) error {
	for _, rel := range []string{
		".cursor-plugin",
		".mcp.json",
		"mcp.json",
		"skills",
		"rules",
		"agents",
		"commands",
		"hooks",
		"assets",
	} {
		if _, err := copyPathIfExists(filepath.Join(sourceRoot, rel), filepath.Join(destRoot, rel)); err != nil {
			return domain.NewError(domain.ErrMutationApply, "copy native Cursor package", err)
		}
	}
	return nil
}

func rewriteCursorPackageMCP(path, sourceRoot string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return domain.NewError(domain.ErrMutationApply, "read Cursor package MCP config", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return domain.NewError(domain.ErrMutationApply, "parse Cursor package MCP config", err)
	}
	servers, wrapped := doc["mcpServers"].(map[string]any)
	if !wrapped {
		servers = doc
	}
	for _, raw := range servers {
		server, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if command, ok := server["command"].(string); ok {
			server["command"] = rewriteCursorPackagePath(command, sourceRoot, true)
		}
		if args, ok := server["args"].([]any); ok {
			for index, rawArg := range args {
				if arg, ok := rawArg.(string); ok {
					args[index] = rewriteCursorPackagePath(arg, sourceRoot, false)
				}
			}
		}
		if env, ok := server["env"].(map[string]any); ok {
			for key, rawValue := range env {
				if value, ok := rawValue.(string); ok {
					env[key] = rewriteCursorPackagePath(value, sourceRoot, false)
				}
			}
		}
	}
	next := doc
	if wrapped {
		next = map[string]any{"mcpServers": servers}
	}
	rendered, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "marshal Cursor package MCP config", err)
	}
	if err := os.WriteFile(path, append(rendered, '\n'), 0o644); err != nil {
		return domain.NewError(domain.ErrMutationApply, "write Cursor package MCP config", err)
	}
	return nil
}

func rewriteCursorPackagePath(value, sourceRoot string, rewriteRelative bool) string {
	value = strings.ReplaceAll(value, "${package.root}", sourceRoot)
	value = strings.ReplaceAll(value, "${extensionPath}", sourceRoot)
	if rewriteRelative && strings.HasPrefix(value, "./") {
		return filepath.Join(sourceRoot, strings.TrimPrefix(value, "./"))
	}
	return value
}

func copyPathIfExists(src, dest string) (bool, error) {
	return filetree.CopyPathIfExists(src, dest)
}

func copyDir(src, dest string) error {
	return filetree.CopyDir(src, dest)
}

func copyFile(src, dest string) error {
	return filetree.CopyFile(src, dest)
}
