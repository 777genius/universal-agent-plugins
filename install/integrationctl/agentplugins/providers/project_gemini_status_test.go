package providers_test

import (
	"context"
	"errors"
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

func projectGeminiActivator(t *testing.T, kernel nativeconfig.Kernel) providers.Activator {
	t.Helper()
	registry, err := clients.NewRegistry(gemini.New())
	if err != nil {
		t.Fatal(err)
	}
	return providers.Activator{Registry: registry, NativeConfig: &kernel}
}

func TestProjectGeminiConfiguredStateDoesNotClaimNativeDiscovery(t *testing.T) {
	request, _, _ := projectGeminiRequest(t)
	// An ambient bypass flag is never proof of durable workspace trust.
	t.Setenv("GEMINI_CLI_TRUST_WORKSPACE", "true")
	activator := projectGeminiActivator(t, nativeconfig.New())
	for pass := range 3 {
		outcome, err := activator.Activate(context.Background(), request)
		if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.ActivationAttested {
			t.Fatalf("pass %d configured status = %+v, %v", pass, outcome, err)
		}
		if !strings.Contains(strings.Join(outcome.UserActions, "\n"), "workspace trust, system policy and MCP initialization remain unobserved") {
			t.Fatalf("native proof boundary missing: %+v", outcome)
		}
		request.PreviousNativeObjects = outcome.NativeObjects
		// A second automatic reconciliation and a fresh verifier both stay
		// exact and prepared; neither config presence nor a restart promotes it.
		request.VerifyOnly = pass > 0
	}
	before := projectGeminiInventory(t, request.Plan.NativeRegistryRoot)
	for _, automatic := range []bool{true, false} {
		request.ActivationComplete = true
		if !automatic {
			request.Plan.Components[0].Support = domain.SupportPrepared
		}
		if _, err := activator.Activate(context.Background(), request); err == nil || !strings.Contains(err.Error(), "cannot be established by activation attestation") {
			t.Fatalf("generic attestation bypassed project preflight: %v", err)
		}
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, request.Plan.NativeRegistryRoot)) {
		t.Fatal("attestation mutated project configuration")
	}
}

func TestProjectGeminiPreservesRestrictionsAndSkillAlias(t *testing.T) {
	request, projectB, user := projectGeminiRequest(t)
	userSettings := `{"skills":{"disabled":["docs"]},"mcp":{"allowed":[]},"mcpServers":{"docs":{"url":"https://user.test/mcp"}}}`
	workspaceSettings := "{ // preserve workspace restriction\n\"skills\":{\"enabled\":false},\"mcp\":{\"excluded\":[\"docs\"]},\"mcpServers\":{}}\n"
	projectGeminiWrite(t, filepath.Join(user, "settings.json"), userSettings)
	projectGeminiWrite(t, filepath.Join(request.Plan.NativeRegistryRoot, "settings.json"), workspaceSettings)
	alias := filepath.Join(request.Plan.TargetContext.ProjectRoot, ".agents", "skills", "docs", "SKILL.md")
	const aliasBody = "---\nname: docs\ndescription: Foreign alias\n---\nKeep alias bytes.\n"
	projectGeminiWrite(t, alias, aliasBody)
	beforeUser, beforeB := projectGeminiInventory(t, user), projectGeminiInventory(t, projectB)
	outcome, err := projectGeminiActivator(t, nativeconfig.New()).Activate(context.Background(), request)
	if err != nil || outcome.Activation != domain.ActivationPrepared {
		t.Fatalf("restricted configuration = %+v, %v", outcome, err)
	}
	actions := strings.Join(outcome.UserActions, "\n")
	for _, reason := range []string{`disable skill "docs"`, `restrict MCP server "docs"`, `.agents/skills/docs/SKILL.md may shadow`} {
		if !strings.Contains(actions, reason) {
			t.Fatalf("missing %q in %s", reason, actions)
		}
	}
	projectGeminiAssert(t, alias, aliasBody)
	settings, err := os.ReadFile(filepath.Join(request.Plan.NativeRegistryRoot, "settings.json"))
	if err != nil || !strings.Contains(string(settings), `"enabled":false`) || !strings.Contains(string(settings), `"excluded":["docs"]`) || !strings.Contains(string(settings), "// preserve workspace restriction") {
		t.Fatalf("workspace policy changed: %s, %v", settings, err)
	}
	if !reflect.DeepEqual(beforeUser, projectGeminiInventory(t, user)) || !reflect.DeepEqual(beforeB, projectGeminiInventory(t, projectB)) {
		t.Fatal("restriction inspection mutated another scope")
	}
}

func TestProjectGeminiMalformedSettingsAndWriterFailureLeaveNativeStateUnchanged(t *testing.T) {
	for name, prepare := range map[string]func(*testing.T, domain.ActivationRequest) nativeconfig.Kernel{
		"malformed project": func(t *testing.T, r domain.ActivationRequest) nativeconfig.Kernel {
			projectGeminiWrite(t, filepath.Join(r.Plan.NativeRegistryRoot, "settings.json"), `{"skills":{"enabled":false},}`)
			return nativeconfig.New()
		},
		"foreign MCP collision": func(t *testing.T, r domain.ActivationRequest) nativeconfig.Kernel {
			projectGeminiWrite(t, filepath.Join(r.Plan.NativeRegistryRoot, "settings.json"), `{"mcpServers":{"docs":{"url":"https://foreign.test/keep"}}}`)
			return nativeconfig.New()
		},
		"MCP writer lock failure": func(*testing.T, domain.ActivationRequest) nativeconfig.Kernel {
			return nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
				return nil, errors.New("TEST writer lock unavailable")
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			request, b, user := projectGeminiRequest(t)
			kernel := prepare(t, request)
			beforeProject, beforeUser, beforeB := projectGeminiInventory(t, request.Plan.NativeRegistryRoot), projectGeminiInventory(t, user), projectGeminiInventory(t, b)
			if _, err := projectGeminiActivator(t, kernel).Activate(context.Background(), request); err == nil {
				t.Fatal("unsafe/failed project mutation succeeded")
			}
			if !reflect.DeepEqual(beforeProject, projectGeminiInventory(t, request.Plan.NativeRegistryRoot)) || !reflect.DeepEqual(beforeUser, projectGeminiInventory(t, user)) || !reflect.DeepEqual(beforeB, projectGeminiInventory(t, b)) {
				t.Fatal("failed project install left native mutations")
			}
		})
	}
}
