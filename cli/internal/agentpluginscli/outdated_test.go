package agentpluginscli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestOutdatedComparesDirectoryReleaseIdentityWithoutAcquisitionOrMutation(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	bundle := readModelBundle()
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	bindDirectoryRelease(&state.Installations[0], bundle.Snapshot, "owner/demo", 1)
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	acquirer := &countingSourceAcquirer{delegate: fixture.app.SourceAcquirer}
	fixture.app.SourceAcquirer = acquirer
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: bundle}

	stdout, _, err := fixture.execute(false, "outdated", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"read_only":true`, `"outdated":1`, `"status":"outdated"`,
		`"release_sequence":1`, `"release_sequence":2`,
		`"revision":"` + strings.Repeat("2", 40) + `"`,
		`"code":"directory_release_revoked"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("outdated result omitted %q:\n%s", want, stdout)
		}
	}
	if acquirer.calls != 0 {
		t.Fatalf("outdated acquired package bytes %d times", acquirer.calls)
	}
	after, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if string(beforeJSON) != string(afterJSON) {
		t.Fatalf("outdated mutated installation state\nbefore=%s\nafter=%s", beforeJSON, afterJSON)
	}
}

func TestOutdatedDistinguishesCurrentUnsafeAndDirectInstallations(t *testing.T) {
	tests := []struct {
		name      string
		sequence  uint64
		configure func(*domain.DirectorySnapshot)
		want      string
		reason    string
	}{
		{name: "current", sequence: 2, want: `"status":"current"`, reason: "no later eligible immutable release exists"},
		{name: "unsafe without successor", sequence: 1, configure: func(snapshot *domain.DirectorySnapshot) {
			distribution := &snapshot.Distributions[0]
			distribution.Releases = distribution.Releases[:1]
			distribution.ReleasePolicies = distribution.ReleasePolicies[:1]
		}, want: `"status":"blocked"`, reason: "installed release is unsafe and no later eligible release is available"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
			if _, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "cursor"); err != nil {
				t.Fatal(err)
			}
			bundle := readModelBundle()
			if test.configure != nil {
				test.configure(&bundle.Snapshot)
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			bindDirectoryRelease(&state.Installations[0], bundle.Snapshot, "owner/demo", test.sequence)
			if err := fixture.store.Save(state); err != nil {
				t.Fatal(err)
			}
			fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: bundle}
			stdout, _, err := fixture.execute(false, "outdated", "demo", "--format", "json")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout, test.want) || !strings.Contains(stdout, test.reason) {
				t.Fatalf("result did not distinguish %s:\n%s", test.name, stdout)
			}
		})
	}

	direct := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	if _, _, err := direct.execute(false, "add", writeCLIPlugin(t), "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := direct.execute(false, "outdated", "--all", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"unmanaged":1`) || !strings.Contains(stdout, "direct source has no signed update channel") {
		t.Fatalf("direct installation was not explicitly unmanaged:\n%s", stdout)
	}
}

func TestOutdatedAllIsDeterministicForMixedReleaseStates(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	if _, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	bundle := readModelBundle()
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	old := cloneDirectoryInstallation(state.Installations[0], bundle.Snapshot, "alpha", "installation-alpha", "source-alpha", 1)
	current := cloneDirectoryInstallation(state.Installations[0], bundle.Snapshot, "zulu", "installation-zulu", "source-zulu", 2)
	state.Installations = []domain.Installation{current, old}
	state.TransactionReceipts = nil
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: bundle}
	first, _, err := fixture.execute(false, "outdated", "--all", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := fixture.execute(false, "outdated", "--all", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || strings.Index(first, `"name":"alpha"`) > strings.Index(first, `"name":"zulu"`) {
		t.Fatalf("outdated --all output is not deterministic:\n%s\n%s", first, second)
	}
	if !strings.Contains(first, `"outdated":1`) || !strings.Contains(first, `"current":1`) || !strings.Contains(first, `"code":"directory_release_revoked"`) {
		t.Fatalf("mixed release states were not preserved:\n%s", first)
	}
}

func TestOutdatedComparesSignedDiscoveryIdentityWithoutAcquisition(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	if _, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	selector := "discovery:owner/demo//plugin"
	installation := &state.Installations[0]
	installation.Source.RequestedSource = selector
	installation.Source.Repository = "owner/demo"
	installation.Source.PackageSubpath = "plugin"
	installation.Source.ResolvedRevision = strings.Repeat("a", 40)
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	record := discoveryv1.Record{
		Slug: selector, Repository: "owner/demo", PackagePath: "plugin", Revision: strings.Repeat("b", 40),
		TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest, Availability: "available",
	}
	fixture.app.DiscoveryClient = &fixedDiscoveryClient{bundle: discoveryv1.VerifiedBundle{
		Snapshot: discoveryv1.Snapshot{Sequence: 11}, Search: discoveryv1.Search{Records: []discoveryv1.Record{record}},
		Digest: "sha256:" + strings.Repeat("4", 64),
	}}
	acquirer := &countingSourceAcquirer{delegate: fixture.app.SourceAcquirer}
	fixture.app.SourceAcquirer = acquirer
	stdout, _, err := fixture.execute(false, "outdated", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"outdated":1`) || !strings.Contains(stdout, `"discovery_snapshot_sequence":11`) ||
		!strings.Contains(stdout, record.Revision) || acquirer.calls != 0 {
		t.Fatalf("Discovery outdated result=%s acquisitions=%d", stdout, acquirer.calls)
	}
}

func bindDirectoryRelease(installation *domain.Installation, snapshot domain.DirectorySnapshot, distributionID string, sequence uint64) {
	distribution := snapshotDistribution(snapshot, distributionID)
	if distribution == nil {
		panic("test Directory distribution is missing")
	}
	var release *domain.DirectoryRelease
	for index := range distribution.Releases {
		if distribution.Releases[index].Sequence == sequence {
			release = &distribution.Releases[index]
			break
		}
	}
	if release == nil {
		panic("test Directory release is missing")
	}
	installation.OriginMode = domain.OriginModeDirectory
	installation.Directory = &domain.DirectoryOrigin{ProductID: distribution.ProductID, DistributionID: distribution.ID,
		DistributionKind: distribution.Kind, DesiredReleaseSequence: sequence, SnapshotSchema: 1,
		SnapshotSequence: snapshot.Sequence, SnapshotDigest: "sha256:" + strings.Repeat("a", 64)}
	installation.Source.RequestedSource = distribution.ProductID
	installation.Source.CanonicalSource = "github:" + release.PackageSource.Repository + "@" + release.PackageSource.Revision + "//" + release.PackageSource.Path
	installation.Source.Repository = release.PackageSource.Repository
	installation.Source.PackageSubpath = release.PackageSource.Path
	installation.Source.ResolvedRevision = release.PackageSource.Revision
	installation.Source.TreeDigest = release.TreeDigest
	installation.Source.SourceBindingID = domain.ComputeSourceBindingID(domain.SourceIdentity{CanonicalSource: installation.Source.CanonicalSource,
		Repository: installation.Source.Repository, PackageSubpath: installation.Source.PackageSubpath})
	installation.Package.Version = release.PackageVersion
	installation.Package.ManifestDigest = release.ManifestDigest
	for key, binding := range installation.Clients {
		binding.PackageRevision = &domain.ClientPackageRevision{Version: release.PackageVersion, ResolvedRevision: release.PackageSource.Revision,
			TreeDigest: release.TreeDigest, ManifestDigest: release.ManifestDigest, DistributionID: distribution.ID, ReleaseSequence: sequence}
		installation.Clients[key] = binding
	}
}

func cloneDirectoryInstallation(base domain.Installation, snapshot domain.DirectorySnapshot, name, installationID, sourceID string, sequence uint64) domain.Installation {
	clone := base
	clone.InstallationID, clone.DeclaredName = installationID, name
	clone.Source.SourceBindingID = sourceID
	clone.Package.DeclaredName = name
	clone.Clients = map[string]domain.ClientBinding{}
	for _, binding := range base.Clients {
		binding.ClientBindingID = domain.ComputeClientBindingID(installationID, binding.ClientID, binding.Scope, binding.TargetLocator)
		binding.PhysicalArtifact = domain.ComputePhysicalArtifactID(name, installationID)
		binding.Receipts = nil
		clone.Clients[binding.ClientBindingID] = binding
	}
	clone.DataReceipts, clone.DataRetained, clone.OperationGroupID = nil, false, ""
	bindDirectoryRelease(&clone, snapshot, "owner/demo", sequence)
	clone.Source.SourceBindingID = sourceID
	return clone
}
