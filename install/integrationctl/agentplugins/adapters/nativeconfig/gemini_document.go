package nativeconfig

import (
	"fmt"

	"github.com/tailscale/hujson"
)

// Gemini v0.65.0 parses JSON after stripping comments. Its settings.json is
// comment-aware, but trailing commas remain invalid. Keep the original AST so
// managed entry mutations preserve comments and unrelated configuration.
func parseCodecDocument(body []byte, jsonc bool, codec Codec) (*document, error) {
	if codec == CodecCodex {
		return nil, fmt.Errorf("codex TOML document parser is required")
	}
	if codec != CodecGemini || jsonc {
		return parseDocument(body, jsonc)
	}
	doc, err := parseDocument(body, true)
	if err != nil {
		return nil, err
	}
	for value := range doc.ast.All() {
		if !geminiWithoutTrailingComma(value) {
			return nil, fmt.Errorf("%w: Gemini settings do not support trailing commas", ErrMalformed)
		}
	}
	doc.commentsOnly = true
	return doc, nil
}

func geminiWithoutTrailingComma(value *hujson.Value) bool {
	switch node := value.Value.(type) {
	case *hujson.Object:
		return len(node.Members) == 0 || node.Members[len(node.Members)-1].Value.AfterExtra == nil
	case *hujson.Array:
		return len(node.Elements) == 0 || node.Elements[len(node.Elements)-1].AfterExtra == nil
	default:
		return true
	}
}

// Deleting a last member can turn the preceding member's comma into a trailing
// comma. Input validation already rejected native-invalid commas; keep the
// surviving foreign trivia while removing only commas created by mutation.
func geminiRemoveTrailingCommas(ast *hujson.Value) {
	for value := range ast.All() {
		switch node := value.Value.(type) {
		case *hujson.Object:
			if len(node.Members) > 0 {
				last := &node.Members[len(node.Members)-1].Value
				node.AfterExtra = append(last.AfterExtra, node.AfterExtra...)
				last.AfterExtra = nil
			}
		case *hujson.Array:
			if len(node.Elements) > 0 {
				last := &node.Elements[len(node.Elements)-1]
				node.AfterExtra = append(last.AfterExtra, node.AfterExtra...)
				last.AfterExtra = nil
			}
		}
	}
}
