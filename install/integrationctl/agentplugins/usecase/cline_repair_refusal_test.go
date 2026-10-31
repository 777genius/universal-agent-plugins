package usecase

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

func TestClineRepairRefusalPreservesEditedSkillAndClearsUnchangedAttempt(t *testing.T) {
	root := t.TempDir()
	settings := filepath.Join(root, "settings.json")
	t.Setenv("CLINE_MCP_SETTINGS_PATH", settings)
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	client := domain.DetectedClient{ClientID: domain.ClientCline, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(root, "cline")}
	input := clinePackageInput(t, client, "1.0.0", "sha256:cline-refusal", "sha256:cline-refusal-manifest", "sh")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(client.ConfigRoot, "skills", "docs", "SKILL.md")
	original, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	edited := append(append([]byte{}, original...), []byte("\nUser edit that must survive.\n")...)
	if err := os.WriteFile(skill, edited, 0600); err != nil {
		t.Fatal(err)
	}
	mcpBefore, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	input.InstallationID, input.OperationID = installed.InstallationID, "refused-cline-repair"
	result, err := service.Repair(context.Background(), input)
	if err == nil {
		t.Fatalf("edited skill repair was accepted: %+v", result)
	}
	if result.Activation.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatalf("refusal effect = %q", result.Activation.NativeEffect)
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range after.Installations[0].Clients {
		prior := before.Installations[0].Clients[key]
		if binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil {
			t.Fatalf("read-only refusal retained attempt: %+v", binding)
		}
		if !reflect.DeepEqual(binding.NativeObjects, prior.NativeObjects) || binding.DataReceiptID != prior.DataReceiptID || !reflect.DeepEqual(binding.Receipts, prior.Receipts) {
			t.Fatal("refusal changed ownership or receipts")
		}
	}
	if !reflect.DeepEqual(before.Installations[0].DataReceipts, after.Installations[0].DataReceipts) {
		t.Fatal("refusal changed data receipts")
	}
	skillAfter, _ := os.ReadFile(skill)
	mcpAfter, _ := os.ReadFile(settings)
	if !bytes.Equal(edited, skillAfter) || !bytes.Equal(mcpBefore, mcpAfter) {
		t.Fatal("refusal changed edited skill or MCP")
	}
	preserved := filepath.Join(root, "preserved-docs")
	if err := os.Rename(filepath.Dir(skill), preserved); err != nil {
		t.Fatal(err)
	}
	input.OperationID = "fresh-cline-repair"
	if _, err := service.Repair(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	repaired, _ := os.ReadFile(skill)
	saved, _ := os.ReadFile(filepath.Join(preserved, "SKILL.md"))
	if !bytes.Equal(repaired, original) || !bytes.Equal(saved, edited) {
		t.Fatal("fresh repair failed to restore original while preserving edit")
	}
}
