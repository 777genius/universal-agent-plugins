package pi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPiProjectOnlyAdmittedThroughSelectedAdapter(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, projectA, projectB := filepath.Join(root, "home", ".pi", "agent"), filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, path := range []string{user, projectA, projectB} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	oldRegistry, err := clients.NewRegistry(New())
	if err != nil {
		t.Fatal(err)
	}
	projectRegistry, err := clients.NewRegistry(NewProject())
	if err != nil {
		t.Fatal(err)
	}
	if clients.SupportsScope(oldRegistry, domain.ClientPi, domain.ScopeProject) {
		t.Fatal("default Pi user registry admitted a project")
	}
	if !clients.SupportsScope(projectRegistry, domain.ClientPi, domain.ScopeProject) {
		t.Fatal("selected Pi project adapter not admitted")
	}
	client := domain.DetectedClient{ClientID: domain.ClientPi, ConfigRoot: user}
	for _, project := range []string{projectA, projectB} {
		target, err := domain.NewTargetContext(domain.ScopeProject, project, user)
		if err != nil {
			t.Fatal(err)
		}
		root, native, err := NewProject().ProjectTargetRoot(client, domain.PackageNative, "", target)
		if err != nil || root != filepath.Join(project, ".pi") || native != filepath.Join(root, "extensions") {
			t.Fatal("Pi project root drifted", root, native, err)
		}
		plan := domain.DeliveryPlan{
			ClientID: domain.ClientPi, Scope: domain.ScopeProject, TargetContext: target,
			NativeRegistryRoot: root, TargetAnchor: root, TargetRoot: native,
			PhysicalArtifactID: "owned-pi-extension", ActivePath: filepath.Join(native, "owned-pi-extension"),
		}
		if err := validatePlan(plan); err != nil {
			t.Fatal("valid isolated Pi project extension", err)
		}
		plan.TargetAnchor = user
		if err := validatePlan(plan); err == nil {
			t.Fatal("Pi adopted user root for project")
		}
		plan.TargetAnchor = root
		foreignProject := projectA
		if project == projectA {
			foreignProject = projectB
		}
		plan.NativeRegistryRoot = filepath.Join(foreignProject, ".pi")
		if err := validatePlan(plan); err == nil {
			t.Fatal("Pi accepted another project's native registry")
		}
		plan.NativeRegistryRoot = root
		plan.ActivePath = filepath.Join(user, "extensions", "owned-pi-extension")
		if err := validatePlan(plan); err == nil {
			t.Fatal("Pi escaped to user profile")
		}
	}
}

func TestPiProjectRejectsAliasedDiscoveryAndUserOverlap(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(root, "user", ".pi", "agent")
	project := filepath.Join(root, "project")
	for _, path := range []string{user, project} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	client := domain.DetectedClient{ClientID: domain.ClientPi, ConfigRoot: user}
	target := domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: user}
	alias := filepath.Join(project, ".pi")
	if err := os.Symlink(user, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := projectPiRoot(client, target, ""); err == nil {
		t.Fatal("Pi followed project discovery symlink")
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if _, err := projectPiRoot(client, target, user); err == nil {
		t.Fatal("Pi adopted user profile as planned project native root")
	}
	target.ProjectRoot = filepath.Join(root, "user")
	if _, err := projectPiRoot(client, target, ""); err == nil {
		t.Fatal("Pi project contains original user profile")
	}
}
