package codextoml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/creachadair/tomledit"
	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

type document struct {
	body     []byte
	ast      *tomledit.Document
	original map[string]json.RawMessage
	settings map[string]any
}

// Parse supplies the retained concrete TOML backend at an explicit composition boundary.
func Parse(body []byte) (nativeconfig.CodexDocument, error) {
	var values map[string]any
	if err := toml.Unmarshal(body, &values); err != nil {
		return nil, fmt.Errorf("%w: invalid Codex TOML", nativeconfig.ErrMalformed)
	}
	if values == nil {
		values = map[string]any{}
	}
	servers, exists := values["mcp_servers"]
	if !exists {
		servers = map[string]any{}
	}
	collection, ok := servers.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: mcp_servers must be a table", nativeconfig.ErrMalformed)
	}
	entries := make(map[string]json.RawMessage, len(collection))
	for name, value := range collection {
		entry, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid Codex MCP entry values", nativeconfig.ErrMalformed)
		}
		entries[name] = entry
	}
	ast, err := tomledit.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid Codex TOML structure", nativeconfig.ErrMalformed)
	}
	delete(values, "mcp_servers")
	return &document{body: bytes.Clone(body), ast: ast, original: entries, settings: values}, nil
}

func (doc *document) MCPEntries() (map[string]json.RawMessage, error) {
	return cloneEntries(doc.original), nil
}

func (doc *document) RenderMCPEntries(current map[string]json.RawMessage) ([]byte, error) {
	current = cloneEntries(current)
	names := changedNames(doc.original, current)
	if len(names) == 0 {
		return bytes.Clone(doc.body), nil
	}
	for _, name := range names {
		removeCodexTOMLEntry(doc.ast, name)
		if body, exists := current[name]; exists {
			if err := setCodexTOMLEntry(doc.ast, name, body); err != nil {
				return nil, err
			}
		}
	}
	var result bytes.Buffer
	if err := tomledit.Format(&result, doc.ast); err != nil {
		return nil, fmt.Errorf("render Codex native config: %w", err)
	}
	parsed, err := Parse(result.Bytes())
	if err != nil {
		return nil, err
	}
	observed := parsed.(*document)
	if !reflect.DeepEqual(doc.settings, observed.settings) {
		return nil, fmt.Errorf("rendered Codex config changed foreign settings")
	}
	if len(changedNames(current, observed.original)) != 0 {
		return nil, fmt.Errorf("rendered Codex MCP entries differ from requested entries")
	}
	return bytes.Clone(result.Bytes()), nil
}

func cloneEntries(entries map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(entries))
	for name, body := range entries {
		result[name] = bytes.Clone(body)
	}
	return result
}

func changedNames(previous, next map[string]json.RawMessage) []string {
	changed := map[string]bool{}
	for name, body := range previous {
		changed[name] = !bytes.Equal(body, next[name])
	}
	for name, body := range next {
		changed[name] = !bytes.Equal(body, previous[name])
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
