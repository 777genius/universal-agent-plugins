package shared

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestApplyStdioDataContractPreservesOpaquePlaceholders(t *testing.T) {
	for _, decoded := range []bool{false, true} {
		name := "typed"
		if decoded {
			name = "json"
		}
		t.Run(name, func(t *testing.T) {
			// A token inside the replacement must not be expanded recursively.
			active := filepath.Join(t.TempDir(), "${PLUGIN_DATA}")
			data := t.TempDir()
			args := []string{"${PLUGIN_ROOT}/file", "${PLUGIN_DATA}/cache", "${package.root}/literal", "${UNKNOWN}", "$HOME", "%HOME%", "$(echo inert)", "../opaque"}
			config := map[string]any{
				"command": "node", "cwd": "${PLUGIN_DATA}", "args": args,
				"env": map[string]string{"ROOT": args[0], "DATA": args[1], "LITERAL": args[2], "UNKNOWN": args[3], "SHELL": args[4]},
			}
			if decoded {
				config = roundTripStdioConfig(t, config)
			}
			if err := ApplyStdioDataContract(config, active, data); err != nil {
				t.Fatal(err)
			}
			actual := roundTripStdioConfig(t, config)
			want := roundTripStdioConfig(t, map[string]any{
				"command": "node", "cwd": data,
				"args": []string{active + "/file", data + "/cache", "${package.root}/literal", "${UNKNOWN}", "$HOME", "%HOME%", "$(echo inert)", "../opaque"},
				"env": map[string]string{
					"ROOT": active + "/file", "DATA": data + "/cache", "LITERAL": "${package.root}/literal",
					"UNKNOWN": "${UNKNOWN}", "SHELL": "$HOME", "PLUGIN_ROOT": active, "PLUGIN_DATA": data,
				},
			})
			if !reflect.DeepEqual(actual, want) {
				t.Fatalf("projected config = %#v; want %#v", actual, want)
			}
		})
	}
}

func TestApplyStdioDataContractRejectsReservedEnvironmentBeforeExpansion(t *testing.T) {
	for _, reserved := range []string{"PLUGIN_ROOT", "PLUGIN_DATA"} {
		t.Run(reserved, func(t *testing.T) {
			for _, env := range []any{map[string]string{reserved: "foreign"}, map[string]any{reserved: "foreign"}} {
				config := map[string]any{"command": "node", "args": []string{"${PLUGIN_ROOT}/literal"}, "env": env}
				before := roundTripStdioConfig(t, config)
				err := ApplyStdioDataContract(config, t.TempDir(), t.TempDir())
				if err == nil || !strings.Contains(err.Error(), reserved+" is reserved") {
					t.Fatalf("reserved environment error = %v", err)
				}
				if !reflect.DeepEqual(roundTripStdioConfig(t, config), before) {
					t.Fatal("reserved environment refusal mutated the input")
				}
			}
		})
	}
}

func roundTripStdioConfig(t *testing.T, config map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
