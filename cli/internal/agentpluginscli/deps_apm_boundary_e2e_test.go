package agentpluginscli

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// The real Cobra deps lock command must reject nonportable source intent before
// it acquires external bytes or writes any lock/store/installer state. This is
// particularly important on Windows, where reserved names can alias host files.
func TestSavedSourceAdmissionRefusalDoesNotPublish(t *testing.T) {
	for _, tc := range []struct {
		name, ref, path string
	}{
		{name: "reflog-expression", ref: "branch@{1}"},
		{name: "double-dot-ref", ref: "branch..main"},
		{name: "lock-suffix-ref", ref: "refs/tags/old.lock"},
		{name: "windows-device-path", path: "plugins/CON"},
		{name: "windows-device-extension", path: "plugins/COM1.txt"},
		{name: "trailing-dot-path", path: "plugins/example."},
		{name: "path-traversal", path: "plugins/../outside"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, source := dependencyLockRaceFixture(t)
			manifest, lock := fixture.app.dependencyPaths()
			selector := "commit = \"" + strings.Repeat("a", 40) + "\""
			if tc.ref != "" {
				selector = fmt.Sprintf("ref = %q", tc.ref)
			}
			original := fmt.Sprintf("schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\npath = %q\n%s\n[targets]\ngemini = [\"demo\"]\n", tc.path, selector)
			dependencyWrite(t, manifest, original)
			source.beforeSnapshot = func() error {
				t.Fatal("untrusted source was acquired before validation")
				return nil
			}
			_, _, err := fixture.execute(false, "deps", "lock", "--format", "json")
			if err == nil {
				t.Fatalf("deps lock accepted unsafe source input %q / %q", tc.ref, tc.path)
			}
			if got := dependencyRead(t, manifest); got != original {
				t.Fatalf("invalid-source refusal rewrote uap.toml: %q", got)
			}
			for _, file := range []string{lock, fixture.store.Path, fixture.app.EnvironmentStoreRoot} {
				if _, statErr := os.Lstat(file); !os.IsNotExist(statErr) {
					t.Fatalf("unsafe source created persistent state at %s: %v", file, statErr)
				}
			}
		})
	}
}
