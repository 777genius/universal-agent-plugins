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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func projectOpenCodeRequest(t *testing.T) (domain.ActivationRequest, string, string) {
	return projectOpenCodeRequestWithNames(t, []string{"docs"})
}

func projectOpenCodeRequestWithNames(t *testing.T, names []string) (domain.ActivationRequest, string, string) {
	t.Helper()
	base := t.TempDir()
	user, projectA, projectB := filepath.Join(base, "user", "opencode"), filepath.Join(base, "A"), filepath.Join(base, "B")
	foreign := "{ // preserve foreign settings\n\"mcp\":{\"foreign\":{\"type\":\"remote\",\"url\":\"https://foreign.test/mcp\",\"enabled\":false}},\"theme\":\"foreign\"}\n"
	for _, root := range []string{user, projectA, projectB} {
		projectGeminiWrite(t, filepath.Join(root, "opencode.jsonc"), foreign)
	}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: user, Status: domain.DetectionDetected, OpenCodeHost: contracttest.OpenCodeV1Host{}}
	target, err := domain.NewTargetContext(domain.ScopeProject, projectA, user)
	if err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(projectA, ".uap", "managed", "clients", "opencode", "archive-demo")
	projectGeminiWrite(t, filepath.Join(active, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Inert project fixture\n---\nProject skill.\n")
	plan := domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Scope: domain.ScopeProject, TargetContext: target, InstallIntent: domain.InstallIntentAutomatic,
		NativeRegistryRoot: projectA, ActivePath: active, OpenCodeHost: client.OpenCodeHost, Components: []domain.ComponentDecision{
			{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative},
		}}
	envelope := domain.PackageEnvelope{Skills: map[string]domain.Skill{"docs": {Name: "docs", RelativePath: "skills/docs/SKILL.md"}},
		MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{}}}
	for _, name := range names {
		plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: name, Support: domain.SupportNative})
		envelope.MCP.Servers[name] = domain.MCPServer{Name: name, Type: "streamable-http", Decoded: map[string]any{"url": "https://managed.test/mcp"}}
	}
	objects, err := opencode.New().Project(context.Background(), clients.ProjectionInput{StagingPath: active, Envelope: envelope, Plan: plan, PluginDataPath: filepath.Join(projectA, ".uap", "plugin-data", "demo")})
	if err != nil {
		t.Fatal(err)
	}
	return domain.ActivationRequest{Client: client, Plan: plan, DeclaredName: "archive-demo", Delivery: domain.StagedDelivery{
		ClientID: domain.ClientOpenCode, ActivePath: active, OwnedBase: filepath.Dir(active), NativeObjects: objects}}, projectB, user
}

func TestProjectOpenCodeSplitNativeLifecycleKeepsUserAndOtherProject(t *testing.T) {
	request, projectB, user := projectOpenCodeRequest(t)
	userBefore, bBefore := projectGeminiInventory(t, user), projectGeminiInventory(t, projectB)
	registry, err := clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	kernel := nativeconfig.New()
	activator := providers.Activator{Registry: registry, NativeConfig: &kernel}
	outcome, err := activator.Activate(context.Background(), request)
	if err != nil || outcome.Verification != domain.VerificationInstalled || outcome.Activation != domain.ActivationPrepared || len(outcome.NativeObjects) != 2 {
		t.Fatalf("project native activation: %+v, %v", outcome, err)
	}
	root := request.Plan.NativeRegistryRoot
	if _, err := os.Stat(filepath.Join(root, ".opencode", "skills", "docs", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills")); !os.IsNotExist(err) {
		t.Fatalf("project used user skill layout: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(root, "opencode.jsonc"))
	if err != nil || !strings.Contains(string(before), "preserve foreign settings") || !strings.Contains(string(before), "managed.test") || !strings.Contains(string(before), "foreign.test") {
		t.Fatalf("native config lost exact foreign entries/comments: %s, %v", before, err)
	}
	request.PreviousNativeObjects = outcome.NativeObjects
	for _, verifyOnly := range []bool{false, true} {
		request.VerifyOnly = verifyOnly
		result, err := activator.Activate(context.Background(), request)
		if err != nil || result.Activation != domain.ActivationPrepared || result.Verification != domain.VerificationInstalled {
			t.Fatalf("repeat project native activation: %+v, %v", result, err)
		}
	}
	// Repair the exact same receipts after both selected native leaves disappear.
	if err := os.RemoveAll(filepath.Join(root, ".opencode", "skills", "docs")); err != nil {
		t.Fatal(err)
	}
	for _, object := range outcome.NativeObjects {
		if object.ObjectID != "opencode-mcp:docs" {
			continue
		}
		owned := nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecOpenCode, Name: "docs", Digest: object.ManagedDigest}
		if _, err := kernel.Apply(nativeconfig.Request{Paths: nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionRemove, Name: "docs", Owned: &owned}); err != nil {
			t.Fatal(err)
		}
	}
	request.VerifyOnly = false
	if result, err := activator.Activate(context.Background(), request); err != nil || result.Verification != domain.VerificationInstalled || result.Activation != domain.ActivationPrepared {
		t.Fatalf("same-pin native repair: %+v, %v", result, err)
	}
	remove := domain.DeactivationRequest{Client: request.Client, TargetContext: request.Plan.TargetContext, NativeRegistryRoot: root, Confirmed: true, NativeObjects: outcome.NativeObjects}
	for range 2 {
		result, err := activator.Deactivate(context.Background(), remove)
		if err != nil || !result.ExternalRemovalComplete {
			t.Fatalf("scoped removal: %+v, %v", result, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".opencode", "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("managed skill survived remove: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "opencode.jsonc"))
	if err != nil || strings.Contains(string(after), "managed.test") || !strings.Contains(string(after), "foreign.test") {
		t.Fatalf("remove lost foreign native entry: %s, %v", after, err)
	}
	if !reflect.DeepEqual(userBefore, projectGeminiInventory(t, user)) || !reflect.DeepEqual(bBefore, projectGeminiInventory(t, projectB)) {
		t.Fatal("project lifecycle mutated user or B")
	}
}

func TestProjectOpenCodeRejectsTamperedOwnershipBeforeNativeWrites(t *testing.T) {
	for _, fault := range []string{"user-skill", "wrong-id", "padded-name", "relative-config"} {
		t.Run(fault, func(t *testing.T) {
			request, _, user := projectOpenCodeRequest(t)
			before := projectGeminiInventory(t, filepath.Dir(request.Plan.TargetContext.ProjectRoot))
			for i := range request.Delivery.NativeObjects {
				object := &request.Delivery.NativeObjects[i]
				switch fault {
				case "user-skill":
					if object.ObjectID == "opencode-skill:docs" {
						object.Path = filepath.Join(user, "skills", "docs")
					}
				case "wrong-id":
					object.ObjectID = "foreign:" + object.LogicalName
				case "padded-name":
					object.LogicalName = " docs "
				case "relative-config":
					if object.ObjectID == "opencode-mcp:docs" {
						object.Path = "opencode.jsonc"
					}
				}
			}
			if _, err := opencode.New().Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, request); err == nil {
				t.Fatal("tampered ownership reached native mutation")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, filepath.Dir(request.Plan.TargetContext.ProjectRoot))) {
				t.Fatal("refused ownership changed files")
			}
		})
	}
}

func TestProjectOpenCodeRefusesScopeMismatchAndAttestationBeforeEffects(t *testing.T) {
	for _, fault := range []string{"user-registry", "user-context", "missing-scope", "attestation"} {
		t.Run(fault, func(t *testing.T) {
			request, _, _ := projectOpenCodeRequest(t)
			root := request.Plan.TargetContext.ProjectRoot
			before := projectGeminiInventory(t, filepath.Dir(root))
			switch fault {
			case "user-registry":
				request.Plan.NativeRegistryRoot = request.Client.ConfigRoot
			case "user-context":
				request.Plan.TargetContext.UserConfigRoot = filepath.Join(request.Client.ConfigRoot, "other")
			case "missing-scope":
				request.Plan.Scope = domain.ScopeUser
			case "attestation":
				request.ActivationComplete = true
			}
			registry, err := clients.NewRegistry(opencode.New())
			if err != nil {
				t.Fatal(err)
			}
			kernel := nativeconfig.New()
			activator := providers.Activator{Registry: registry, NativeConfig: &kernel}
			if _, err := activator.Activate(context.Background(), request); err == nil {
				t.Fatal("provider accepted invalid scoped authority")
			}
			if _, err := opencode.New().Activate(context.Background(), clients.Env{NativeConfig: kernel}, request); err == nil {
				t.Fatal("direct adapter reuse accepted invalid scoped authority")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, filepath.Dir(root))) {
				t.Fatal("refused project operation changed files")
			}
		})
	}
}

func TestProjectOpenCodeNamespacePreflightUsesSelectedProject(t *testing.T) {
	request, _, user := projectOpenCodeRequest(t)
	guard := providers.OpenCodeNamespacePreflight{Kernel: nativeconfig.New()}
	// A same-name user entry is a distinct configured scope and is preserved.
	projectGeminiWrite(t, filepath.Join(user, "opencode.jsonc"), `{"mcp":{"docs":{"type":"remote","url":"https://user.test/mcp","enabled":false}}}`)
	if err := guard.CheckMCPNamespace(context.Background(), request.Client, request.Plan, nil); err != nil {
		t.Fatalf("project preflight used user config: %v", err)
	}
	// A disabled same-name entry in the selected project reserves that exact key.
	projectGeminiWrite(t, filepath.Join(request.Plan.NativeRegistryRoot, "opencode.jsonc"), `{"mcp":{"docs":{"type":"remote","url":"https://project.test/mcp","enabled":false}}}`)
	if err := guard.CheckMCPNamespace(context.Background(), request.Client, request.Plan, nil); err == nil {
		t.Fatal("project preflight ignored its disabled native collision")
	}
}
