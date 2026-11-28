package agentpluginscli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type legacyRepairSourceSpy struct {
	SourceAcquirer
	calls int
}

func (spy *legacyRepairSourceSpy) AcquireGitHub(context.Context, string, string, string) (domain.PackageSnapshot, error) {
	spy.calls++
	return domain.PackageSnapshot{}, errors.New("unexpected repair acquisition")
}

func TestLegacyRepairCannotPromoteReplayRevisionToRequestedPin(t *testing.T) {
	for _, requested := range []string{"", "acme/demo@main", "discovery:demo"} {
		for _, grouped := range []bool{false, true} {
			t.Run(requested+map[bool]string{false: "/single", true: "/group"}[grouped], func(t *testing.T) {
				app := sourcePolicyApp(t, "schema_version: 2\nintegrity:\n  require_pinned_git: true\n")
				spy := &legacyRepairSourceSpy{}
				app.SourceAcquirer = spy
				installation := legacyRepairInstallation(requested)
				replay := "github:acme/demo@" + installation.Source.ResolvedRevision + "//plugin"
				var err error
				if grouped {
					session := repairManySession{ctx: t.Context(), app: app, installation: installation,
						requests: map[string]*repairRevisionRequest{"legacy": {source: replay}}}
					_, err = session.resolveRevision("legacy")
				} else {
					_, err = app.loadLegacyRepairPackage(t.Context(), installation, replay, packageResolutionRequest{Operation: domain.DirectoryRepair})
				}
				var denied *domain.InstallPolicyError
				if !errors.As(err, &denied) || spy.calls != 0 {
					t.Fatalf("synthetic replay pin reached acquisition: calls=%d err=%v", spy.calls, err)
				}
			})
		}
	}
}

func TestLegacyRepairRetainsOriginalIntentAndAcquiredIdentity(t *testing.T) {
	commit := strings.Repeat("a", 40)
	for _, requested := range []string{"", "acme/demo@main", "acme/demo@" + commit} {
		t.Run(requested, func(t *testing.T) {
			fixture := newCLIFixture(t, nil)
			acquirer := &localBackedSourceAcquirer{delegate: fixture.app.SourceAcquirer, root: writeCLIPlugin(t)}
			fixture.app.SourceAcquirer = acquirer
			fixture.app.InstallationPolicy = sourcePolicyApp(t, "schema_version: 2\n").InstallationPolicy
			installation := legacyRepairInstallation(requested)
			loaded, err := fixture.app.loadLegacyRepairPackage(t.Context(), installation, "github:acme/demo@"+commit+"//plugin", packageResolutionRequest{Operation: domain.DirectoryRepair})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = loaded.cleanup() })
			source := loaded.envelope.Source
			if acquirer.directGitCalls != 1 || source.RequestedSource != requested || source.Repository != "acme/demo" || source.PackageSubpath != "plugin" || source.ResolvedRevision != commit || loaded.envelope.TreeDigest == "" {
				t.Fatalf("repair lost original intent or immutable identity: calls=%d source=%+v", acquirer.directGitCalls, source)
			}
			input := withEnterprisePins(usecase.AddInput{Envelope: loaded.envelope}, loaded)
			pinned := input.GitPinned != nil && *input.GitPinned
			if pinned != (requested == "acme/demo@"+commit) {
				t.Fatalf("final input promoted replay into a requested pin: %v", pinned)
			}
			fixture.app.InstallationPolicy = sourcePolicyApp(t, "schema_version: 2\nintegrity:\n  require_pinned_git: true\n").InstallationPolicy
			_, err = fixture.app.checkEnterprisePolicy(t.Context(), enterprisePolicyRequest(loaded, domain.ScopeUser, nil))
			var denied *domain.InstallPolicyError
			if pinned && err != nil || !pinned && !errors.As(err, &denied) {
				t.Fatalf("final policy did not see original pin intent: %v", err)
			}
		})
	}
}

func TestLegacyRepairLocalSourceRemainsClassifiable(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	root := writeCLIPlugin(t)
	fixture.app.InstallationPolicy = sourcePolicyApp(t, "schema_version: 2\nsources:\n  allowed_kinds: [local]\n").InstallationPolicy
	installation := domain.Installation{OriginMode: domain.OriginModeDirect,
		Source: domain.SourceBinding{CanonicalSource: root, RequestedSource: root}}
	loaded, err := fixture.app.loadLegacyRepairPackage(t.Context(), installation, root, packageResolutionRequest{Operation: domain.DirectoryRepair})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = loaded.cleanup() })
	if loaded.envelope.Source.RequestedSource != root || loaded.envelope.Source.CanonicalSource != root {
		t.Fatalf("local repair changed source authority: %+v", loaded.envelope.Source)
	}
}

func legacyRepairInstallation(requested string) domain.Installation {
	return domain.Installation{OriginMode: domain.OriginModeDirect, Source: domain.SourceBinding{
		RequestedSource: requested, Repository: "acme/demo", PackageSubpath: "plugin",
		ResolvedRevision: strings.Repeat("a", 40),
		CanonicalSource:  "https://github.com/acme/demo@" + strings.Repeat("a", 40) + "//plugin",
	}}
}
