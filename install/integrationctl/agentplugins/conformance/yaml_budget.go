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
		if indent == len(line) || line[indent] == '#' {
			continue
		}
		if blockIndent >= 0 {
			if indent > blockIndent {
				continue
			}
			blockIndent = -1
		}
		if quote == 0 && flow == 0 {
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
			boundary := i == indent || bytes.ContainsRune([]byte(" \t:[{,-?"), rune(line[i-1]))
			if c == '#' && (i == indent || line[i-1] == ' ' || line[i-1] == '\t') {
				break
			}
			if (c == '\'' || c == '"') && boundary {
				quote = c
				continue
			}
			if (c == '|' || c == '>') && boundary && flow == 0 {
				blockIndent = indent
				break
			}
			switch c {
			case '[', '{':
				flow++
			case ']', '}':
				if flow > 0 {
					flow--
				}
			}
			if len(indents)-1+flow+compact > limit {
				return &parseFailure{Code: "yaml_depth_limit"}
			}
		}
	}
	return nil
}
