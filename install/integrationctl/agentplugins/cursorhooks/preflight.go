package cursorhooks

import "fmt"

// Allocation-free lexical budgeting precedes JSON validation, AST parsing and
// traversal. This is not a second JSON parser: encoding/json validates grammar.
// Escaped quotes/braces in strings never count as containers or extra nodes.
func preflight(body []byte) error {
	if len(body) > MaxDocumentBytes {
		return fmt.Errorf("document exceeds byte bound")
	}
	depth, nodes := 0, 0
	atom := false
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case ' ', '\t', '\r', '\n', ',', ':':
			atom = false
		case '"':
			nodes++
			i = stringEnd(body, i)
			atom = false
		case '{', '[':
			depth++
			nodes++
			atom = false
			if depth > MaxDepth {
				return fmt.Errorf("document exceeds depth bound")
			}
		case '}', ']':
			depth--
			atom = false
			if depth < 0 {
				return fmt.Errorf("unbalanced document")
			}
		default:
			if !atom {
				nodes++
				atom = true
			}
		}
		if nodes > MaxNodes {
			return fmt.Errorf("document exceeds node bound")
		}
	}
	return nil
}

func stringEnd(body []byte, start int) int {
	for i := start + 1; i < len(body); i++ {
		if body[i] == '\\' {
			i++
		} else if body[i] == '"' {
			return i
		}
	}
	return len(body) - 1
}
