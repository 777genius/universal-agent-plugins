package cline

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestClineNeutralArgsRejectsMalformedValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args any
	}{
		{"null", nil},
		{"scalar-string", "--unsafe"},
		{"number", 17},
		{"boolean", true},
		{"object", map[string]any{"value": "-y"}},
		{"mixed-array", []any{"-y", 17}},
		{"null-element", []any{nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ClineNeutralServer(domain.MCPServer{
				Type:    "stdio",
				Decoded: map[string]any{"command": "node", "args": tc.args},
			})
			if err == nil {
				t.Fatalf("malformed authored args %T were silently discarded", tc.args)
			}
		})
	}
}

func TestClineNeutralArgsKeepsAbsentAndValidLists(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		decoded map[string]any
		want    []string
	}{
		{"absent", map[string]any{"command": "node"}, nil},
		{"empty", map[string]any{"command": "node", "args": []any{}}, nil},
		{"json-array", map[string]any{"command": "node", "args": []any{"-y", "tool"}}, []string{"-y", "tool"}},
		{"typed-array", map[string]any{"command": "node", "args": []string{"-y", "tool"}}, []string{"-y", "tool"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClineNeutralServer(domain.MCPServer{Type: "stdio", Decoded: tc.decoded})
			if err != nil || !reflect.DeepEqual(got.Args, tc.want) {
				t.Fatalf("valid authored args changed: got=%#v want=%#v err=%v", got.Args, tc.want, err)
			}
		})
	}
}
