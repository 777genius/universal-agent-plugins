package usecase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

// This is a file/receipt usecase E2E, not a simulated Devin model session.
// The selected version is a fixture; no fake MCP RPC or native CLI is invoked.
func TestDevinGlobalUserUsecaseOfflineAddUpdateRepairRemove(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	home := t.TempDir()
	legacyRoot := filepath.Join(home, ".codeium", "windsurf")
	nativeRoot := filepath.Join(home, ".config", "devin")
	legacyPath := filepath.Join(legacyRoot, "mcp_config.json")
	nativePath := filepath.Join(nativeRoot, "mcp_config.json")
	legacy := `{"mcpServers":{"cascade-foreign":{"serverUrl":"https://legacy.example/mcp"}},"keep":"legacy"}`
	global := `{"mcpServers":{"global-foreign":{"url":"https://global.example/mcp"}},"keep":"global"}`
	for path, body := range map[string]string{legacyPath: legacy, nativePath: global} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	client := domain.DetectedClient{
		ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected,
		ConfigRoot: legacyRoot, NativeConfigRoot: nativeRoot,
		ExecutablePath: filepath.Join(home, "bin", "devin"), Version: "3000.11.205",
	}
	add := windsurfUsecaseInput(t, client, "one")
	add.Confirmed = true
	added, err := service.Add(context.Background(), add)
	if err != nil || added.Activation.Verification != domain.VerificationInstalled ||
		added.Activation.Activation != domain.ActivationPrepared {
		t.Fatalf("offline Global add must install files but not attest a model session: %+v, %v", added, err)
	}
	assertDevinGlobalUsecaseMCP(t, nativePath, "one")
	assertDevinGlobalLegacyUntouched(t, legacyPath, legacy)

	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if binding.NativeProfileRoot != nativeRoot || binding.TargetLocator != added.Plan.ActivePath {
		t.Fatalf("new Global receipt lost exact native/package authorities: %+v", binding)
	}
	if strings.HasPrefix(binding.TargetLocator, nativeRoot+string(filepath.Separator)) {
		t.Fatal("managed package was incorrectly placed under the original native root")
	}

	update := windsurfUsecaseInput(t, client, "two")
	update.OperationID = "devin-global-offline-update"
	update.Confirmed = true
	updated, err := service.Update(context.Background(), update)
	if err != nil || !updated.Mutated || updated.Activation.Verification != domain.VerificationInstalled ||
		updated.Activation.Activation != domain.ActivationPrepared {
		t.Fatalf("offline Global update: %+v, %v", updated, err)
	}
	assertDevinGlobalUsecaseMCP(t, nativePath, "two")
	assertDevinGlobalLegacyUntouched(t, legacyPath, legacy)

	// Mimic a native registry being wiped without changing the owned package.
	if err := os.WriteFile(nativePath, []byte(global), 0o600); err != nil {
		t.Fatal(err)
	}
	update.OperationID = "devin-global-offline-repair"
	repaired, err := service.Repair(context.Background(), update)
	if err != nil || !repaired.Mutated || repaired.Activation.Verification != domain.VerificationInstalled ||
		repaired.Activation.Activation != domain.ActivationPrepared {
		t.Fatalf("offline Global native repair: %+v, %v", repaired, err)
	}
	assertDevinGlobalUsecaseMCP(t, nativePath, "two")

	removed, err := service.Remove(context.Background(), RemoveInput{
		Selector: added.InstallationID, Client: client, Scope: domain.ScopeUser,
		Confirmed: true, OperationID: "devin-global-offline-remove",
	})
	if err != nil || !removed.Mutated {
		t.Fatalf("offline Global receipt-only removal: %+v, %v", removed, err)
	}
	body, err := os.ReadFile(nativePath)
	if err != nil || !strings.Contains(string(body), "global-foreign") ||
		strings.Contains(string(body), "\"docs\"") {
		t.Fatalf("Global owned-only removal lost foreign or retained managed server: %v, %s", err, body)
	}
	assertDevinGlobalLegacyUntouched(t, legacyPath, legacy)
	state, err = store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 {
		t.Fatalf("Global receipt survived removal: %+v, %v", state.Installations, err)
	}
}

func assertDevinGlobalUsecaseMCP(t *testing.T, path, revision string) {
	t.Helper()
	servers := readUsecaseObject(t, path)["mcpServers"].(map[string]any)
	if _, ok := servers["global-foreign"]; !ok {
		t.Fatal("foreign Global MCP was lost")
	}
	managed, ok := servers["docs"].(map[string]any)
	if !ok || managed["url"] != "https://docs.test/"+revision {
		t.Fatalf("incorrect owned Global MCP source: %+v", servers)
	}
}

func assertDevinGlobalLegacyUntouched(t *testing.T, path, expected string) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != expected {
		t.Fatalf("selected Devin Global usecase touched legacy Cascade: %v, %s", err, actual)
	}
}
