package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/gemini"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func projectGeminiRequest(t *testing.T) (domain.ActivationRequest, string, string) {
	t.Helper()
	base := projectGeminiTemp(t)
	user, projectA, projectB := filepath.Join(base, "user", ".gemini"), filepath.Join(base, "A"), filepath.Join(base, "B")
	for _, root := range []string{user, filepath.Join(projectA, ".gemini"), filepath.Join(projectB, ".gemini")} {
		projectGeminiWrite(t, filepath.Join(root, "settings.json"), "{\n // preserve foreign settings\n \"mcpServers\": {\"foreign\": {\"url\":\"https://foreign.test/mcp\"}}, \"theme\":\"foreign\"\n}\n")
		projectGeminiWrite(t, filepath.Join(root, "skills", "foreign", "SKILL.md"), "foreign skill\n")
	}
	client := domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: user, Status: domain.DetectionDetected}
	target, err := domain.NewTargetContext(domain.ScopeProject, projectA, user)
	if err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(projectA, ".uap", "managed", "clients", "gemini", "archive-demo")
	projectGeminiWrite(t, filepath.Join(active, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Project fixture\n---\nInert fixture.\n")
	projectGeminiWrite(t, filepath.Join(active, "mcp.json"), `{"mcpServers":{"docs":{"type":"sse","url":"https://managed.test/mcp"}}}`)
	plan := domain.DeliveryPlan{ClientID: domain.ClientGemini, Scope: domain.ScopeProject, TargetContext: target, InstallIntent: domain.InstallIntentAutomatic,
		NativeRegistryRoot: filepath.Join(projectA, ".gemini"), ActivePath: active, Components: []domain.ComponentDecision{
			{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative}, {Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative},
		}}
	envelope := domain.PackageEnvelope{Skills: map[string]domain.Skill{"docs": {Name: "docs", RelativePath: "skills/docs/SKILL.md"}},
		MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"docs": {Name: "docs", Type: "sse", Decoded: map[string]any{"url": "https://managed.test/mcp"}}}}}
	objects, err := gemini.BuildGeminiNativeObjects(active, envelope, plan, filepath.Join(projectA, ".uap", "data", "demo"))
	if err != nil {
		t.Fatal(err)
	}
	return domain.ActivationRequest{Client: client, Plan: plan, DeclaredName: "archive-demo", Delivery: domain.StagedDelivery{
		ClientID: domain.ClientGemini, ActivePath: active, OwnedBase: filepath.Dir(active), NativeObjects: objects}}, projectB, user
}

func TestProjectGeminiNativeLifecycleKeepsUserAndOtherProject(t *testing.T) {
	request, projectB, user := projectGeminiRequest(t)
	userBefore, bBefore := projectGeminiInventory(t, user), projectGeminiInventory(t, projectB)
	registry, err := clients.NewRegistry(gemini.New())
	if err != nil {
		t.Fatal(err)
	}
	kernel := nativeconfig.New()
	activator := providers.Activator{Registry: registry, NativeConfig: &kernel}
	outcome, err := activator.Activate(context.Background(), request)
	if err != nil || outcome.Verification != domain.VerificationInstalled || len(outcome.NativeObjects) != 2 {
		t.Fatalf("project config activation = %+v, %v", outcome, err)
	}
	root := request.Plan.NativeRegistryRoot
	projectGeminiAssert(t, filepath.Join(root, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Project fixture\n---\nInert fixture.\n")
	settings, err := os.ReadFile(filepath.Join(root, "settings.json"))
	if err != nil || !strings.Contains(string(settings), "preserve foreign settings") || !strings.Contains(string(settings), "managed.test") || !strings.Contains(string(settings), "foreign.test") {
		t.Fatalf("project MCP merge lost entries/comments: %s, %v", settings, err)
	}
	request.VerifyOnly, request.PreviousNativeObjects = true, outcome.NativeObjects
	if _, err := activator.Activate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	remove := domain.DeactivationRequest{Client: request.Client, TargetContext: request.Plan.TargetContext, NativeRegistryRoot: root, Confirmed: true, NativeObjects: outcome.NativeObjects}
	for range 2 {
		removed, err := activator.Deactivate(context.Background(), remove)
		if err != nil || !removed.ExternalRemovalComplete {
			t.Fatalf("exact project removal = %+v, %v", removed, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("managed project skill survived removal: %v", err)
	}
	if !reflect.DeepEqual(userBefore, projectGeminiInventory(t, user)) || !reflect.DeepEqual(bBefore, projectGeminiInventory(t, projectB)) {
		t.Fatal("project lifecycle mutated user profile or project B")
	}
}

func TestProjectGeminiTargetContextRejectsUserRegistryAndAliases(t *testing.T) {
	request, _, user := projectGeminiRequest(t)
	adapter := gemini.New()
	managed := filepath.Join(request.Plan.TargetContext.ProjectRoot, ".uap", "managed")
	if _, root, err := adapter.ProjectTargetRoot(request.Client, domain.PackageNative, managed, request.Plan.TargetContext); err != nil || root != filepath.Join(managed, "clients", "gemini") {
		t.Fatalf("project artifact layout = %q, %v", root, err)
	}
	if _, _, err := adapter.ProjectTargetRoot(request.Client, domain.PackageNative, filepath.Dir(user), request.Plan.TargetContext); err == nil {
		t.Fatal("project artifacts escaped to user profile")
	}
	request.Plan.NativeRegistryRoot = user
	if _, err := adapter.Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, request); err == nil {
		t.Fatal("project plan accepted user registry")
	}
	request.Plan.NativeRegistryRoot = filepath.Join(request.Plan.TargetContext.ProjectRoot, ".gemini")
	request.Plan.TargetContext.UserConfigRoot = filepath.Join(user, "other")
	if _, err := adapter.Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, request); err == nil {
		t.Fatal("project plan replaced original user profile authority")
	}
}
