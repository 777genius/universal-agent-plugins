package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/dependencymanifest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Historical state is created through the single-package SDK lifecycle, whose
// standard decoder preserves opaque JSON extensions. The old lock records the
// real legacy -> child edge; normal managed resolution never pretends it is a leaf.
func legacyCleanupFixture(t *testing.T, withOther bool) freshRestoreEnvironment {
	t.Helper()
	fixture, _, _ := dependencyReviewFixture(t)
	revision := strings.Repeat("1", 40)
	aliases := []string{"legacy", "child"}
	if withOther {
		aliases = append(aliases, "other")
	}
	packages := make([]environment.LockedPackage, 0, len(aliases))
	for _, alias := range aliases {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "skills", alias), 0700); err != nil {
			t.Fatal(err)
		}
		manifest := map[string]any{"$schema": domain.PluginSchemaV1, "name": alias, "version": "1.0.0", "description": "Historical cleanup fixture"}
		if alias == "legacy" {
			manifest["extensions"] = map[string]any{"io.github.777genius.agentplugins": map[string]any{"dependencies": map[string]string{"acme/child": "^1.0"}}}
		}
		body, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		dependencyWrite(t, filepath.Join(root, "plugin.json"), string(body))
		dependencyWrite(t, filepath.Join(root, "skills", alias, "SKILL.md"), "---\nname: "+alias+"\ndescription: Historical fixture\n---\nFixture instructions.\n")
		identity := githubIdentity("acme/"+alias, revision, "")
		source := domain.SourceIdentity{RequestedSource: identity, CanonicalSource: identity, Repository: "acme/" + alias, ResolvedRevision: revision}
		snapshot, err := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, source)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = packagedigest.Remove(snapshot) })
		envelope, err := fixture.app.PackageLoader.Load(context.Background(), domain.LoadInput{SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: source})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.app.dependencyStore().Put(context.Background(), snapshot); err != nil {
			t.Fatal(err)
		}
		pin := environment.LockedPackage{Alias: alias, Version: "1.0.0", SourceKey: "github:acme/" + alias + ":", DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: snapshot.TreeDigest, ManifestDigest: envelope.ManifestDigest, ExecutableFiles: snapshot.ExecutableFiles, Targets: []string{"codex"}, Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: "acme/" + alias, Requested: environment.Selector{Kind: environment.SelectorCommit, Value: revision}, Commit: revision}}}
		// The legacy JSON edge uses a registry child, as the historical engine did.
		if alias == "child" {
			pin.SourceKey = "directory:acme/child"
			pin.Source = environment.LockedSource{Registry: &environment.LockedRegistrySource{Distribution: "acme/child", DistributionID: "acme/child", ProductID: "child", ReleaseSequence: 1, Requested: environment.Selector{Kind: environment.SelectorRelease, Value: "1"}, Repository: "acme/child", Commit: revision}}
			pin.RequiredBy = []environment.RequiredBy{{Alias: "legacy", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^1.0"}}}
		}
		acquired := &domain.AcquiredSnapshot{OriginMode: domain.OriginModeDirect, Source: source, TreeDigest: snapshot.TreeDigest, DigestAlgorithm: domain.TreeDigestAlgorithm}
		if alias == "child" {
			acquired.OriginMode = domain.OriginModeDirectory
			acquired.Directory = &domain.DirectoryOrigin{ProductID: "child", DistributionID: "acme/child", DistributionKind: domain.DistributionUpstream, DesiredReleaseSequence: 1, SnapshotSchema: 1, SnapshotSequence: 1, SnapshotDigest: "sha256:" + strings.Repeat("a", 64)}
		}
		prepared, err := fixture.app.Installer.Prepare(context.Background(), installer.Request{Operation: installer.OpInstall, OperationID: "historical-" + alias, ClientID: "codex", ClientConfigRoot: fixture.client.ConfigRoot, ClientExecutable: fixture.client.ExecutablePath, PackageRoot: snapshot.Root,
			AcquiredSnapshot:     acquired,
			Assessment:           &installer.Assessment{TreeDigest: snapshot.TreeDigest, Outcome: installer.AssessmentAllow},
			EnvironmentAuthority: &domain.EnvironmentAuthority{EnvironmentID: environment.EnvironmentUserDefault, Alias: alias, ClientID: "codex", SourceKey: pin.SourceKey, ExpectedAction: "install"}})
		if err != nil {
			t.Fatal(err)
		}
		result, err := fixture.app.Installer.Apply(context.Background(), prepared, installer.Decision{Confirmed: true})
		_ = prepared.Close()
		if err != nil || result.Outcome != installer.OutcomeCompleted {
			t.Fatalf("historical install %s: %+v %v", alias, result, err)
		}
		packages = append(packages, pin)
	}
	req := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{"codex": {"legacy"}}}
	req.Dependencies["legacy"] = environment.Requirement{Alias: "legacy", Git: &environment.GitSource{Repository: "acme/legacy", Selector: environment.Selector{Kind: environment.SelectorCommit, Value: revision}}}
	manifest := "schema_version = 1\n[dependencies.legacy]\ngit = \"acme/legacy\"\ncommit = \"" + revision + "\"\n"
	if withOther {
		req.Dependencies["other"] = environment.Requirement{Alias: "other", Git: &environment.GitSource{Repository: "acme/other", Selector: environment.Selector{Kind: environment.SelectorCommit, Value: revision}}}
		req.Targets["codex"] = append(req.Targets["codex"], "other")
		manifest += "[dependencies.other]\ngit = \"acme/other\"\ncommit = \"" + revision + "\"\n"
	}
	manifest += "[targets]\ncodex = [\"legacy\""
	if withOther {
		manifest += ", \"other\""
	}
	manifest += "]\n"
	digest, err := environment.ManifestDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	lock := environment.Lock{LockVersion: 1, ManifestDigest: digest, Packages: packages}
	if err := lock.Validate(req); err != nil {
		t.Fatal(err)
	}
	mp, lp := fixture.app.dependencyPaths()
	dependencyWrite(t, mp, manifest)
	if err := writeDependencyLock(lp, lock); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.EnvironmentReceipts) != len(aliases) {
		t.Fatalf("historical owned receipts: %+v %v", state, err)
	}
	return fixture
}

func TestDependencyLegacyOwnedCleanup(t *testing.T) {
	t.Run("removed closure needs no old package bytes", func(t *testing.T) {
		fixture := legacyCleanupFixture(t, false)
		// Delete only this disposable fixture's immutable cache. The installer
		// retains its actual owned projection and receipts for authorized removal.
		if err := os.RemoveAll(fixture.app.EnvironmentStoreRoot); err != nil {
			t.Fatal(err)
		}
		downloads, lookups := fixture.source.downloads, fixture.source.lookups
		dependencyRun(t, fixture.cliFixture, "remove", "legacy", "--external-uninstalled", "--format", "json")
		if fixture.source.downloads != downloads || fixture.source.lookups != lookups {
			t.Fatal("cleanup reacquired a removed historical package")
		}
		state, err := fixture.store.Load()
		if err != nil || len(state.EnvironmentReceipts) != 0 {
			t.Fatalf("removed closure retained owner receipts: %+v %v", state, err)
		}
		for _, installation := range state.Installations {
			if len(installation.Clients) != 0 {
				t.Fatalf("cleanup retained live bindings: %+v", installation.Clients)
			}
		}
		lock := freshRestoreReadLock(t, fixture.cliFixture)
		if len(lock.Packages) != 0 {
			t.Fatalf("removed closure remains locked: %+v", lock)
		}
	})
	t.Run("retained legacy refuses before publication", func(t *testing.T) {
		fixture := legacyCleanupFixture(t, true)
		before := freshRestoreFiles(t, fixture.root, fixture.client.ConfigRoot)
		mutations := fixture.runner.mutations
		_, _, err := fixture.execute(false, "remove", "other", "--external-uninstalled", "--format", "json")
		if !errors.Is(err, dependencymanifest.ErrDependencyDeclarationMoved) {
			t.Fatalf("retained legacy refusal: %v", err)
		}
		if !reflect.DeepEqual(before, freshRestoreFiles(t, fixture.root, fixture.client.ConfigRoot)) || fixture.runner.mutations != mutations {
			t.Fatal("retained legacy refusal changed manifest, lock, state or profile")
		}
	})
	t.Run("owned pending recovery keeps completion contract", func(t *testing.T) {
		fixture := legacyCleanupFixture(t, false)
		state, err := fixture.store.Load()
		if err != nil {
			t.Fatal(err)
		}
		for i := range state.Installations {
			for key, binding := range state.Installations[i].Clients {
				binding.Receipts[len(binding.Receipts)-1].Phase = "state_committed"
				state.Installations[i].Clients[key] = binding
			}
		}
		if err := fixture.store.Save(state); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(fixture.app.EnvironmentStoreRoot); err != nil {
			t.Fatal(err)
		}
		downloads, lookups := fixture.source.downloads, fixture.source.lookups
		dependencyRun(t, fixture.cliFixture, "deps", "recover", "--yes", "--format", "json")
		view, err := fixture.app.Installer.Inspect(context.Background())
		if err != nil || view.Recovery.Required || len(view.EnvironmentReceipts) != 2 {
			t.Fatalf("historical recovery incomplete: %+v %v", view, err)
		}
		if fixture.source.downloads != downloads || fixture.source.lookups != lookups {
			t.Fatal("recovery acquired removed legacy metadata")
		}
	})
}
