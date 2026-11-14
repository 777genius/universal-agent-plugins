//go:build uappm_e2e

package agentpluginscli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Keep more packages than a fresh search admits. Retained payloads are real
// verified snapshots; the removed package deliberately has no cached bytes.
func pmIntegrityRetainedGraphContraction(t *testing.T) {
	f := pmIntegrityGraphFixture(t)
	pins, roots := pmIntegrityRecordedGraph("independent", 1026)
	for i := range 1025 {
		pins[i] = seedDependencyCheckPackage(t, f, roots[i], "")
	}
	writeDependencyCheckAgreement(t, f, pins, roots...)
	manifest, lockPath := f.app.dependencyPaths()
	previous := dependencyRead(t, lockPath)
	writeDependencyCheckAgreement(t, f, pins[:1025], roots[:1025]...)
	dependencyWrite(t, lockPath, previous)
	for _, root := range []string{f.app.UserHome, f.app.ManagedRoot} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		dependencyWrite(t, filepath.Join(root, "foreign-sentinel.txt"), "unrelated owner bytes\n")
	}
	before := pmIntegrityPersistentFiles(t, f)
	var preview dependencyLockResult
	if err := json.Unmarshal([]byte(dependencyRun(t, f, "deps", "lock", "--dry-run", "--format", "json")), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || !preview.Changed {
		t.Fatalf("contraction preview flags: %+v", preview)
	}
	pmAssertIndependentRetainedPins(t, preview.Lock, pins[:1025])
	pmArchiveAssertEqual(t, "large contraction dry-run files", before, pmIntegrityPersistentFiles(t, f))

	unrelated := freshRestoreFiles(t, f.app.EnvironmentStoreRoot, f.store.Path, f.app.ManagedRoot, f.operations, f.app.UserHome)
	dependencyRun(t, f, "deps", "lock", "--format", "json")
	requirements, err := readDependencyManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	locked, err := readDependencyLock(lockPath, requirements)
	if err != nil {
		t.Fatal(err)
	}
	pmAssertIndependentRetainedPins(t, locked, pins[:1025])
	pmArchiveAssertEqual(t, "large contraction unrelated bytes", unrelated, freshRestoreFiles(t, f.app.EnvironmentStoreRoot, f.store.Path, f.app.ManagedRoot, f.operations, f.app.UserHome))

	stable := pmIntegrityPersistentFiles(t, f)
	report, _ := executeDependencyCheck(t, f, dependencyCheckValid)
	packages := 0
	for _, check := range report.Checks {
		if check.Stage == "package" && check.Outcome == dependencyCheckValid {
			packages++
		}
	}
	if packages != 1025 {
		t.Fatalf("offline check verified %d retained packages; want 1025", packages)
	}
	dependencyRun(t, f, "deps", "lock", "--format", "json")
	pmArchiveAssertEqual(t, "large contraction replay/check bytes and mtimes", stable, pmIntegrityPersistentFiles(t, f))
}

func pmAssertIndependentRetainedPins(t *testing.T, lock environment.Lock, expected []environment.LockedPackage) {
	t.Helper()
	if len(lock.Packages) != len(expected) {
		t.Fatalf("retained packages=%d; want %d", len(lock.Packages), len(expected))
	}
	for i, pin := range lock.Packages {
		if pin.Alias != fmt.Sprintf("node%04d", i) || !environment.SamePin(pin, expected[i]) || !slices.Equal(pin.Targets, []string{"codex"}) || len(pin.RequiredBy) != 0 {
			t.Fatalf("retained independent pin %d changed: %+v", i, pin)
		}
	}
}
