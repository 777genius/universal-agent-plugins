package agentpluginscli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
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
	if _, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loaded, locked, nil); err == nil {
		t.Fatal("lock alone gained superseded-release eligibility")
	}
	recorded := &domain.RecordedDirectoryRelease{ProductID: distribution.ProductID, DistributionID: distribution.ID, ReleaseSequence: 1, Repository: release.PackageSource.Repository, Path: release.PackageSource.Path, ResolvedRevision: release.PackageSource.Revision, TreeDigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: release.TreeDigest, ManifestDigest: release.ManifestDigest}
	if _, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loaded, locked, recorded); err != nil {
		t.Fatalf("installed exact repair lost recorded eligibility: %v", err)
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
	requirements.Dependencies["docs"].Registry.Distribution = distribution.ProductID
	distribution.ReleasePolicies[0].Status = domain.ReleaseSuperseded
	canonical, err := fixture.cli.app.canonicalizeDependencySyncRequirements(context.Background(), requirements, path, true)
	if err != nil || canonical.Dependencies["docs"].Registry.Distribution != "owner/rollout" {
		t.Fatalf("locked identity canonicalization: %+v %v", canonical, err)
	}
	if _, err := readDependencyLock(path, canonical); err != nil {
		t.Fatal(err)
	}
	// Canonicalization grants no fresh-install eligibility for that recorded identity.
	loaded, err := fixture.cli.app.acquireLocal(context.Background(), fixture.v1.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = loaded.cleanup() }()
	if _, err := verifyLockedDirectory(context.Background(), fixture.cli.app, loaded, lock.Packages[0], nil); err == nil {
		t.Fatal("lock-based canonicalization admitted a superseded new install")
	}
}
