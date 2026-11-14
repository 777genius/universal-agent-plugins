package gemini

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/portablemcp"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) copyNativeGeminiPackage(sourceRoot, destRoot string) error {
	if err := copyFile(filepath.Join(sourceRoot, "gemini-extension.json"), filepath.Join(destRoot, "gemini-extension.json")); err != nil {
		return domain.NewError(domain.ErrMutationApply, "copy Gemini manifest", err)
	}
	manifestBody, err := os.ReadFile(filepath.Join(sourceRoot, "gemini-extension.json"))
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "read Gemini manifest", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		return domain.NewError(domain.ErrMutationApply, "parse Gemini manifest", err)
	}
	contextName, _ := manifest["contextFileName"].(string)
	contextName = strings.TrimSpace(filepath.Base(contextName))
	if contextName == "" && fileExists(filepath.Join(sourceRoot, "GEMINI.md")) {
		contextName = "GEMINI.md"
	}
	if contextName != "" && fileExists(filepath.Join(sourceRoot, contextName)) {
		if err := copyFile(filepath.Join(sourceRoot, contextName), filepath.Join(destRoot, contextName)); err != nil {
			return domain.NewError(domain.ErrMutationApply, "copy Gemini primary context", err)
		}
	}
	for _, dir := range []string{"commands", "contexts", "hooks", "policies", "skills", "agents"} {
		if err := copyDirIfExists(filepath.Join(sourceRoot, dir), filepath.Join(destRoot, dir)); err != nil {
			return domain.NewError(domain.ErrMutationApply, "copy Gemini package directory "+dir, err)
		}
	}
	return nil
}

func (a Adapter) loadProjectedMCP(ctx context.Context, sourceRoot string) (map[string]any, error) {
	loader := portablemcp.Loader{FS: a.fs()}
	loaded, err := loader.LoadForTarget(ctx, sourceRoot, domain.TargetGemini)
	if err != nil {
		if derr, ok := err.(*domain.Error); ok && derr.Code == domain.ErrManifestLoad && strings.Contains(strings.ToLower(derr.Message), "portable mcp file not found") {
			return nil, nil
		}
		return nil, err
	}
	out := make(map[string]any, len(loaded.Servers))
	aliases := make([]string, 0, len(loaded.Servers))
	for alias := range loaded.Servers {
		aliases = append(aliases, alias)
	}
	slices.Sort(aliases)
	for _, alias := range aliases {
		server := loaded.Servers[alias]
		switch server.Type {
		case "stdio":
			doc := map[string]any{
				"command": replaceGeminiPackageRoot(server.Stdio.Command, sourceRoot),
			}
			if len(server.Stdio.Args) > 0 {
				args := make([]string, 0, len(server.Stdio.Args))
				for _, arg := range server.Stdio.Args {
					args = append(args, replaceGeminiPackageRoot(arg, sourceRoot))
				}
				doc["args"] = args
			}
			if len(server.Stdio.Env) > 0 {
				env := map[string]string{}
				for key, value := range server.Stdio.Env {
					env[key] = replaceGeminiPackageRoot(value, sourceRoot)
				}
				doc["env"] = env
			}
			out[alias] = doc
		case "remote":
			doc := map[string]any{}
			switch strings.ToLower(strings.TrimSpace(server.Remote.Protocol)) {
			case "streamable_http":
				doc["httpUrl"] = replaceGeminiPackageRoot(server.Remote.URL, sourceRoot)
			default:
				doc["url"] = replaceGeminiPackageRoot(server.Remote.URL, sourceRoot)
			}
			if len(server.Remote.Headers) > 0 {
				headers := map[string]string{}
				for key, value := range server.Remote.Headers {
					headers[key] = replaceGeminiPackageRoot(value, sourceRoot)
				}
				doc["headers"] = headers
			}
			out[alias] = doc
		default:
			return nil, domain.NewError(domain.ErrMutationApply, "unsupported Gemini portable MCP server type "+server.Type, nil)
		}
	}
	return out, nil
}

func replaceGeminiPackageRoot(value, sourceRoot string) string {
	value = strings.ReplaceAll(value, "${package.root}", sourceRoot)
	return strings.ReplaceAll(value, "${extensionPath}", sourceRoot)
}
