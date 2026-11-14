package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) materializeAuthoredClaudeSource(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot, destRoot string) error {
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return domain.NewError(domain.ErrMutationApply, "prepare Claude materialized package root", err)
	}
	authoredRoot := authoredpath.Dir(sourceRoot)
	doc := map[string]any{
		"name":        manifest.IntegrationID,
		"version":     manifest.Version,
		"description": manifest.Description,
	}
	if hasSkills, err := copyPathIfExists(filepath.Join(authoredRoot, "skills"), filepath.Join(destRoot, "skills")); err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Claude skills", err)
	} else if hasSkills {
		doc["skills"] = "./skills/"
	}
	if hasAgents, err := copyPathIfExists(filepath.Join(authoredRoot, "targets", "claude", "agents"), filepath.Join(destRoot, "agents")); err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Claude agents", err)
	} else if hasAgents {
		doc["agents"] = "./agents/"
	}
	if fileExists(filepath.Join(authoredRoot, "targets", "claude", "settings.json")) {
		if err := copyFile(filepath.Join(authoredRoot, "targets", "claude", "settings.json"), filepath.Join(destRoot, "settings.json")); err != nil {
			return domain.NewError(domain.ErrMutationApply, "copy Claude settings", err)
		}
	}
	if fileExists(filepath.Join(authoredRoot, "targets", "claude", "lsp.json")) {
		if err := copyFile(filepath.Join(authoredRoot, "targets", "claude", "lsp.json"), filepath.Join(destRoot, ".lsp.json")); err != nil {
			return domain.NewError(domain.ErrMutationApply, "copy Claude LSP config", err)
		}
	}
	if fileExists(filepath.Join(authoredRoot, "targets", "claude", "user-config.json")) {
		raw, err := readJSONObject(filepath.Join(authoredRoot, "targets", "claude", "user-config.json"))
		if err != nil {
			return err
		}
		doc["userConfig"] = raw
	}
	if hasHooks, err := copyPathIfExists(filepath.Join(authoredRoot, "targets", "claude", "hooks"), filepath.Join(destRoot, "hooks")); err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Claude hooks", err)
	} else if !hasHooks && fileExists(filepath.Join(authoredRoot, "launcher.yaml")) {
		return domain.NewError(domain.ErrMutationApply, "Claude authored source with launcher.yaml requires generated native hooks or authored targets/claude/hooks", nil)
	}
	if _, err := copyPathIfExists(filepath.Join(authoredRoot, "targets", "claude", "commands"), filepath.Join(destRoot, "commands")); err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Claude commands", err)
	}
	if mcp, err := a.renderClaudeMCP(ctx, sourceRoot); err != nil {
		return err
	} else if len(mcp) > 0 {
		body, err := marshalJSON(mcp)
		if err != nil {
			return domain.NewError(domain.ErrMutationApply, "marshal Claude MCP config", err)
		}
		if err := os.WriteFile(filepath.Join(destRoot, ".mcp.json"), body, 0o644); err != nil {
			return domain.NewError(domain.ErrMutationApply, "write Claude MCP config", err)
		}
		doc["mcpServers"] = "./.mcp.json"
	}
	if err := mergeClaudeManifestExtra(doc, filepath.Join(authoredRoot, "targets", "claude", "manifest.extra.json")); err != nil {
		return err
	}
	body, err := marshalJSON(doc)
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "marshal Claude plugin manifest", err)
	}
	if err := os.MkdirAll(filepath.Join(destRoot, ".claude-plugin"), 0o755); err != nil {
		return domain.NewError(domain.ErrMutationApply, "prepare Claude plugin manifest dir", err)
	}
	if err := os.WriteFile(filepath.Join(destRoot, ".claude-plugin", "plugin.json"), body, 0o644); err != nil {
		return domain.NewError(domain.ErrMutationApply, "write Claude plugin manifest", err)
	}
	return nil
}

func mergeClaudeManifestExtra(doc map[string]any, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return domain.NewError(domain.ErrMutationApply, "read Claude manifest.extra.json", err)
	}
	var extra map[string]any
	if err := json.Unmarshal(body, &extra); err != nil {
		return domain.NewError(domain.ErrMutationApply, "parse Claude manifest.extra.json", err)
	}
	for key, value := range extra {
		switch strings.TrimSpace(key) {
		case "", "name", "version", "description", "mcpServers", "userConfig", "skills", "agents":
			return domain.NewError(domain.ErrMutationApply, "Claude manifest.extra.json may not override managed key "+key, nil)
		default:
			doc[key] = value
		}
	}
	return nil
}

func readJSONObject(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, domain.NewError(domain.ErrMutationApply, "read JSON object", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, domain.NewError(domain.ErrMutationApply, "parse JSON object", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}
