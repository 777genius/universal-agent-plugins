package providers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestClineLifecycleInstallsUpdatesAndRemovesExactOwnedObjects(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".cline")
	settings := filepath.Join(root, "isolated", "cline_mcp_settings.json")
	t.Setenv("CLINE_MCP_SETTINGS_PATH", settings)
	writeTestFile(t, settings, `{"theme":"night","mcpServers":{"foreign":{"command":"foreign"}}}`)

	active := filepath.Join(root, "managed", "demo")
	writeTestFile(t, filepath.Join(active, "skills", "guide", "SKILL.md"), "---\nname: guide\ndescription: Guide\n---\n")
	server := nativeconfig.Server{Type: "stdio", Command: "node", Args: []string{filepath.Join(active, "server.js")}}
	writeClineProjectionFixture(t, active, map[string]nativeconfig.Server{"docs": server})
	desired := clineFixtureObjects(t, configRoot, active, "guide", "docs", server)

	request := domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientCline, Status: domain.DetectionDetected, ConfigRoot: configRoot},
		Plan: domain.DeliveryPlan{ClientID: domain.ClientCline, ActivePath: active, Components: []domain.ComponentDecision{
			{Kind: domain.ComponentSkill, Name: "guide", Support: domain.SupportPrepared},
			{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportPrepared},
		}},
		Delivery:     domain.StagedDelivery{ClientID: domain.ClientCline, OwnedBase: filepath.Dir(active), ActivePath: active, NativeObjects: desired},
		DeclaredName: "demo",
	}
	outcome, err := (Activator{}).Activate(context.Background(), request)
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("activation = %+v, %v", outcome, err)
	}
	doc := readObject(t, settings)
	if doc["theme"] != "night" || doc["mcpServers"].(map[string]any)["foreign"] == nil {
		t.Fatalf("foreign config changed: %+v", doc)
	}
	transport := doc["mcpServers"].(map[string]any)["docs"].(map[string]any)["transport"].(map[string]any)
	if transport["type"] != "stdio" || transport["command"] != "node" {
		t.Fatalf("Cline modern transport = %+v", transport)
	}
	if _, err := os.Stat(filepath.Join(configRoot, "skills", "guide", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	request.VerifyOnly = true
	if _, err := (Activator{}).Activate(context.Background(), request); err != nil {
		t.Fatalf("verify-only: %v", err)
	}
	request.VerifyOnly = false
	request.Replacing = true
	request.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), desired...)
	updatedServer := nativeconfig.Server{Type: "stdio", Command: "deno", Args: []string{filepath.Join(active, "server.ts")}}
	writeClineProjectionFixture(t, active, map[string]nativeconfig.Server{"docs": updatedServer})
	updated := clineFixtureObjects(t, configRoot, active, "guide", "docs", updatedServer)
	request.Delivery.NativeObjects = updated
	if _, err := (Activator{}).Activate(context.Background(), request); err != nil {
		t.Fatalf("update: %v", err)
	}
	doc = readObject(t, settings)
	transport = doc["mcpServers"].(map[string]any)["docs"].(map[string]any)["transport"].(map[string]any)
	if transport["command"] != "deno" {
		t.Fatalf("update was not applied: %+v", transport)
	}
	desired = updated

	// A confirmed repair may recreate an absent entry only when the desired
	// receipt is exactly the one already owned by state.
	writeTestFile(t, settings, `{"theme":"night","mcpServers":{"foreign":{"command":"foreign"}}}`)
	request.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), desired...)
	request.Delivery.NativeObjects = append([]domain.NativeObjectOwnership(nil), desired...)
	if _, err := (Activator{}).Activate(context.Background(), request); err != nil {
		t.Fatalf("repair absent exact-owned Cline entry: %v", err)
	}
	doc = readObject(t, settings)
	if doc["mcpServers"].(map[string]any)["docs"] == nil {
		t.Fatalf("Cline repair did not recreate the exact entry: %+v", doc)
	}

	foreign := `{"mcpServers":{"docs":{"transport":{"type":"stdio","command":"foreign"}}}}`
	writeTestFile(t, settings, foreign)
	if _, err := (Activator{}).Activate(context.Background(), request); !errors.Is(err, nativeconfig.ErrNotOwned) && !strings.Contains(err.Error(), "changed outside") {
		t.Fatalf("tampered Cline entry was not rejected: %v", err)
	}
	if body, err := os.ReadFile(settings); err != nil || string(body) != foreign {
		t.Fatalf("failed Cline repair changed tampered entry: %s, %v", body, err)
	}

	// Missing at removal is an idempotent success; the owned skill is still
	// removed while unrelated config survives.
	writeTestFile(t, settings, `{"theme":"night","mcpServers":{"foreign":{"command":"foreign"}}}`)

	remove := domain.DeactivationRequest{Client: request.Client, DeclaredName: "demo", CurrentActivation: domain.ActivationActive, Confirmed: true, NativeObjects: desired}
	removed, err := (Activator{}).Deactivate(context.Background(), remove)
	if err != nil || !removed.ExternalRemovalComplete || !removed.ArtifactRemovalAllowed {
		t.Fatalf("remove = %+v, %v", removed, err)
	}
	doc = readObject(t, settings)
	servers := doc["mcpServers"].(map[string]any)
	if servers["foreign"] == nil || servers["docs"] != nil || doc["theme"] != "night" {
		t.Fatalf("remove touched foreign config: %+v", doc)
	}
	if _, err := os.Lstat(filepath.Join(configRoot, "skills", "guide")); !os.IsNotExist(err) {
		t.Fatalf("managed skill survived removal: %v", err)
	}
}

func TestStagerBuildsClineNestedTransportAndOwnership(t *testing.T) {
	t.Setenv("CLINE_MCP_SETTINGS_PATH", "")
	t.Setenv("CLINE_DATA_DIR", "")
	envelope := stagingEnvelope(t)
	plan := stagingPlan(t, domain.ClientCline, domain.PackageNative)
	plan.NativeRegistryRoot = filepath.Join(t.TempDir(), ".cline")
	delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "cline-operation", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := readClineProjection(delivery.StagingPath)
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(t.TempDir(), "cline_mcp_settings.json")
	if _, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: nativeconfig.Paths{JSON: settings}, Codec: nativeconfig.CodecCline, Action: nativeconfig.ActionAdd, Name: "notion", Server: projection.Servers["notion"]}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	projected := document["mcpServers"].(map[string]any)["notion"].(map[string]any)
	transport := projected["transport"].(map[string]any)
	if transport["type"] != "streamableHttp" || transport["url"] != "https://mcp.notion.com/mcp" {
		t.Fatalf("projection = %+v", projected)
	}
	var foundSkill, foundMCP bool
	for _, object := range delivery.NativeObjects {
		foundSkill = foundSkill || object.Kind == "cline_global_skill_directory"
		foundMCP = foundMCP || object.Kind == "cline_global_mcp_server"
	}
	if !foundSkill || !foundMCP {
		t.Fatalf("Cline ownership missing: %+v", delivery.NativeObjects)
	}
}

func TestClineRejectsRelativeSettingsOverrideBeforeMutation(t *testing.T) {
	t.Setenv("CLINE_MCP_SETTINGS_PATH", "relative/settings.json")
	envelope := stagingEnvelope(t)
	plan := stagingPlan(t, domain.ClientCline, domain.PackageNative)
	plan.NativeRegistryRoot = filepath.Join(t.TempDir(), ".cline")
	if _, err := (Stager{}).Stage(context.Background(), envelope, plan, "cline-relative", domain.CompatibilityHints{}); err == nil || !strings.Contains(err.Error(), "must be absolute") {
		t.Fatalf("relative Cline override was accepted: %v", err)
	}
}

func writeClineProjectionFixture(t *testing.T, active string, servers map[string]nativeconfig.Server) {
	t.Helper()
	body, err := json.Marshal(struct {
		Servers map[string]nativeconfig.Server `json:"servers"`
	}{Servers: servers})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(active, ".agentplugins-cline-native.json"), string(body))
}

func clineFixtureObjects(t *testing.T, configRoot, active, skillName, serverName string, server nativeconfig.Server) []domain.NativeObjectOwnership {
	t.Helper()
	var objects []domain.NativeObjectOwnership
	if skillName != "" {
		digest, err := shared.DigestSkillDirectory(filepath.Join(active, "skills", skillName))
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, domain.NativeObjectOwnership{ObjectID: "cline-skill:" + skillName, Kind: "cline_global_skill_directory", LogicalName: skillName,
			Path: filepath.Join(configRoot, "skills", skillName), SourceRelative: filepath.ToSlash(filepath.Join("skills", skillName)), ManagedDigest: digest, ProtectionClass: "managed"})
	}
	if serverName != "" {
		path := filepath.Join(configRoot, "cline_mcp_settings.json")
		if override := strings.TrimSpace(os.Getenv("CLINE_MCP_SETTINGS_PATH")); override != "" {
			path = filepath.Clean(override)
		}
		receipt, err := nativeconfig.DesiredReceipt(path, nativeconfig.CodecCline, serverName, server, nativeconfig.Placeholders{})
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, domain.NativeObjectOwnership{ObjectID: "cline-mcp:" + serverName, Kind: "cline_global_mcp_server", LogicalName: serverName,
			Path: path, SourceRelative: ".agentplugins-cline-native.json", ManagedDigest: receipt.Digest, ProtectionClass: "managed"})
	}
	return objects
}

func readClineProjection(root string) (struct {
	Servers map[string]nativeconfig.Server `json:"servers"`
}, error) {
	var projection struct {
		Servers map[string]nativeconfig.Server `json:"servers"`
	}
	body, err := os.ReadFile(filepath.Join(root, ".agentplugins-cline-native.json"))
	if err != nil {
		return projection, err
	}
	err = json.Unmarshal(body, &projection)
	return projection, err
}
