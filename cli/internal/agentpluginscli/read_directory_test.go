package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type localOnlyDirectoryClient struct {
	bundle       directoryv1.VerifiedBundle
	networkCalls int
	localCalls   int
}

type conflictingDirectoryClient struct {
	bundle       directoryv1.VerifiedBundle
	networkCalls int
	localCalls   int
}

func TestPublicClientRevisionOnlyExposesImmutableFullSHA(t *testing.T) {
	sha := strings.Repeat("a", 40)
	if publicImmutableRevision(sha) != sha || publicImmutableRevision("/private/source/revision") != "" || publicImmutableRevision("client-commit") != "" {
		t.Fatal("public revision sanitization exposed a non-immutable source value")
	}
}

func (client *localOnlyDirectoryClient) Load(context.Context, uint64) (directoryv1.VerifiedBundle, error) {
	client.networkCalls++
	return directoryv1.VerifiedBundle{}, context.DeadlineExceeded
}

func (client *localOnlyDirectoryClient) LoadLocal(uint64) (directoryv1.VerifiedBundle, error) {
	client.localCalls++
	return client.bundle, nil
}

func (client *conflictingDirectoryClient) Load(context.Context, uint64) (directoryv1.VerifiedBundle, error) {
	client.networkCalls++
	return directoryv1.VerifiedBundle{}, directoryv1.ErrSequenceConflict
}

func (client *conflictingDirectoryClient) LoadLocal(uint64) (directoryv1.VerifiedBundle, error) {
	client.localCalls++
	return client.bundle, nil
}

func TestDirectoryReadDoesNotHideEquivocationBehindTrustedLocalFallback(t *testing.T) {
	client := &conflictingDirectoryClient{bundle: readModelBundle()}
	app := App{DirectoryClient: client}
	_, ok, err := directoryBundleForRead(context.Background(), app, domain.StateFileV2{}, true)
	if ok || !errors.Is(err, directoryv1.ErrSequenceConflict) {
		t.Fatalf("Directory read result ok=%t err=%v, want propagated sequence conflict", ok, err)
	}
	if client.networkCalls != 1 || client.localCalls != 0 {
		t.Fatalf("Directory read calls network/local=%d/%d, want 1/0", client.networkCalls, client.localCalls)
	}

	_, ok, err = directoryBundleForRead(context.Background(), app, domain.StateFileV2{}, false)
	if err != nil || !ok || client.networkCalls != 1 || client.localCalls != 1 {
		t.Fatalf("direct read result ok=%t err=%v calls=%d/%d, want local-only best effort", ok, err, client.networkCalls, client.localCalls)
	}
}

func TestInfoShortNameShowsOneDirectoryProductDefaultAlternativesCompatibilityAndEvidence(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	bundle := readModelBundle()
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: bundle}
	stdout, _, err := fixture.execute(false, "info", "demo", "--target", "cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"product_id":"demo"`, `"reviewed_default":"owner/demo"`,
		`"selected_distribution":"owner/demo"`, `"selection_reason":"reviewed default is eligible`,
		`"selected_release":{"id":"owner/demo"`, `"release_sequence":2`, `"resolved_revision":"` + strings.Repeat("2", 40),
		`"repository":"owner/demo"`, `"package_path":"plugin"`,
		`"tree_digest":"sha256:` + strings.Repeat("2", 64), `"manifest_digest":"sha256:` + strings.Repeat("4", 64),
		`"target_compatibility"`, `"client":"cursor"`, `"immutable_evidence"`, `"id":"cursor-runtime-current"`,
		`"artifact":{"repository":"owner/evidence"`, `"trust":{"kind":"reviewed_external"}`, `"trusted_for_eligibility":true`,
		`"alternatives":[{"id":"community/demo"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("Directory info omitted %q:\n%s", want, stdout)
		}
	}
	if strings.Count(stdout, `"product_id":"demo"`) != 1 {
		t.Fatalf("Directory info returned duplicate product cards:\n%s", stdout)
	}
}

func TestPublicDirectoryEvidenceDistinguishesMissingTrust(t *testing.T) {
	bundle := readModelBundle()
	bundle.Snapshot.Evidence[0].Trust = nil
	public := evidenceForRelease(bundle.Snapshot, "owner/demo", 1)
	if len(public) != 1 || public[0].Trust != nil || public[0].TrustedForEligibility {
		t.Fatalf("untrusted evidence provenance was misrepresented: %+v", public)
	}
	var output bytes.Buffer
	if err := writeJSONOutput(&output, "test", public); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"trusted_for_eligibility":false`) || strings.Contains(output.String(), `"trust":`) {
		t.Fatalf("public evidence did not distinguish missing trust: %s", output.String())
	}
}

func TestHumanDirectoryEvidenceLabelsTrustedAndInformationalProvenance(t *testing.T) {
	trusted := publicEvidence{ID: "trusted-runtime", Level: "runtime", Outcome: "passed",
		Artifact: domain.DirectoryEvidenceArtifact{Repository: "owner/evidence", Revision: strings.Repeat("e", 40), Path: "trusted.json", Digest: "sha256:" + strings.Repeat("f", 64)},
		Trust:    &domain.DirectoryEvidenceTrust{Kind: "reviewed_external"}}
	informational := trusted
	informational.ID, informational.Artifact.Path, informational.Trust = "self-reported-runtime", "self-reported.json", nil
	var output bytes.Buffer
	if err := renderProductInspection(&output, publicProductInspection{ImmutableEvidence: []publicEvidence{trusted, informational}}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Evidence: trusted-runtime runtime=passed trust=trusted ",
		"Evidence: self-reported-runtime runtime=passed trust=self-reported/informational ",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("human evidence omitted %q:\n%s", want, output.String())
		}
	}
}

func TestInstalledInfoAndDoctorExposeRevisionConvergenceSurfacesAndRevocation(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation := &state.Installations[0]
	installation.OriginMode = domain.OriginModeDirectory
	installation.Directory = &domain.DirectoryOrigin{ProductID: "demo", DistributionID: "owner/demo", DistributionKind: domain.DistributionUpstream,
		DesiredReleaseSequence: 2, SnapshotSchema: 1, SnapshotSequence: 20, SnapshotDigest: "sha256:" + strings.Repeat("9", 64)}
	installation.Source.Repository = "owner/demo"
	installation.Source.ResolvedRevision = strings.Repeat("2", 40)
	installation.Source.PackageSubpath = "plugin"
	for key, binding := range installation.Clients {
		binding.PackageRevision = &domain.ClientPackageRevision{Version: "1.0.0", ResolvedRevision: strings.Repeat("1", 40),
			TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest,
			DistributionID: "owner/demo", ReleaseSequence: 1}
		binding.AffectedSurfaces = []string{"cursor.mcp", "cursor.skills"}
		installation.Clients[key] = binding
	}
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	bundle := readModelBundle()
	// Bind the fixture's real installed bytes so the signed-policy read model
	// identifies the exact revoked package rather than merely matching a name.
	bundle.Snapshot.Distributions[0].Releases[0].TreeDigest = installation.Source.TreeDigest
	bundle.Snapshot.Distributions[0].Releases[0].ManifestDigest = installation.Package.ManifestDigest
	bundle.Snapshot.Evidence[0].PackageTreeDigest = installation.Source.TreeDigest
	bundle.Snapshot.Distributions[0].Status = domain.DistributionSuspended
	fixture.app.DirectoryClient = &fixedDirectoryClient{bundle: bundle}
	list, _, err := fixture.execute(false, "list", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(list, `"installation_id"`) != 1 || strings.Count(list, `"product_id":"demo"`) != 1 {
		t.Fatalf("list duplicated or replaced the installed product card:\n%s", list)
	}
	stdout, _, err := fixture.execute(false, "info", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"recorded_distribution":"owner/demo"`, `"current_distribution":"owner/demo"`,
		`"recorded_revision":"` + strings.Repeat("2", 40), `"current_revision":"` + strings.Repeat("2", 40),
		`"recorded_release_sequence":2`, `"current_release_sequence":2`, `"release_sequence":1`,
		`"package_revision":{"version":"1.0.0","resolved_revision":"` + strings.Repeat("1", 40),
		`"evidence":[{"id":"cursor-runtime"`, `"artifact":{"repository":"owner/evidence"`, `"affected_surfaces":["cursor.mcp","cursor.skills"]`,
		`"mixed_version":true`, `"convergence_action":"run ` + "`" + `agentplugins update`, `"code":"directory_release_revoked"`,
		`"code":"directory_distribution_suspended"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("installed Directory info omitted %q:\n%s", want, stdout)
		}
	}
	doctor, _, err := fixture.execute(false, "doctor", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doctor, `"code":"directory_release_revoked"`) || !strings.Contains(doctor, `"code":"directory_distribution_suspended"`) || strings.Contains(doctor, `"code":"no_degradation_detected"`) {
		t.Fatalf("doctor did not surface signed revocation cleanly:\n%s", doctor)
	}
}

func TestDirectInfoUsesOnlyTrustedLocalPolicyForExactDigestWarning(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	bundle := readModelBundle()
	bundle.Snapshot.Distributions[0].Releases[0].TreeDigest = state.Installations[0].Source.TreeDigest
	bundle.Snapshot.Distributions[0].Releases[0].ManifestDigest = state.Installations[0].Package.ManifestDigest
	bundle.Snapshot.Evidence[0].PackageTreeDigest = state.Installations[0].Source.TreeDigest
	client := &localOnlyDirectoryClient{bundle: bundle}
	fixture.app.DirectoryClient = client
	stdout, _, err := fixture.execute(false, "info", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if client.networkCalls != 0 || client.localCalls != 1 || !strings.Contains(stdout, `"code":"directory_release_revoked"`) {
		t.Fatalf("direct warning network/local calls=%d/%d output=%s", client.networkCalls, client.localCalls, stdout)
	}
	doctor, _, err := fixture.execute(false, "doctor", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if client.networkCalls != 0 || client.localCalls != 2 || !strings.Contains(doctor, `"code":"directory_release_revoked"`) {
		t.Fatalf("direct doctor warning network/local calls=%d/%d output=%s", client.networkCalls, client.localCalls, doctor)
	}
}

func TestDirectAcquisitionWarnsFromLocalPolicyWithoutNetworkDependency(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	plugin := writeCLIPlugin(t)
	first, err := fixture.app.acquireLocal(context.Background(), plugin)
	if err != nil {
		t.Fatal(err)
	}
	tree, manifest := first.envelope.TreeDigest, first.envelope.ManifestDigest
	_ = first.cleanup()
	bundle := readModelBundle()
	bundle.Snapshot.Distributions[0].Releases[0].TreeDigest = tree
	bundle.Snapshot.Distributions[0].Releases[0].ManifestDigest = manifest
	bundle.Snapshot.Evidence[0].PackageTreeDigest = tree
	client := &localOnlyDirectoryClient{bundle: bundle}
	var warnings bytes.Buffer
	fixture.app.DirectoryClient, fixture.app.ErrorOutput = client, &warnings
	loaded, err := fixture.app.acquireLocal(context.Background(), plugin)
	if err != nil {
		t.Fatal(err)
	}
	_ = loaded.cleanup()
	if client.networkCalls != 0 || client.localCalls != 1 || !strings.Contains(warnings.String(), "directory_release_revoked") {
		t.Fatalf("direct acquisition warning network/local=%d/%d warning=%q", client.networkCalls, client.localCalls, warnings.String())
	}
	client.localCalls = 0
	warnings.Reset()
	revision := strings.Repeat("d", 40)
	fixture.app.SourceAcquirer = &localBackedSourceAcquirer{delegate: fixture.app.SourceAcquirer, root: plugin}
	loaded, err = fixture.app.acquireGitHub(context.Background(), "owner/repo@"+revision+"//plugin", "owner/repo", revision, "plugin", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = loaded.cleanup()
	if client.networkCalls != 0 || client.localCalls != 1 || !strings.Contains(warnings.String(), "directory_release_revoked") {
		t.Fatalf("full-SHA acquisition warning network/local=%d/%d warning=%q", client.networkCalls, client.localCalls, warnings.String())
	}
}

func readModelBundle() directoryv1.VerifiedBundle {
	tree1, tree2 := "sha256:"+strings.Repeat("1", 64), "sha256:"+strings.Repeat("2", 64)
	manifest1, manifest2 := "sha256:"+strings.Repeat("3", 64), "sha256:"+strings.Repeat("4", 64)
	release := func(sequence uint64, revision, tree, manifest, version string) domain.DirectoryRelease {
		return domain.DirectoryRelease{Sequence: sequence, PackageVersion: version, ManifestName: "demo",
			AgentPluginsSchema:  "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
			PackageSource:       domain.DirectorySource{Repository: "owner/demo", Revision: revision, Path: "plugin"},
			TreeDigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: tree, ManifestDigest: manifest, Components: []string{"mcp"}}
	}
	target := domain.DirectoryTarget{Client: domain.ClientCursor, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementUnknown}
	policy := func(sequence uint64, status domain.ReleaseStatus, evidence []string) domain.DirectoryReleasePolicy {
		return domain.DirectoryReleasePolicy{ReleaseSequence: sequence, Status: status, MinimumInstallerVersion: "0.1.0", Targets: []domain.DirectoryTarget{target}, CurrentEvidence: evidence}
	}
	owner := domain.DirectoryDistribution{SchemaVersion: 1, ID: "owner/demo", ProductID: "demo", Kind: domain.DistributionUpstream,
		Status: domain.DistributionActive, Packager: "owner",
		Releases:        []domain.DirectoryRelease{release(1, strings.Repeat("1", 40), tree1, manifest1, "1.0.0"), release(2, strings.Repeat("2", 40), tree2, manifest2, "2.0.0")},
		ReleasePolicies: []domain.DirectoryReleasePolicy{policy(1, domain.ReleaseRevoked, []string{"cursor-runtime"}), policy(2, domain.ReleaseActive, []string{"cursor-runtime-current", "cursor-materialization-current"})}}
	community := owner
	community.ID, community.Kind, community.Packager = "community/demo", domain.DistributionCommunity, "community"
	community.Releases = []domain.DirectoryRelease{release(1, strings.Repeat("5", 40), tree2, manifest2, "community")}
	community.Releases[0].PackageSource.Repository = "community/demo"
	community.ReleasePolicies = []domain.DirectoryReleasePolicy{policy(1, domain.ReleaseActive, nil)}
	product := domain.DirectoryProduct{SchemaVersion: 1, ID: "demo", DisplayName: "Demo", Description: "Reviewed demo product", ManifestName: "demo",
		Aliases: []string{"demo"}, ReservedAliases: []string{"demo"}, Categories: []string{"tools"},
		MinimumCapabilities: domain.DirectoryMinimumCapabilities{Skills: "optional", MCP: "optional"},
		DefaultDistribution: "owner/demo", Distributions: []string{"owner/demo", "community/demo"}}
	evidence := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{SchemaVersion: 1, ID: "cursor-runtime", DistributionID: "owner/demo", ReleaseSequence: 1,
		PackageTreeDigest: tree1, Level: "runtime", Outcome: "passed", Client: domain.ClientCursor,
		Artifact: domain.DirectoryEvidenceArtifact{Repository: "owner/evidence", Revision: strings.Repeat("e", 40), Path: "runtime/cursor.json", Digest: "sha256:" + strings.Repeat("f", 64)}})
	currentEvidence := evidence
	currentEvidence.ID, currentEvidence.ReleaseSequence, currentEvidence.PackageTreeDigest = "cursor-runtime-current", 2, tree2
	currentEvidence.Artifact.Path = "runtime/cursor-current.json"
	materializationEvidence := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{SchemaVersion: 1, ID: "cursor-materialization-current", DistributionID: "owner/demo", ReleaseSequence: 2,
		PackageTreeDigest: tree2, Level: "materialization", Outcome: "passed", Client: domain.ClientCursor})
	return directoryv1.VerifiedBundle{Snapshot: domain.DirectorySnapshot{SnapshotSchemaVersion: 1, Sequence: 21, Products: []domain.DirectoryProduct{product},
		Distributions: []domain.DirectoryDistribution{owner, community}, Evidence: []domain.DirectoryEvidence{evidence, currentEvidence, materializationEvidence},
		Revocations: []domain.DirectoryRevocation{{DistributionID: "owner/demo", ReleaseSequence: 1}}}, Digest: "sha256:" + strings.Repeat("a", 64)}
}
