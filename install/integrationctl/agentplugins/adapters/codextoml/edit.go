package codextoml

import (
	"bytes"
	"fmt"

	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"github.com/creachadair/tomledit/transform"
	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

func removeCodexTOMLEntry(doc *tomledit.Document, name string) {
	var remove []*tomledit.Entry
	prefix := parser.Key{"mcp_servers", name}
	doc.Scan(func(key parser.Key, entry *tomledit.Entry) bool {
		if prefix.IsPrefixOf(key) {
			remove = append(remove, entry)
		}
		return true
	})
	for _, entry := range remove {
		entry.Remove()
	}
}

func setCodexTOMLEntry(doc *tomledit.Document, name string, body []byte) error {
	value, err := decodeTOMLJSON(body)
	if err != nil || value == nil {
		return fmt.Errorf("%w: Codex MCP entry must be a table", nativeconfig.ErrMalformed)
	}
	var encoded bytes.Buffer
	if err := toml.NewEncoder(&encoded).SetTablesInline(true).Encode(map[string]any{name: value}); err != nil {
		return fmt.Errorf("encode Codex MCP entry: %w", err)
	}
	parsed, err := tomledit.Parse(&encoded)
	if err != nil {
		return fmt.Errorf("parse projected Codex MCP entry: %w", err)
	}
	entry := parsed.First(name)
	if entry == nil || !entry.IsMapping() {
		return fmt.Errorf("projected Codex MCP entry is not a mapping")
	}
	parent := doc.First("mcp_servers")
	if parent != nil && parent.IsMapping() {
		if inline, ok := parent.Value.X.(parser.Inline); ok {
			inline = append(inline, entry.KeyValue)
			parent.Value.X = inline
			return nil
		}
		return fmt.Errorf("%w: Codex MCP collection is not a table", nativeconfig.ErrMalformed)
	}
	if parent != nil && parent.IsSection() {
		transform.InsertMapping(parent.Section, entry.KeyValue, false)
		return nil
	}
	// An implicit parent may come from dotted keys or child tables. Adding a
	// dotted global member handles both without redefining an existing table.
	if doc.Global == nil {
		doc.Global = &tomledit.Section{}
	}
	entry.Name = parser.Key{"mcp_servers", name}
	transform.InsertMapping(doc.Global, entry.KeyValue, false)
	return nil
}
