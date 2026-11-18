package agentpluginscli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Real Go CLI, immutable Git acquisition, package store and installer journal.
// Only the native Codex process is a disposable fixture, never a model claim.
func TestDevelopmentRootsInstallAndColdLockedRestore(t *testing.T) {
	repo := t.TempDir()
	dependencyGit(t, repo, "init", "-b", "main")
	directory := &fixedDirectoryClient{bundle: directoryv1.VerifiedBundle{
		Digest: "sha256:" + strings.Repeat("a", 64),
		Snapshot: domain.DirectorySnapshot{
			SnapshotSchemaVersion: 1, Sequence: 1,
			SourceCommit:  strings.Repeat("a", 40),
			Products:      []domain.DirectoryProduct{},
			Distributions: []domain.DirectoryDistribution{},
			Evidence:      []domain.DirectoryEvidence{},
			Revocations:   []domain.DirectoryRevocation{},
		},
	}}
	source := newFreshRestoreEnvironment(t, repo, directory)
	freshRestorePackage(t, source, repo, "c", "1.2.0", "c", "", 1, true)
	freshRestorePackage(t, source, repo, "normal", "1.0.0", "normal", "^1.2", 1, true)
	freshRestorePackage(t, source, repo, "build", "1.0.0", "build", "^1.2", 1, true)
	manifest, lockPath := source.app.dependencyPaths()
	dependencyWrite(t, manifest, `schema_version = 1
[dependencies]
normal = "^1.0"
[dev-dependencies]
build = "^1.0"
[targets]
codex = ["normal", "build"]
`)
	dependencyRun(t, source.cliFixture, "install", "--yes", "--format", "json")
	first := freshRestoreReadLock(t, source.cliFixture)
	if len(first.Packages) != 3 {
		t.Fatalf("development root or shared transitive dependency not installed: %+v", first.Packages)
	}
	shared, ok := first.Package("c")
	if !ok || len(shared.RequiredBy) != 2 || shared.RequiredBy[0].Alias != "build" || shared.RequiredBy[1].Alias != "normal" {
		t.Fatalf("shared dependency lost prod/dev consumer edges: %+v", shared)
	}
	for _, alias := range []string{"normal", "build"} {
		pkg, ok := first.Package(alias)
		if !ok || len(pkg.Targets) != 1 || pkg.Targets[0] != "codex" || len(pkg.RequiredBy) != 0 {
			t.Fatalf("root %s missing native target or unexpectedly transitive: %+v", alias, pkg)
		}
	}
	manifestBytes, lockBytes := dependencyRead(t, manifest), dependencyRead(t, lockPath)
	cold := newFreshRestoreEnvironment(t, repo, directory)
	freshRestoreCopyAgreement(t, cold, manifestBytes, lockBytes)
	dependencyRun(t, cold.cliFixture, "install", "--locked", "--yes", "--format", "json")
	if got := freshRestoreReadLock(t, cold.cliFixture); !reflect.DeepEqual(first, got) {
		t.Fatalf("cold locked restore changed immutable graph: %+v", got)
	}
	if cold.source.lookups != 0 || len(cold.source.verified) != 3 {
		t.Fatalf("cold restore did not acquire exactly three immutable nodes: lookups=%d fetches=%v",
			cold.source.lookups, cold.source.verified)
	}
	if got := dependencyRead(t, manifest); got != manifestBytes {
		t.Fatal("first environment was rewritten by another environment")
	}
}
