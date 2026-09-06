package domain

import (
	"errors"
	"strings"
	"testing"
)

func testRelease(sequence uint64, version string) DirectoryRelease {
	return DirectoryRelease{Sequence: sequence, PackageVersion: version, ManifestName: "tool", AgentPluginsSchema: "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", PackageSource: DirectorySource{Repository: "owner/repo", Revision: "0123456789012345678901234567890123456789", Path: "plugin"}, TreeDigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ManifestDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Components: []string{"mcp", "skills"}, PublishedAt: "2026-08-20T00:00:00Z"}
}

func testPolicy(sequence uint64, clients ...ClientID) DirectoryReleasePolicy {
	targets := make([]DirectoryTarget, len(clients))
	for i, c := range clients {
		delivery, _ := ExpectedDirectoryDelivery(c)
		targets[i] = DirectoryTarget{Client: c, Scopes: []InstallScope{ScopeUser}, Delivery: delivery, Authentication: AuthenticationRequirementUnknown}
	}
	return DirectoryReleasePolicy{ReleaseSequence: sequence, Status: ReleaseActive, MinimumInstallerVersion: "1.0.0", Targets: targets, CurrentEvidence: []string{}}
}

func testTrustedEvidence(e DirectoryEvidence) DirectoryEvidence {
	e.Artifact = DirectoryEvidenceArtifact{
		Repository: "owner/evidence",
		Revision:   "abcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Path:       "evidence/result.json",
		Digest:     "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	e.Trust = &DirectoryEvidenceTrust{
		Kind:         "github_actions",
		Workflow:     "owner/evidence/.github/workflows/directory.yml",
		SourceRef:    "refs/heads/main",
		SourceDigest: e.Artifact.Revision,
	}
	return e
}

func testDistribution(id string, kind DistributionKind, releases []DirectoryRelease, policies []DirectoryReleasePolicy) DirectoryDistribution {
	return DirectoryDistribution{SchemaVersion: 1, ID: id, ProductID: "tool", Kind: kind, Status: DistributionActive, Packager: "owner", Releases: releases, ReleasePolicies: policies}
}

func testDirectory() DirectorySnapshot {
	upstream := testDistribution("owner/tool", DistributionUpstream, []DirectoryRelease{testRelease(2, "0.1.0"), testRelease(3, "not-semver")}, []DirectoryReleasePolicy{testPolicy(2, ClientCodex), testPolicy(3, ClientCodex)})
	bridge := testDistribution("community/tool-bridge", DistributionCommunityBridge, []DirectoryRelease{testRelease(8, "99.0.0")}, []DirectoryReleasePolicy{testPolicy(8, ClientCodex, ClientCursor)})
	community := testDistribution("other/tool", DistributionCommunity, []DirectoryRelease{testRelease(1, "1.0.0")}, []DirectoryReleasePolicy{testPolicy(1, ClientCodex, ClientCursor)})
	product := DirectoryProduct{SchemaVersion: 1, ID: "tool", DisplayName: "Tool", Description: "Tool", ManifestName: "tool", Aliases: []string{"tool", "old-tool"}, ReservedAliases: []string{"tool"}, Categories: []string{"tools"}, MinimumCapabilities: DirectoryMinimumCapabilities{MCP: "required", Skills: "optional"}, DefaultDistribution: "owner/tool", Distributions: []string{"other/tool", "community/tool-bridge", "owner/tool"}}
	snapshot := DirectorySnapshot{SnapshotSchemaVersion: 1, Sequence: 42, Products: []DirectoryProduct{product}, Distributions: []DirectoryDistribution{community, bridge, upstream}, Evidence: []DirectoryEvidence{}, Revocations: []DirectoryRevocation{}}
	upstreamDistribution := &snapshot.Distributions[2]
	for policyIndex := range upstreamDistribution.ReleasePolicies {
		policy := &upstreamDistribution.ReleasePolicies[policyIndex]
		release := upstreamDistribution.Releases[policyIndex]
		for _, target := range policy.Targets {
			id := "passed/materialization/" + string(target.Client) + "/" + release.PackageVersion
			policy.CurrentEvidence = append(policy.CurrentEvidence, id)
			snapshot.Evidence = append(snapshot.Evidence, testTrustedEvidence(DirectoryEvidence{ID: id, DistributionID: upstreamDistribution.ID, ReleaseSequence: release.Sequence,
				PackageTreeDigest: release.TreeDigest, Level: "materialization", Outcome: "passed", Client: target.Client,
				InstallerVersion: "promotion-installer", ClientVersion: "promotion-client", OS: "promotion-os", Architecture: "promotion-arch"}))
		}
	}
	return snapshot
}

func request(selector string, targets ...ClientID) DirectoryResolveRequest {
	return DirectoryResolveRequest{Selector: selector, Targets: targets, Scope: ScopeUser, InstallerVersion: "1.2.3", ClientVersions: map[ClientID]string{ClientCodex: "test-client"}, OS: "linux", Architecture: "amd64", SchemaVersion: "1.0.0", Operation: DirectoryInstall}
}

func TestResolveDirectoryDeclaredDefaultFallbackQualifiedAndSequence(t *testing.T) {
	s := testDirectory()
	got, err := ResolveDirectory(s, request("old-tool", ClientCodex))
	if err != nil || got.DistributionID != "owner/tool" || got.ReleaseSequence != 3 || got.Fallback {
		t.Fatalf("default: %+v %v", got, err)
	}
	got, err = ResolveDirectory(s, request("tool", ClientCodex, ClientCursor))
	if err != nil || got.DistributionID != "community/tool-bridge" || !got.Fallback || len(got.Diagnostics) == 0 {
		t.Fatalf("fallback: %+v %v", got, err)
	}
	got, err = ResolveDirectory(s, request("other/tool", ClientCursor))
	if err != nil || got.DistributionID != "other/tool" {
		t.Fatalf("qualified: %+v %v", got, err)
	}
}

func TestResolveDirectoryAcceptsReleaseBoundLegacyEvidence(t *testing.T) {
	snapshot := testDirectory()
	snapshot.Sequence = 13
	upstream := snapshot.Distributions[2]
	for index := range snapshot.Evidence {
		evidence := &snapshot.Evidence[index]
		for _, release := range upstream.Releases {
			if release.Sequence != evidence.ReleaseSequence {
				continue
			}
			evidence.ProductID = upstream.ProductID
			evidence.ManifestDigest = release.ManifestDigest
			evidence.SourceRepository = release.PackageSource.Repository
			evidence.SourceRevision = release.PackageSource.Revision
			evidence.SourcePath = release.PackageSource.Path
			evidence.AdapterVersion = "0.1.13"
			evidence.Trust = nil
		}
	}
	selected, err := ResolveDirectory(snapshot, request("owner/tool", ClientCodex))
	if err != nil || selected.ReleaseSequence != 3 {
		t.Fatalf("legacy upstream evidence was not eligible: %+v %v", selected, err)
	}
}

func TestResolveDirectoryRejectsUnknownAndMismatchedDelivery(t *testing.T) {
	for _, test := range []struct {
		name, delivery string
	}{
		{name: "unknown", delivery: "future_delivery"},
		{name: "mismatched", delivery: "manual_activation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testDirectory()
			s.Distributions[2].ReleasePolicies[0].Status = ReleaseSuperseded
			s.Distributions[2].ReleasePolicies[1].Targets[0].Delivery = test.delivery
			if _, err := ResolveDirectory(s, request("owner/tool", ClientCodex)); !errors.Is(err, ErrDirectoryIneligible) {
				t.Fatalf("delivery %q was accepted: %v", test.delivery, err)
			}
		})
	}
}

func TestResolveDirectoryDeclaredDefaultPrecedesKindAndFallbackUsesKindOrder(t *testing.T) {
	s := testDirectory()
	s.Products[0].DefaultDistribution = "other/tool"
	got, err := ResolveDirectory(s, request("tool", ClientCodex))
	if err != nil || got.DistributionID != "other/tool" || got.Fallback {
		t.Fatalf("eligible declared community default was implicitly promoted away: %+v %v", got, err)
	}
	s.Distributions[0].ReleasePolicies[0].Targets = []DirectoryTarget{{Client: ClientCursor, Scopes: []InstallScope{ScopeUser}, Delivery: "managed", Authentication: AuthenticationRequirementUnknown}}
	got, err = ResolveDirectory(s, request("tool", ClientCodex))
	if err != nil || got.DistributionID != "owner/tool" || !got.Fallback {
		t.Fatalf("fallback did not prefer eligible upstream: %+v %v", got, err)
	}
}

func TestResolveDirectoryNoMixingAndAmbiguity(t *testing.T) {
	s := testDirectory()
	s.Distributions[0].ReleasePolicies[0].Targets = []DirectoryTarget{{Client: ClientCursor, Scopes: []InstallScope{ScopeUser}, Delivery: "managed", Authentication: AuthenticationRequirementUnknown}}
	s.Distributions[1].ReleasePolicies[0].Targets = []DirectoryTarget{{Client: ClientCursor, Scopes: []InstallScope{ScopeUser}, Delivery: "managed", Authentication: AuthenticationRequirementUnknown}}
	if _, err := ResolveDirectory(s, request("tool", ClientCodex, ClientCursor)); !errors.Is(err, ErrDirectoryIneligible) {
		t.Fatalf("mixing: %v", err)
	}
	p := s.Products[0]
	p.ID = "tool-two"
	p.ManifestName = "tool-two"
	p.DefaultDistribution = "other/tool-two"
	p.Distributions = []string{"other/tool-two"}
	d := testDistribution("other/tool-two", DistributionCommunity, []DirectoryRelease{testRelease(1, "1")}, []DirectoryReleasePolicy{testPolicy(1, ClientCodex)})
	d.ProductID = "tool-two"
	d.Releases[0].ManifestName = "tool-two"
	s.Products = append(s.Products, p)
	s.Distributions = append(s.Distributions, d)
	if _, err := ResolveDirectory(s, request("old-tool", ClientCodex)); !errors.Is(err, ErrDirectoryAmbiguous) {
		t.Fatalf("ambiguity: %v", err)
	}
}

func TestResolveDirectoryOperationMatrixAndTopLevelRevocation(t *testing.T) {
	s := testDirectory()
	d := &s.Distributions[2]
	recorded := &RecordedDirectoryRelease{ProductID: "tool", DistributionID: "owner/tool", ReleaseSequence: 3, ResolvedRevision: testRelease(3, "").PackageSource.Revision}
	base := request("owner/tool", ClientCodex)
	base.Recorded = recorded
	s.Revocations = []DirectoryRevocation{{DistributionID: "owner/tool", ReleaseSequence: 3}}
	for _, op := range []DirectoryOperation{DirectoryInstall, DirectoryNewTarget, DirectoryRepair, DirectoryRematerialize} {
		r := base
		r.Operation = op
		if _, err := ResolveDirectory(s, r); err == nil {
			t.Fatalf("%s accepted revoked", op)
		}
	}
	r := base
	r.Operation = DirectoryRemove
	if _, err := ResolveDirectory(s, r); err != nil {
		t.Fatalf("remove: %v", err)
	}
	d.Releases = append(d.Releases, testRelease(4, "0.0.1"))
	updatePolicy := testPolicy(4, ClientCodex)
	updatePolicy.CurrentEvidence = []string{"passed/materialization/codex/4"}
	d.ReleasePolicies = append(d.ReleasePolicies, updatePolicy)
	s.Evidence = append(s.Evidence, testTrustedEvidence(DirectoryEvidence{ID: updatePolicy.CurrentEvidence[0], DistributionID: d.ID, ReleaseSequence: 4,
		PackageTreeDigest: d.Releases[len(d.Releases)-1].TreeDigest, Level: "materialization", Outcome: "passed", Client: ClientCodex}))
	r = base
	r.Operation = DirectoryUpdate
	if got, err := ResolveDirectory(s, r); err != nil || got.ReleaseSequence != 4 {
		t.Fatalf("safe update: %+v %v", got, err)
	}
	s.Revocations = nil
	d.ReleasePolicies[1].Status = ReleaseSuperseded
	for _, op := range []DirectoryOperation{DirectoryInstall, DirectoryNewTarget, DirectoryRepair, DirectoryRematerialize, DirectoryReproduce} {
		r = base
		r.Operation = op
		if got, err := ResolveDirectory(s, r); err != nil || got.ReleaseSequence != recorded.ReleaseSequence {
			t.Fatalf("exact recorded superseded %s: %+v %v", op, got, err)
		}
	}
	d.ReleasePolicies[1].Status = ReleaseActive
	d.Status = DistributionSuspended
	for _, op := range []DirectoryOperation{DirectoryInstall, DirectoryNewTarget} {
		r.Operation = op
		if got, err := ResolveDirectory(s, r); err != nil || got.ReleaseSequence != recorded.ReleaseSequence {
			t.Fatalf("suspended exact recorded %s: %+v %v", op, got, err)
		}
	}
	r.Operation = DirectoryUpdate
	if _, err := ResolveDirectory(s, r); err == nil {
		t.Fatal("suspended update accepted")
	}
	r.Recorded = nil
	for _, op := range []DirectoryOperation{DirectoryInstall, DirectoryNewTarget} {
		r.Operation = op
		if _, err := ResolveDirectory(s, r); err == nil {
			t.Fatalf("suspended %s created unrelated exposure", op)
		}
	}
	r.Recorded = recorded
	r.Operation = DirectoryRepair
	if _, err := ResolveDirectory(s, r); err != nil {
		t.Fatalf("suspended repair: %v", err)
	}
	r.Operation = DirectoryRematerialize
	if _, err := ResolveDirectory(s, r); err != nil {
		t.Fatalf("suspended rematerialization: %v", err)
	}
}

func TestResolveDirectoryUpdateWithoutSuccessorReturnsTypedNoSafeUpdate(t *testing.T) {
	snapshot := testDirectory()
	request := request("owner/tool", ClientCodex)
	request.Operation = DirectoryUpdate
	request.Recorded = &RecordedDirectoryRelease{
		ProductID: "tool", DistributionID: "owner/tool", ReleaseSequence: 3,
		ResolvedRevision: testRelease(3, "").PackageSource.Revision,
	}
	_, err := ResolveDirectory(snapshot, request)
	if !errors.Is(err, ErrDirectoryIneligible) || !errors.Is(err, ErrDirectoryNoSafeUpdate) {
		t.Fatalf("no-successor update error = %v", err)
	}
}

func TestResolveDirectoryRecordedReAddRetainsExactRelease(t *testing.T) {
	s := testDirectory()
	recorded := &RecordedDirectoryRelease{ProductID: "tool", DistributionID: "owner/tool", ReleaseSequence: 2, ResolvedRevision: testRelease(2, "").PackageSource.Revision}
	r := request("tool", ClientCodex)
	r.Recorded = recorded
	got, err := ResolveDirectory(s, r)
	if err != nil || got.DistributionID != recorded.DistributionID || got.ReleaseSequence != recorded.ReleaseSequence {
		t.Fatalf("recorded re-add moved release: %+v %v", got, err)
	}
	recorded.ResolvedRevision = ""
	if _, err := ResolveDirectory(s, r); err == nil || !strings.Contains(err.Error(), "no resolved package-source revision") {
		t.Fatalf("recorded release without resolved revision was accepted: %v", err)
	}
}

func TestResolveDirectoryRejectsRecordedPackageSourceRebindBeforeSelectingExactOrUpdate(t *testing.T) {
	s := testDirectory()
	recordedRevision := s.Distributions[2].Releases[0].PackageSource.Revision
	recorded := &RecordedDirectoryRelease{ProductID: "tool", DistributionID: "owner/tool", ReleaseSequence: 2, ResolvedRevision: recordedRevision}
	s.Distributions[2].Releases[0].PackageSource.Revision = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	for _, operation := range []DirectoryOperation{DirectoryRepair, DirectoryRematerialize, DirectoryNewTarget, DirectoryUpdate} {
		r := request("owner/tool", ClientCodex)
		r.Operation = operation
		r.Recorded = recorded
		if _, err := ResolveDirectory(s, r); err == nil || !strings.Contains(err.Error(), "package-source revision") {
			t.Fatalf("%s accepted rebound recorded release: %v", operation, err)
		}
	}
}

func TestResolveDirectoryRejectsEveryRecordedImmutableReleaseRebind(t *testing.T) {
	release := testRelease(2, "1.0.0")
	full := &RecordedDirectoryRelease{
		ProductID: "tool", DistributionID: "owner/tool", ReleaseSequence: release.Sequence,
		Repository: release.PackageSource.Repository, ResolvedRevision: release.PackageSource.Revision, Path: release.PackageSource.Path,
		TreeDigestAlgorithm: release.TreeDigestAlgorithm, TreeDigest: release.TreeDigest, ManifestDigest: release.ManifestDigest,
	}
	tests := []struct {
		name, field string
		mutate      func(*DirectoryRelease)
	}{
		{name: "repository", field: "repository", mutate: func(value *DirectoryRelease) { value.PackageSource.Repository = "other/repo" }},
		{name: "path", field: "path", mutate: func(value *DirectoryRelease) { value.PackageSource.Path = "other/plugin" }},
		{name: "tree algorithm", field: "tree digest algorithm", mutate: func(value *DirectoryRelease) { value.TreeDigestAlgorithm = "other-tree-v1" }},
		{name: "tree digest", field: "tree digest", mutate: func(value *DirectoryRelease) { value.TreeDigest = "sha256:" + strings.Repeat("c", 64) }},
		{name: "manifest digest", field: "manifest digest", mutate: func(value *DirectoryRelease) { value.ManifestDigest = "sha256:" + strings.Repeat("d", 64) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := testDirectory()
			distribution := &s.Distributions[2]
			distribution.Releases[0] = release
			test.mutate(&distribution.Releases[0])
			r := request("owner/tool", ClientCodex)
			r.Operation = DirectoryRepair
			recorded := *full
			r.Recorded = &recorded
			if _, err := ResolveDirectory(s, r); err == nil || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("accepted recorded %s rebind: %v", test.field, err)
			}
		})
	}
}

func TestResolveDirectoryRequiresCompleteSharedSurfaceEligibility(t *testing.T) {
	for _, selected := range []ClientID{ClientCopilot, ClientVSCode} {
		t.Run(string(selected), func(t *testing.T) {
			release := testRelease(1, "1.0.0")
			distribution := testDistribution("owner/tool", DistributionCommunity, []DirectoryRelease{release}, []DirectoryReleasePolicy{testPolicy(1, selected)})
			s := DirectorySnapshot{Sequence: 1, Products: []DirectoryProduct{{ID: "tool", ManifestName: "tool", DefaultDistribution: distribution.ID, Distributions: []string{distribution.ID}}}, Distributions: []DirectoryDistribution{distribution}}
			if _, err := ResolveDirectory(s, request("tool", selected)); !errors.Is(err, ErrDirectoryIneligible) {
				t.Fatalf("single %s shared-surface policy was accepted: %v", selected, err)
			}
		})
	}
}

func TestResolveDirectoryGates(t *testing.T) {
	s := testDirectory()
	p := &s.Distributions[2].ReleasePolicies[1]
	s.Distributions[2].ReleasePolicies[0].Status = ReleaseSuperseded
	p.MinimumInstallerVersion = "2.0.0"
	if _, err := ResolveDirectory(s, request("owner/tool", ClientCodex)); err == nil {
		t.Fatal("installer gate")
	}
	p.MinimumInstallerVersion = "1.0.0"
	s.Evidence = []DirectoryEvidence{testTrustedEvidence(DirectoryEvidence{ID: "failed/runtime", DistributionID: "owner/tool", ReleaseSequence: 3, PackageTreeDigest: testRelease(3, "").TreeDigest, Level: "runtime", Outcome: "failed", Client: ClientCodex, ClientVersion: "test-client", InstallerVersion: "1.2.3", OS: "linux", Architecture: "amd64"})}
	p.CurrentEvidence = []string{"failed/runtime"}
	if _, err := ResolveDirectory(s, request("owner/tool", ClientCodex)); err == nil {
		t.Fatal("evidence gate")
	}
	p.CurrentEvidence = nil
	s.Evidence = nil
	r := request("owner/tool", ClientCodex)
	r.RequiredComponents = []string{"extensions"}
	if _, err := ResolveDirectory(s, r); err == nil {
		t.Fatal("component gate")
	}
	r = request("owner/tool", ClientCodex)
	r.Scope = ScopeProject
	if _, err := ResolveDirectory(s, r); err == nil {
		t.Fatal("scope gate")
	}

	s = testDirectory()
	p = &s.Distributions[2].ReleasePolicies[1]
	s.Distributions[2].ReleasePolicies[0].Status = ReleaseSuperseded
	s.Evidence = []DirectoryEvidence{testTrustedEvidence(DirectoryEvidence{ID: "failed/schema", DistributionID: "owner/tool", ReleaseSequence: 3, PackageTreeDigest: testRelease(3, "").TreeDigest, Level: "schema", Outcome: "failed"})}
	p.CurrentEvidence = []string{"failed/schema"}
	if _, err := ResolveDirectory(s, request("owner/tool", ClientCodex)); err == nil {
		t.Fatal("schema evidence gate")
	}
}

func TestResolveDirectoryUpstreamMaterializationAndExactFailedTuple(t *testing.T) {
	s := testDirectory()
	policy := &s.Distributions[2].ReleasePolicies[1]
	passed := append([]string(nil), policy.CurrentEvidence...)
	policy.CurrentEvidence = nil
	if selected, err := ResolveDirectory(s, request("owner/tool", ClientCodex)); err != nil || selected.ReleaseSequence != 2 {
		t.Fatalf("newest upstream without passed materialization did not fall back to eligible release 2: %+v %v", selected, err)
	}
	policy.CurrentEvidence = passed
	s.Distributions[2].ReleasePolicies[0].Status = ReleaseSuperseded
	exactFailure := testTrustedEvidence(DirectoryEvidence{ID: "failed/exact", DistributionID: "owner/tool", ReleaseSequence: 3,
		PackageTreeDigest: testRelease(3, "").TreeDigest, Level: "runtime", Outcome: "failed", Client: ClientCodex,
		ClientVersion: "test-client", InstallerVersion: "1.2.3", OS: "linux", Architecture: "amd64", DependencyIdentity: "npx"})
	s.Evidence = append(s.Evidence, exactFailure)
	policy.CurrentEvidence = append(policy.CurrentEvidence, exactFailure.ID)
	exact := request("owner/tool", ClientCodex)
	exact.DependencyIdentity = map[ClientID]string{ClientCodex: "npx"}
	if _, err := ResolveDirectory(s, exact); !errors.Is(err, ErrDirectoryIneligible) {
		t.Fatalf("exact failed tuple was eligible: %v", err)
	}
	for name, change := range map[string]func(*DirectoryResolveRequest){
		"client version":             func(value *DirectoryResolveRequest) { value.ClientVersions[ClientCodex] = "changed" },
		"unavailable client version": func(value *DirectoryResolveRequest) { delete(value.ClientVersions, ClientCodex) },
		"dependency":                 func(value *DirectoryResolveRequest) { value.DependencyIdentity[ClientCodex] = "node" },
		"os":                         func(value *DirectoryResolveRequest) { value.OS = "darwin" },
		"architecture":               func(value *DirectoryResolveRequest) { value.Architecture = "arm64" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := exact
			changed.ClientVersions = map[ClientID]string{ClientCodex: exact.ClientVersions[ClientCodex]}
			changed.DependencyIdentity = map[ClientID]string{ClientCodex: exact.DependencyIdentity[ClientCodex]}
			change(&changed)
			if _, err := ResolveDirectory(s, changed); err != nil {
				t.Fatalf("changed tuple blocked by historical failure: %v", err)
			}
		})
	}
}

func TestResolveDirectoryExactFailedCompatibilityLevelsBlock(t *testing.T) {
	for _, level := range []string{"materialization", "discovery", "runtime"} {
		t.Run(level, func(t *testing.T) {
			s := testDirectory()
			s.Distributions[2].ReleasePolicies[0].Status = ReleaseSuperseded
			policy := &s.Distributions[2].ReleasePolicies[1]
			failure := testTrustedEvidence(DirectoryEvidence{ID: "failed/" + level, DistributionID: "owner/tool", ReleaseSequence: 3,
				PackageTreeDigest: testRelease(3, "").TreeDigest, Level: level, Outcome: "failed", Client: ClientCodex,
				ClientVersion: "test-client", InstallerVersion: "1.2.3", OS: "linux", Architecture: "amd64"})
			s.Evidence = append(s.Evidence, failure)
			policy.CurrentEvidence = append(policy.CurrentEvidence, failure.ID)
			if _, err := ResolveDirectory(s, request("owner/tool", ClientCodex)); !errors.Is(err, ErrDirectoryIneligible) {
				t.Fatalf("exact failed %s evidence did not block: %v", level, err)
			}
		})
	}
}

func TestResolveDirectoryEligibilityRequiresTrustedEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*DirectoryReleasePolicy, *DirectoryEvidence)
		wantOK bool
	}{
		{name: "trusted pass", wantOK: true},
		{name: "missing trust", mutate: func(_ *DirectoryReleasePolicy, e *DirectoryEvidence) { e.Trust = nil }},
		{name: "unknown trust", mutate: func(_ *DirectoryReleasePolicy, e *DirectoryEvidence) { e.Trust.Kind = "contributor_asserted" }},
		{name: "forged workflow", mutate: func(_ *DirectoryReleasePolicy, e *DirectoryEvidence) {
			e.Trust.Workflow = "contributor/evidence/.github/workflows/directory.yml"
		}},
		{name: "publisher-reviewed materialization", wantOK: true, mutate: func(_ *DirectoryReleasePolicy, e *DirectoryEvidence) {
			e.Trust = &DirectoryEvidenceTrust{Kind: "reviewed_external"}
		}},
		{name: "malformed reviewed trust", mutate: func(_ *DirectoryReleasePolicy, e *DirectoryEvidence) {
			e.Trust = &DirectoryEvidenceTrust{Kind: "reviewed_external", Workflow: "owner/evidence/.github/workflows/forged.yml"}
		}},
		{name: "non-current trusted evidence", mutate: func(p *DirectoryReleasePolicy, _ *DirectoryEvidence) { p.CurrentEvidence = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testDirectory()
			s.Distributions[2].ReleasePolicies[0].Status = ReleaseSuperseded
			policy := &s.Distributions[2].ReleasePolicies[1]
			evidence := evidenceByID(s, policy.CurrentEvidence[0])
			if tc.mutate != nil {
				tc.mutate(policy, evidence)
			}
			_, err := ResolveDirectory(s, request("owner/tool", ClientCodex))
			if tc.wantOK && err != nil {
				t.Fatalf("trusted pass did not promote release: %v", err)
			}
			if !tc.wantOK && !errors.Is(err, ErrDirectoryIneligible) {
				t.Fatalf("untrusted pass promoted release: %v", err)
			}
		})
	}
}

func TestResolveDirectoryOnlyTrustedCurrentFailureBlocks(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*DirectoryEvidence, *DirectoryReleasePolicy)
		blocked bool
	}{
		{name: "trusted fail", blocked: true},
		{name: "reviewed runtime fail", mutate: func(e *DirectoryEvidence, _ *DirectoryReleasePolicy) {
			e.Trust = &DirectoryEvidenceTrust{Kind: "reviewed_external"}
		}, blocked: true},
		{name: "missing trust", mutate: func(e *DirectoryEvidence, _ *DirectoryReleasePolicy) { e.Trust = nil }},
		{name: "unknown trust", mutate: func(e *DirectoryEvidence, _ *DirectoryReleasePolicy) { e.Trust.Kind = "self_declared" }},
		{name: "forged trust", mutate: func(e *DirectoryEvidence, _ *DirectoryReleasePolicy) {
			e.Artifact.Revision = "dddddddddddddddddddddddddddddddddddddddd"
		}},
		{name: "non-current fail", mutate: func(_ *DirectoryEvidence, p *DirectoryReleasePolicy) { p.CurrentEvidence = p.CurrentEvidence[:1] }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testDirectory()
			s.Distributions[2].ReleasePolicies[0].Status = ReleaseSuperseded
			policy := &s.Distributions[2].ReleasePolicies[1]
			failure := testTrustedEvidence(DirectoryEvidence{ID: "runtime/fail", DistributionID: "owner/tool", ReleaseSequence: 3,
				PackageTreeDigest: testRelease(3, "").TreeDigest, Level: "runtime", Outcome: "failed", Client: ClientCodex,
				ClientVersion: "test-client", InstallerVersion: "1.2.3", OS: "linux", Architecture: "amd64"})
			s.Evidence = append(s.Evidence, failure)
			policy.CurrentEvidence = append(policy.CurrentEvidence, failure.ID)
			if tc.mutate != nil {
				tc.mutate(&s.Evidence[len(s.Evidence)-1], policy)
			}
			_, err := ResolveDirectory(s, request("owner/tool", ClientCodex))
			if tc.blocked && !errors.Is(err, ErrDirectoryIneligible) {
				t.Fatalf("trusted failure did not block: %v", err)
			}
			if !tc.blocked && err != nil {
				t.Fatalf("untrusted or non-current failure blocked: %v", err)
			}
		})
	}
}

func TestResolveDirectoryMultiTargetOrderIsDeterministic(t *testing.T) {
	s := testDirectory()
	first, err := ResolveDirectory(s, request("tool", ClientCodex, ClientCursor))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ResolveDirectory(s, request("tool", ClientCursor, ClientCodex))
	if err != nil {
		t.Fatal(err)
	}
	if first.DistributionID != second.DistributionID || first.ReleaseSequence != second.ReleaseSequence {
		t.Fatalf("target order changed selection: %+v != %+v", first, second)
	}
}
