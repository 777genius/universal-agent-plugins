package vscodeprofile

import (
	"fmt"
	"unicode/utf8"
)

// Adapted from the admitted Gemini planner's private lexical budget pass.
// Bound recursion/allocation before hujson parses. This is not a grammar parser;
// hujson remains the single syntax authority. Count keys as well as values.
func preflight(body []byte) error {
	if len(body) > MaxSettingsBytes {
		return fmt.Errorf("settings exceed byte budget")
	}
	if !utf8.Valid(body) {
		return fmt.Errorf("settings contain invalid UTF-8")
	}
	depth, nodes := 0, 0
	atom := false
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case ' ', '\t', '\r', '\n', ',', ':':
			atom = false
		case '/':
			if end, ok := commentEnd(body, i); ok {
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
			if depth > MaxSettingsDepth {
				return fmt.Errorf("settings exceed depth budget")
			}
		case '}', ']':
			depth--
			atom = false
			if depth < 0 {
				return fmt.Errorf("unbalanced settings")
			}
		default:
			if !atom {
				nodes++
				atom = true
			}
		}
		if nodes > MaxSettingsNodes {
			return fmt.Errorf("settings exceed node budget")
		}
	}
	return nil
}

func commentEnd(body []byte, i int) (int, bool) {
	if i+1 >= len(body) {
		return i, false
	}
	switch body[i+1] {
	case '/':
		for i += 2; i < len(body) && body[i] != '\n'; i++ {
		}
		return i, true
	case '*':
		for i += 2; i+1 < len(body) && (body[i] != '*' || body[i+1] != '/'); i++ {
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
