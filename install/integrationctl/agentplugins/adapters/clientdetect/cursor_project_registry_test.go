package clientdetect

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// These are inert host probes through the actual registries and detector, not
// execution of an editor, Agent, package payload or native-registry command.
func TestCursorProjectRegistrySelectionPreservesUserAuthority(t *testing.T) {
	for _, row := range []struct {
		name            string
		editor, agent   bool
		config, invalid bool
	}{
		{name: "agent", agent: true},
		{name: "editor", editor: true},
		{name: "both", editor: true, agent: true},
		{name: "config", config: true},
		{name: "absent"},
		{name: "invalid-profile", editor: true, agent: true, invalid: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			home := cursorSelectionHome(t, row.config, row.invalid)
			binaries := map[string]string{}
			if row.editor {
				binaries["cursor"] = filepath.Join(home, "editor")
			}
			if row.agent {
				binaries["cursor-agent"] = filepath.Join(home, "agent")
			}
			user := cursorSelectionClient(t, cursorSelectionDetector(home, all.Default(), binaries), false)
			project := cursorSelectionClient(t, cursorSelectionDetector(home, all.Project(), binaries), false)
			assertCursorSelection(t, user, row.editor && !row.invalid, row.invalid, home, binaries["cursor"])
			assertCursorSelection(t, project, (row.editor || row.agent) && !row.invalid, row.invalid, home, binaries["cursor"])
			if !reflect.DeepEqual(user.Surfaces, project.Surfaces) {
				t.Fatal("project selection changed raw editor/Agent/profile evidence")
			}
		})
	}
}

func TestCursorProjectAgentSelectionNeverBorrowsExecutableOrVersion(t *testing.T) {
	home := cursorSelectionHome(t, false, false)
	detector := cursorSelectionDetector(home, all.Project(), map[string]string{"cursor-agent": filepath.Join(home, "agent")})
	detector.ProbeVersion = func(context.Context, string) (string, error) {
		t.Fatal("Agent-only project selection invoked an editor version probe")
		return "", nil
	}
	client := cursorSelectionClient(t, detector, true)
	assertCursorSelection(t, client, true, false, home, "")
	inspector, ok := clients.As[clients.RegistryInspector](all.Project(), domain.ClientCursor)
	if !ok || inspector.UsesNativeRegistryExecutable() {
		t.Fatal("project components gained a native executable registry probe")
	}
	adapter, ok := clients.As[clients.NativeRegistryLayout](all.Project(), domain.ClientCursor)
	if !ok {
		t.Fatal("project Cursor registry layout disappeared")
	}
	project := filepath.Join(home, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	root, executable := adapter.NativeRegistry(clients.PlanInput{Client: client, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: client.ConfigRoot}})
	if root != filepath.Join(project, ".cursor") || executable != "" {
		t.Fatalf("project native registry borrowed Agent identity: root=%q executable=%q", root, executable)
	}
}

func TestCursorProjectRegistryKeepsPreparedAndAttestationBoundary(t *testing.T) {
	home := cursorSelectionHome(t, false, false)
	registry := all.Project()
	client := cursorSelectionClient(t, cursorSelectionDetector(home, registry, map[string]string{"cursor-agent": filepath.Join(home, "agent")}), false)
	project := filepath.Join(home, "project")
	target := domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: client.ConfigRoot}
	plan := domain.DeliveryPlan{Scope: domain.ScopeProject, TargetContext: target, Status: domain.PlanReady,
		NativeRegistryRoot: filepath.Join(project, ".cursor"),
		Components:         []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "inert", Support: domain.SupportNative}}}
	refiner, ok := clients.As[clients.PlanRefiner](registry, domain.ClientCursor)
	if !ok {
		t.Fatal("project Cursor planning capability disappeared")
	}
	if err := refiner.RefinePlan(context.Background(), clients.PlanInput{Client: client, TargetContext: target}, &plan); err != nil || plan.Activation != domain.ActivationPrepared {
		t.Fatalf("Agent presence upgraded project configuration beyond Prepared: %+v, %v", plan, err)
	}
	preflight, ok := clients.As[clients.ActivationPreflighter](registry, domain.ClientCursor)
	if !ok {
		t.Fatal("project Cursor activation preflight disappeared")
	}
	err := preflight.PreflightActivation(clients.Env{}, domain.ActivationRequest{Client: client, Plan: plan, ActivationComplete: true})
	if err == nil || !strings.Contains(err.Error(), "cannot be established by activation attestation") {
		t.Fatalf("Agent presence bypassed native discovery attestation refusal: %v", err)
	}
}

func cursorSelectionHome(t *testing.T, config, invalid bool) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, ".cursor")
	if invalid {
		if err := os.WriteFile(root, []byte("inert non-directory profile"), 0o600); err != nil {
			t.Fatal(err)
		}
	} else if config {
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func cursorSelectionDetector(home string, registry *clients.Registry, binaries map[string]string) Detector {
	return Detector{
		HomeDir: home, WorkingDir: home, GOOS: "linux", Registry: registry,
		Lstat: os.Lstat, ReadDir: os.ReadDir, EvalSymlinks: filepath.EvalSymlinks,
		LookPath: func(name string) (string, error) {
			if path := binaries[name]; path != "" {
				return path, nil
			}
			return "", fmt.Errorf("inert absent binary %s", name)
		},
	}
}

func cursorSelectionClient(t *testing.T, detector Detector, probe bool) domain.DetectedClient {
	t.Helper()
	var detected []domain.DetectedClient
	var err error
	if probe {
		detected, err = detector.DetectTargetsWithVersionProbe(context.Background(), []domain.ClientID{domain.ClientCursor})
	} else {
		detected, err = detector.Detect(context.Background())
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range detected {
		if client.ClientID == domain.ClientCursor {
			return client
		}
	}
	t.Fatal("real registry lost Cursor")
	return domain.DetectedClient{}
}

func assertCursorSelection(t *testing.T, client domain.DetectedClient, detected, invalid bool, home, editor string) {
	t.Helper()
	if (client.Status == domain.DetectionDetected) != detected || (client.DetectionError != nil) != invalid {
		t.Fatalf("Cursor selection or profile-error precedence changed: %+v", client)
	}
	if invalid {
		if client.ConfigRoot != "" || client.ExecutablePath != "" || client.Version != "" || len(client.Surfaces) != 0 {
			t.Fatalf("invalid Cursor profile retained authority: %+v", client)
		}
		return
	}
	if client.ConfigRoot != filepath.Join(home, ".cursor") || client.ExecutablePath != editor || client.Version != "" {
		t.Fatalf("Cursor project selection replaced editor/profile identity: %+v", client)
	}
}
