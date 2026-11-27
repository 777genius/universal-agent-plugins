package agentpluginscli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type enterpriseRepairAcquisitionSpy struct {
	sourceacquisition.Acquirer
	calls int
}

func (spy *enterpriseRepairAcquisitionSpy) AcquireGit(context.Context, sourcespec.Git, string, string) (domain.PackageSnapshot, error) {
	spy.calls++
	return domain.PackageSnapshot{}, errors.New("stopped at acquisition")
}

func (spy *enterpriseRepairAcquisitionSpy) AcquireArchive(context.Context, sourcespec.Archive, string, string) (domain.PackageSnapshot, error) {
	spy.calls++
	return domain.PackageSnapshot{}, errors.New("stopped at acquisition")
}

func TestEnterprisePolicyChecksRecordedRepairBeforeAcquisition(t *testing.T) {
	for _, source := range []domain.SourceBinding{
		{SourceKind: "git", SourceURL: "https://github.com/acme/demo.git", RequestedSource: "acme/demo@main", ResolvedRevision: strings.Repeat("a", 40)},
		{SourceKind: "archive", SourceURL: "https://packages.example/demo.zip", RequestedSource: "https://packages.example/demo.zip", ResolvedRevision: "sha256:" + strings.Repeat("b", 64), ArchiveSHA256: strings.Repeat("b", 64)},
	} {
		for _, tc := range []struct {
			name, policy string
			invalid      bool
			allowed      bool
		}{
			{name: "denied source", policy: "schema_version: 2\nsources:\n  allowed_kinds: []\n"},
			{name: "invalid policy", policy: "schema_version: 2\nunknown: true\n", invalid: true},
			{name: "original selector unpinned", policy: "schema_version: 2\nintegrity:\n  require_pinned_git: true\n  require_archive_sha256: true\n"},
			{name: "allowed source", policy: "schema_version: 2\n", allowed: true},
		} {
			t.Run(source.SourceKind+"/"+tc.name, func(t *testing.T) {
				app := sourcePolicyApp(t, tc.policy)
				spy := &enterpriseRepairAcquisitionSpy{}
				app.SourceAcquirer = spy
				installation := domain.Installation{Source: source, OriginMode: domain.OriginModeDirect}
				binding := domain.ClientBinding{PackageRevision: &domain.ClientPackageRevision{
					ResolvedRevision: source.ResolvedRevision, TreeDigest: "sha256:tree", ManifestDigest: "sha256:manifest",
				}}
				_, err := app.loadRecordedDirect(t.Context(), installation, binding)
				if tc.allowed {
					if spy.calls != 1 || err == nil {
						t.Fatalf("allowed repair did not reach acquisition: calls=%d err=%v", spy.calls, err)
					}
					return
				}
				if spy.calls != 0 {
					t.Fatalf("repair acquired a source before policy refusal: calls=%d err=%v", spy.calls, err)
				}
				var invalid *enterprisepolicy.ConfigError
				var denied *domain.InstallPolicyError
				if tc.invalid && !errors.As(err, &invalid) || !tc.invalid && !errors.As(err, &denied) {
					t.Fatalf("repair did not return the policy refusal: %v", err)
				}
			})
		}
	}
}
