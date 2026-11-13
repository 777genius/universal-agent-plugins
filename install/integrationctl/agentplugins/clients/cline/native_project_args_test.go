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

func TestClineNeutralMapsRejectMalformedValues(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"env", "headers"} {
		for _, tc := range []struct {
			name string
			value any
		}{
			{"explicit-null", nil},
			{"scalar", "TOKEN=foreign"},
			{"array", []any{"TOKEN=foreign"}},
			{"non-string-member", map[string]any{"TOKEN": nil}},
			{"numeric-member", map[string]any{"TOKEN": 7}},
		} {
			t.Run(key+"/"+tc.name, func(t *testing.T) {
				_, err := ClineNeutralServer(domain.MCPServer{
					Type: "stdio",
					Decoded: map[string]any{"command": "node", key: tc.value},
				})
				if err == nil {
					t.Fatalf("malformed authored %s value %T was silently accepted", key, tc.value)
				}
			})
		}
	}
}

func TestClineNeutralMapsPreserveAbsentAndValidValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		decoded map[string]any
		wantEnv, wantHeaders map[string]string
	}{
		{"absent", map[string]any{"command": "node"}, nil, nil},
		{"empty", map[string]any{"command": "node", "env": map[string]any{}, "headers": map[string]string{}}, map[string]string{}, map[string]string{}},
		{"json-object", map[string]any{"command": "node", "env": map[string]any{"A": "one"}, "headers": map[string]any{"B": "two"}}, map[string]string{"A": "one"}, map[string]string{"B": "two"}},
		{"typed-object", map[string]any{"command": "node", "env": map[string]string{"A": "one"}, "headers": map[string]string{"B": "two"}}, map[string]string{"A": "one"}, map[string]string{"B": "two"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClineNeutralServer(domain.MCPServer{Type: "stdio", Decoded: tc.decoded})
			if err != nil || !reflect.DeepEqual(got.Env, tc.wantEnv) || !reflect.DeepEqual(got.Headers, tc.wantHeaders) {
				t.Fatalf("Cline maps differ: env=%#v headers=%#v err=%v", got.Env, got.Headers, err)
			}
		})
	}
}

func TestClineNeutralStringFieldsRejectMalformedValues(t *testing.T) {
	t.Parallel()
	for _, field := range []string{"command", "url"} {
		for _, tc := range []struct {
			name  string
			value any
		}{
			{"explicit-null", nil},
			{"number", 7},
			{"boolean", true},
			{"array", []any{"node"}},
			{"object", map[string]any{"value": "node"}},
		} {
			t.Run(field+"/"+tc.name, func(t *testing.T) {
				kind := "stdio"
				if field == "url" {
					kind = "streamable-http"
				}
				_, err := ClineNeutralServer(domain.MCPServer{
					Type: kind, Decoded: map[string]any{field: tc.value},
				})
				if err == nil {
					t.Fatalf("malformed authored %s value %T was silently discarded", field, tc.value)
				}
			})
		}
	}
}

func TestClineNeutralStringFieldsPreserveAbsentAndValidValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		kind    string
		decoded map[string]any
		command string
		url     string
	}{
		{"absent-stdio", "stdio", map[string]any{}, "", ""},
		{"stdio-command", "stdio", map[string]any{"command": "node"}, "node", ""},
		{"absent-remote", "streamable-http", map[string]any{}, "", ""},
		{"remote-url", "streamable-http", map[string]any{"url": "https://example.invalid/mcp"}, "", "https://example.invalid/mcp"},
		{"empty-fields", "stdio", map[string]any{"command": "", "url": ""}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClineNeutralServer(domain.MCPServer{Type: tc.kind, Decoded: tc.decoded})
			if err != nil || got.Command != tc.command || got.URL != tc.url {
				t.Fatalf("authored string fields changed: command=%q url=%q err=%v", got.Command, got.URL, err)
			}
		})
	}
}
