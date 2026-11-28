package agentpluginscli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The picker must not resolve refs or open remote Git before a known source
// is admitted. A deliberately unavailable acquirer proves denial came first.
func TestEnterprisePolicyDeniesGitSkillsBeforeResolve(t *testing.T) {
	f := newCLIFixture(t, nil)
	policyPath := filepath.Join(f.root, "deny-git.yaml")
	dependencyWrite(t, policyPath, "schema_version: 2\nsources:\n  allowed_kinds: []\n")
	setEnterpriseCommandFiles(&f, enterprisepolicy.Path{Layer: "machine", Path: policyPath})
	f.app.SourceAcquirer = nil
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct{ name, input, ref string }{
		{"shorthand", "acme/demo@main", ""},
		{"pinned shorthand", "acme/demo@" + sha, ""},
		{"git clone", "git+https://git.example/team/repo.git#main//skills/review", ""},
		{"browser folder", "https://github.com/acme/demo/tree/main/skills/review", ""},
		{"explicit ref", "acme/demo", sha},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.app.openSkillsCatalog(context.Background(), SkillsPickerRequest{Input: tc.input, Ref: tc.ref})
			var denied *domain.InstallPolicyError
			if !errors.As(err, &denied) {
				t.Fatalf("Git catalog was resolved before enterprise denial: %v", err)
			}
		})
	}
}

func TestEnterprisePolicyDeniesLocalSkillsBeforeScan(t *testing.T) {
	f := newCLIFixture(t, nil)
	policyPath := filepath.Join(f.root, "deny-local.yaml")
	dependencyWrite(t, policyPath, "schema_version: 2\nsources:\n  allowed_kinds: []\n")
	setEnterpriseCommandFiles(&f, enterprisepolicy.Path{Layer: "machine", Path: policyPath})
	absent := filepath.Join(t.TempDir(), "unreadable-catalog")
	_, _, err := f.execute(false, "skills", "add", absent, "--all", "--target", "codex", "--no-ui", "--yes", "--format", "json")
	var denied *domain.InstallPolicyError
	if !errors.As(err, &denied) {
		t.Fatalf("local collection was scanned before enterprise denial: %v", err)
	}
}
