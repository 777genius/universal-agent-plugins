package shared

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// MCPDialect is the client-facing shape of a projected MCP selection document.
type MCPDialect uint8

const (
	// MCPDialectOpenAI writes .mcp.json, rewrites transport types to the OpenAI
	// spelling, applies the catalog auth hints, and removes a stale document
	// when nothing is selected.
	MCPDialectOpenAI MCPDialect = iota
	// MCPDialectCursor writes mcp.json with the same transport rewriting and no
	// auth hints.
	MCPDialectCursor
	// MCPDialectKiro writes a schema-tagged mcp.json and passes every entry
	// through as authored: it reads the authored transport members directly.
	MCPDialectKiro
)

type mcpDialectRules struct {
	file         string
	label        string
	schema       string
	rewriteTypes bool
	applyHints   bool
	// removeLabel is set for the dialects that delete a stale document when
	// nothing is selected; empty means an empty selection writes nothing.
	removeLabel string
}

func (dialect MCPDialect) rules() (mcpDialectRules, error) {
	switch dialect {
	case MCPDialectOpenAI:
		return mcpDialectRules{file: ".mcp.json", label: "stdio MCP server", rewriteTypes: true, applyHints: true, removeLabel: "remove empty OpenAI MCP projection"}, nil
	case MCPDialectCursor:
		return mcpDialectRules{file: "mcp.json", label: "project Cursor stdio MCP server", rewriteTypes: true}, nil
	case MCPDialectKiro:
		return mcpDialectRules{file: "mcp.json", label: "project Kiro stdio MCP server", schema: domain.MCPSchemaV1}, nil
	default:
		return mcpDialectRules{}, fmt.Errorf("unknown MCP projection dialect %d", dialect)
	}
}

// MCPProjection is one request to project the selected MCP servers into a
// staged tree. Root is the tree being written; PluginRoot is the future active
// path the entries have to encode, and the two are deliberately different.
type MCPProjection struct {
	Root       string
	Envelope   domain.PackageEnvelope
	Names      []string
	Dialect    MCPDialect
	PluginRoot string
	DataPath   string
	Hints      domain.CompatibilityHints
}

// ProjectMCPServers writes the selected MCP servers in one client's dialect.
func ProjectMCPServers(in MCPProjection) error {
	rules, err := in.Dialect.rules()
	if err != nil {
		return err
	}
	target := filepath.Join(in.Root, rules.file)
	if len(in.Names) == 0 {
		if rules.removeLabel == "" {
			return nil
		}
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%s: %w", rules.removeLabel, err)
		}
		return nil
	}
	servers := make(map[string]map[string]any, len(in.Names))
	for _, name := range in.Names {
		config, included, err := in.projectServer(rules, name)
		if err != nil {
			return err
		}
		if included {
			servers[name] = config
		}
	}
	document := map[string]any{"mcpServers": servers}
	if rules.schema != "" {
		document["$schema"] = rules.schema
	}
	return WriteJSON(target, document)
}

func (in MCPProjection) projectServer(rules mcpDialectRules, name string) (map[string]any, bool, error) {
	server := in.Envelope.MCP.Servers[name]
	config := CloneObject(server.Decoded)
	if !rules.rewriteTypes {
		if server.Type == "stdio" {
			if err := ApplyStdioDataContract(config, in.PluginRoot, in.DataPath, in.Root); err != nil {
				return nil, false, fmt.Errorf("%s %s: %w", rules.label, name, err)
			}
		}
		return config, true, nil
	}
	switch server.Type {
	case "stdio":
		delete(config, "type")
		if err := ApplyStdioDataContract(config, in.PluginRoot, in.DataPath, in.Root); err != nil {
			return nil, false, fmt.Errorf("%s %s: %w", rules.label, name, err)
		}
	case "streamable-http":
		config["type"] = "http"
	case "sse":
		config["type"] = "sse"
	default:
		return nil, false, nil
	}
	if rules.applyHints {
		applyOpenAIMCPAuthHint(config, in.Hints, name)
	}
	return config, true, nil
}

func applyOpenAIMCPAuthHint(config map[string]any, hints domain.CompatibilityHints, name string) {
	hint, ok := hints.OpenAIMCPAuth[name]
	if !ok {
		return
	}
	if hint.OAuthResource != "" {
		config["oauth_resource"] = hint.OAuthResource
	}
	if hint.BearerTokenEnvVar != "" {
		config["bearer_token_env_var"] = hint.BearerTokenEnvVar
	}
}
