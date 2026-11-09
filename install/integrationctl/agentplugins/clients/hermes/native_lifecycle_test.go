package hermes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type hermesSkillFixture struct {
	plan domain.DeliveryPlan
	objects []domain.NativeObjectOwnership
	layout shared.ProjectComponents
	skill string
	foreign string
}

func newHermesSkillFixture(t *testing.T, root string, scope domain.InstallScope) hermesSkillFixture {
	t.Helper()
	profile := root
	if scope == domain.ScopeProject {
		profile = filepath.Join(root, ".hermes")
	}
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, ".uap", "managed", "clients", "hermes", "fixture")
	skill := filepath.Join(active, "skills", "document", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: document\ndescription: Demo\n---\nWorks.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientHermes, Scope: scope,
		NativeRegistryRoot: profile, ActivePath: active,
		TargetContext: domain.TargetContext{Scope: scope},
		Components: []domain.ComponentDecision{{
			Kind: domain.ComponentSkill, Name: "document", Support: domain.SupportNative,
		}},
	}
	if scope == domain.ScopeProject {
		plan.TargetContext.ProjectRoot = root
	} else {
		plan.TargetContext.UserConfigRoot = root
	}
	envelope := domain.PackageEnvelope{
		Skills: map[string]domain.Skill{
			"document": {Name: "document", RelativePath: "skills/document/SKILL.md"},
		},
	}
	objects, err := buildSkillObjects(context.Background(), active, envelope, plan)
	if err != nil {
		t.Fatal(err)
	}
	layout, err := components(plan, active)
	if err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(profile, "skills", "not-ours", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(foreign), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("foreign\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return hermesSkillFixture{plan: plan, objects: objects, layout: layout,
		skill: filepath.Join(profile, "skills", "document", "SKILL.md"), foreign: foreign}
}

func applyHermesSkills(t *testing.T, f hermesSkillFixture, previous, desired []domain.NativeObjectOwnership) error {
	t.Helper()
	return (shared.ProjectComponentMutation{
		Layout: f.layout, ActivePath: f.plan.ActivePath, Previous: previous, Desired: desired,
		Kernel: nativeconfig.New(),
	}).Apply(context.Background())
}

func TestHermesScopedSkillAddRepeatRepairRemove(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			root := t.TempDir()
			f := newHermesSkillFixture(t, root, scope)
			if err := applyHermesSkills(t, f, nil, f.objects); err != nil {
				t.Fatalf("add: %v", err)
			}
			if err := f.layout.Verify(f.objects, false, nativeconfig.New()); err != nil {
				t.Fatalf("installed ownership: %v", err)
			}
			if err := applyHermesSkills(t, f, f.objects, f.objects); err != nil {
				t.Fatalf("repeat should be idempotent: %v", err)
			}
			if err := os.Remove(f.skill); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Dir(f.skill)); err != nil {
				t.Fatal(err)
			}
			if err := applyHermesSkills(t, f, f.objects, f.objects); err != nil {
				t.Fatalf("repair missing exact-owned skill: %v", err)
			}
			if err := applyHermesSkills(t, f, f.objects, nil); err != nil {
				t.Fatalf("remove: %v", err)
			}
			if _, err := os.Lstat(f.skill); !os.IsNotExist(err) {
				t.Fatalf("removed skill remains: %v", err)
			}
			if body, err := os.ReadFile(f.foreign); err != nil || string(body) != "foreign\n" {
				t.Fatalf("foreign skill changed: %q, %v", body, err)
			}
		})
	}
}

func TestHermesCannotAdoptForeignEditedSkill(t *testing.T) {
	f := newHermesSkillFixture(t, t.TempDir(), domain.ScopeProject)
	if err := applyHermesSkills(t, f, nil, f.objects); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.skill, []byte("foreign edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := applyHermesSkills(t, f, f.objects, f.objects); err == nil {
		t.Fatal("modified native skill must reject repair")
	}
	if body, err := os.ReadFile(f.skill); err != nil || !strings.Contains(string(body), "foreign edit") {
		t.Fatalf("foreign edit was overwritten: %q, %v", body, err)
	}
}

func TestHermesProjectAAndBIndependent(t *testing.T) {
	base := t.TempDir()
	a := newHermesSkillFixture(t, filepath.Join(base, "project-a"), domain.ScopeProject)
	b := newHermesSkillFixture(t, filepath.Join(base, "project-b"), domain.ScopeProject)
	if err := applyHermesSkills(t, a, nil, a.objects); err != nil {
		t.Fatal(err)
	}
	if err := applyHermesSkills(t, b, nil, b.objects); err != nil {
		t.Fatal(err)
	}
	if err := applyHermesSkills(t, a, a.objects, nil); err != nil {
		t.Fatal(err)
	}
	if err := b.layout.Verify(b.objects, false, nativeconfig.New()); err != nil {
		t.Fatalf("project B must survive removal in project A: %v", err)
	}
	if _, err := os.Lstat(a.skill); !os.IsNotExist(err) {
		t.Fatalf("project A skill was not removed: %v", err)
	}
}
