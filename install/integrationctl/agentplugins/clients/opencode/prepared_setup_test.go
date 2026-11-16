package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func qualifyOpenCodeObservation(request *domain.ActivationRequest, version string) {
	evidence := opencodehost.VersionEvidence{Version: version, Source: "executable_version", ProbeStatus: "ok"}
	request.Plan.OpenCodeHost = opencodehost.NewNativePrepared(request.Client.ExecutablePath, request.Client.ConfigRoot, nil, evidence, opencodehost.Resolve(evidence), nil)
}

func preparedOpenCodeObservation() domain.ActivationOutcome {
	return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged, UserActions: []string{"preserve unrelated warning"}}
}

// The preview proves exact owned bytes and policy, but cannot start MCP servers.
// The existing confirmed repair performs a fresh native status observation.
func TestOpenCodeMCPPreviewRequiresConfirmation(t *testing.T) {
	request, env, config, skills := nativeProbeFailureFixture(t)
	request.Client.ExecutablePath = filepath.Join(request.Plan.NativeRegistryRoot, "opencode")
	request.Plan.ActivePath = request.Delivery.ActivePath
	qualifyOpenCodeObservation(&request, "1.18.34")
	var calls []string
	env.Runner = skillObservationRunner(func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		name := strings.Join(command.Argv[1:], " ")
		calls = append(calls, name)
		switch name {
		case "debug config":
			return legacyports.CommandResult{Stdout: config}, nil
		case "debug skill":
			return legacyports.CommandResult{Stdout: skills}, nil
		case "mcp list":
			return legacyports.CommandResult{Stdout: []byte("┌  MCP Servers\n●  ✓ local connected\n●  ✗ unrelated failed\n└  2 server(s)\n")}, nil
		default:
			t.Fatalf("unexpected native command: %v", command.Argv)
			return legacyports.CommandResult{}, nil
		}
	})
	preview, err := observeOpenCodeNativeComponents(t.Context(), env, request, preparedOpenCodeObservation())
	setup := request
	setup.VerifyOnly = false // The core classifies the future acting repair packet.
	if err != nil || preview.Activation != domain.ActivationPrepared || !New().CanCompletePreparedSetup(setup, preview) {
		t.Fatalf("fresh owned preview did not require confirmed completion: %+v %v", preview, err)
	}
	if !slices.Equal(calls, []string{"debug config", "debug skill"}) {
		t.Fatalf("preview started MCP health: %v", calls)
	}
	request.VerifyOnly = false
	calls = nil
	actual, err := observeOpenCodeNativeComponents(t.Context(), env, request, preview)
	if err != nil || actual.Activation != domain.ActivationActive || actual.ActivationAttested {
		t.Fatalf("confirmed native observation failed: %+v %v", actual, err)
	}
	if !slices.Equal(calls, []string{"debug config", "debug skill", "mcp list"}) || !slices.Equal(actual.UserActions, []string{"preserve unrelated warning"}) {
		t.Fatalf("wrong observation sequence or stale action removal: %v %+v", calls, actual)
	}
	projection, err := ReadOpenCodeProjection(request.Delivery.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(projection.ConfigPath)
	if err != nil || string(after) != string(config) {
		t.Fatalf("observation changed owned configuration: %v", err)
	}
}

func TestOpenCodeUnqualifiedABIStartsNoNativeCommands(t *testing.T) {
	for _, version := range []string{"", "0.1.0", "1.17.0", "2.2.5", "1.18.34-beta.1", "1.18.34+local", "unknown"} {
		t.Run(version, func(t *testing.T) {
			request, env, _, _ := nativeProbeFailureFixture(t)
			qualifyOpenCodeObservation(&request, version)
			if version == "" {
				request.Plan.OpenCodeHost = nil
			}
			env.Runner = skillObservationRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				t.Fatal("unqualified ABI launched a native command")
				return legacyports.CommandResult{}, nil
			})
			actual, err := observeOpenCodeNativeComponents(t.Context(), env, request, preparedOpenCodeObservation())
			if err != nil || actual.Activation != domain.ActivationPrepared || New().CanCompletePreparedSetup(request, actual) {
				t.Fatalf("unqualified ABI claimed readiness/completion: %+v %v", actual, err)
			}
		})
	}
}

func TestOpenCodePreparedCompletionRequiresFreshIntactObservation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*domain.ActivationRequest, *domain.ActivationOutcome)
	}{
		{"missing fresh marker", func(_ *domain.ActivationRequest, out *domain.ActivationOutcome) { out.UserActions = nil }},
		{"uncertain effect", func(_ *domain.ActivationRequest, out *domain.ActivationOutcome) {
			out.NativeEffect = domain.NativeEffectUncertain
		}},
		{"unverified", func(_ *domain.ActivationRequest, out *domain.ActivationOutcome) { out.Verification = "" }},
		{"policy unknown", func(_ *domain.ActivationRequest, out *domain.ActivationOutcome) { out.Policy = "" }},
		{"already active", func(_ *domain.ActivationRequest, out *domain.ActivationOutcome) {
			out.Activation = domain.ActivationActive
		}},
		{"nonautomatic intent", func(in *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			in.Plan.InstallIntent = domain.InstallIntentPrepare
		}},
		{"different package", func(in *domain.ActivationRequest, _ *domain.ActivationOutcome) { in.Plan.ActivePath += "-foreign" }},
		{"relative executable", func(in *domain.ActivationRequest, _ *domain.ActivationOutcome) { in.Client.ExecutablePath = "opencode" }},
		{"scope mismatch", func(in *domain.ActivationRequest, _ *domain.ActivationOutcome) { in.Plan.Scope = domain.ScopeUser }},
		{"no MCP", func(in *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			in.Plan.Components = in.Plan.Components[:1]
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, _, _, _ := nativeProbeFailureFixture(t)
			request.Client.ExecutablePath = filepath.Join(request.Plan.NativeRegistryRoot, "opencode")
			request.Plan.ActivePath = request.Delivery.ActivePath
			qualifyOpenCodeObservation(&request, "1.18.34")
			outcome := preparedOpenCodeObservation()
			outcome.UserActions = append(outcome.UserActions, openCodeMCPConfirmationAction)
			if !New().CanCompletePreparedSetup(request, outcome) {
				t.Fatal("positive classifier fixture is invalid")
			}
			tc.change(&request, &outcome)
			if New().CanCompletePreparedSetup(request, outcome) {
				t.Fatal("incomplete or mismatched evidence authorized completion")
			}
		})
	}
}

func TestOpenCodeMCPPreviewRejectsForeignAndDisabledConfig(t *testing.T) {
	for _, variant := range []string{"permission", "foreign command", "remote", "disabled", "wrong skill"} {
		t.Run(variant, func(t *testing.T) {
			request, env, config, skills := nativeProbeFailureFixture(t)
			var document map[string]any
			if err := json.Unmarshal(config, &document); err != nil {
				t.Fatal(err)
			}
			server := document["mcp"].(map[string]any)["local"].(map[string]any)
			switch variant {
			case "permission":
				document["permission"] = map[string]any{"*": "deny"}
			case "foreign command":
				server["command"] = []string{"/FOREIGN/server"}
			case "remote":
				server["type"] = "remote"
			case "disabled":
				server["enabled"] = false
			case "wrong skill":
				skills = []byte("[]")
			}
			config, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			env.Runner = skillObservationRunner(func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				if command.Argv[1] != "debug" {
					t.Fatal("refused preview started MCP health")
				}
				if command.Argv[2] == "config" {
					return legacyports.CommandResult{Stdout: config}, nil
				}
				return legacyports.CommandResult{Stdout: skills}, nil
			})
			outcome, err := observeOpenCodeNativeComponents(t.Context(), env, request, preparedOpenCodeObservation())
			if err != nil || outcome.Activation != domain.ActivationPrepared || slices.Contains(outcome.UserActions, openCodeMCPConfirmationAction) {
				t.Fatalf("refused observation granted completion evidence: %+v %v", outcome, err)
			}
		})
	}
}

func TestOpenCodeMCPStartupIsDisclosedBeforeInstallation(t *testing.T) {
	request, _, _, _ := nativeProbeFailureFixture(t)
	request.Client.OpenCodeHost = request.Plan.OpenCodeHost
	plan := request.Plan
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"local": {Type: "stdio"}}}}
	if err := New().RefinePlan(t.Context(), clients.PlanInput{Client: request.Client, TargetContext: plan.TargetContext, Envelope: envelope}, &plan); err != nil {
		t.Fatal(err)
	}
	warning := strings.Join(plan.Warnings, "\n")
	if !strings.Contains(warning, "every currently enabled MCP server") || !strings.Contains(warning, fmt.Sprintf("%q", plan.TargetContext.ProjectRoot)) {
		t.Fatalf("review did not disclose native startup and exact workspace: %v", plan.Warnings)
	}
}
