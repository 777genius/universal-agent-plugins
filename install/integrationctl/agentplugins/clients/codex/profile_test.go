package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestBindingProfileRequiresRootOrExactLegacyEvidence(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "artifact")
	if err := os.Mkdir(artifact, 0700); err != nil {
		t.Fatal(err)
	}
	market := shared.ManagedMarketplaceName("artifact")
	binding := domain.ClientBinding{PhysicalArtifact: "artifact", TargetLocator: artifact}
	for _, tc := range []struct {
		name, config, recorded string
		wantErr                bool
	}{
		{name: "legacy empty", wantErr: true},
		{name: "legacy name only", config: "[plugins.\"demo@" + market + "\"]\nenabled = true\n", wantErr: true},
		{name: "legacy other marketplace", config: fmt.Sprintf("[marketplaces.other]\nsource = %q\n", artifact), wantErr: true},
		{name: "legacy wrong artifact", config: fmt.Sprintf("[marketplaces.%s]\nsource = %q\n", market, filepath.Join(root, "other")), wantErr: true},
		{name: "legacy relative artifact", config: fmt.Sprintf("[marketplaces.%s]\nsource = \"artifact\"\n", market), wantErr: true},
		{name: "legacy remote source", config: fmt.Sprintf("[marketplaces.%s]\nsource = %q\nsource_type = \"git\"\n", market, artifact), wantErr: true},
		{name: "legacy exact", config: fmt.Sprintf("[marketplaces.%s]\nsource = %q\nsource_type = \"local\"\n", market, artifact)},
		{name: "recorded match", recorded: root},
		{name: "recorded mismatch", recorded: filepath.Join(root, "other-profile"), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			binding.NativeProfileRoot = tc.recorded
			if err := ValidateBindingProfile(root, binding); (err != nil) != tc.wantErr {
				t.Fatalf("profile validation: %v", err)
			}
			before, err := os.ReadFile(filepath.Join(root, "config.toml"))
			if err != nil {
				t.Fatal(err)
			}
			runner := &profileRunner{}
			plan := domain.DeliveryPlan{NativeRegistryRoot: root, NativeRegistryExecutable: filepath.Join(root, "codex"), PhysicalArtifactID: "artifact", DeclaredName: "demo"}
			if tc.wantErr {
				finding, err := New().InspectNativeRegistry(context.Background(), clients.Env{Runner: runner}, domain.DetectedClient{ConfigRoot: root}, plan, &binding)
				if err == nil || finding != clients.RegistryIndeterminate || len(runner.commands) != 0 {
					t.Fatalf("unsafe registry probe: %v, %v, %+v", finding, err, runner.commands)
				}
			}
			after, err := os.ReadFile(filepath.Join(root, "config.toml"))
			if err != nil || string(after) != string(before) {
				t.Fatal("profile validation mutated config")
			}
		})
	}
}

func TestExplicitProfileNormalizesAliasesOnlyAtBoundary(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	got, err := New().ResolveProfileRoot(alias)
	if err != nil || got != root {
		t.Fatalf("resolve: %s %v", got, err)
	}
	if err := validateProfile(alias, ""); err == nil {
		t.Fatal("native command accepted unresolved alias")
	}
	for _, bad := range []string{"", "relative", root + string(filepath.Separator), root + "\n"} {
		if _, err := New().ResolveProfileRoot(bad); err == nil {
			t.Fatalf("normalized ambiguous explicit path %q", bad)
		}
	}
}

func TestPlanAndManualLifecycleRejectMismatchedProfiles(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	plan := domain.DeliveryPlan{NativeRegistryRoot: other}
	client := domain.DetectedClient{ConfigRoot: root}
	if err := New().CheckPlanPrecondition(clients.PlanInput{Client: client}, &plan); err == nil {
		t.Fatal("plan accepted conflicting profiles")
	}
	if _, err := New().Activate(context.Background(), clients.Env{}, domain.ActivationRequest{Client: client, Plan: plan}); err == nil {
		t.Fatal("manual activation accepted conflicting profiles")
	}
	if _, err := New().Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{Client: domain.DetectedClient{ConfigRoot: "relative"}}); err == nil {
		t.Fatal("manual removal accepted relative profile")
	}
}
