package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type discoveryRunner struct {
	response string
	cleanup  error
	command  legacyports.Command
	written  bytes.Buffer
	called   bool
}

func (*discoveryRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, errors.New("ordinary process execution was not expected")
}

func (runner *discoveryRunner) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	runner.called, runner.command = true, command
	if _, deadline := ctx.Deadline(); !deadline {
		return errors.New("native discovery has no deadline")
	}
	if err := exchange(&runner.written, strings.NewReader(runner.response)); err != nil {
		return err
	}
	return runner.cleanup
}

func projectDiscoveryFixture(t *testing.T) (clients.Env, domain.ActivationRequest, *discoveryRunner) {
	t.Helper()
	root, profile := tempRoot(t), tempRoot(t)
	active := filepath.Join(root, ".uap", "managed")
	plan := domain.DeliveryPlan{ClientID: domain.ClientCodex, Scope: domain.ScopeProject,
		InstallIntent: domain.InstallIntentAutomatic, NativeRegistryRoot: root, ActivePath: active,
		TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: root, UserConfigRoot: profile},
		Components:    []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "first", Support: domain.SupportNative}, {Kind: domain.ComponentSkill, Name: "second", Support: domain.SupportNative}},
	}
	request := domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: profile}, Plan: plan,
		BackendExecutable: filepath.Join(root, "codex"), Delivery: domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: active, ArtifactDigest: "sha256:verified-test-delivery"}}
	for _, name := range []string{"first", "second"} {
		path := filepath.Join(active, "skills", name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("---\nname: "+name+"\ndescription: Inert native discovery fixture\n---\nNo model call.\n"), 0600); err != nil {
			t.Fatal(err)
		}
		object, err := codexProjectLayout(root).SkillObject(name, "skills/"+name, active)
		if err != nil {
			t.Fatal(err)
		}
		request.Delivery.NativeObjects = append(request.Delivery.NativeObjects, object)
	}
	runner := &discoveryRunner{response: projectDiscoveryResponses(t, request, "valid")}
	return clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}, request, runner
}

func projectDiscoveryResponses(t *testing.T, request domain.ActivationRequest, scenario string) string {
	t.Helper()
	root := request.Plan.NativeRegistryRoot
	initialize := map[string]any{"id": 1, "result": map[string]any{"userAgent": "codex-cli/test", "codexHome": request.Client.ConfigRoot}}
	skills := []any{}
	for _, component := range request.Plan.Components {
		skills = append(skills, map[string]any{"name": component.Name, "path": filepath.Join(root, ".agents", "skills", component.Name, "SKILL.md"), "scope": "repo", "enabled": true, "pluginId": nil})
	}
	group := map[string]any{"cwd": root, "skills": skills, "errors": []any{}}
	first := skills[0].(map[string]any)
	switch scenario {
	case "wrong-profile":
		initialize["result"].(map[string]any)["codexHome"] = root
	case "wrong-cwd":
		group["cwd"] = root + "-other"
	case "missing":
		group["skills"] = skills[:1]
	case "disabled":
		first["enabled"] = false
	case "foreign-path":
		first["path"] = filepath.Join(root+"-other", ".agents", "skills", "first", "SKILL.md")
	case "plugin":
		first["pluginId"] = "first@foreign"
	case "wrong-scope":
		first["scope"] = "user"
	case "duplicate":
		group["skills"] = append(skills, first)
	case "errors":
		group["errors"] = []any{map[string]any{"message": "skill load failed"}}
	case "null-errors":
		group["errors"] = nil
	}
	var response bytes.Buffer
	encoder := json.NewEncoder(&response)
	if err := encoder.Encode(initialize); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(map[string]any{"id": 2, "result": map[string]any{"config": map[string]any{"projects": map[string]any{root: map[string]any{"trust_level": "trusted"}}}}}); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(map[string]any{"id": 3, "result": map[string]any{"data": []any{group}}}); err != nil {
		t.Fatal(err)
	}
	return response.String()
}

func TestCodexProjectSkillsBecomeActiveOnlyAfterNativeDiscovery(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong-profile", "wrong-cwd", "missing", "disabled", "foreign-path", "plugin", "wrong-scope", "duplicate", "errors", "null-errors", "cleanup-failed", "missing-cli"} {
		t.Run(scenario, func(t *testing.T) {
			env, request, runner := projectDiscoveryFixture(t)
			runner.response = projectDiscoveryResponses(t, request, scenario)
			if scenario == "cleanup-failed" {
				runner.cleanup = errors.New("native child cleanup was not proved")
			}
			if scenario == "missing-cli" {
				request.BackendExecutable = ""
			}
			outcome, err := New().Activate(context.Background(), env, request)
			if err != nil || outcome.Verification != domain.VerificationInstalled || outcome.ActivationAttested || outcome.NativeEffect != domain.NativeEffectCommitted {
				t.Fatalf("durable installed files lost their truthful state: %+v, %v", outcome, err)
			}
			if err := verifyCodexProjectObjects(request.Plan.NativeRegistryRoot, request.Delivery.NativeObjects, false, env.NativeConfig); err != nil {
				t.Fatal(err)
			}
			if scenario == "valid" {
				if outcome.Activation != domain.ActivationActive || len(outcome.UserActions) != 0 || len(outcome.LocalActions) != 0 {
					t.Fatalf("native discovery did not finish installation: %+v", outcome)
				}
				if runner.command.Dir != request.Plan.NativeRegistryRoot || strings.Join(runner.command.Argv, " ") != request.BackendExecutable+" app-server" {
					t.Fatalf("native observation used another context: %+v", runner.command)
				}
				if !strings.Contains(runner.written.String(), `"method":"skills/list"`) || strings.Contains(runner.written.String(), "thread/") || strings.Contains(runner.written.String(), "turn/") {
					t.Fatalf("unexpected native protocol: %s", &runner.written)
				}
			} else if outcome.Activation != domain.ActivationPrepared || len(outcome.LocalActions) == 0 {
				t.Fatalf("unproved native state was upgraded: %+v", outcome)
			}
		})
	}
}

func TestCodexProjectDiscoveryPreservesRestrictionsAndCancellation(t *testing.T) {
	for _, scenario := range []string{"disabled-setting", "canceled", "mixed-mcp"} {
		t.Run(scenario, func(t *testing.T) {
			env, request, runner := projectDiscoveryFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled" {
				cancel()
				if err := observeCodexProjectComponents(ctx, env, request, request.Plan.NativeRegistryRoot); !errors.Is(err, context.Canceled) || runner.called {
					t.Fatalf("canceled observer started or lost cancellation: %v, %v", runner.called, err)
				}
				return
			}
			conditions := []string{}
			if scenario == "disabled-setting" {
				conditions = append(conditions, "preserved disabled skill")
			} else {
				request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "server", Support: domain.SupportNative})
			}
			outcome, err := completeCodexProjectDiscovery(ctx, env, request, request.Plan.NativeRegistryRoot, conditions, domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled})
			if err != nil || outcome.Activation != domain.ActivationPrepared || runner.called {
				t.Fatalf("native observation bypassed restrictions or claimed MCP initialization: %+v %v called=%v", outcome, err, runner.called)
			}
		})
	}
}

func TestCodexProjectDiscoveryRejectsMalformedProtocol(t *testing.T) {
	for _, response := range []string{
		`{"id":1,"result":{"userAgent":"codex"},"ID":1}`,
		`{"id":1,"error":{"code":-32601,"message":"unsupported"}}`,
		`{"id":1,"result":{},"method":"approval/request"}`,
		`{"id":999,"result":{"userAgent":"codex"}}`,
		strings.Repeat("x", (2<<20)+1),
		strings.Repeat("{\"method\":\"notice\"}\n", 65),
	} {
		env, request, runner := projectDiscoveryFixture(t)
		runner.response = response
		if err := observeCodexProjectComponents(context.Background(), env, request, request.Plan.NativeRegistryRoot); err == nil {
			t.Fatal("malformed protocol was accepted")
		}
	}
}

func TestCodexDiscoveryPathMatchesOnlySelectedNativeObjects(t *testing.T) {
	root := tempRoot(t)
	document := filepath.Join(root, "SKILL.md")
	if err := os.WriteFile(document, []byte("inert skill"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{root, document} {
		for _, actual := range []string{expected, filepath.ToSlash(expected), strings.ToUpper(expected), expected + "-other", "relative", expected + string(filepath.Separator) + "."} {
			want := actual == expected || runtime.GOOS == "windows" && (actual == filepath.ToSlash(expected) || actual == strings.ToUpper(expected))
			if got := codexDiscoveryPathMatches(actual, expected); got != want {
				t.Errorf("native path %q matches %q = %v, want %v", actual, expected, got, want)
			}
		}
		if codexDiscoveryPathMatches(nil, expected) {
			t.Fatal("missing native path was accepted")
		}
	}
	missing := filepath.Join(root, "missing")
	if codexDiscoveryPathMatches(strings.ToUpper(missing), missing) {
		t.Fatal("a Windows spelling alias without an existing native object was accepted")
	}
}
