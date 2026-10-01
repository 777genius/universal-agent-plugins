package geminihooks

import "fmt"

const (
	maxSettingsDepth = 64
	maxSettingsNodes = 65536
)

// Bound work before hujson's recursive parser and All iterator allocate/traverse
// the tree. Count containers, keys and scalar values, ignoring string/comment
// contents. This allocation-free lexical pass is not a second grammar parser;
// hujson still validates syntax. The byte ceiling bounds even a single token.
func preflightSettings(body []byte) error {
	depth, nodes := 0, 0
	atom := false
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case ' ', '\t', '\r', '\n', ',', ':':
			atom = false
		case '/':
			if i+1 < len(body) && body[i+1] == '/' {
				i += 2
				for i < len(body) && body[i] != '\n' {
					i++
				}
				atom = false
			} else if i+1 < len(body) && body[i+1] == '*' {
				i += 2
				for i+1 < len(body) && !(body[i] == '*' && body[i+1] == '/') {
					i++
				}
				i++
				atom = false
			}
		case '"':
			nodes++
			for i++; i < len(body); i++ {
				if body[i] == '\\' {
					i++
				} else if body[i] == '"' {
					break
				}
			}
			atom = false
		case '{', '[':
			depth++
			nodes++
			atom = false
			if depth > maxSettingsDepth {
				return fmt.Errorf("settings exceed depth bound (%d)", maxSettingsDepth)
			}
		case '}', ']':
			depth--
			atom = false
			if depth < 0 {
				return fmt.Errorf("unbalanced settings containers")
			}
		default:
			if !atom {
				nodes++
				atom = true
			}
		}
		if nodes > maxSettingsNodes {
			return fmt.Errorf("settings exceed node bound (%d)", maxSettingsNodes)
		}
	}
	return nil
}
