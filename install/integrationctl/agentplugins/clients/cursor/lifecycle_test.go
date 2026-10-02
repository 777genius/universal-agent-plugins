package cursor

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestPublicSelectedProfileLifecycleIsolation(t *testing.T) {
	base := testBase(t)
	home := filepath.Join(base, "TEST-ambient-home")
	mustMkdir(t, home)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Chdir(home)
	helper := filepath.Join(base, "TEST-helper")
	mustWrite(t, helper, "TEST fixture bytes; never executed", 0700)
	pkg := filepath.Join(base, "package")
	writeSkillPackage(t, pkg, "1.0.0")
	a, b := filepath.Join(base, "TEST-A", "missing", "profile"), filepath.Join(base, "TEST-B")
	engines := []*installer.Engine{testEngine(t, filepath.Join(base, "state-A"), helper), testEngine(t, filepath.Join(base, "state-B"), helper)}
	requests := []installer.Request{
		{Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "cursor", ClientConfigRoot: a, ClientExecutable: helper, InstallationID: "00000000-0000-4000-8000-000000000001", RequiredComponents: []string{"skills"}},
		{Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "cursor", ClientConfigRoot: b, ClientExecutable: helper, InstallationID: "00000000-0000-4000-8000-000000000002", RequiredComponents: []string{"skills"}},
	}
	for i, root := range []string{a, b} {
		mustWrite(t, filepath.Join(root, "mcp.json"), `{"mcpServers":{"foreign":{"disabled":true}}}`, 0600)
		mustWrite(t, filepath.Join(root, "hooks.json"), `{"version":1,"hooks":{}}`, 0600)
		result := applyPublic(t, engines[i], requests[i], "install")
		if result.Client.Activation != string(domain.ActivationManual) || result.Client.Verification == string(domain.VerificationInstalled) {
			t.Fatalf("preparation falsely verified native: %+v", result)
		}
	}
	sibling := snapshotTree(t, b)
	siblingState := snapshotTree(t, filepath.Join(base, "state-B"))
	selected := publicBinding(t, engines[0])
	for _, operation := range []installer.Operation{installer.OpUpdate, installer.OpRepair, installer.OpRemove} {
		wrong := requests[0]
		wrong.Operation, wrong.ClientConfigRoot = operation, b
		handle, err := engines[0].Prepare(t.Context(), wrong)
		if handle != nil {
			if closeErr := handle.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		}
		if err == nil {
			t.Fatalf("%s accepted another profile", operation)
		}
	}
	alias := filepath.Join(base, "TEST-alias")
	if err := os.Symlink(a, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	requests[0].ClientConfigRoot = alias
	repeat := applyPublic(t, engines[0], requests[0], "repeat")
	if repeat.Mutated || repeat.Binding.BindingID != selected.BindingID {
		t.Fatalf("alias repeat: %+v", repeat)
	}
	otherHome := filepath.Join(base, "TEST-other-home")
	mustMkdir(t, otherHome)
	t.Setenv("HOME", otherHome)
	t.Setenv("USERPROFILE", otherHome)
	t.Chdir(otherHome)
	requests[0].ClientConfigRoot = a
	writeSkillPackage(t, pkg, "1.1.0")
	requests[0].Operation = installer.OpUpdate
	applyPublic(t, engines[0], requests[0], "update")
	if err := os.RemoveAll(selected.TargetPath); err != nil {
		t.Fatal(err)
	}
	requests[0].Operation = installer.OpRepair
	applyPublic(t, engines[0], requests[0], "repair")
	requests[0].Operation = installer.OpRemove
	applyPublic(t, engines[0], requests[0], "remove")
	if _, err := os.Lstat(selected.TargetPath); !os.IsNotExist(err) {
		t.Fatalf("selected package not removed: %v", err)
	}
	if !reflect.DeepEqual(sibling, snapshotTree(t, b)) || !reflect.DeepEqual(siblingState, snapshotTree(t, filepath.Join(base, "state-B"))) {
		t.Fatal("sibling namespace changed")
	}
	for _, root := range []string{a, b} {
		assertUserConfig(t, root)
	}
	if _, err := os.Lstat(filepath.Join(otherHome, ".cursor")); !os.IsNotExist(err) {
		t.Fatalf("ambient profile touched: %v", err)
	}
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		var body []byte
		if !entry.IsDir() {
			body, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		snapshot[rel] = fmt.Sprintf("%s:%x", info.Mode(), sha256.Sum256(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestPlanAndActivationRemainPreparation(t *testing.T) {
	base := testBase(t)
	adapter := New()
	plan := domain.DeliveryPlan{ClientID: domain.ClientCursor, Status: domain.PlanReady, PackageMode: domain.PackageNative, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Support: domain.SupportNative}}}
	if err := adapter.RefinePlan(context.Background(), clients.PlanInput{}, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanManualActivationRequired || plan.Activation != domain.ActivationPrepared || plan.PackageMode != domain.PackageNative || plan.Components[0].Support != domain.SupportPrepared {
		t.Fatalf("unqualified native preparation: %+v", plan)
	}
	client := domain.DetectedClient{ClientID: domain.ClientCursor, ConfigRoot: base}
	plan.PhysicalArtifactID = "test-plugin-123456789abc"
	plan.ActivePath = filepath.Join(base, "plugins", "local", plan.PhysicalArtifactID)
	delivery := domain.StagedDelivery{ClientID: domain.ClientCursor, ActivePath: plan.ActivePath}
	for _, verifyOnly := range []bool{false, true} {
		outcome, err := adapter.Activate(context.Background(), clients.Env{}, domain.ActivationRequest{Client: client, Plan: plan, Delivery: delivery, VerifyOnly: verifyOnly, ActivationComplete: true})
		if err != nil || outcome.Activation != domain.ActivationManual || outcome.Verification != domain.VerificationPackageValid || outcome.ActivationAttested {
			t.Fatalf("unproven native activation: %+v %v", outcome, err)
		}
	}
}

func writeSkillPackage(t *testing.T, root, version string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-plugin","version":"`+version+`"}`, 0600)
	mustWrite(t, filepath.Join(root, "skills", "test-skill", "SKILL.md"), "---\nname: test-skill\ndescription: TEST isolated profile skill\n---\nTEST only.\n", 0600)
}
func applyPublic(t *testing.T, engine *installer.Engine, request installer.Request, operation string) installer.Result {
	t.Helper()
	request.OperationID = operation
	prepared, err := engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatalf("%s prepare: %v", operation, err)
	}
	defer func() {
		if err := prepared.Close(); err != nil {
			t.Error(err)
		}
	}()
	result, err := engine.Apply(t.Context(), prepared, installer.Decision{Confirmed: true})
	if err != nil {
		t.Fatalf("%s apply: %+v %v", operation, result, err)
	}
	return result
}
func publicBinding(t *testing.T, engine *installer.Engine) installer.InspectedBinding {
	t.Helper()
	view, err := engine.Inspect(t.Context())
	if err != nil || len(view.Installations) != 1 || len(view.Installations[0].Bindings) != 1 {
		t.Fatalf("inspection: %+v %v", view, err)
	}
	return view.Installations[0].Bindings[0]
}
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
func assertUserConfig(t *testing.T, root string) {
	t.Helper()
	if string(readFile(t, filepath.Join(root, "mcp.json"))) != `{"mcpServers":{"foreign":{"disabled":true}}}` || string(readFile(t, filepath.Join(root, "hooks.json"))) != `{"version":1,"hooks":{}}` {
		t.Fatal("unmanaged native config changed")
	}
}
