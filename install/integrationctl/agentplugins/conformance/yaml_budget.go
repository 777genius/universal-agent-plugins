package conformance

import (
	"bytes"
	"context"
)

// preflightYAML caps potential structural depth before yaml.v3 allocates nodes.
// It is a conservative host work guard, not a YAML validator: quoted values,
// comments and block scalar contents are opaque. The node walk subsequently
// checks actual depth, members and alias expansion before map materialization.
func preflightYAML(ctx context.Context, data []byte, limit int) error {
	indents := []int{-1}
	flow := 0
	quote := byte(0)
	blockIndent := -1
	plain := false
	plainIndent := -1
	valuePending := false
	valueIndent := -1
	space := func(c byte) bool { return c == ' ' || c == '\t' || c == '\r' }
	for len(data) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, rest, ok := bytes.Cut(data, []byte{'\n'})
		if !ok {
			rest = nil
		}
		data = rest
		indent := 0
		for indent < len(line) && (line[indent] == ' ' || line[indent] == '\t') {
			indent++
		}
		if quote == 0 && (indent == len(line) || line[indent] == '#') {
			continue
		}
		if blockIndent >= 0 {
			if indent > blockIndent {
				continue
			}
			blockIndent = -1
		}
		// Block plain scalars continue beyond their parent indentation, even
		// when their first text starts on a separate line. In flow
		// collections a newline is whitespace; only a delimiter ends the scalar.
		if quote == 0 && flow == 0 && indent <= plainIndent {
			plain = false
		}
		if quote == 0 && flow == 0 && plain {
			// A continuation cannot contain a mapping separator. This also
			// recognizes the next key of a compact sequence mapping, whose
			// indentation can exceed the preceding scalar's line indentation.
			for i := indent; i < len(line); i++ {
				if line[i] == ':' && (i+1 == len(line) || space(line[i+1])) {
					plain = false
					break
				}
			}
		}
		if quote == 0 && flow == 0 && !plain {
			for len(indents) > 1 && indent < indents[len(indents)-1] {
				indents = indents[:len(indents)-1]
			}
			if indent > indents[len(indents)-1] {
				if len(indents) > limit {
					return &parseFailure{Code: "yaml_depth_limit"}
				}
				indents = append(indents, indent)
			}
		}
		compact := 0
		for pos := indent; pos+1 < len(line) && line[pos] == '-' && (line[pos+1] == ' ' || line[pos+1] == '\t'); {
			compact++
			pos += 2
			for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
				pos++
			}
		}
		if compact > 0 {
			compact--
		} // First sequence level is already counted by indentation.
		for i := indent; i < len(line); i++ {
			if i%4096 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			c := line[i]
			if quote != 0 {
				if quote == '"' && c == '\\' {
					i++
					continue
				}
				if c == quote {
					if quote == '\'' && i+1 < len(line) && line[i+1] == '\'' {
						i++
						continue
					}
					quote = 0
				}
				continue
			}
			separated := i+1 == len(line) || space(line[i+1])
			if c == '#' && (i == indent || space(line[i-1])) {
				plain = false
				break
			}
			if space(c) {
				continue
			}
			// A mapping separator or flow delimiter ends even a plain scalar.
			// Colons embedded in block text (URLs, for example) do not.
			if c == ':' && (!plain || separated || flow > 0 && bytes.ContainsRune([]byte(",[]{}"), rune(line[i+1]))) {
				plain = false
				valuePending, valueIndent = true, indent
				continue
			}
			if flow > 0 && (c == ',' || c == ']' || c == '}') {
				if c != ',' {
					flow--
				}
				plain = false
				continue
			}
			if plain {
				continue
			}
			if (c == '-' || c == '?') && separated {
				valuePending, valueIndent = true, indent
				continue
			}
			// Node properties precede the scalar/collection, rather than
			// starting plain text. Skip their token, including verbatim tags.
			if c == '&' || c == '!' {
				verbatim := c == '!' && i+1 < len(line) && line[i+1] == '<'
				for i+1 < len(line) && !space(line[i+1]) {
					if !verbatim && bytes.ContainsRune([]byte(",[]{}"), rune(line[i+1])) {
						break
					}
					i++
					if verbatim && line[i] == '>' {
						break
					}
				}
				continue
			}
			if c == '\'' || c == '"' {
				valuePending = false
				quote = c
				continue
			}
			if (c == '|' || c == '>') && flow == 0 {
				valuePending = false
				blockIndent = indent
				break
			}
			switch c {
			case '[', '{':
				valuePending = false
				flow++
			default:
				plain = true
				plainIndent = indent
				if valuePending {
					plainIndent = valueIndent
				}
				valuePending = false
			}
			if len(indents)-1+flow+compact > limit {
				return &parseFailure{Code: "yaml_depth_limit"}
			}
		}
	}
	return ctx.Err()
}
