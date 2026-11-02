//go:build uappm_e2e

package agentpluginscli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The checked-in agreement can advance while the installed environment still
// contains the old release. Its first locked install must use the new version,
// digest and bytes; a second invocation must not repair stale metadata.
func pmEvolutionOldCacheNewLock(t *testing.T) {
	origin := newPMArchiveOrigin(t, pmArchiveGraph{"a": nil})
	installed := newPMArchiveEnvironment(t, origin, false)
	roots := []string{"a"}
	targets := map[string][]string{"gemini": roots}
	installed.manifest(t, origin, roots, targets)
	installed.run(t, "install", "--yes", "--format", "json")

	oldPin, ok := installed.lock(t).Package("a")
	if !ok || oldPin.Version != "1.0.0" {
		t.Fatalf("initial v1 pin missing: %+v", oldPin)
	}
	old := installed.assertInstalled(t, pmArchiveTargets{"a": {"gemini"}})
	if got := dependencyRead(t, filepath.Join(old["a/gemini"].TargetPath, "marker.txt")); got != pmArchiveMarker("a") {
		t.Fatalf("initial payload wrong: %q", got)
	}
	const newMarker = "independent ZIP payload for a version 2.0.0\n"
	const newSkill = "---\nname: a\ndescription: Disposable graph marker\n---\n" + newMarker
	v2 := fmt.Sprintf("{\"$schema\":%q,\"name\":\"a\",\"version\":\"2.0.0\",\"description\":\"Disposable package-manager graph\"}", domain.PluginSchemaV1)
	pmRewriteZIP(t, origin, "a", map[string]string{
		"publisher/plugin.json":       v2,
		"publisher/marker.txt":        newMarker,
		"publisher/skills/a/SKILL.md": newSkill,
	})

	// A separate checkout independently resolves v2. Copy only the portable
	// manifest/lock over the old installation (not its store or receipts).
	other := newPMArchiveEnvironment(t, origin, false)
	other.manifest(t, origin, roots, targets)
	other.run(t, "deps", "lock", "--format", "json")
	newPin, ok := other.lock(t).Package("a")
	origin.mu.Lock()
	sum := sha256.Sum256(origin.payloads["/a.zip"])
	origin.mu.Unlock()
	wantSHA := hex.EncodeToString(sum[:])
	if !ok || newPin.Source.Archive == nil || newPin.Version != "2.0.0" ||
		newPin.Source.Archive.SHA256 != wantSHA || newPin.TreeDigest == oldPin.TreeDigest ||
		newPin.ManifestDigest == oldPin.ManifestDigest {
		t.Fatalf("independently resolved v2 pin wrong: %+v", newPin)
	}
	pmArchiveAgreement(t, other, installed)
	mp, lp := installed.app.dependencyPaths()
	portable := freshRestoreFiles(t, mp, lp)
	requests := origin.counts()
	installed.run(t, "install", "--locked", "--yes", "--format", "json")
	if origin.counts()["GET /a.zip"] <= requests["GET /a.zip"] {
		t.Fatal("old cache incorrectly fulfilled a different v2 archive pin")
	}
	actualPin, ok := installed.lock(t).Package("a")
	if !ok || actualPin.Version != newPin.Version || actualPin.TreeDigest != newPin.TreeDigest ||
		actualPin.ManifestDigest != newPin.ManifestDigest || actualPin.Source.Archive == nil ||
		actualPin.Source.Archive.SHA256 != wantSHA {
		t.Fatalf("first replay retained stale pin or version: %+v", actualPin)
	}
	pmArchiveAssertEqual(t, "portable branch agreement", portable, freshRestoreFiles(t, mp, lp))
	view, err := installed.app.Installer.Inspect(t.Context())
	if err != nil || view.Recovery.Required || len(view.EnvironmentReceipts) != 1 {
		t.Fatalf("inconsistent installed state: %+v %v", view, err)
	}
	var bindings int
	for _, installation := range view.Installations {
		for _, binding := range installation.Bindings {
			bindings++
			if binding.Owner == nil || binding.Owner.Alias != "a" ||
				binding.Owner.PackageDigest != newPin.TreeDigest ||
				binding.TreeDigest != newPin.TreeDigest || binding.Revision != "sha256:"+wantSHA ||
				binding.Activation != "active" || binding.Verification != "installation_verified" {
				t.Fatalf("old version/receipt survived: %+v", binding)
			}
			if got := dependencyRead(t, filepath.Join(binding.TargetPath, "marker.txt")); got != newMarker {
				t.Fatalf("old managed bytes survived first replay: %q", got)
			}
			if got := dependencyRead(t, filepath.Join(binding.TargetPath, "plugin.json")); got != v2 {
				t.Fatalf("materialized package manifest differs from v2: %q", got)
			}
			if got := dependencyRead(t, filepath.Join(binding.TargetPath, "skills", "a", "SKILL.md")); got != newSkill {
				t.Fatalf("materialized Skill differs from exact v2 bytes: %q", got)
			}
		}
	}
	if bindings != 1 {
		t.Fatalf("expected exactly one v2 binding, got %d", bindings)
	}
	if got := dependencyRead(t, filepath.Join(installed.clients["gemini"].ConfigRoot, "skills", "a", "SKILL.md")); got != newSkill {
		t.Fatalf("native Skill not replaced with v2: %q", got)
	}
	settings := dependencyRead(t, filepath.Join(installed.clients["gemini"].ConfigRoot, "settings.json"))
	if !strings.Contains(settings, "preserve foreign settings") || !strings.Contains(settings, "https://foreign.test/mcp") {
		t.Fatal("first replay damaged foreign native Gemini settings")
	}

	// Verify the outcome did not require a second install to correct version
	// metadata, and an intact repeat performs zero source acquisition/writes.
	before := installed.activeFiles(t)
	requests = origin.counts()
	installed.run(t, "install", "--locked", "--yes", "--format", "json")
	pmArchiveAssertEqual(t, "repeat agreement", portable, freshRestoreFiles(t, mp, lp))
	pmArchiveAssertEqual(t, "repeat native files/receipts", before, installed.activeFiles(t))
	pmArchiveAssertEqual(t, "repeat source requests", requests, origin.counts())
}

// A failed install can retain a partial package receipt. Check persisted
// native-object claims independently of the existing Skill bytes: only the
// original package can own "shared" and a cold profile must not own it at all.
func pmAssertSharedNativeOwnership(t *testing.T, f pmArchiveEnvironment, expectedAlias string) {
	t.Helper()
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.ClientID == "gemini" {
			ids[receipt.Alias] = receipt.InstallationID
		}
	}
	count := 0
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if binding.ClientID != "gemini" {
				continue
			}
			for _, object := range binding.NativeObjects {
				if object.LogicalName != "shared" {
					continue
				}
				count++
				wantPath := filepath.Join(f.clients["gemini"].ConfigRoot, "skills", "shared")
				if expectedAlias == "" || installation.InstallationID != ids[expectedAlias] ||
					object.Path != wantPath {
					t.Fatalf("unexpected shared Skill native claim: installation=%q object=%+v expected alias=%q",
						installation.InstallationID, object, expectedAlias)
				}
			}
		}
	}
	wantCount := 0
	if expectedAlias != "" {
		wantCount = 1
	}
	if count != wantCount || expectedAlias != "" && ids[expectedAlias] == "" {
		t.Fatalf("shared Skill claims=%d, want=%d; expected alias receipt=%q", count, wantCount, ids[expectedAlias])
	}
}

// Two independently selected sources contain the same native Skill. Neither
// same bytes nor absence of user receipts grants authority to adopt or delete
// a committed Skill in a fresh disposable profile.
func pmAdversarialColdSharedSkillOwners(t *testing.T) {
	origin := newPMArchiveOrigin(t, pmArchiveGraph{"a": nil, "b": nil})
	const shared = "---\nname: shared\ndescription: Shared source Skill\n---\nIdentical and independently retained.\n"
	for _, name := range []string{"a", "b"} {
		pmRewriteZIP(t, origin, name, nil, pmZIPMember{
			name: "publisher/skills/shared/SKILL.md", body: shared, mode: 0600,
		})
	}
	original := newPMArchiveEnvironment(t, origin, false)
	original.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	original.run(t, "install", "--yes", "--format", "json")
	sharedPath := filepath.Join(original.clients["gemini"].ConfigRoot, "skills", "shared", "SKILL.md")
	if body := dependencyRead(t, sharedPath); body != shared {
		t.Fatalf("first owner did not materialize common native Skill: %q", body)
	}
	pmAssertSharedNativeOwnership(t, original, "a")
	before := freshRestoreFiles(t, sharedPath)
	original.manifest(t, origin, []string{"a", "b"}, map[string][]string{"gemini": {"a", "b"}})
	original.run(t, "deps", "lock", "--format", "json")
	original.assertGraph(t, origin, pmArchiveGraph{"a": nil, "b": nil},
		pmArchiveTargets{"a": {"gemini"}, "b": {"gemini"}})
	if _, err := original.executePM(t.Context(), t, "", false, "install", "--locked", "--yes", "--format", "json"); err == nil ||
		!strings.Contains(err.Error(), "Gemini skill \"shared\" already exists without agentplugins ownership") {
		t.Fatalf("the shared-Skill ownership guard was not the actual refusal: %v", err)
	}
	pmAssertSharedNativeOwnership(t, original, "a")
	pmArchiveAssertEqual(t, "original owner remained intact", before, freshRestoreFiles(t, sharedPath))

	cold := newPMArchiveEnvironment(t, origin, false)
	pmArchiveAgreement(t, original, cold)
	coldShared := filepath.Join(cold.clients["gemini"].ConfigRoot, "skills", "shared", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(coldShared), 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, coldShared, shared)
	if _, err := os.Lstat(cold.store.Path); !os.IsNotExist(err) {
		t.Fatalf("cold profile unexpectedly has receipts: %v", err)
	}
	mp, lp := cold.app.dependencyPaths()
	portable := freshRestoreFiles(t, mp, lp)
	settings := filepath.Join(cold.clients["gemini"].ConfigRoot, "settings.json")
	native := freshRestoreFiles(t, coldShared, settings)
	if _, err := cold.executePM(t.Context(), t, "", false, "install", "--locked", "--yes", "--format", "json"); err == nil ||
		!strings.Contains(err.Error(), "Gemini skill \"shared\" already exists without agentplugins ownership") {
		t.Fatalf("cold profile did not refuse an independently supplied shared Skill by ownership: %v", err)
	}
	pmAssertSharedNativeOwnership(t, cold, "")
	pmArchiveAssertEqual(t, "committed Skill and unrelated settings", native, freshRestoreFiles(t, coldShared, settings))
	pmArchiveAssertEqual(t, "cold portable agreement", portable, freshRestoreFiles(t, mp, lp))
}

// Lock resolution is an agreement-only operation, not an implicit uninstall.
// APM #2296 is an adversarial reminder: even dropping the last declared root
// must not delete its already-deployed package, native Skill or foreign config.
// The actual CLI and installer are used with a disposable HTTPS ZIP origin.
func pmAgreementLockOnlyPreservesInstalled(t *testing.T) {
	origin := newPMArchiveOrigin(t, pmArchiveGraph{"a": nil, "b": nil})
	fixture := newPMArchiveEnvironment(t, origin, false)
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	fixture.run(t, "install", "--yes", "--format", "json")
	fixture.assertInstalled(t, pmArchiveTargets{"a": {"gemini"}})
	before := fixture.activeFiles(t)
	_, lockPath := fixture.app.dependencyPaths()
	previousLock := dependencyRead(t, lockPath)

	// Deliberately change the portable agreement without applying it.
	fixture.manifest(t, origin, []string{"b"}, map[string][]string{"gemini": {"b"}})
	fixture.run(t, "deps", "lock", "--format", "json")
	next := fixture.lock(t)
	if _, present := next.Package("a"); present {
		t.Fatal("new lock retained removed root a")
	}
	if _, present := next.Package("b"); !present {
		t.Fatal("new lock omitted replacement root b")
	}
	if dependencyRead(t, lockPath) == previousLock {
		t.Fatal("lock-only command did not publish new dependency intent")
	}
	pmArchiveAssertEqual(t, "lock-only preserved deployed bytes, native Skill, foreign settings and receipts", before, fixture.activeFiles(t))

	// Repeating the exact agreement is idempotent for both lock bytes and
	// the materialized environment; a no-op must not rewrite timestamps.
	stableLock := freshRestoreFiles(t, lockPath)
	fixture.run(t, "deps", "lock", "--format", "json")
	pmArchiveAssertEqual(t, "unchanged lock bytes and metadata", stableLock, freshRestoreFiles(t, lockPath))
	pmArchiveAssertEqual(t, "unchanged native and managed files", before, fixture.activeFiles(t))
}
