package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestCurrentClaudeMarketplaceTrustFreshVerificationAndConfirmedResume(t *testing.T) {
	request := newClaudeMarketplaceRequest(t)
	market := ProjectMarketplaceName(request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot)
	cache := filepath.Join(request.Client.ConfigRoot, "plugins", "cache", market, request.DeclaredName, "1.0.0")
	installed, enabled, mutations := false, true, 0
	runner := claudeMarketplaceRunnerFunc(func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		args := command.Argv[1:]
		if reflect.DeepEqual(args, []string{"plugin", "list", "--json"}) {
			body := []byte("[]")
			if installed {
				body, _ = json.Marshal([]any{map[string]any{"id": request.DeclaredName + "@" + market, "version": "1.0.0", "scope": "project", "projectPath": request.Plan.TargetContext.ProjectRoot, "enabled": enabled, "projectEnabled": enabled, "installPath": cache, "readFromFolder": filepath.Join(request.Plan.ActivePath, claudeProjectBundleDirectory), "folderVersion": "1.0.0"}})
			}
			return legacyports.CommandResult{Stdout: body, NoChildStarted: true}, nil
		}
		mutations++
		switch {
		case reflect.DeepEqual(args, []string{"plugin", "marketplace", "add", request.Plan.ActivePath, "--scope", "project"}):
			err := shared.WriteJSON(filepath.Join(request.Client.ConfigRoot, "plugins", "known_marketplaces.json"), map[string]any{market: map[string]any{"source": map[string]any{"source": "directory", "path": request.Plan.ActivePath}, "installLocation": request.Plan.ActivePath}})
			return legacyports.CommandResult{NoChildStarted: true}, err
		case reflect.DeepEqual(args, []string{"plugin", "install", request.DeclaredName + "@" + market, "--scope", "project"}):
			if err := filetree.CopyDir(filepath.Join(request.Plan.ActivePath, claudeProjectBundleDirectory), cache); err != nil {
				return legacyports.CommandResult{}, err
			}
			err := shared.WriteJSON(filepath.Join(request.Client.ConfigRoot, "plugins", "installed_plugins.json"), map[string]any{"version": 2, "plugins": map[string]any{request.DeclaredName + "@" + market: []any{map[string]any{"scope": "project", "projectPath": request.Plan.TargetContext.ProjectRoot, "installPath": cache, "version": "1.0.0"}}}})
			installed = true
			return legacyports.CommandResult{NoChildStarted: true}, err
		default:
			t.Fatalf("unexpected native mutation: %v", args)
			return legacyports.CommandResult{}, nil
		}
	})
	env := clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}
	profile := filepath.Join(request.Client.ConfigRoot, ".claude.json")
	original := []byte(`{"projects":{"/unrelated":{"hasTrustDialogAccepted":false}},"allowedTools":[]}`)
	if err := os.WriteFile(profile, original, 0600); err != nil {
		t.Fatal(err)
	}
	assertTrusted := func() {
		t.Helper()
		body, err := os.ReadFile(profile)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := shared.DecodeStrictJSONObject(body)
		if err != nil {
			t.Fatal(err)
		}
		projects := doc["projects"].(map[string]any)
		if projects[filepath.ToSlash(request.Plan.TargetContext.ProjectRoot)].(map[string]any)["hasTrustDialogAccepted"] != true || projects["/unrelated"].(map[string]any)["hasTrustDialogAccepted"] != false {
			t.Fatalf("trust was not exact: %s", body)
		}
		if _, ok := doc["allowedTools"].([]any); !ok {
			t.Fatalf("unrelated empty array changed: %s", body)
		}
	}
	out, err := New().Activate(t.Context(), env, request)
	if err != nil || out.Activation != domain.ActivationActive || out.NativeEffect != domain.NativeEffectCommitted {
		t.Fatalf("fresh current marketplace setup: %+v %v", out, err)
	}
	assertTrusted()
	request.Replacing, request.PreviousNativeObjects = true, out.NativeObjects
	beforeMutations := mutations
	if err := os.WriteFile(profile, original, 0600); err != nil {
		t.Fatal(err)
	}
	request.VerifyOnly = true
	out, err = New().Activate(t.Context(), env, request)
	body, _ := os.ReadFile(profile)
	if err != nil || out.Activation != domain.ActivationPrepared || out.Verification != domain.VerificationInstalled || out.NativeEffect != domain.NativeEffectUnchanged || string(body) != string(original) || mutations != beforeMutations {
		t.Fatalf("read-only untrusted setup changed authority: %+v %v %s", out, err, body)
	}
	request.VerifyOnly = false
	out, err = New().Activate(t.Context(), env, request)
	if err != nil || out.Activation != domain.ActivationActive || out.NativeEffect != domain.NativeEffectCommitted || mutations != beforeMutations {
		t.Fatalf("confirmed intact trust completion: %+v %v", out, err)
	}
	assertTrusted()
	before, _ := os.ReadFile(profile)
	out, err = New().Activate(t.Context(), env, request)
	after, _ := os.ReadFile(profile)
	if err != nil || out.NativeEffect != domain.NativeEffectUnchanged || string(before) != string(after) || mutations != beforeMutations {
		t.Fatalf("trusted repeat mutated: %+v %v", out, err)
	}
	enabled = false
	if err := os.WriteFile(profile, original, 0600); err != nil {
		t.Fatal(err)
	}
	out, err = New().Activate(t.Context(), env, request)
	after, _ = os.ReadFile(profile)
	if err != nil || out.Activation != domain.ActivationPrepared || string(after) != string(original) {
		t.Fatalf("disabled plugin granted workspace trust: %+v %v", out, err)
	}
	enabled = true
	malformed := []byte(`{"projects":[]}`)
	if err := os.WriteFile(profile, malformed, 0600); err != nil {
		t.Fatal(err)
	}
	out, err = New().Activate(t.Context(), env, request)
	after, _ = os.ReadFile(profile)
	if err == nil || string(after) != string(malformed) || mutations != beforeMutations {
		t.Fatalf("unknown schema was overwritten: %+v %v", out, err)
	}
}
