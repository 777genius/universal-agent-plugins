package codextoml

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"github.com/creachadair/tomledit/transform"
	"github.com/pelletier/go-toml/v2"
)

// TrustProject adds one exact user preference without replacing native
// settings or an explicit trust decision. The caller validates project scope.
func TrustProject(body []byte, project string) ([]byte, error) {
	var values map[string]any
	if err := toml.Unmarshal(body, &values); err != nil {
		return nil, fmt.Errorf("invalid Codex trust TOML: %w", err)
	}
	if values == nil {
		values = map[string]any{}
	}
	projects, err := trustTable(values, "projects")
	if err != nil {
		return nil, err
	}
	selected, err := trustTable(projects, project)
	if err != nil {
		return nil, err
	}
	if trust, exists := selected["trust_level"]; exists {
		if trust == "trusted" {
			return bytes.Clone(body), nil
		}
		return nil, fmt.Errorf("existing Codex project trust decision is not trusted; preserved")
	}
	ast, err := tomledit.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := insertProjectTrust(ast, project); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	if err := tomledit.Format(&output, ast); err != nil {
		return nil, err
	}
	selected["trust_level"] = "trusted"
	projects[project] = selected
	values["projects"] = projects
	var observed map[string]any
	if err := toml.Unmarshal(output.Bytes(), &observed); err != nil || !reflect.DeepEqual(values, observed) {
		return nil, fmt.Errorf("codex project trust edit changed unrelated settings")
	}
	return bytes.Clone(output.Bytes()), nil
}
func trustTable(parent map[string]any, key string) (map[string]any, error) {
	value, exists := parent[key]
	if !exists {
		return map[string]any{}, nil
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("codex project trust requires table parents")
	}
	return table, nil
}

func insertProjectTrust(ast *tomledit.Document, project string) error {
	leaf, err := tomledit.Parse(bytes.NewBufferString("trust_level = \"trusted\"\n"))
	if err != nil {
		return err
	}
	mapping := leaf.First("trust_level").KeyValue
	// Prefer the deepest explicit parent. Inline tables must be edited inside
	// their existing mapping rather than redefined through a dotted key.
	key := parser.Key{"projects", project, "trust_level"}
	inserted := false
	for depth := 2; depth >= 1; depth-- {
		parent := ast.First(key[:depth]...)
		if parent == nil {
			continue
		}
		mapping.Name = key[depth:]
		if parent.IsSection() {
			transform.InsertMapping(parent.Section, mapping, false)
			inserted = true
			break
		}
		if parent.IsMapping() {
			inline, ok := parent.Value.X.(parser.Inline)
			if !ok {
				return fmt.Errorf("codex project trust parent is not a table")
			}
			inline = append(inline, mapping)
			parent.Value.X = inline
			inserted = true
			break
		}
	}
	if !inserted {
		if ast.Global == nil {
			ast.Global = &tomledit.Section{}
		}
		mapping.Name = key
		transform.InsertMapping(ast.Global, mapping, false)
	}
	return nil
}
