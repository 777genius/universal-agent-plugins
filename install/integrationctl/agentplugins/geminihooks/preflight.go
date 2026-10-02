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
			if end, comment := commentEnd(body, i); comment {
				i = end
				atom = false
			}
		case '"':
			nodes++
			i = stringEnd(body, i)
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

// Return the last scanned byte so the caller's loop advances exactly once.
// Unterminated tokens are left for hujson's grammar validation.
func commentEnd(body []byte, i int) (int, bool) {
	if i+1 >= len(body) {
		return i, false
	}
	switch body[i+1] {
	case '/':
		i += 2
		for i < len(body) && body[i] != '\n' {
			i++
		}
		return i, true
	case '*':
		i += 2
		for i+1 < len(body) && (body[i] != '*' || body[i+1] != '/') {
			i++
		}
		return i + 1, true
	default:
		return i, false
	}
}

func stringEnd(body []byte, i int) int {
	for i++; i < len(body); i++ {
		if body[i] == '\\' {
			i++
		} else if body[i] == '"' {
			break
		}
	}
	return i
}
