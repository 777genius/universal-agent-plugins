package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type geminiReviewedMCPProbe struct {
	t      *testing.T
	calls  int
	result legacyports.CommandResult
	err    error
}

func (probe *geminiReviewedMCPProbe) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	probe.t.Helper()
	probe.calls++
	if !reflect.DeepEqual(command.Argv[1:], []string{"mcp", "list"}) {
		probe.t.Fatalf("unexpected native command: %v", command.Argv)
	}
	for _, item := range command.Env {
		if strings.HasPrefix(item, "GEMINI_CLI_SYSTEM_") {
			probe.t.Fatalf("native policy was overridden: %s", item)
		}
	}
	return probe.result, probe.err
}

func geminiMCPReviewFixture(t *testing.T, server nativeconfig.Server) (domain.ActivationRequest, nativeconfig.Kernel) {
	t.Helper()
	request, kernel := geminiProbeFixture(t)
	request.Client.ExecutablePath = filepath.Join(filepath.Dir(request.Client.ConfigRoot), "gemini")
	request.Plan.ClientID = domain.ClientGemini
	request.Plan.InstallIntent = domain.InstallIntentAutomatic
	request.Plan.TargetContext.UserConfigRoot = request.Client.ConfigRoot
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "audit", Support: domain.SupportNative}}
	settings := filepath.Join(request.Plan.NativeRegistryRoot, "settings.json")
	if err := os.WriteFile(settings, []byte(`{"sentinel":"keep","mcpServers":{"foreign":{"command":"foreign-server"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	receipt, err := kernel.Apply(nativeconfig.Request{Paths: GeminiConfigPaths(request.Plan.NativeRegistryRoot), Codec: nativeconfig.CodecGemini, Action: nativeconfig.ActionAdd, Name: "audit", Server: server})
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.ClientID = domain.ClientGemini
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{{ObjectID: "gemini-mcp:audit", Kind: GeminiMCPObjectKind, LogicalName: "audit", Path: receipt.Path, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}}
	request.PreviousNativeObjects = slices.Clone(request.Delivery.NativeObjects)
	if err := os.MkdirAll(request.Client.ConfigRoot, 0700); err != nil {
		t.Fatal(err)
	}
	trust, err := json.Marshal(map[string]string{request.Plan.TargetContext.ProjectRoot: "TRUST_FOLDER", filepath.Join(filepath.Dir(request.Client.ConfigRoot), "foreign-project"): "DO_NOT_TRUST"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(request.Client.ConfigRoot, "trustedFolders.json"), trust, 0600); err != nil {
		t.Fatal(err)
	}
	return request, kernel
}

func geminiMCPReviewOutcome() domain.ActivationOutcome {
	return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged}
}

func TestGeminiMCPReviewAndRepeatRequireConfirmedConnectionCheck(t *testing.T) {
	request, kernel := geminiMCPReviewFixture(t, nativeconfig.Server{Type: "stdio", Command: "owned-server", Args: []string{"--audit"}})
	probe := &geminiReviewedMCPProbe{t: t, result: legacyports.CommandResult{CleanupComplete: true, Stderr: []byte("Configured MCP servers:\n\n✓ foreign: foreign-server  (stdio) - Connected\n✓ audit: owned-server --audit (stdio) - Connected\n")}}
	env := clients.Env{NativeConfig: kernel, Runner: probe}
	paths := []string{filepath.Join(request.Plan.NativeRegistryRoot, "settings.json"), filepath.Join(request.Client.ConfigRoot, "trustedFolders.json")}
	before := map[string]string{}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = string(body)
	}
	for cycle := range 2 {
		request.VerifyOnly = true
		preview, err := New().Activate(t.Context(), env, request)
		if err != nil || preview.Activation != domain.ActivationPrepared || !New().CanCompletePreparedSetup(request, preview) || probe.calls != cycle {
			t.Fatalf("read-only review ran MCP or lost confirmed repair: %+v; %v; calls=%d", preview, err, probe.calls)
		}
		// Stopping after review is the declined path: the native profile remains
		// exactly the same and no server has been started by the adapter.
		for _, path := range paths {
			body, err := os.ReadFile(path)
			if err != nil || string(body) != before[path] {
				t.Fatalf("review changed foreign/native preferences at %q: %v", path, err)
			}
		}
		request.VerifyOnly = false
		applied, err := completeGeminiProject(t.Context(), env, request, geminiMCPReviewOutcome())
		if err != nil || applied.Activation != domain.ActivationActive || slices.Contains(applied.UserActions, geminiMCPConfirmationAction) || probe.calls != cycle+1 {
			t.Fatalf("confirmed native check failed: %+v; %v; calls=%d", applied, err, probe.calls)
		}
	}
}

func TestGeminiMCPReviewRefusesRemoteBlockedAndDriftedEntries(t *testing.T) {
	for _, mode := range []string{"remote", "blocked", "drift"} {
		t.Run(mode, func(t *testing.T) {
			server := nativeconfig.Server{Type: "stdio", Command: "owned-server"}
			if mode == "remote" {
				server = nativeconfig.Server{Type: "remote", RemoteTransport: "streamable-http", URL: "https://example.test/mcp"}
			}
			request, kernel := geminiMCPReviewFixture(t, server)
			settings := filepath.Join(request.Plan.NativeRegistryRoot, "settings.json")
			if mode != "remote" {
				body, err := os.ReadFile(settings)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "blocked" {
					body = append([]byte(`{"mcp":{"excluded":["audit"]},`), body[1:]...)
				} else {
					body = []byte(strings.ReplaceAll(string(body), "owned-server", "foreign-replacement"))
				}
				if err := os.WriteFile(settings, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			probe := &geminiReviewedMCPProbe{t: t}
			observed, err := completeGeminiProject(t.Context(), clients.Env{NativeConfig: kernel, Runner: probe}, request, geminiMCPReviewOutcome())
			if err != nil || observed.Activation == domain.ActivationActive || New().CanCompletePreparedSetup(request, observed) || probe.calls != 0 {
				t.Fatalf("unsupported preview admitted acting repair: %+v; %v; calls=%d", observed, err, probe.calls)
			}
		})
	}
}

func TestGeminiConfirmedMCPFailureRetainsCleanupBoundary(t *testing.T) {
	for _, closed := range []bool{true, false} {
		t.Run(map[bool]string{true: "closed", false: "unclosed"}[closed], func(t *testing.T) {
			request, kernel := geminiMCPReviewFixture(t, nativeconfig.Server{Type: "stdio", Command: "owned-server"})
			request.VerifyOnly = false
			cause := errors.New("native connection check failed")
			probe := &geminiReviewedMCPProbe{t: t, result: legacyports.CommandResult{CleanupComplete: closed}, err: cause}
			observed, err := completeGeminiProject(t.Context(), clients.Env{NativeConfig: kernel, Runner: probe}, request, geminiMCPReviewOutcome())
			if probe.calls != 1 || observed.Activation == domain.ActivationActive || domain.IsUnclosedClientProbe(err) != !closed || New().CanCompletePreparedSetup(request, observed) {
				t.Fatalf("failed connection check was accepted: %+v; %v", observed, err)
			}
			if !closed && (observed.NativeEffect != domain.NativeEffectUncertain || !errors.Is(err, cause)) {
				t.Fatalf("unclosed native effect/cause lost: %+v; %v", observed, err)
			}
		})
	}
}

func TestGeminiMCPReviewWarningIsScopedAndIdempotent(t *testing.T) {
	for _, test := range []struct {
		name   string
		scope  domain.InstallScope
		intent domain.InstallIntent
		kind   domain.ComponentKind
		want   bool
	}{
		{"project MCP", domain.ScopeProject, domain.InstallIntentAutomatic, domain.ComponentMCPServer, true},
		{"project Skill", domain.ScopeProject, domain.InstallIntentAutomatic, domain.ComponentSkill, false},
		{"prepared MCP", domain.ScopeProject, domain.InstallIntentPrepare, domain.ComponentMCPServer, false},
		{"user MCP", domain.ScopeUser, domain.InstallIntentAutomatic, domain.ComponentMCPServer, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			project, profile := t.TempDir(), filepath.Join(t.TempDir(), ".gemini")
			input := clients.PlanInput{Intent: test.intent, Client: domain.DetectedClient{ConfigRoot: profile}, TargetContext: domain.TargetContext{Scope: test.scope, ProjectRoot: project}}
			plan := domain.DeliveryPlan{Scope: test.scope, NativeRegistryRoot: filepath.Join(project, ".gemini"), Components: []domain.ComponentDecision{{Kind: test.kind, Name: "audit", Support: domain.SupportNative}}}
			for range 2 {
				if err := New().RefinePlan(t.Context(), input, &plan); err != nil {
					t.Fatal(err)
				}
			}
			count := 0
			for _, warning := range plan.Warnings {
				if warning == geminiMCPReviewWarning {
					count++
				}
			}
			if (count == 1) != test.want || count > 1 {
				t.Fatalf("wrong reviewed MCP scope: %v", plan.Warnings)
			}
		})
	}
}
