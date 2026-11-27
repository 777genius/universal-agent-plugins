package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func TestEnterprisePolicyHTTPSAutoSelectsAdmittedTransport(t *testing.T) {
	for _, tc := range []struct {
		name, policy, response, kind string
		archive, git                 int
		denied, acquisitionFailure   bool
	}{
		{name: "git only skips archive HTTP", policy: "sources:\n  allowed_kinds: [git]\n", kind: "git", git: 1},
		{name: "archive only accepts archive", policy: "sources:\n  allowed_kinds: [archive]\n", kind: "archive", archive: 1},
		{name: "both allowed keep archive first", policy: "sources:\n  allowed_kinds: [archive, git]\n", kind: "archive", archive: 1},
		{name: "both allowed use Git fallback", policy: "sources:\n  allowed_kinds: [archive, git]\n", response: "not archive", kind: "git", archive: 1, git: 1},
		{name: "archive checksum rule selects Git", policy: "integrity:\n  require_archive_sha256: true\n", kind: "git", git: 1},
		{name: "Git pin rule still admits archive", policy: "integrity:\n  require_pinned_git: true\n", kind: "archive", archive: 1},
		{name: "archive only refuses Git fallback", policy: "sources:\n  allowed_kinds: [archive]\n", response: "not archive", archive: 1, denied: true},
		{name: "both kinds denied before HTTP", policy: "sources:\n  allowed_kinds: []\n", denied: true},
		{name: "both integrity rules deny before HTTP", policy: "integrity:\n  require_pinned_git: true\n  require_archive_sha256: true\n", denied: true},
		{name: "shared host denial before HTTP", policy: "sources:\n  deny_hosts: [127.0.0.1]\n", denied: true},
		{name: "shared scope denial before HTTP", policy: "install:\n  allowed_scopes: [project]\n", denied: true},
		{name: "recognized corrupt archive cannot fall back", response: "corrupt archive", archive: 1, acquisitionFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newEnterpriseHTTPSFixture(t, tc.response)
			fixture.app = fixture.app.withEnterprisePolicyGate(sourcePolicyApp(t, "schema_version: 2\n"+tc.policy).InstallationPolicy).withEnterprisePolicySession()
			loaded, err := fixture.app.acquireDirectInput(t.Context(), fixture.spec)
			fixture.assertTransports(t, tc.archive, tc.git)
			if tc.denied || tc.acquisitionFailure {
				var denied *domain.InstallPolicyError
				if err == nil || errors.As(err, &denied) != tc.denied {
					t.Fatalf("unexpected refusal: %v", err)
				}
				fixture.assertNoSnapshots(t)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = loaded.cleanup() }()
			source := loaded.envelope.Source
			if source.SourceKind != tc.kind || source.RequestedSource != fixture.spec.Requested || loaded.envelope.TreeDigest == "" {
				t.Fatalf("resolved source lost kind, original request or immutable bytes: %+v", source)
			}
			if _, err := fixture.app.checkEnterprisePolicy(t.Context(), enterprisePolicyRequest(loaded, domain.ScopeUser, nil)); err != nil {
				t.Fatalf("admitted source failed final actual-kind check: %v", err)
			}
			strict := sourcePolicyApp(t, "schema_version: 2\nintegrity:\n  require_pinned_git: true\n  require_archive_sha256: true\n")
			_, err = strict.checkEnterprisePolicy(t.Context(), enterprisePolicyRequest(loaded, domain.ScopeUser, nil))
			var denied *domain.InstallPolicyError
			if !errors.As(err, &denied) {
				t.Fatalf("resolved revision/checksum was promoted to original pin intent: %v", err)
			}
		})
	}
}

func TestEnterprisePolicyHTTPSAutoGateFailuresStopTransportSelection(t *testing.T) {
	invalid := sourcePolicyApp(t, "schema_version: 2\nunknown_policy_rule: true\n")
	_, configErr := invalid.InstallationPolicy.Check(t.Context(), domain.InstallPolicyRequest{})
	var config *enterprisepolicy.ConfigError
	if !errors.As(configErr, &config) {
		t.Fatalf("fixture must return a real policy ConfigError: %v", configErr)
	}
	denial := &domain.InstallPolicyError{}
	unknown := errors.New("embedded host policy unavailable")
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"invalid policy", configErr},
		{"changed policy", domain.ErrInstallPolicyChanged},
		{"embedded host failure", unknown},
		{"canceled check", context.Canceled},
		{"denial joined with invalid policy", errors.Join(denial, configErr)},
		{"denial joined with changed policy", errors.Join(denial, domain.ErrInstallPolicyChanged)},
		{"denial joined with unknown failure", fmt.Errorf("wrapped: %w", errors.Join(denial, unknown))},
	} {
		for _, phase := range []string{"archive", "git after denied archive", "git after archive response"} {
			t.Run(tc.name+"/"+phase, func(t *testing.T) {
				fixture := newEnterpriseHTTPSFixture(t, "not archive")
				fixture.app.InstallationPolicy = enterpriseHTTPSPolicyFunc(func(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
					if phase == "archive" || request.Source.SourceKind == "git" {
						return "policy", tc.err
					}
					if phase == "git after denied archive" {
						return "policy", denial
					}
					return "policy", nil
				})
				fixture.app = fixture.app.withEnterprisePolicySession()
				_, err := fixture.app.acquireDirectInput(t.Context(), fixture.spec)
				if !errors.Is(err, tc.err) {
					t.Fatalf("policy failure was replaced during transport selection: got %v want %v", err, tc.err)
				}
				archive := 0
				if phase == "git after archive response" {
					archive = 1
				}
				fixture.assertTransports(t, archive, 0)
				fixture.assertNoSnapshots(t)
			})
		}
	}
}

func TestEnterprisePolicyHTTPSAutoWrappedDenialsCanSelectGit(t *testing.T) {
	denial := &domain.InstallPolicyError{}
	for _, refusal := range []error{fmt.Errorf("wrapped: %w", denial), errors.Join(denial, fmt.Errorf("other layer: %w", denial))} {
		fixture := newEnterpriseHTTPSFixture(t, "")
		fixture.app.InstallationPolicy = enterpriseHTTPSPolicyFunc(func(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
			if request.Source.SourceKind == "archive" {
				return "policy", refusal
			}
			return "policy", nil
		})
		loaded, err := fixture.app.acquireDirectInput(t.Context(), fixture.spec)
		if err != nil {
			t.Fatal(err)
		}
		if err := loaded.cleanup(); err != nil {
			t.Fatal(err)
		}
		fixture.assertTransports(t, 0, 1)
		fixture.assertNoSnapshots(t)
	}
}

func TestEnterprisePolicyExplicitSourcesDoNotSelectAnotherKind(t *testing.T) {
	for _, kind := range []string{"git", "archive"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newEnterpriseHTTPSFixture(t, "")
			allowed, suffix := "git", ".zip"
			if kind == "git" {
				allowed, suffix = "archive", ".git"
			}
			spec, err := sourcespec.Parse(fixture.spec.Requested + suffix)
			if err != nil {
				t.Fatal(err)
			}
			fixture.app.InstallationPolicy = sourcePolicyApp(t, "schema_version: 2\nsources:\n  allowed_kinds: ["+allowed+"]\n").InstallationPolicy
			_, err = fixture.app.acquireDirectInput(t.Context(), spec)
			var denied *domain.InstallPolicyError
			if !errors.As(err, &denied) {
				t.Fatalf("explicit source was reinterpreted: %v", err)
			}
			fixture.assertTransports(t, 0, 0)
		})
	}
}

type enterpriseHTTPSPolicyFunc func(context.Context, domain.InstallPolicyRequest) (string, error)

func (gate enterpriseHTTPSPolicyFunc) Check(ctx context.Context, request domain.InstallPolicyRequest) (string, error) {
	return gate(ctx, request)
}

type enterpriseHTTPSFixture struct {
	app      App
	spec     sourcespec.Spec
	source   *enterpriseHTTPSAcquirer
	requests atomic.Int32
}

func newEnterpriseHTTPSFixture(t *testing.T, response string) *enterpriseHTTPSFixture {
	t.Helper()
	root := writeCLIPlugin(t)
	payload := directArchiveBytes(t, root, "zip", "package/")
	switch response {
	case "not archive":
		payload = []byte("not an archive")
	case "corrupt archive":
		payload = []byte("PK\x03\x04broken zip")
	}
	fixture := &enterpriseHTTPSFixture{app: newCLIFixture(t, nil).app}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fixture.requests.Add(1)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)
	fixture.source = &enterpriseHTTPSAcquirer{Acquirer: sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}, root: root}
	fixture.app.SourceAcquirer = fixture.source
	fixture.app.Scope = domain.ScopeUser
	var err error
	fixture.spec, err = sourcespec.Parse(server.URL + "/download")
	if err != nil || fixture.spec.Kind != sourcespec.KindHTTPSAuto {
		t.Fatalf("fixture URL is not ambiguous HTTPS: %+v %v", fixture.spec, err)
	}
	return fixture
}

func (fixture *enterpriseHTTPSFixture) assertTransports(t *testing.T, archive, git int) {
	t.Helper()
	if fixture.source.archiveCalls != archive || int(fixture.requests.Load()) != archive || fixture.source.gitLookups != git || fixture.source.gitAcquisitions != git {
		t.Fatalf("unexpected transport effects: archives=%d HTTP=%d Git lookups=%d Git acquisitions=%d; want archive=%d Git=%d", fixture.source.archiveCalls, fixture.requests.Load(), fixture.source.gitLookups, fixture.source.gitAcquisitions, archive, git)
	}
}

func (fixture *enterpriseHTTPSFixture) assertNoSnapshots(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(fixture.source.TempRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("refusal retained acquisition effects: %+v %v", entries, err)
	}
}

// Archive acquisition is real HTTPS/extraction. Only native Git is injected;
// its returned package still uses the production immutable snapshot and loader.
type enterpriseHTTPSAcquirer struct {
	sourceacquisition.Acquirer
	root                                      string
	archiveCalls, gitLookups, gitAcquisitions int
}

func (source *enterpriseHTTPSAcquirer) AcquireArchive(ctx context.Context, spec sourcespec.Archive, archiveDigest, treeDigest string) (domain.PackageSnapshot, error) {
	source.archiveCalls++
	return source.Acquirer.AcquireArchive(ctx, spec, archiveDigest, treeDigest)
}

func (source *enterpriseHTTPSAcquirer) ResolveGitSource(_ context.Context, spec sourcespec.Git) (sourceacquisition.ResolvedGit, error) {
	source.gitLookups++
	spec.Commit = strings.Repeat("a", 40)
	return sourceacquisition.ResolvedGit{Git: spec, Resolution: sourceacquisition.RefResolution{
		Commit: spec.Commit, RequestedRef: "default", ResolvedRef: "refs/heads/main", DefaultBranch: "main",
	}}, nil
}

func (source *enterpriseHTTPSAcquirer) AcquireGit(ctx context.Context, _ sourcespec.Git, _, _ string) (domain.PackageSnapshot, error) {
	source.gitAcquisitions++
	return source.AcquireLocal(ctx, source.root)
}
