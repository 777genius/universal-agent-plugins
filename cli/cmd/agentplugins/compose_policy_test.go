package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
)

func TestEnterprisePolicyCompositionKeepsMachineAuthority(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	t.Setenv("UAP_POLICY_FILE", "deployment/policy.yaml")
	app, err := configureEnterprisePolicy(agentpluginscli.App{}, home, project)
	if err != nil {
		t.Fatal(err)
	}
	machine, err := machinePolicyPath()
	if err != nil {
		t.Fatal(err)
	}
	want := []enterprisepolicy.Path{
		{Layer: "machine", Path: machine},
		{Layer: "user", Path: filepath.Join(home, ".config", "agentplugins", "policy.yaml")},
		{Layer: "project", Path: filepath.Join(project, "uap-policy.yaml")},
		{Layer: "deployment", Path: filepath.Join(project, "deployment", "policy.yaml"), Required: true},
	}
	if app.PolicyFiles == nil || len(app.PolicyFiles.Paths) != len(want) {
		t.Fatalf("unexpected policy composition: %+v", app.PolicyFiles)
	}
	for i, expected := range want {
		if app.PolicyFiles.Paths[i] != expected {
			t.Fatalf("policy layer %d = %+v, want %+v", i, app.PolicyFiles.Paths[i], expected)
		}
	}
	if app.InstallationPolicy != app.PolicyFiles || app.Lifecycle.InstallationPolicy != app.InstallationPolicy {
		t.Fatal("composition did not share the authoritative policy port")
	}
}

func TestEnterprisePolicyCompositionRequiresConfiguredDeployment(t *testing.T) {
	for _, path := range []string{"", "absent.yaml"} {
		t.Run(path, func(t *testing.T) {
			t.Setenv("UAP_POLICY_FILE", path)
			app, err := configureEnterprisePolicy(agentpluginscli.App{}, t.TempDir(), t.TempDir())
			if err != nil {
				t.Fatalf("policy loaded during composition instead of installation: %v", err)
			}
			_, err = app.PolicyFiles.Load(context.Background())
			var invalid *enterprisepolicy.ConfigError
			if !errors.As(err, &invalid) || invalid.Layer != "deployment" {
				t.Fatalf("configured deployment became optional: %v", err)
			}
		})
	}
}

func TestEnterprisePolicyUserScopeCapturesInvocationProject(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("UAP_POLICY_FILE", filepath.Join(root, "deployment.yaml"))
	app, err := configureEnterprisePolicy(agentpluginscli.App{}, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	captured, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if app.PolicyFiles.Paths[2].Path != filepath.Join(captured, "uap-policy.yaml") {
		t.Fatal("user scope project authority followed a later working directory")
	}
}
