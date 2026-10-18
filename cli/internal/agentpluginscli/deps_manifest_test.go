package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestDependencyManifestRejectsUnknownConflictingAndMissingIntent(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"unknown", `schema_version = 1
[dependencies.review]
git = "acme/tools"
typo = true
[targets]
codex = ["review"]`, "unknown field"},
		{"selector conflict", `schema_version = 1
[dependencies.review]
git = "acme/tools"
commit = "0123456789012345678901234567890123456789"
ref = "main"
[targets]
codex = ["review"]`, "mutually exclusive"},
		{"missing target", `schema_version = 1
[dependencies.review]
git = "acme/tools"
ref = "main"
[targets]
codex = ["other"]`, "unknown alias"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "uap.toml")
			if err := os.WriteFile(path, []byte(test.body), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readDependencyManifest(path)
			if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), "uap.toml:") {
				t.Fatalf("manifest error = %v", err)
			}
		})
	}
}

func TestDependencyManifestNormalizesDefaultIntent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uap.toml")
	body := `schema_version = 1
[dependencies.review]
git = "acme/tools"
path = "plugins/review"
[targets]
codex = ["review"]
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	requirements, err := readDependencyManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if requirements.Dependencies["review"].Git.Selector.Kind != "default" {
		t.Fatalf("default selector = %+v", requirements.Dependencies["review"].Git.Selector)
	}
}

func TestDependencyShortDirectoryNameUsesExactReleaseAndOnlyAssignedTargets(t *testing.T) {
	fixture := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCodex}, []domain.ClientID{domain.ClientClaude})
	requirements := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{
		"docs":  {Alias: "docs", Registry: &environment.RegistrySource{Distribution: "rollout-demo", Selector: environment.Selector{Kind: environment.SelectorRelease, Value: "1"}}},
		"other": {Alias: "other", Git: &environment.GitSource{Repository: "acme/other", Selector: environment.Selector{Kind: environment.SelectorDefault}}},
	}, Targets: map[string][]string{"codex": {"docs"}, "claude": {"other"}}}
	canonical, err := fixture.cli.app.canonicalizeDependencyRequirements(context.Background(), requirements)
	if err != nil || canonical.Dependencies["docs"].Registry.Distribution != "owner/rollout" {
		t.Fatalf("exact short name: %+v %v", canonical, err)
	}
	requirements.Dependencies["docs"].Registry.Distribution = "rollout-demo"
	requirements.Dependencies["docs"].Registry.Selector.Value = "2"
	if _, err := fixture.cli.app.canonicalizeDependencyRequirements(context.Background(), requirements); err == nil {
		t.Fatal("superseded exact publication admitted as new install")
	}
}

func TestDependencyDirectoryRepairUsesInstalledReceiptNotLockEligibility(t *testing.T) {
	fixture := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCodex}, []domain.ClientID{domain.ClientCodex})
	loaded, err := fixture.cli.app.acquireLocal(context.Background(), fixture.v1.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = loaded.cleanup() }()
	distribution := &fixture.directory.bundle.Snapshot.Distributions[0]
	distribution.ReleasePolicies[0].Status = domain.ReleaseSuperseded
	release := distribution.Releases[0]
	locked := environment.LockedPackage{TreeDigest: release.TreeDigest, ManifestDigest: release.ManifestDigest, Targets: []string{"codex"}, Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{
		ProductID: distribution.ProductID, DistributionID: distribution.ID, Distribution: distribution.ID, ReleaseSequence: 1, Repository: release.PackageSource.Repository, Path: release.PackageSource.Path, Commit: release.PackageSource.Revision,
	}}}
	if _, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loaded, locked, nil); err != nil {
		t.Fatalf("exact locked restore rejected a superseded non-revoked release: %v", err)
	}
	if _, err := fixture.cli.app.acquireDirectory(context.Background(), distribution.ID, packageResolutionRequest{StandardOnly: true, Operation: domain.DirectoryInstall, DesiredReleaseSequence: 1, Targets: []domain.ClientID{domain.ClientCodex}}); !errors.Is(err, domain.ErrDirectoryIneligible) {
		t.Fatalf("ordinary fresh selection admitted a superseded release: %v", err)
	}
	recorded := &domain.RecordedDirectoryRelease{ProductID: distribution.ProductID, DistributionID: distribution.ID, ReleaseSequence: 1, Repository: release.PackageSource.Repository, Path: release.PackageSource.Path, ResolvedRevision: release.PackageSource.Revision, TreeDigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: release.TreeDigest, ManifestDigest: release.ManifestDigest}
	if _, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loaded, locked, recorded); err != nil {
		t.Fatalf("installed exact repair lost recorded eligibility: %v", err)
	}
	if _, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loaded, locked, recorded, domain.DirectoryNewTarget); err != nil {
		t.Fatalf("live installed new-target eligibility: %v", err)
	}
	distribution.ReleasePolicies[0].Status = domain.ReleaseRevoked
	if _, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loaded, locked, nil); !errors.Is(err, domain.ErrDirectoryIneligible) {
		t.Fatalf("exact lock bypassed revoked-release policy: %v", err)
	}
}

func TestDependencyManifestErrorLocations(t *testing.T) {
	cases := []struct {
		name, body string
		line       int
	}{
		{"syntax", "# prior line\nschema_version = [\n", 2},
		{"type", "# comment\nschema_version = \"wrong\"\n", 2},
		{"unknown key ignores comments", "# unknown typo\nschema_version = 1\n[dependencies.review]\ngit = \"acme/tools\"\ntypo = true\n", 5},
		{"quoted table selector conflict", "# dependencies.review: misleading\nschema_version = 1\n[dependencies.\"review\"]\ngit = \"acme/tools\"\nref = \"main\"\ncommit = \"0123456789012345678901234567890123456789\"\n", 3},
		{"release ignores unrelated comments", "# release mentioned here\nschema_version = 1\n[dependencies.review]\nregistry = \"uap\"\ndistribution = \"owner/plugin\"\nrelease = false\n", 3},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "uap.toml")
			if err := os.WriteFile(path, []byte(tt.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := readDependencyManifest(path)
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("uap.toml:%d:", tt.line)) {
				t.Fatalf("location: %v", err)
			}
		})
	}
}

func TestDependencyLockedShortNameDoesNotRecheckInstallEligibility(t *testing.T) {
	fixture := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCodex}, []domain.ClientID{domain.ClientCodex})
	requirements := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{"docs": {Alias: "docs", Registry: &environment.RegistrySource{Distribution: "owner/rollout", Selector: environment.Selector{Kind: environment.SelectorRelease, Value: "1"}}}}, Targets: map[string][]string{"codex": {"docs"}}}
	distribution := &fixture.directory.bundle.Snapshot.Distributions[0]
	release := distribution.Releases[0]
	digest, err := environment.ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	lock := environment.Lock{LockVersion: environment.LockVersion, ManifestDigest: digest, Packages: []environment.LockedPackage{{Alias: "docs", SourceKey: "directory:owner/rollout", DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: release.TreeDigest, ManifestDigest: release.ManifestDigest, Targets: []string{"codex"}, Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{Distribution: "owner/rollout", DistributionID: "owner/rollout", ProductID: distribution.ProductID, ReleaseSequence: 1, Requested: requirements.Dependencies["docs"].Registry.Selector, Repository: release.PackageSource.Repository, Path: release.PackageSource.Path, Commit: release.PackageSource.Revision}}}}}
	path := filepath.Join(t.TempDir(), "uap.lock")
	if err := writeDependencyLock(path, lock); err != nil {
		t.Fatal(err)
	}
	// Replay verifies declarations from authentic pinned bytes in the store,
	// independently of whether the Directory still admits a fresh install.
	seed, err := fixture.cli.app.acquireLocal(context.Background(), fixture.v1.root)
	if err != nil {
		t.Fatal(err)
	}
	if seed.envelope.TreeDigest != release.TreeDigest || seed.envelope.ManifestDigest != release.ManifestDigest {
		_ = seed.cleanup()
		t.Fatal("fixture source does not match the recorded release digests")
	}
	if err := (dependencyResolver{app: fixture.cli.app}).finishLoaded(seed); err != nil {
		t.Fatal(err)
	}
	requirements.Dependencies["docs"].Registry.Distribution = distribution.ProductID
	distribution.ReleasePolicies[0].Status = domain.ReleaseSuperseded
	canonical, err := fixture.cli.app.canonicalizeDependencySyncRequirements(context.Background(), requirements, path, true)
	if err != nil || canonical.Dependencies["docs"].Registry.Distribution != "owner/rollout" {
		t.Fatalf("locked identity canonicalization: %+v %v", canonical, err)
	}
	if _, err := readDependencyLock(path, canonical); err != nil {
		t.Fatal(err)
	}
	// Resolving commands must reuse lock identity before Directory lookup.
	directoryCalls := fixture.directory.calls
	sourceCalls := fixture.acquirer.revisionCalls[release.PackageSource.Revision]
	previousClient := fixture.cli.app.DirectoryClient
	fixture.cli.app.DirectoryClient = nil
	requirements.Dependencies["docs"].Registry.Distribution = distribution.ProductID
	resolved, _, next, err := resolveDependencyLock(context.Background(), fixture.cli.app, &options{dryRun: true}, requirements, path, "")
	if err != nil || resolved.Dependencies["docs"].Registry.Distribution != "owner/rollout" || next.Packages[0].TreeDigest != lock.Packages[0].TreeDigest {
		t.Fatalf("resolve lock-first identity: %+v %v", next, err)
	}
	if fixture.directory.calls != directoryCalls || fixture.acquirer.revisionCalls[release.PackageSource.Revision] != sourceCalls {
		t.Fatal("local pinned replay performed a Directory lookup or source acquisition")
	}
	fixture.cli.app.DirectoryClient = previousClient
	// Exact locked restore permits supersession without claiming installed history.
	loaded, err := fixture.cli.app.acquireLocal(context.Background(), fixture.v1.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = loaded.cleanup() }()
	if _, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loaded, lock.Packages[0], nil); err != nil {
		t.Fatalf("lock-based canonicalization lost exact superseded restore: %v", err)
	}
	distribution.ReleasePolicies[0].Status = domain.ReleaseRevoked
	if _, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loaded, lock.Packages[0], nil); !errors.Is(err, domain.ErrDirectoryIneligible) {
		t.Fatalf("canonicalized lock bypassed revoked-release policy: %v", err)
	}
}

func TestDependencyInitColdRootAndDryRun(t *testing.T) {
	for _, dry := range []bool{true, false} {
		f := newCLIFixture(t, nil)
		f.app.DependenciesRoot = filepath.Join(t.TempDir(), "cold")
		args := []string{"deps", "init", "--format", "json"}
		if dry {
			args = append(args, "--dry-run")
		}
		out, _, err := f.execute(false, args...)
		if err != nil {
			t.Fatal(err)
		}
		mp, _ := f.app.dependencyPaths()
		if dry {
			if _, err := os.Stat(f.app.DependenciesRoot); !os.IsNotExist(err) {
				t.Fatal("dry init created root")
			}
			continue
		}
		if !strings.Contains(out, "manifest_path") || !strings.Contains(out, "created") {
			t.Fatal(out)
		}
		if _, err := readDependencyManifest(mp); err != nil {
			t.Fatal(err)
		}
		if _, _, err := f.execute(false, args...); err == nil || !strings.Contains(err.Error(), mp) {
			t.Fatalf("existing file: %v", err)
		}
	}
}

func TestDependencyStatusColdRoot(t *testing.T) {
	f := newCLIFixture(t, nil)
	f.app.DependenciesRoot = filepath.Join(t.TempDir(), "cold")
	var err error
	f.app.Installer, err = installer.New(installer.Config{StateRoot: f.app.DependenciesRoot, Registry: f.app.ClientRegistry})
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := f.execute(false, "deps", "status", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"scope", "manifest_path", "lock_path", "supported_targets", "manifest_missing"} {
		if !strings.Contains(out, field) {
			t.Fatal(out)
		}
	}
	if _, err := os.Stat(f.app.DependenciesRoot); !os.IsNotExist(err) {
		t.Fatal("status created cold root")
	}
}

func TestDependencyStatusMissingManifestRetainsOwnedFactsAndRecovery(t *testing.T) {
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	f := newCLIFixture(t, []domain.DetectedClient{client})
	root := filepath.Join(f.root, "data")
	f.app.DependenciesRoot = root
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.app.Installer, err = installer.New(installer.Config{StateRoot: root, StateFile: f.store.Path, Registry: f.app.ClientRegistry, HelperExecutable: helper, Runner: &dependencyClaudeListing{}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := f.app.acquireLocal(context.Background(), writeCLIPlugin(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = loaded.cleanup() }()
	prepared, err := f.app.Installer.Prepare(context.Background(), installer.Request{
		Operation: installer.OpInstall, OperationID: "missing-intent-status", ClientID: "codex", ClientConfigRoot: client.ConfigRoot, ClientExecutable: client.ExecutablePath, PackageRoot: loaded.envelope.SnapshotRoot,
		AcquiredSnapshot:     &domain.AcquiredSnapshot{OriginMode: domain.OriginModeDirect, Source: domain.SourceIdentity{RequestedSource: githubIdentity("acme/demo", strings.Repeat("1", 40), ""), CanonicalSource: githubIdentity("acme/demo", strings.Repeat("1", 40), ""), Repository: "acme/demo", ResolvedRevision: strings.Repeat("1", 40)}, TreeDigest: loaded.envelope.TreeDigest, DigestAlgorithm: domain.TreeDigestAlgorithm},
		Assessment:           &installer.Assessment{TreeDigest: loaded.envelope.TreeDigest, Outcome: installer.AssessmentAllow},
		EnvironmentAuthority: &domain.EnvironmentAuthority{EnvironmentID: environment.EnvironmentUserDefault, Alias: "demo", ClientID: "codex", SourceKey: "github:acme/demo:", ExpectedAction: "install"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = prepared.Close() }()
	result, err := f.app.Installer.Apply(context.Background(), prepared, installer.Decision{Confirmed: true})
	if err != nil || result.Outcome != installer.OutcomeCompleted {
		t.Fatalf("install fixture: %+v %v", result, err)
	}
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.EnvironmentReceipts) != 1 {
		t.Fatal("fixture lacks owned receipt")
	}
	receipt := state.EnvironmentReceipts[0]
	_, lockPath := f.app.dependencyPaths()
	// An orphaned lock is not desired intent, even if it cannot be decoded.
	if err := os.WriteFile(lockPath, []byte("orphaned lock is not intent"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, pending := range []bool{false, true} {
		if pending {
			for key, binding := range state.Installations[0].Clients {
				if len(binding.Receipts) == 0 {
					t.Fatal("fixture lacks transaction receipt")
				}
				binding.Receipts[len(binding.Receipts)-1].Phase = "state_committed"
				state.Installations[0].Clients[key] = binding
			}
			if err := f.store.Save(state); err != nil {
				t.Fatal(err)
			}
		}
		before, err := os.ReadFile(f.store.Path)
		if err != nil {
			t.Fatal(err)
		}
		out, _, err := f.execute(false, "deps", "status", "--format", "json")
		if err != nil {
			t.Fatal(err)
		}
		var status dependencyStatus
		if err := json.Unmarshal([]byte(out), &status); err != nil {
			t.Fatal(err)
		}
		if status.PendingRecovery != pending || status.ManifestDigest != "" || status.LockDigest != "" {
			t.Fatalf("missing-intent status: %+v", status)
		}
		missing, owned, recoveryFact := false, false, false
		for _, fact := range status.Facts {
			for _, value := range fact.Facts {
				missing = missing || value == "manifest_missing"
				recoveryFact = recoveryFact || value == "pending_recovery"
			}
			owned = owned || fact.Alias == receipt.Alias && fact.ClientID == receipt.ClientID && fact.InstallationID == receipt.InstallationID && fact.BindingID == receipt.ClientBindingID && fact.Owner == receipt.EnvironmentID
		}
		if !missing || !owned || recoveryFact != pending {
			t.Fatal("status hid missing intent, owner identity or recovery: " + out)
		}
		after, err := os.ReadFile(f.store.Path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("status mutated state")
		}
		manifestPath, _ := f.app.dependencyPaths()
		if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
			t.Fatal("status created missing manifest")
		}
	}
}
