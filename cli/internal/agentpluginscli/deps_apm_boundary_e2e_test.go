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

// The real deps lock command must reject an oversized pre-existing agreement
// *before* source acquisition, receipt publication or lock replacement.
// Both manifest and lock entry points are checked on every qualification OS.
func TestDependencyAgreementOversizedCLIRefusesWithoutSideEffects(t *testing.T) {
	for _, which := range []string{"uap.toml", "uap.lock"} {
		t.Run(which, func(t *testing.T) {
			fixture, source := dependencyLockRaceFixture(t)
			manifest, lock := fixture.app.dependencyPaths()
			path, limit := manifest, maxDependencyManifestBytes
			if which == "uap.lock" {
				path, limit = lock, maxDependencyLockBytes
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Truncate(limit + 1); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			acquired := false
			source.beforeSnapshot = func() error {
				acquired = true
				return fmt.Errorf("untrusted source acquired before oversized agreement refusal")
			}
			_, _, err = fixture.execute(false, "deps", "lock", "--format", "json")
			if err == nil || !strings.Contains(err.Error(), "exceeds") {
				t.Fatalf("oversized %s did not refuse at admission: %v", which, err)
			}
			if acquired {
				t.Fatal("oversized agreement attempted untrusted acquisition")
			}
			after, err := os.Stat(path)
			if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("oversized %s was changed by refusal: %v", which, err)
			}
			for _, candidate := range []string{fixture.store.Path, fixture.app.EnvironmentStoreRoot} {
				if _, statErr := os.Lstat(candidate); !os.IsNotExist(statErr) {
					t.Fatalf("oversized agreement published persistent state at %s: %v", candidate, statErr)
				}
			}
		})
	}
}
