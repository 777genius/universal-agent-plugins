package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/dependencymanifest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDependencyPackageManifestOversizedCLIRefuses(t *testing.T) {
	for _, operation := range []string{"lock", "install", "warm locked install"} {
		t.Run(operation, func(t *testing.T) {
			fixture, source := dependencyLockRaceFixture(t)
			packageManifest := "schema_version = 1\n#" + strings.Repeat("x", int(dependencymanifest.MaxBytes))
			dependencyWrite(t, filepath.Join(source.root, "uap.toml"), packageManifest)
			manifest, lock := fixture.app.dependencyPaths()
			before := dependencyRead(t, manifest)
			var lockBefore, metadataBefore, metadataPath string
			args := []string{"deps", "lock", "--format", "json"}
			if operation == "install" {
				args = []string{"install", "--yes", "--format", "json"}
			}
			if operation == "warm locked install" {
				metadataPath = seedOversizedPackageManifestCache(t, fixture, source.root)
				lockBefore, metadataBefore = dependencyRead(t, lock), dependencyRead(t, metadataPath)
				args = []string{"install", "--locked", "--yes", "--format", "json"}
				source.beforeSnapshot = func() error {
					t.Fatal("warm refusal must not reacquire the source")
					return nil
				}
			}
			_, _, err := fixture.execute(false, args...)
			if err == nil || !strings.Contains(err.Error(), "exceeds 4194304 bytes") {
				t.Fatalf("oversized embedded manifest was not refused: %v", err)
			}
			if dependencyRead(t, manifest) != before || dependencyRead(t, filepath.Join(source.root, "uap.toml")) != packageManifest {
				t.Fatal("refusal changed the root agreement or source sidecar")
			}
			absent := []string{fixture.store.Path, fixture.app.ManagedRoot}
			if operation == "warm locked install" {
				if dependencyRead(t, lock) != lockBefore || dependencyRead(t, metadataPath) != metadataBefore {
					t.Fatal("warm refusal changed the existing lock or cached metadata")
				}
			} else {
				absent = append(absent, lock, fixture.app.EnvironmentStoreRoot)
			}
			for _, candidate := range absent {
				if _, err := os.Lstat(candidate); !os.IsNotExist(err) {
					t.Fatalf("refusal published persistent data at %s: %v", candidate, err)
				}
			}
		})
	}
}

// Older releases admitted these bytes. Construct that exact saved agreement and
// store entry without using the newly tightened dependency-sidecar admission.
func seedOversizedPackageManifestCache(t *testing.T, fixture cliFixture, sourceRoot string) string {
	t.Helper()
	ctx := context.Background()
	manifest, lockPath := fixture.app.dependencyPaths()
	requirements, err := readDependencyManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := (packagedigest.Builder{}).Snapshot(ctx, sourceRoot, domain.SourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = packagedigest.Remove(snapshot) })
	envelope, err := fixture.app.loadStandardEnvelope(ctx, domain.LoadInput{SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles})
	if err != nil {
		t.Fatal(err)
	}
	key, err := requirements.Dependencies["demo"].SourceKey()
	if err != nil {
		t.Fatal(err)
	}
	pin := environment.LockedPackage{Alias: "demo", SourceKey: key, Version: envelope.Manifest.Version, DigestAlgorithm: snapshot.DigestAlgorithm, TreeDigest: snapshot.TreeDigest, ManifestDigest: envelope.ManifestDigest, ExecutableFiles: snapshot.ExecutableFiles, Targets: []string{"gemini"}, Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: "acme/demo", Commit: strings.Repeat("1", 40), Requested: environment.Selector{Kind: environment.SelectorCommit, Value: strings.Repeat("1", 40)}}}}
	digest, err := environment.ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	locked := environment.Lock{LockVersion: environment.LockVersion, ManifestDigest: digest, Packages: []environment.LockedPackage{pin}}
	if err := locked.Validate(requirements); err != nil {
		t.Fatal(err)
	}
	if err := writeDependencyLock(lockPath, locked); err != nil {
		t.Fatal(err)
	}
	snapshot.Source = identityFromLocked(pin)
	pkg, err := fixture.app.dependencyStore().Put(ctx, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(filepath.Dir(pkg.Root), "metadata.json")
}

func TestDependencyPackageManifestImperativeRefuses(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientGemini)})
	root := t.TempDir()
	aliasGraphPackage(t, root, "oversized")
	dependencyWrite(t, filepath.Join(root, "uap.toml"), "schema_version = 1\n#"+strings.Repeat("x", int(dependencymanifest.MaxBytes)))
	_, _, err := fixture.execute(false, "add", root, "--no-save", "--target", "gemini", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "exceeds 4194304 bytes") {
		t.Fatalf("imperative oversized sidecar admission: %v", err)
	}
	for _, candidate := range []string{fixture.store.Path, fixture.app.ManagedRoot} {
		if _, err := os.Lstat(candidate); !os.IsNotExist(err) {
			t.Fatalf("imperative refusal wrote %s: %v", candidate, err)
		}
	}
}

func TestDependencyPackageManifestBoundedRead(t *testing.T) {
	prefix := "schema_version = 1\n#"
	for _, containedLink := range []bool{false, true} {
		name := "regular"
		if containedLink {
			name = "contained link"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "uap.toml")
			if containedLink {
				target = filepath.Join(root, "author", "dependencies.toml")
				if err := os.Mkdir(filepath.Dir(target), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join("author", "dependencies.toml"), filepath.Join(root, "uap.toml")); err != nil {
					t.Skipf("contained symlink unavailable: %v", err)
				}
			}
			body := prefix + strings.Repeat("x", int(dependencymanifest.MaxBytes)-len(prefix))
			dependencyWrite(t, target, body)
			facts, err := packageDependencyFacts(domain.PackageEnvelope{SnapshotRoot: root})
			if err != nil || len(facts) != 0 {
				t.Fatalf("valid sidecar at the ceiling: %+v %v", facts, err)
			}
			dependencyWrite(t, target, body+"x")
			if _, err := packageDependencyFacts(domain.PackageEnvelope{SnapshotRoot: root}); err == nil || !strings.Contains(err.Error(), "exceeds 4194304 bytes") {
				t.Fatalf("oversized sidecar was not rejected: %v", err)
			}
		})
	}
}
