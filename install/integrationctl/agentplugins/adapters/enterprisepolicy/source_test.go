package enterprisepolicy

import (
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func testSnapshot(t *testing.T, bodies ...string) Snapshot {
	t.Helper()
	snapshot := Snapshot{Fingerprint: "test-policy"}
	for _, body := range bodies {
		policy, err := Decode([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		snapshot.Layers = append(snapshot.Layers, Layer{Layer: "test", Path: "test-policy.yaml", Present: true, Policy: policy})
	}
	return snapshot
}

func assertPolicyResult(t *testing.T, snapshot Snapshot, req domain.InstallPolicyRequest, denied bool) {
	t.Helper()
	_, err := snapshot.Check(req)
	var refusal *domain.InstallPolicyError
	if denied && !errors.As(err, &refusal) {
		t.Fatalf("wanted policy denial, got %v", err)
	}
	if !denied && err != nil {
		t.Fatalf("unexpected denial: %v", err)
	}
}

func TestSourceHostUsesActualURLBeforeGitHubShorthand(t *testing.T) {
	t.Parallel()
	snapshot := testSnapshot(t, "schema_version: 2\nsources:\n  allow_hosts: [github.com]\n")
	for name, source := range map[string]domain.SourceIdentity{
		"git-url": {SourceKind: "git", SourceURL: "https://evil.test/acme/repo.git", Repository: "acme/repo"},
		"archive": {SourceKind: "archive", SourceURL: "https://evil.test/archive.zip", Repository: "acme/repo"},
		"web":     {SourceKind: "https-skill", SourceURL: "https://evil.test/skills/index.json", Repository: "acme/repo"},
	} {
		t.Run(name, func(t *testing.T) {
			assertPolicyResult(t, snapshot, domain.InstallPolicyRequest{Source: source}, true)
		})
	}
	assertPolicyResult(t, snapshot, domain.InstallPolicyRequest{Source: domain.SourceIdentity{Repository: "Acme/Repo"}}, false)
}

func TestRepositoryPolicyUsesConjunctiveNamespacesAndActualOrigins(t *testing.T) {
	t.Parallel()
	snapshot := testSnapshot(t,
		"schema_version: 2\nsources:\n  allow_repositories: [github.com/acme/*]\n",
		"schema_version: 2\nsources:\n  allow_repositories: [github.com/acme/safe]\n")
	for _, raw := range []string{"https://github.com/Acme/Safe.git", "ssh://git@github.com/acme/safe.git", "git@github.com:acme/safe.git"} {
		assertPolicyResult(t, snapshot, domain.InstallPolicyRequest{Source: domain.SourceIdentity{SourceKind: "git", SourceURL: raw}}, false)
	}
	for _, raw := range []string{"https://github.com/acme/other.git", "https://github.com.evil.test/acme/safe.git", "ssh://git@github.com:2222/acme/safe.git", "https://github.com/acme%2fsafe.git"} {
		assertPolicyResult(t, snapshot, domain.InstallPolicyRequest{Source: domain.SourceIdentity{SourceKind: "git", SourceURL: raw}}, true)
	}
	assertPolicyResult(t, snapshot, domain.InstallPolicyRequest{Source: domain.SourceIdentity{SourceKind: "archive", SourceURL: "https://github.com/acme/safe/archive.zip"}}, true)
	assertPolicyResult(t, snapshot, domain.InstallPolicyRequest{Source: domain.SourceIdentity{Repository: "acme/safe", PackageSubpath: "nested/skill"}, OriginMode: domain.OriginModeDirectory}, false)
	assertPolicyResult(t, snapshot, domain.InstallPolicyRequest{Source: domain.SourceIdentity{SourceKind: "git", Repository: "acme/safe"}, OriginMode: domain.OriginModeDirectory}, true)
	denied := testSnapshot(t, "schema_version: 2\nsources:\n  allow_repositories: [git.example.test/Team/*]\n  deny_repositories: [git.example.test/Team/Forbidden]\n")
	assertPolicyResult(t, denied, domain.InstallPolicyRequest{Source: domain.SourceIdentity{SourceKind: "git", SourceURL: "https://git.example.test/Team/nested/Safe.git"}}, false)
	assertPolicyResult(t, denied, domain.InstallPolicyRequest{Source: domain.SourceIdentity{SourceKind: "git", SourceURL: "https://git.example.test/team/nested/Safe.git"}}, true)
	assertPolicyResult(t, denied, domain.InstallPolicyRequest{Source: domain.SourceIdentity{SourceKind: "git", SourceURL: "https://git.example.test/Team/Forbidden.git"}}, true)
}

func TestLocalNeutralAndLegacyRules(t *testing.T) {
	t.Parallel()
	local := domain.InstallPolicyRequest{Source: domain.SourceIdentity{SourceBindingHint: "direct-local", CanonicalSource: "/workspace/plugin"}}
	for _, test := range []struct {
		body   string
		denied bool
	}{
		{"schema_version: 1\n", true},
		{"schema_version: 2\n", false},
		{"schema_version: 2\nsources:\n  allowed_kinds: [local]\n", false},
		{"schema_version: 2\nsources:\n  deny_hosts: [github.com]\n", false},
		{"schema_version: 2\nsources:\n  allow_hosts: [github.com]\n", true},
		{"schema_version: 2\nsources:\n  allow_repositories: [github.com/acme/*]\n", true},
		{"schema_version: 2\nsources:\n  allowed_kinds: []\n", true},
	} {
		assertPolicyResult(t, testSnapshot(t, test.body), local, test.denied)
	}
	assertPolicyResult(t, testSnapshot(t, "schema_version: 2\n"), domain.InstallPolicyRequest{}, false)
	assertPolicyResult(t, testSnapshot(t, "schema_version: 1\n"), domain.InstallPolicyRequest{}, true)
	local.Source = domain.SourceIdentity{SourceKind: "local-skill", SourceURL: "C:/repo/skills/review"}
	assertPolicyResult(t, testSnapshot(t, "schema_version: 2\nsources:\n  allowed_kinds: [local]\n"), local, false)
}

func TestRequestedPinsCannotBeInferredFromResolvedBytes(t *testing.T) {
	t.Parallel()
	snapshot := testSnapshot(t, "schema_version: 2\nintegrity:\n  require_pinned_git: true\n  require_archive_sha256: true\n")
	git := domain.InstallPolicyRequest{Source: domain.SourceIdentity{Repository: "acme/safe", ResolvedRevision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	archive := domain.InstallPolicyRequest{Source: domain.SourceIdentity{SourceKind: "archive", SourceURL: "https://source.test/release.zip", ArchiveSHA256: "resolved-hash"}}
	assertPolicyResult(t, snapshot, git, true)
	assertPolicyResult(t, snapshot, archive, true)
	no, yes := false, true
	git.GitPinned, archive.ArchivePinned = &no, &no
	assertPolicyResult(t, snapshot, git, true)
	assertPolicyResult(t, snapshot, archive, true)
	git.GitPinned, archive.ArchivePinned = &yes, &yes
	assertPolicyResult(t, snapshot, git, false)
	assertPolicyResult(t, snapshot, archive, false)
	git.OriginMode, git.GitPinned = domain.OriginModeDirectory, nil
	assertPolicyResult(t, snapshot, git, false)
}

func TestSourceOverrideCannotHideActualEnvelopeSource(t *testing.T) {
	t.Parallel()
	snapshot := testSnapshot(t, "schema_version: 2\nsources:\n  allow_hosts: [github.com]\n")
	req := domain.InstallPolicyRequest{
		Source:   domain.SourceIdentity{Repository: "acme/safe"},
		Envelope: &domain.PackageEnvelope{Source: domain.SourceIdentity{SourceKind: "git", SourceURL: "https://evil.test/acme/safe.git"}},
	}
	assertPolicyResult(t, snapshot, req, true)
	req.Source = domain.SourceIdentity{}
	assertPolicyResult(t, snapshot, req, true)
	req.Envelope.Source = domain.SourceIdentity{Repository: "acme/safe"}
	assertPolicyResult(t, snapshot, req, false)
}
