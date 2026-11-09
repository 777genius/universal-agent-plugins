package nativeconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"gopkg.in/yaml.v3"
)

// hermesYAMLDocument is a single configuration authority: preserve foreign
// nodes, comments and key order while changing only selected mcp_servers keys.
// Reject aliases, custom tags and duplicate keys rather than normalize a
// credential-bearing user config whose semantics cannot be proven.
type hermesYAMLDocument struct {
	root yaml.Node
	original []byte
}

const maxHermesConfigBytes = 4 << 20

func parseHermesDocument(body []byte, jsonc bool) (*document, error) {
	if jsonc {
		return nil, fmt.Errorf("%w: Hermes accepts only config.yaml", ErrMalformed)
	}
	native, err := parseHermesYAML(body)
	if err != nil { return nil, err }
	entries, err := native.MCPEntries()
	if err != nil { return nil, err }
	neutral, err := json.Marshal(map[string]any{"mcp_servers": entries})
	if err != nil { return nil, err }
	doc, err := parseDocument(neutral, false)
	if err != nil { return nil, err }
	doc.hermesDocument = native
	return doc, nil
}

func parseHermesYAML(body []byte) (*hermesYAMLDocument, error) {
	if len(body) > maxHermesConfigBytes {
		return nil, fmt.Errorf("%w: Hermes configuration exceeds %d bytes", ErrMalformed, maxHermesConfigBytes)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		body = []byte("{}\n")
	}
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("%w: Hermes YAML could not be decoded: %v", ErrMalformed, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: Hermes YAML must contain one document", ErrMalformed)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: Hermes config root must be a YAML mapping", ErrMalformed)
	}
	if err := validateHermesYAMLNode(&root); err != nil { return nil, err }
	return &hermesYAMLDocument{root: root, original: bytes.Clone(body)}, nil
}

func validateHermesYAMLNode(node *yaml.Node) error {
	if node == nil || node.Kind == yaml.AliasNode || node.Alias != nil || node.Anchor != "" {
		return fmt.Errorf("%w: YAML anchors or aliases are not safe to rewrite", ErrMalformed)
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		if len(node.Content)%2 != 0 { return fmt.Errorf("%w: invalid Hermes mapping", ErrMalformed) }
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
				return fmt.Errorf("%w: duplicate or nonstring YAML key", ErrMalformed)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := validateHermesYAMLNode(child); err != nil { return err }
	}
	return nil
}

func hermesMappingValue(mapping *yaml.Node, key string) (*yaml.Node, int) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key { return mapping.Content[i+1], i }
	}
	return nil, -1
}

func (doc *hermesYAMLDocument) collection(create bool) (*yaml.Node, error) {
	parent := doc.root.Content[0]
	collection, _ := hermesMappingValue(parent, "mcp_servers")
	if collection == nil {
		if !create { return nil, nil }
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "mcp_servers"}
		collection = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		parent.Content = append(parent.Content, key, collection)
	}
	if collection.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: Hermes mcp_servers must be a mapping", ErrMalformed)
	}
	return collection, nil
}

func (doc *hermesYAMLDocument) MCPEntries() (map[string]json.RawMessage, error) {
	result := map[string]json.RawMessage{}
	list, err := doc.collection(false)
	if err != nil || list == nil { return result, err }
	for i := 0; i+1 < len(list.Content); i += 2 {
		key, value := list.Content[i], list.Content[i+1]
		if value.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%w: Hermes MCP server is not a mapping", ErrMalformed)
		}
		var decoded map[string]any
		if err := value.Decode(&decoded); err != nil { return nil, fmt.Errorf("%w: invalid Hermes MCP entry", ErrMalformed) }
		encoded, err := json.Marshal(decoded)
		if err != nil { return nil, fmt.Errorf("%w: invalid Hermes MCP entry values", ErrMalformed) }
		result[key.Value] = encoded
	}
	return result, nil
}

func (doc *hermesYAMLDocument) RenderMCPEntries(desired map[string]json.RawMessage) ([]byte, error) {
	previous, err := doc.MCPEntries()
	if err != nil { return nil, err }
	if len(desired) == len(previous) {
		same := true
		for name, value := range desired {
			if !bytes.Equal(value, previous[name]) { same = false; break }
		}
		if same { return bytes.Clone(doc.original), nil }
	}
	list, err := doc.collection(true)
	if err != nil { return nil, err }
	remaining := map[string]json.RawMessage{}
	for key, val := range desired { remaining[key] = val }
	nodes := make([]*yaml.Node, 0, len(desired)*2)
	for i := 0; i+1 < len(list.Content); i += 2 {
		key, value := list.Content[i], list.Content[i+1]
		encoded, keep := remaining[key.Value]
		if !keep { continue }
		if !bytes.Equal(previous[key.Value], encoded) {
			next, err := hermesValueNode(encoded)
			if err != nil { return nil, err }
			next.HeadComment, next.LineComment, next.FootComment = value.HeadComment, value.LineComment, value.FootComment
			value = next
		}
		nodes = append(nodes, key, value)
		delete(remaining, key.Value)
	}
	names := make([]string, 0, len(remaining))
	for name := range remaining { names = append(names, name) }
	sort.Strings(names)
	for _, name := range names {
		value, err := hermesValueNode(remaining[name])
		if err != nil { return nil, err }
		nodes = append(nodes, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, value)
	}
	list.Content = nodes
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc.root); err != nil { return nil, err }
	if err := encoder.Close(); err != nil { return nil, err }
	check, err := parseHermesYAML(out.Bytes())
	if err != nil { return nil, err }
	actual, err := check.MCPEntries()
	if err != nil || len(actual) != len(desired) { return nil, fmt.Errorf("%w: Hermes YAML rendered different MCP entries", ErrMalformed) }
	for name, value := range desired {
		if !bytes.Equal(value, actual[name]) { return nil, fmt.Errorf("%w: Hermes YAML rendered a modified MCP server", ErrMalformed) }
	}
	return out.Bytes(), nil
}

func hermesValueNode(encoded json.RawMessage) (*yaml.Node, error) {
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		return nil, fmt.Errorf("%w: Hermes MCP entry must be a JSON object", ErrMalformed)
	}
	var value yaml.Node
	if err := value.Encode(object); err != nil { return nil, err }
	return &value, nil
}
