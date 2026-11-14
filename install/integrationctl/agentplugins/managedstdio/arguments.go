package managedstdio

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

const encodedArgumentsSeparator = "--argv-base64-v1"

// EncodedArguments preserves opaque child arguments through clients that expand
// relative paths or environment-looking strings in every MCP argument.
func EncodedArguments(plugin, data, cwd string, anchor pathcontract.Anchor, command string, args []string) []string {
	result := Arguments(plugin, data, cwd, anchor, command, args)
	result[5] = encodedArgumentsSeparator
	for i := 6; i < len(result); i++ {
		result[i] = base64.StdEncoding.EncodeToString([]byte(result[i]))
	}
	return result
}

// DecodeCommandArguments validates the explicit private transport. Decoding is
// linear in the incoming argv bytes and never expands their aggregate size.
// The legacy raw separator retains its original argument semantics.
func DecodeCommandArguments(separator string, args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("invalid protocol v1 arguments")
	}
	if separator == "--" {
		return args, nil
	}
	if separator != encodedArgumentsSeparator {
		return nil, fmt.Errorf("invalid protocol v1 arguments")
	}
	decoded := make([]string, len(args))
	for i, value := range args {
		raw, err := base64.StdEncoding.Strict().DecodeString(value)
		if err != nil || strings.ContainsAny(value, "\r\n") || base64.StdEncoding.EncodeToString(raw) != value || strings.IndexByte(string(raw), 0) >= 0 {
			return nil, fmt.Errorf("invalid encoded protocol v1 argument")
		}
		decoded[i] = string(raw)
	}
	if decoded[0] == "" {
		return nil, fmt.Errorf("missing encoded protocol v1 command")
	}
	return decoded, nil
}
