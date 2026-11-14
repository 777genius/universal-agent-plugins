package cursor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/portablemcp"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func renderCursorServers(loaded portablemcp.Loaded, packageRoot string) (map[string]any, []string, error) {
	projected := make(map[string]any, len(loaded.Servers))
	aliases := make([]string, 0, len(loaded.Servers))
	for alias, server := range loaded.Servers {
		switch server.Type {
		case "stdio":
			item := map[string]any{
				"command": interpolatePackageRoot(server.Stdio.Command, packageRoot),
			}
			if len(server.Stdio.Args) > 0 {
				args := make([]any, 0, len(server.Stdio.Args))
				for _, arg := range server.Stdio.Args {
					args = append(args, interpolatePackageRoot(arg, packageRoot))
				}
				item["args"] = args
			}
			if len(server.Stdio.Env) > 0 {
				env := make(map[string]any, len(server.Stdio.Env))
				for key, value := range server.Stdio.Env {
					env[key] = interpolatePackageRoot(value, packageRoot)
				}
				item["env"] = env
			}
			projected[alias] = item
		case "remote":
			item := map[string]any{
				"url": interpolatePackageRoot(server.Remote.URL, packageRoot),
			}
			if len(server.Remote.Headers) > 0 {
				headers := make(map[string]any, len(server.Remote.Headers))
				for key, value := range server.Remote.Headers {
					headers[key] = interpolatePackageRoot(value, packageRoot)
				}
				item["headers"] = headers
			}
			projected[alias] = item
		default:
			return nil, nil, domain.NewError(domain.ErrUnsupportedTarget, "unsupported Cursor portable MCP server type "+server.Type, nil)
		}
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return projected, aliases, nil
}

func interpolatePackageRoot(value, packageRoot string) string {
	return strings.ReplaceAll(value, "${package.root}", packageRoot)
}

func mergeServers(existing, owned map[string]any) map[string]any {
	out := make(map[string]any, len(existing)+len(owned))
	for key, value := range existing {
		out[key] = value
	}
	for key, value := range owned {
		out[key] = value
	}
	return out
}

func marshalCursorDocument(servers map[string]any, wrapped bool) ([]byte, error) {
	var body []byte
	var err error
	if wrapped {
		body, err = json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
	} else {
		body, err = json.MarshalIndent(servers, "", "  ")
	}
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func (a Adapter) readDocument(ctx context.Context, path string) (map[string]any, bool, []byte, error) {
	body, err := a.fs().ReadFile(ctx, path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, true, nil, nil
		}
		return nil, false, nil, domain.NewError(domain.ErrMutationApply, "read Cursor MCP config", err)
	}
	doc, wrapped, err := a.readDocumentBytes(body)
	if err != nil {
		return nil, false, nil, err
	}
	return doc, wrapped, body, nil
}

func (a Adapter) readDocumentBytes(body []byte) (map[string]any, bool, error) {
	doc := map[string]any{}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, false, domain.NewError(domain.ErrMutationApply, "parse Cursor MCP config", err)
	}
	if raw, ok := doc["mcpServers"]; ok {
		servers, ok := raw.(map[string]any)
		if !ok {
			return nil, false, domain.NewError(domain.ErrMutationApply, "Cursor mcpServers must be a JSON object", nil)
		}
		return servers, true, nil
	}
	return doc, false, nil
}

func (a Adapter) verifyAliases(ctx context.Context, path string, aliases []string) error {
	doc, _, _, err := a.readDocument(ctx, path)
	if err != nil {
		return err
	}
	for _, alias := range aliases {
		if _, ok := doc[alias]; !ok {
			return domain.NewError(domain.ErrMutationApply, fmt.Sprintf("Cursor MCP alias %q was not persisted", alias), nil)
		}
	}
	return nil
}

func (a Adapter) verifyMissingAliases(ctx context.Context, path string, aliases []string) error {
	doc, _, _, err := a.readDocument(ctx, path)
	if err != nil {
		return err
	}
	for _, alias := range aliases {
		if _, ok := doc[alias]; ok {
			return domain.NewError(domain.ErrMutationApply, fmt.Sprintf("Cursor MCP alias %q still exists after removal", alias), nil)
		}
	}
	return nil
}
