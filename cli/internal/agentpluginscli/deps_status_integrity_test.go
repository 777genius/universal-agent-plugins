package agentpluginscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func newDependencyStatusIntegrityFixture(t *testing.T) cliFixture {
	t.Helper()
	f := newDependencyCheckFixture(t)
	var err error
	f.app.Installer, err = installer.New(installer.Config{
		StateRoot: f.app.DependenciesRoot, StateFile: f.store.Path,
		Registry: f.app.ClientRegistry,
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func executeDependencyStatusIntegrity(t *testing.T, f cliFixture) dependencyStatus {
	t.Helper()
	out, stderr, err := f.execute(false, "deps", "status", "--format", "json")
	if err != nil {
		t.Fatalf("descriptive status failed: %v stdout=%s stderr=%s", err, out, stderr)
	}
	var status dependencyStatus
	if err := json.Unmarshal([]byte(out), &status); err != nil {
		t.Fatalf("status JSON: %v %s", err, out)
	}
	mp, lp := f.app.dependencyPaths()
	if status.Scope != "user" || status.ManifestPath != mp || status.LockPath != lp || status.ManifestDigest == "" {
		t.Fatalf("status lost existing environment identity fields: %+v", status)
	}
	return status
}

func TestDependencyStatusIntegrityLockDiagnostics(t *testing.T) {
	for _, name := range []string{"missing", "malformed", "conflicted", "stale"} {
		t.Run(name, func(t *testing.T) {
			f := newDependencyStatusIntegrityFixture(t)
			pin := seedDependencyCheckPackage(t, f, "demo", "")
			lock := writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
			_, lp := f.app.dependencyPaths()
			code, outcome := "lock_invalid", dependencyCheckInvalid
			var err error
			switch name {
			case "missing":
				err = os.Remove(lp)
				code, outcome = "lock_missing", dependencyCheckUnavailable
			case "malformed":
				dependencyWrite(t, lp, `{"lock_version":`)
			case "conflicted":
				dependencyWrite(t, lp, "<<<<<<< HEAD\r\n{}\r\n=======\r\n{}\r\n>>>>>>> other\r\n")
			case "stale":
				lock.ManifestDigest = "sha256:" + strings.Repeat("b", 64)
				err = writeDependencyLock(lp, lock)
				code = "lock_stale"
			}
			if err != nil {
				t.Fatal(err)
			}
			before := observeDependencyCheckFiles(t, f.root)
			status := executeDependencyStatusIntegrity(t, f)
			if len(status.Diagnostics) != 1 {
				t.Fatalf("expected one additive lock diagnostic: %+v", status)
			}
			issue := status.Diagnostics[0]
			if issue.Stage != "lock" || issue.Code != code || issue.Outcome != outcome || issue.Path != lp || issue.Message == "" {
				t.Fatalf("incorrect lock diagnostic: %+v", issue)
			}
			if len(status.Facts) != 1 || !slices.Equal(status.Facts[0].Facts, []string{"not_synced"}) {
				t.Fatalf("diagnostic replaced the existing not_synced fact: %+v", status.Facts)
			}
			out, stderr, err := f.execute(false, "deps", "status")
			if err != nil || !strings.Contains(out, "("+code+")") || !strings.Contains(out, "environment: not_synced") {
				t.Fatalf("descriptive human status: %v stdout=%s stderr=%s", err, out, stderr)
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

func TestDependencyStatusIntegrityStoreClassification(t *testing.T) {
	for _, name := range []string{"healthy", "missing", "corrupt", "unreadable"} {
		t.Run(name, func(t *testing.T) {
			f := newDependencyStatusIntegrityFixture(t)
			pin := seedDependencyCheckPackage(t, f, "demo", "")
			writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
			stored, err := f.app.dependencyStore().Get(t.Context(), pin.DigestAlgorithm, pin.TreeDigest)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"not_synced"}
			switch name {
			case "missing":
				err = os.RemoveAll(f.app.EnvironmentStoreRoot)
				want = []string{"store_missing", "not_synced"}
			case "corrupt":
				dependencyWrite(t, filepath.Join(filepath.Dir(stored.Root), "metadata.json"), `{"schema_version":1,"schema_version":1}`)
				want = []string{"store_mismatch", "not_synced"}
			case "unreadable":
				// A non-directory ancestor is an actual operational read failure on
				// every platform, including hosts where chmod cannot deny root.
				blocked := filepath.Join(f.root, "store-obstruction")
				dependencyWrite(t, blocked, "foreign file")
				f.app.EnvironmentStoreRoot = filepath.Join(blocked, "store")
				want = []string{"store_unreadable", "not_synced"}
			}
			if err != nil {
				t.Fatal(err)
			}
			before := observeDependencyCheckFiles(t, f.root)
			status := executeDependencyStatusIntegrity(t, f)
			if len(status.Diagnostics) != 0 || len(status.Facts) != 1 {
				t.Fatalf("valid lock classification changed with cache availability: %+v", status)
			}
			fact := status.Facts[0]
			if fact.Alias != "demo" || fact.ClientID != "codex" || !slices.Equal(fact.Facts, want) {
				t.Fatalf("store %s misclassified: %+v, want %v", name, fact, want)
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

func TestDependencyStatusIntegrityStoreCancellation(t *testing.T) {
	f := newDependencyStatusIntegrityFixture(t)
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	before := observeDependencyCheckFiles(t, f.root)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	expired, release := context.WithDeadline(t.Context(), time.Time{})
	defer release()
	for _, ctx := range []context.Context{canceled, expired} {
		fact := dependencyStatusFact{Alias: pin.Alias, PackageDigest: pin.TreeDigest}
		appendStoreStatus(ctx, f.app, &fact, pin.DigestAlgorithm)
		if !slices.Equal(fact.Facts, []string{"store_canceled"}) {
			t.Fatalf("canceled store observation became missing/corrupt/unreadable: %+v", fact)
		}
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
}
