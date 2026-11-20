package nativeconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"sort"

	"gopkg.in/yaml.v3"
)

// hermesYAMLDocument is a single configuration authority: preserve foreign
// nodes, comments and key order while changing only selected mcp_servers keys.
// Reject aliases, custom tags and duplicate keys rather than normalize a
// credential-bearing user config whose semantics cannot be proven.
type hermesYAMLDocument struct {
	root     yaml.Node
	original []byte
}

const maxHermesConfigBytes = 4 << 20

func parseHermesDocument(body []byte, jsonc bool) (*document, error) {
	if jsonc {
		return nil, fmt.Errorf("%w: Hermes accepts only config.yaml", ErrMalformed)
	}
	native, err := parseHermesYAML(body)
	if err != nil {
		return nil, err
	}
	entries, err := native.MCPEntries()
	if err != nil {
		return nil, err
	}
	neutral, err := json.Marshal(map[string]any{"mcp_servers": entries})
	if err != nil {
		return nil, err
	}
	doc, err := parseDocument(neutral, false)
	if err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("%w: hermes YAML could not be decoded: %w", ErrMalformed, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: Hermes YAML must contain one document", ErrMalformed)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: Hermes config root must be a YAML mapping", ErrMalformed)
	}
	if err := validateHermesYAMLNode(&root); err != nil {
		return nil, err
	}
	return &hermesYAMLDocument{root: root, original: bytes.Clone(body)}, nil
}

func validateHermesYAMLNode(node *yaml.Node) error {
	if node == nil || node.Kind == yaml.AliasNode || node.Alias != nil || node.Anchor != "" {
		return fmt.Errorf("%w: YAML anchors or aliases are not safe to rewrite", ErrMalformed)
	}
	// No non-standard YAML tags may be accepted in a credential-bearing
	// config: a later native loader could interpret those differently.
	switch node.Tag {
	case "", "!!str", "!!int", "!!bool", "!!float", "!!null", "!!map", "!!seq":
	default:
		return fmt.Errorf("%w: unsafe or unsupported YAML tag", ErrMalformed)
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		if len(node.Content)%2 != 0 {
			return fmt.Errorf("%w: invalid Hermes mapping", ErrMalformed)
		}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
				return fmt.Errorf("%w: duplicate or nonstring YAML key", ErrMalformed)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := validateHermesYAMLNode(child); err != nil {
			return err
		}
	}
	return nil
}

func hermesMappingValue(mapping *yaml.Node, key string) (*yaml.Node, int) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1], i
		}
	}
	return nil, -1
}

func (doc *hermesYAMLDocument) collection(create bool) (*yaml.Node, error) {
	parent := doc.root.Content[0]
	collection, _ := hermesMappingValue(parent, "mcp_servers")
	if collection == nil {
		if !create {
			return nil, nil
		}
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
	if err != nil || list == nil {
		return result, err
	}
	for i := 0; i+1 < len(list.Content); i += 2 {
		key, value := list.Content[i], list.Content[i+1]
		if value.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%w: Hermes MCP server is not a mapping", ErrMalformed)
		}
		var decoded map[string]any
		if err := value.Decode(&decoded); err != nil {
			return nil, fmt.Errorf("%w: invalid Hermes MCP entry", ErrMalformed)
		}
		encoded, err := json.Marshal(decoded)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid Hermes MCP entry values", ErrMalformed)
		}
		result[key.Value] = encoded
	}
	return result, nil
}

func (doc *hermesYAMLDocument) RenderMCPEntries(desired map[string]json.RawMessage) ([]byte, error) {
	previous, err := doc.MCPEntries()
	if err != nil {
		return nil, err
	}
	if equalHermesMCPEntries(previous, desired) {
		return bytes.Clone(doc.original), nil
	}
	list, err := doc.collection(true)
	if err != nil {
		return nil, err
	}
	list.Content, err = replaceHermesMCPEntryNodes(list.Content, previous, desired)
	if err != nil {
		return nil, err
	}
	return doc.encodeVerifiedMCPEntries(desired)
}

func equalHermesMCPEntries(previous, desired map[string]json.RawMessage) bool {
	if len(previous) != len(desired) {
		return false
	}
	for name, value := range desired {
		if !bytes.Equal(value, previous[name]) {
			return false
		}
	}
	return true
}

func replaceHermesMCPEntryNodes(
	original []*yaml.Node, previous, desired map[string]json.RawMessage,
) ([]*yaml.Node, error) {
	remaining := make(map[string]json.RawMessage, len(desired))
	for name, value := range desired {
		remaining[name] = value
	}
	nodes := make([]*yaml.Node, 0, len(desired)*2)
	for i := 0; i+1 < len(original); i += 2 {
		key, value := original[i], original[i+1]
		encoded, keep := remaining[key.Value]
		if !keep {
			continue
		}
		if !bytes.Equal(previous[key.Value], encoded) {
			next, err := hermesValueNode(encoded)
			if err != nil {
				return nil, err
			}
			next.HeadComment, next.LineComment, next.FootComment = value.HeadComment, value.LineComment, value.FootComment
			value = next
		}
		nodes = append(nodes, key, value)
		delete(remaining, key.Value)
	}
	names := make([]string, 0, len(remaining))
	for name := range remaining {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value, err := hermesValueNode(remaining[name])
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, value)
	}
	return nodes, nil
}

// encodeVerifiedMCPEntries prefers re-encoding only the top-level mcp_servers
// lines so Hermes' comments and blank lines elsewhere stay byte-for-byte. A
// layout the splice cannot address falls back to re-encoding the whole tree,
// which keeps every setting and comment but may drop blank lines.
func (doc *hermesYAMLDocument) encodeVerifiedMCPEntries(desired map[string]json.RawMessage) ([]byte, error) {
	if spliced, err := doc.spliceMCPBlock(); err == nil {
		if check, err := verifyHermesMCPEntries(spliced, desired); err == nil && sameHermesForeignSettings(&check.root, &doc.root) {
			return spliced, nil
		}
	}
	out, err := encodeHermesNode(&doc.root)
	if err != nil {
		return nil, err
	}
	if _, err := verifyHermesMCPEntries(out, desired); err != nil {
		return nil, err
	}
	return out, nil
}

func encodeHermesNode(node *yaml.Node) ([]byte, error) {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func verifyHermesMCPEntries(body []byte, desired map[string]json.RawMessage) (*hermesYAMLDocument, error) {
	check, err := parseHermesYAML(body)
	if err != nil {
		return nil, err
	}
	actual, err := check.MCPEntries()
	if err != nil {
		return nil, err
	}
	if !equalHermesMCPEntries(actual, desired) {
		return nil, fmt.Errorf("%w: hermes YAML rendered different MCP entries", ErrMalformed)
	}
	return check, nil
}

// spliceMCPBlock replaces the column-1 mcp_servers key line and the indented
// or blank lines after it, or appends the block when the key is new.
func (doc *hermesYAMLDocument) spliceMCPBlock() ([]byte, error) {
	parent := doc.root.Content[0]
	value, index := hermesMappingValue(parent, "mcp_servers")
	if value == nil {
		return nil, fmt.Errorf("hermes mcp_servers is absent")
	}
	key := parent.Content[index]
	if len(value.Content) > 0 {
		value.Style &^= yaml.FlowStyle
	}
	block, err := encodeHermesNode(&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: key.Value, LineComment: key.LineComment}, value}})
	if err != nil {
		return nil, err
	}
	lines := bytes.SplitAfter(doc.original, []byte("\n"))
	start, end := len(lines), len(lines)
	if key.Line > 0 {
		if key.Column != 1 {
			return nil, fmt.Errorf("hermes mcp_servers is not a column-1 key")
		}
		start, end = key.Line-1, key.Line
		for end < len(lines) && (len(bytes.TrimSpace(lines[end])) == 0 || lines[end][0] == ' ') {
			end++
		}
		for end > key.Line && len(bytes.TrimSpace(lines[end-1])) == 0 {
			end--
		}
	}
	out := bytes.Join(lines[:start], nil)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	if key.Line == 0 && len(out) > 0 && !bytes.HasSuffix(out, []byte("\n\n")) {
		out = append(out, '\n') // keep a new block apart from the comments above it
	}
	return append(append(out, block...), bytes.Join(lines[end:], nil)...), nil
}

// sameHermesForeignSettings proves a text splice changed no setting outside
// mcp_servers.
func sameHermesForeignSettings(rendered, original *yaml.Node) bool {
	var left, right map[string]any
	if rendered.Content[0].Decode(&left) != nil || original.Content[0].Decode(&right) != nil {
		return false
	}
	delete(left, "mcp_servers")
	delete(right, "mcp_servers")
	return reflect.DeepEqual(left, right)
}

func hermesValueNode(encoded json.RawMessage) (*yaml.Node, error) {
	var object map[string]any
	if err := json.Unmarshal(encoded, &object); err != nil {
		return nil, fmt.Errorf("%w: Hermes MCP entry must be a JSON object", ErrMalformed)
	}
	var value yaml.Node
	if err := value.Encode(object); err != nil {
		return nil, err
	}
	return &value, nil
}

// Refuse to mutate an existing provider config that is readable by another
// local Unix user. The mutation kernel retains existing file modes by design,
// so writing credentials into a pre-existing 0644 file would preserve the leak.
// Windows uses ACLs rather than meaningful POSIX permission bits.
func requireHermesPrivateConfig(file resolvedFile, codec Codec) error {
	if codec != CodecHermes || !file.exists || runtime.GOOS == "windows" {
		return nil
	}
	if file.mode.Perm()&0o077 != 0 {
		return fmt.Errorf("hermes config.yaml must have owner-only permissions before UAP manages MCP entries")
	}
	return nil
}

// resolveForCodec applies codec-specific safety invariants at the shared read boundary.
// The generic kernel remains agnostic of Hermes YAML semantics and permissions.
func (kernel Kernel) resolveForCodec(paths Paths, codec Codec) (resolvedFile, error) {
	file, err := kernel.resolve(paths)
	if err != nil {
		return file, err
	}
	return file, requireHermesPrivateConfig(file, codec)
}
