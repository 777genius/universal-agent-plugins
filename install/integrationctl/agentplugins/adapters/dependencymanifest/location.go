package dependencymanifest

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Error reports a manifest failure with its original source and line.
// Err retains the underlying validation or TOML decoding error.
type Error struct {
	Source string
	Line   int
	Err    error
}

func (err *Error) Error() string { return fmt.Sprintf("%s:%d: %v", err.Source, err.Line, err.Err) }

func (err *Error) Unwrap() error { return err.Err }

func manifestError(path string, body []byte, cause error, location ...string) error {
	line := 1
	var decode *toml.DecodeError
	if errors.As(cause, &decode) {
		line, _ = decode.Position()
	} else if len(location) != 0 {
		line = manifestLocation(body, location)
	}
	return &Error{Source: path, Line: line, Err: cause}
}

// Use the decoder's syntax tree only for source locations, never rewriting TOML.
// Its decoded keys distinguish comments, quoted keys and unrelated tables.
func manifestLocation(body []byte, wanted []string) int {
	var parser unstable.Parser
	parser.Reset(body)
	var table []string
	bestLine, bestDepth := 1, 0
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind != unstable.Table && node.Kind != unstable.KeyValue {
			continue
		}
		keys, line := manifestNodeKey(&parser, node)
		if node.Kind == unstable.Table {
			table = keys
		} else {
			keys = append(append([]string{}, table...), keys...)
		}
		if slices.Equal(keys, wanted) {
			return line
		}
		if len(keys) > bestDepth && len(keys) < len(wanted) && slices.Equal(keys, wanted[:len(keys)]) {
			bestLine, bestDepth = line, len(keys)
		}
	}
	return bestLine
}

func manifestNodeKey(parser *unstable.Parser, node *unstable.Node) ([]string, int) {
	var keys []string
	line := 1
	iterator := node.Key()
	for iterator.Next() {
		key := iterator.Node()
		if len(keys) == 0 {
			line = parser.Shape(key.Raw).Start.Line
		}
		keys = append(keys, string(key.Data))
	}
	return keys, line
}

func requirementErrorLocation(requirements environment.Requirements, cause error) []string {
	if requirements.SchemaVersion != environment.ManifestSchemaVersion {
		return []string{"schema_version"}
	}
	var name string
	if _, err := fmt.Sscanf(cause.Error(), "target %q", &name); err == nil {
		return []string{"targets", name}
	}
	if _, err := fmt.Sscanf(cause.Error(), "alias %q", &name); err == nil {
		return []string{"dependencies", name}
	}
	if _, err := fmt.Sscanf(cause.Error(), "aliases %q", &name); err == nil {
		return []string{"dependencies", name}
	}
	return nil
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
