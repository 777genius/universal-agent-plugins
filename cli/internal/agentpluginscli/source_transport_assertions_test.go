package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Expectations come from the authored fixture input, independently of the
// resolver, immutable store, lock and installer output.
func directExpectedPackage(t *testing.T, root string) map[string]string {
	t.Helper()
	expected := map[string]string{}
	directWalkPackage(t, root, func(name string, body []byte) error {
		expected[name] = string(body)
		return nil
	})
	if expected["plugin.json"] == "" {
		t.Fatal("expected package is empty")
	}
	return expected
}

func directAssertInstalledPackage(t *testing.T, fixture cliFixture, expected map[string]string) {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("expected one actual installed binding: %+v %v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		for name, want := range expected {
			got := dependencyRead(t, filepath.Join(binding.TargetLocator, filepath.FromSlash(name)))
			// The native stager emits normalized metadata; skill/resource bytes
			// remain exact, while metadata must preserve the complete JSON value.
			if name == "plugin.json" || name == "mcp.json" {
				var actual, original any
				if err := json.Unmarshal([]byte(got), &actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(want), &original); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, original) {
					t.Fatalf("wrong installed %s JSON: got %s want %s", name, got, want)
				}
			} else if got != want {
				t.Fatalf("wrong installed %s bytes: got %q want %q", name, got, want)
			}
		}
		if _, wantRevision := expected["revision.txt"]; !wantRevision {
			if _, err := os.Stat(filepath.Join(binding.TargetLocator, "revision.txt")); !os.IsNotExist(err) {
				t.Fatalf("initial package contains unexpected updated revision: %v", err)
			}
		}
	}
}
