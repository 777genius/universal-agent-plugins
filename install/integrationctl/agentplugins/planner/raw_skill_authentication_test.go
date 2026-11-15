package planner

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestRawSkillInstallationAuthenticationRequiresAdapterOptIn(t *testing.T) {
	for _, adapter := range []struct {
		id            domain.ClientID
		user, project domain.AuthenticationState
	}{
		{domain.ClientCodex, domain.AuthenticationNotRequired, domain.AuthenticationNotRequired},
		{domain.ClientClaude, domain.AuthenticationNotRequired, domain.AuthenticationNotRequired},
		{domain.ClientOpenCode, domain.AuthenticationNotRequired, domain.AuthenticationNotRequired},
		{domain.ClientCursor, domain.AuthenticationNotChecked, domain.AuthenticationNotRequired},
		{domain.ClientGemini, domain.AuthenticationNotChecked, domain.AuthenticationNotRequired},
	} {
		for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
			t.Run(string(adapter.id)+"/"+string(scope), func(t *testing.T) {
				root := tempRoot(t)
				project := ""
				if scope == domain.ScopeProject {
					project = root
				}
				planner := testPlanner(Planner{ProjectRoot: project, ManagedRoot: filepath.Join(root, ".uap", "managed")})
				client := detectedClient(adapter.id, filepath.Join(root, "profile"))
				client.ExecutablePath = filepath.Join(root, "native-client")
				if adapter.id == domain.ClientOpenCode {
					client.OpenCodeHost = contracttest.OpenCodeV1Host{}
				}
				envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "raw-skill"}, Skills: map[string]domain.Skill{"docs": {Name: "docs"}}, Source: domain.SourceIdentity{StableSourceKey: "github-skill:example/skills:docs:skills/docs", SourceBindingHint: "uap-dependency-skill", Skill: &domain.SkillSourceFacts{Name: "docs", ResolvedPath: "skills/docs", Normalizer: "agent-skill-package/v1", RawTreeDigest: "sha256:" + strings.Repeat("a", 64)}}}
				want := adapter.user
				if scope == domain.ScopeProject {
					want = adapter.project
				}
				for _, raw := range []bool{true, false} {
					selected := envelope
					expected := want
					if !raw {
						selected.Source = domain.SourceIdentity{}
						expected = domain.AuthenticationNotChecked
					}
					plan, err := planner.Planner.Plan(context.Background(), domain.PlanRequest{Client: client, Envelope: selected, Scope: scope, ProjectRoot: project, PhysicalArtifactID: "raw-skill-123456789abc"})
					if err != nil {
						t.Fatal(err)
					}
					if plan.Status == domain.PlanUnsupported || plan.Authentication != expected {
						t.Fatalf("adapter opt-in was lost or leaked: raw=%t authentication=%s want=%s plan=%+v", raw, plan.Authentication, expected, plan)
					}
				}
			})
		}
	}
}
