package nativeconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/tailscale/hujson"
)

// CodexDocument retains concrete syntax outside the contract dependency closure.
// Implementations return independent entry/output byte slices.
type CodexDocument interface {
	MCPEntries() (map[string]json.RawMessage, error)
	RenderMCPEntries(map[string]json.RawMessage) ([]byte, error)
}

type CodexDocumentParser func([]byte) (CodexDocument, error)

// WithCodexDocumentParser retains this Kernel's exact FileIO and lock authority.
func (kernel Kernel) WithCodexDocumentParser(parser CodexDocumentParser) Kernel {
	kernel.codexParser = parser
	return kernel
}

func (kernel Kernel) requireDocumentParser(codec Codec) error {
	if codec == CodecCodex && kernel.codexParser == nil {
		return fmt.Errorf("codex TOML document parser is required")
	}
	return nil
}

func (kernel Kernel) parseDocument(body []byte, jsonc bool, codec Codec) (*document, error) {
	if codec != CodecCodex {
		return parseCodecDocument(body, jsonc, codec)
	}
	if err := kernel.requireDocumentParser(codec); err != nil {
		return nil, err
	}
	if jsonc {
		return nil, fmt.Errorf("%w: Codex requires one TOML config path", ErrMalformed)
	}
	native, err := kernel.codexParser(bytes.Clone(body))
	if err != nil {
		return nil, err
	}
	if nilCodexDocument(native) {
		return nil, fmt.Errorf("codex TOML parser returned no document")
	}
	entries, err := native.MCPEntries()
	if err != nil {
		return nil, err
	}
	neutral, err := json.Marshal(map[string]any{"mcp_servers": cloneCodexEntries(entries)})
	if err != nil {
		return nil, err
	}
	doc, err := parseDocument(neutral, false)
	if err != nil {
		return nil, err
	}
	doc.codexDocument = native
	return doc, nil
}

func nilCodexDocument(value CodexDocument) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func cloneCodexEntries(entries map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(entries))
	for name, body := range entries {
		result[name] = bytes.Clone(body)
	}
	return result
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
		result[member.Name.Value.(hujson.Literal).String()] = bytes.Clone(body)
	}
	return result, nil
}

func (doc *document) renderCodexDocument() ([]byte, error) {
	entries, err := doc.codexEntries()
	if err != nil {
		return nil, err
	}
	body, err := doc.codexDocument.RenderMCPEntries(cloneCodexEntries(entries))
	return bytes.Clone(body), err
}
