package providers_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Production standard loader, planner, renderer, stager, provider and native
// ownership kernel. No Codex session, model or configured MCP is executed.
func TestProjectCodexComponentsUseNativePathsAndPreserveOtherScopes(t *testing.T) {
	f := newProjectCodexFixture(t)
	request := f.stage(t, domain.CompatibilityHints{OpenAIMCPAuth: map[string]domain.OpenAIMCPAuthHint{"docs": {BearerTokenEnvVar: "TEST_TOKEN_REFERENCE", OAuthResource: "https://managed.test/resource"}}})
	userBefore, bBefore := projectGeminiInventory(t, f.user), projectGeminiInventory(t, f.b)
	for pass := range 3 {
		outcome, err := f.activator.Activate(t.Context(), request)
		if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.ActivationAttested || len(outcome.NativeObjects) != 5 {
			t.Fatalf("pass %d project activation = %+v, %v", pass, outcome, err)
		}
		request.PreviousNativeObjects = outcome.NativeObjects
		request.VerifyOnly = pass == 0
		if pass == 1 {
			if err := os.RemoveAll(filepath.Join(f.a, ".agents", "skills", "docs")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if body := projectCodexRead(t, filepath.Join(f.a, ".agents", "skills", "docs", "SKILL.md")); !strings.Contains(body, "UAP-CODEX-PROJECT") {
		t.Fatal("same-pin missing skill was not repaired")
	}
	if body := projectCodexRead(t, filepath.Join(f.a, ".agents", "skills", "demo-echo", "SKILL.md")); !strings.Contains(body, "UAP-CODEX-COMMAND") {
		t.Fatal("rendered command skill lost its preserved instructions")
	}
	settings := projectCodexRead(t, filepath.Join(f.a, ".codex", "config.toml"))
	for _, value := range []string{"foreign native policy", "foreign.test", "http_headers", "TEST_TOKEN_REFERENCE", "oauth_resource"} {
		if !strings.Contains(settings, value) {
			t.Fatalf("native TOML lost %q: %s", value, settings)
		}
	}
	remove := domain.DeactivationRequest{Client: request.Client, TargetContext: request.Plan.TargetContext, NativeRegistryRoot: f.a, Confirmed: true, NativeObjects: request.PreviousNativeObjects}
	for range 2 {
		outcome, err := f.activator.Deactivate(t.Context(), remove)
		if err != nil || !outcome.ExternalRemovalComplete {
			t.Fatalf("project Codex exact remove = %+v, %v", outcome, err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.a, ".agents", "skills", "docs")); !os.IsNotExist(err) {
		t.Fatal("managed skill survived removal")
	}
	if !reflect.DeepEqual(userBefore, projectGeminiInventory(t, f.user)) || !reflect.DeepEqual(bBefore, projectGeminiInventory(t, f.b)) {
		t.Fatal("Codex project lifecycle changed user profile/cache or project B")
	}
	if strings.Contains(projectCodexRead(t, filepath.Join(f.a, ".codex", "config.toml")), "managed.test") {
		t.Fatal("managed MCP registration survived removal")
	}
}

func TestProjectCodexRestrictionsAndAttestationRemainPrepared(t *testing.T) {
	f := newProjectCodexFixture(t)
	request := f.stage(t, domain.CompatibilityHints{})
	projectCodexWrite(t, filepath.Join(f.user, "config.toml"), "[mcp_servers.docs]\nenabled=false\n[[skills.config]]\nname='docs'\nenabled=false\n")
	outcome, err := f.activator.Activate(t.Context(), request)
	if err != nil || outcome.Activation != domain.ActivationPrepared {
		t.Fatalf("restricted project configuration = %+v, %v", outcome, err)
	}
	actions := strings.Join(outcome.UserActions, "\n")
	for _, value := range []string{`disable skill "docs"`, `disable MCP server "docs"`, "project trust", "remain unobserved"} {
		if !strings.Contains(actions, value) {
			t.Fatalf("project restriction/proof boundary missing %q: %s", value, actions)
		}
	}
	before := projectGeminiInventory(t, f.a)
	for _, automatic := range []bool{true, false} {
		request.ActivationComplete = true
		if !automatic {
			request.Plan.Components[0].Support = domain.SupportPrepared
		}
		if _, err := f.activator.Activate(t.Context(), request); err == nil || !strings.Contains(err.Error(), "cannot be established by activation attestation") {
			t.Fatalf("project attestation bypassed preflight: %v", err)
		}
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, f.a)) {
		t.Fatal("attestation changed project native objects")
	}
}
