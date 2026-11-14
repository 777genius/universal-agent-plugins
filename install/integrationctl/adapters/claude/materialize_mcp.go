package claude

import (
	"context"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/portablemcp"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) renderClaudeMCP(ctx context.Context, sourceRoot string) (map[string]any, error) {
	loader := portablemcp.Loader{FS: a.fs()}
	loaded, err := loader.LoadForTarget(ctx, sourceRoot, domain.TargetClaude)
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
			doc := map[string]any{"command": replaceClaudePackageRoot(server.Stdio.Command, sourceRoot)}
			if len(server.Stdio.Args) > 0 {
				args := make([]string, 0, len(server.Stdio.Args))
				for _, arg := range server.Stdio.Args {
					args = append(args, replaceClaudePackageRoot(arg, sourceRoot))
				}
				doc["args"] = args
			}
			if len(server.Stdio.Env) > 0 {
				env := map[string]string{}
				for key, value := range server.Stdio.Env {
					env[key] = replaceClaudePackageRoot(value, sourceRoot)
				}
				doc["env"] = env
			}
			out[alias] = doc
		case "remote":
			doc := map[string]any{"url": replaceClaudePackageRoot(server.Remote.URL, sourceRoot)}
			switch strings.ToLower(strings.TrimSpace(server.Remote.Protocol)) {
			case "streamable_http":
				doc["type"] = "http"
			default:
				doc["type"] = "sse"
			}
			if len(server.Remote.Headers) > 0 {
				headers := map[string]string{}
				for key, value := range server.Remote.Headers {
					headers[key] = replaceClaudePackageRoot(value, sourceRoot)
				}
				doc["headers"] = headers
			}
			out[alias] = doc
		default:
			return nil, domain.NewError(domain.ErrMutationApply, "unsupported Claude portable MCP server type "+server.Type, nil)
		}
	}
	return out, nil
}

func replaceClaudePackageRoot(value, sourceRoot string) string {
	value = strings.ReplaceAll(value, "${package.root}", sourceRoot)
	return strings.ReplaceAll(value, "${CLAUDE_PLUGIN_ROOT}", sourceRoot)
}
