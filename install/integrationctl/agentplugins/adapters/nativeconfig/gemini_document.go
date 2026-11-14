package nativeconfig

import (
	"fmt"

	"github.com/tailscale/hujson"
)

// Gemini v0.65.0 and Devin3000.11.205 accept native JSON comments. Keep
// the original AST so mutations preserve comments and unrelated configuration;
// neither this admission nor a read-only policy check accepts trailing commas.
func parseCodecDocument(body []byte, jsonc bool, codec Codec) (*document, error) {
	if codec == CodecCodex {
		return nil, fmt.Errorf("codex TOML document parser is required")
	}
	// VS Code's actual mcp.json accepts comments and trailing commas. It has
	// no alternative JSONC filename; preserve its native AST in that one file.
	if codec == CodecVSCode {
		return parseDocument(body, true)
	}
	if (codec != CodecGemini && codec != CodecDevin) || jsonc {
		return parseDocument(body, jsonc)
	}
	label := "Gemini"
	if codec == CodecDevin {
		label = "Devin"
	}
	return parseCommentsOnlyDocument(body, label)
}

func parseCommentsOnlyDocument(body []byte, label string) (*document, error) {
	doc, err := parseDocument(body, true)
	if err != nil {
		return nil, err
	}
	for value := range doc.ast.All() {
		if !geminiWithoutTrailingComma(value) {
			return nil, fmt.Errorf("%w: %s settings do not support trailing commas", ErrMalformed, label)
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

// StandardizeCommentedJSON normalizes native JSON-with-comments grammar for
// read-only policy inspection. Duplicate keys and trailing commas refuse;
// callers retain original file bytes for mutation and ownership checks.
func StandardizeCommentedJSON(body []byte) ([]byte, error) {
	if _, err := parseCommentsOnlyDocument(body, "native config"); err != nil {
		return nil, err
	}
	return hujson.Standardize(body)
}
