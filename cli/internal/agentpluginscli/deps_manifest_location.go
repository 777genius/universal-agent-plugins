package agentpluginscli

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func manifestError(path string, body []byte, cause error, location ...string) error {
	line := 1
	var decode *toml.DecodeError
	if errors.As(cause, &decode) {
		line, _ = decode.Position()
	} else if len(location) != 0 {
		line = manifestLocation(body, location)
	}
	return fmt.Errorf("%s:%d: %w", filepath.Base(path), line, cause)
}

// Use the decoder's syntax tree only for source locations, never rewriting TOML.
// Its decoded keys distinguish comments, quoted keys and unrelated tables.
func manifestLocation(body []byte, wanted []string) int {
	var parser unstable.Parser
	parser.Reset(body)
	var table []string
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
	}
	return 1
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
