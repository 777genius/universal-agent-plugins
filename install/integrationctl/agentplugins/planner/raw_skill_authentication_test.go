package planner

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestRawSkillInstallationAuthenticationRequiresAdapterOptIn(t *testing.T) {
	for _, id := range []domain.ClientID{domain.ClientCodex, domain.ClientClaude, domain.ClientCursor, domain.ClientGemini} {
		for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
			t.Run(string(id)+"/"+string(scope), func(t *testing.T) {
				root := tempRoot(t)
				project := ""
				if scope == domain.ScopeProject {
					project = root
				}
				planner := testPlanner(Planner{ProjectRoot: project, ManagedRoot: filepath.Join(root, ".uap", "managed")})
				client := detectedClient(id, filepath.Join(root, "profile"))
				client.ExecutablePath = filepath.Join(root, "native-client")
				envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "raw-skill"},
					Skills: map[string]domain.Skill{"docs": {Name: "docs"}},
					Source: domain.SourceIdentity{StableSourceKey: "github-skill:example/skills:docs:skills/docs", SourceBindingHint: "uap-dependency-skill",
						Skill: &domain.SkillSourceFacts{Name: "docs", ResolvedPath: "skills/docs", Normalizer: "agent-skill-package/v1", RawTreeDigest: "sha256:" + strings.Repeat("a", 64)}},
				}
				plan, err := planner.Planner.Plan(context.Background(), domain.PlanRequest{Client: client, Envelope: envelope, Scope: scope, ProjectRoot: project, PhysicalArtifactID: "raw-skill-123456789abc"})
				if err != nil {
					t.Fatal(err)
				}
				want := domain.AuthenticationNotChecked
				if id == domain.ClientCodex || id == domain.ClientClaude {
					want = domain.AuthenticationNotRequired
				}
				if plan.Status == domain.PlanUnsupported || plan.Authentication != want {
					t.Fatalf("adapter opt-in was lost or leaked: authentication=%s, want=%s; plan=%+v", plan.Authentication, want, plan)
				}
			})
		}
	}
}
