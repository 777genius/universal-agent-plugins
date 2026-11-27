package agentpluginscli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func sourcePolicyApp(t *testing.T, body string) App {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	dependencyWrite(t, path, body)
	return App{Scope: domain.ScopeUser, InstallationPolicy: enterprisepolicy.Files{Paths: []enterprisepolicy.Path{{Layer: "test", Path: path}}}}
}

func TestEnterprisePolicyRefusesBeforeSourceAcquisition(t *testing.T) {
	app := sourcePolicyApp(t, "schema_version: 2\nsources:\n  allowed_kinds: []\n")
	ctx := context.Background()
	_, local := app.acquireLocal(ctx, t.TempDir())
	_, git := app.acquireGitHub(ctx, "acme/demo@"+strings.Repeat("a", 40), "acme/demo", strings.Repeat("a", 40), "", "")
	replay := verifyInstallationDependencyDeclarations(ctx, app, environment.Lock{Packages: []environment.LockedPackage{{Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: "acme/demo"}}}}})
	_, saved := loadSavedAddPackage(ctx, app, "acme/demo", nil, []string{"exact"}, environment.Requirement{Git: &environment.GitSource{Repository: "acme/demo"}})
	for _, err := range []error{local, git, replay, saved} {
		var denied *domain.InstallPolicyError
		if !errors.As(err, &denied) {
			t.Fatalf("source reached unavailable acquirer/store instead of policy: %v", err)
		}
	}
}

func TestEnterprisePolicyPreservesRequestedPins(t *testing.T) {
	app := sourcePolicyApp(t, "schema_version: 2\nintegrity:\n  require_pinned_git: true\n")
	source := domain.SourceIdentity{SourceKind: "git", Repository: "acme/demo", RequestedSource: "acme/demo@main", ResolvedRevision: strings.Repeat("a", 40)}
	if err := app.checkEnterpriseSource(context.Background(), source, domain.OriginModeDirect); err == nil {
		t.Fatal("resolved commit was used as original requested pin")
	}
	source.RequestedSource = "acme/demo@" + source.ResolvedRevision
	if err := app.checkEnterpriseSource(context.Background(), source, domain.OriginModeDirect); err != nil {
		t.Fatalf("explicit full commit refused: %v", err)
	}
}

func TestEnterprisePolicyRelativeLocalArchive(t *testing.T) {
	app := sourcePolicyApp(t, "schema_version: 2\nsources:\n  allowed_kinds: [local]\nintegrity:\n  require_archive_sha256: true\n")
	archive := sourcespec.Archive{URL: "./package.zip", SHA256: strings.Repeat("a", 64)}
	if err := app.checkEnterpriseArchive(context.Background(), archive); err != nil {
		t.Fatalf("pinned local archive has no canonical policy authority: %v", err)
	}
	archive.SHA256 = ""
	if err := app.checkEnterpriseArchive(context.Background(), archive); err == nil {
		t.Fatal("local archive bypassed requested checksum rule")
	}
}
