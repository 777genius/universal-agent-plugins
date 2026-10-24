package nativeconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/creachadair/tomledit"
	"github.com/pelletier/go-toml/v2"
	"github.com/tailscale/hujson"
)

// Only MCP entries enter the existing kernel's neutral mutation representation.
// The native TOML AST retains all other settings, types and comments.
type codexTOMLDocument struct {
	body     []byte
	ast      *tomledit.Document
	original map[string]json.RawMessage
	settings map[string]any
}

func parseCodexTOML(body []byte, jsonc bool) (*document, error) {
	if jsonc {
		return nil, fmt.Errorf("%w: Codex requires one TOML config path", ErrMalformed)
	}
	var native map[string]any
	if err := toml.Unmarshal(body, &native); err != nil {
		return nil, fmt.Errorf("%w: invalid Codex TOML", ErrMalformed)
	}
	if native == nil {
		native = map[string]any{}
	}
	servers, ok := native["mcp_servers"]
	if !ok {
		servers = map[string]any{}
	}
	if _, ok := servers.(map[string]any); !ok {
		return nil, fmt.Errorf("%w: mcp_servers must be a table", ErrMalformed)
	}
	neutral, err := json.Marshal(map[string]any{"mcp_servers": servers})
	if err != nil {
		return nil, fmt.Errorf("%w: invalid Codex MCP entry values", ErrMalformed)
	}
	doc, err := parseDocument(neutral, false)
	if err != nil {
		return nil, err
	}
	ast, err := tomledit.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid Codex TOML structure", ErrMalformed)
	}
	original, err := doc.codexEntries()
	if err != nil {
		return nil, err
	}
	delete(native, "mcp_servers")
	doc.codexTOML = &codexTOMLDocument{body: bytes.Clone(body), ast: ast, original: original, settings: native}
	return doc, nil
}

func (doc *document) codexEntries() (map[string]json.RawMessage, error) {
	entries, err := codecCollection(doc, CodecCodex, false)
	if err != nil || entries == nil {
		return nil, err
	}
	result := make(map[string]json.RawMessage, len(entries.Members))
	for i := range entries.Members {
		member := &entries.Members[i]
		body, err := entryCanonical(member)
		if err != nil {
			return nil, err
		}
		result[member.Name.Value.(hujson.Literal).String()] = body
	}
	return result, nil
}

func (doc *document) renderCodexTOML() ([]byte, error) {
	current, err := doc.codexEntries()
	if err != nil {
		return nil, err
	}
	names := codexChangedNames(doc.codexTOML.original, current)
	if len(names) == 0 {
		return bytes.Clone(doc.codexTOML.body), nil
	}
	for _, name := range names {
		removeCodexTOMLEntry(doc.codexTOML.ast, name)
		if body, exists := current[name]; exists {
			if err := setCodexTOMLEntry(doc.codexTOML.ast, name, body); err != nil {
				return nil, err
			}
		}
	}
	var result bytes.Buffer
	if err := tomledit.Format(&result, doc.codexTOML.ast); err != nil {
		return nil, fmt.Errorf("render Codex native config: %w", err)
	}
	next := result.Bytes()
	parsed, err := parseCodexTOML(next, false)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(doc.codexTOML.settings, parsed.codexTOML.settings) {
		return nil, fmt.Errorf("rendered Codex config changed foreign settings")
	}
	observed, err := parsed.codexEntries()
	if err != nil || !equalCodexEntries(current, observed) {
		return nil, fmt.Errorf("rendered Codex MCP entries differ from requested entries")
	}
	return next, nil
}

func codexChangedNames(previous, next map[string]json.RawMessage) []string {
	changed := map[string]bool{}
	for name, value := range previous {
		changed[name] = !bytes.Equal(value, next[name])
	}
	for name, value := range next {
		changed[name] = !bytes.Equal(value, previous[name])
	}
	result := make([]string, 0, len(changed))
	for name, differs := range changed {
		if differs {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func equalCodexEntries(a, b map[string]json.RawMessage) bool {
	return len(a) == len(b) && len(codexChangedNames(a, b)) == 0
}

func projectCodexServer(server Server, placeholders Placeholders) (map[string]any, error) {
	if server.OpenCodeV2 != nil {
		return nil, fmt.Errorf("codex does not accept OpenCode V2 options")
	}
	if strings.EqualFold(strings.TrimSpace(server.Type), "remote") && server.RemoteTransport == "streamable-http" {
		server.RemoteTransport = ""
	}
	projected, err := projectBaseServer(CodecMCPServers, server, placeholders)
	if err != nil {
		return nil, err
	}
	if headers, ok := projected["headers"]; ok {
		delete(projected, "headers")
		projected["http_headers"] = headers
	}
	if server.CodexAuth != nil {
		if server.Type != "remote" {
			return nil, fmt.Errorf("codex stdio servers do not accept remote authentication options")
		}
		if server.CodexAuth.BearerTokenEnvVar != "" {
			projected["bearer_token_env_var"] = server.CodexAuth.BearerTokenEnvVar
		}
		if server.CodexAuth.OAuthResource != "" {
			projected["oauth_resource"] = server.CodexAuth.OAuthResource
		}
	}
	return projected, nil
}
