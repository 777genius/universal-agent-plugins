package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPreparationCannotOverrideUnsupportedPackage(t *testing.T) {
	plan := domain.DeliveryPlan{ClientID: domain.ClientKiro, Scope: domain.ScopeUser, Status: domain.PlanUnsupported}
	if err := ApplyInstallIntent(all.Default(), &plan, domain.InstallIntentPrepare); err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanUnsupported {
		t.Fatal("preparation bypassed compatibility")
	}
}

func TestPreparationRequiresQualifiedScopeAndNativeTarget(t *testing.T) {
	for _, tc := range []struct {
		client domain.ClientID
		scope  domain.InstallScope
		root   string
		kind   domain.ComponentKind
	}{
		{domain.ClientChatGPT, domain.ScopeUser, "native", domain.ComponentMCPServer},
		{domain.ClientKiro, domain.ScopeProject, "native", domain.ComponentMCPServer},
		{domain.ClientKiro, domain.ScopeUser, "", domain.ComponentMCPServer},
	} {
		plan := domain.DeliveryPlan{ClientID: tc.client, Scope: tc.scope, NativeRegistryRoot: tc.root, Components: []domain.ComponentDecision{{Kind: tc.kind, Support: domain.SupportPrepared}}}
		if err := ApplyInstallIntent(all.Default(), &plan, domain.InstallIntentPrepare); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}

func qualifiedKiroPreparation(t *testing.T) domain.DeliveryPlan {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project, user := filepath.Join(base, "project"), filepath.Join(base, "user", ".kiro")
	for _, path := range []string{project, user} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(user, "keep.txt"), []byte("original user bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	target, err := domain.NewTargetContext(domain.ScopeProject, project, user)
	if err != nil {
		t.Fatal(err)
	}
	managed := filepath.Join(project, ".uap", "managed")
	native := filepath.Join(project, ".kiro")
	return domain.DeliveryPlan{ClientID: domain.ClientKiro, Scope: domain.ScopeProject, TargetContext: target, Status: domain.PlanReady, NativeRegistryRoot: native, TargetAnchor: managed, TargetRoot: filepath.Join(managed, "clients", "kiro"), PhysicalArtifactID: "demo-0123456789ab", ActivePath: filepath.Join(managed, "clients", "kiro", "demo-0123456789ab"), Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportPrepared}}}
}
func TestPreparationQualifiedKiroProjectIsReadonlyPrepared(t *testing.T) {
	plan := qualifiedKiroPreparation(t)
	if err := ApplyInstallIntent(all.Default(), &plan, domain.InstallIntentPrepare); err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanReady || plan.Activation != domain.ActivationPrepared || plan.Verification != domain.VerificationPackageValid || plan.InstallIntent != domain.InstallIntentPrepare {
		t.Fatalf("qualified preparation %+v", plan)
	}
	if len(plan.UserActions) != 1 || plan.UserActions[0] != kiro.KiroProjectAction {
		t.Fatal("project preparation lost scoped next action")
	}
	entries, err := os.ReadDir(plan.TargetContext.ProjectRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preparation mutated project: %v %v", entries, err)
	}
	body, err := os.ReadFile(filepath.Join(plan.TargetContext.UserConfigRoot, "keep.txt"))
	if err != nil || string(body) != "original user bytes" {
		t.Fatal("preparation changed original user bytes")
	}
}
func TestPreparationMalformedProjectRefusesBeforeReadiness(t *testing.T) {
	for _, name := range []string{"missing root", "relative native", "wrong native", "wrong artifact", "wrong active", "user labels", "wrong target scope"} {
		t.Run(name, func(t *testing.T) {
			plan := qualifiedKiroPreparation(t)
			switch name {
			case "missing root":
				plan.TargetContext.ProjectRoot = ""
			case "relative native":
				plan.NativeRegistryRoot = "native"
			case "wrong native":
				plan.NativeRegistryRoot = plan.TargetContext.UserConfigRoot
			case "wrong artifact":
				plan.PhysicalArtifactID = "foreign"
			case "wrong active":
				plan.ActivePath = filepath.Join(plan.TargetRoot, "foreign")
			case "user labels":
				plan.Scope = domain.ScopeUser
				plan.TargetContext.Scope = domain.ScopeUser
			case "wrong target scope":
				plan.TargetContext.Scope = domain.ScopeUser
			}
			for _, direct := range []bool{false, true} {
				candidate := plan
				var err error
				if direct {
					err = kiro.New().RefinePreparation(&candidate)
				} else {
					err = ApplyInstallIntent(all.Default(), &candidate, domain.InstallIntentPrepare)
				}
				if err == nil || candidate.Activation == domain.ActivationPrepared || candidate.Verification == domain.VerificationPackageValid {
					t.Fatalf("malformed project promoted direct%t: %+v %v", direct, candidate, err)
				}
			}
		})
	}
}

func TestPreparationProjectCannotPromoteUnsupportedCompatibility(t *testing.T) {
	plan := qualifiedKiroPreparation(t)
	plan.Status = domain.PlanUnsupported
	for _, direct := range []bool{false, true} {
		candidate := plan
		var err error
		if direct {
			err = kiro.New().RefinePreparation(&candidate)
		} else {
			err = ApplyInstallIntent(all.Default(), &candidate, domain.InstallIntentPrepare)
		}
		if err != nil || candidate.Status != domain.PlanUnsupported || candidate.Activation == domain.ActivationPrepared || candidate.Verification == domain.VerificationPackageValid {
			t.Fatalf("unsupported project compatibility promoted direct%t %+v %v", direct, candidate, err)
		}
	}
}
