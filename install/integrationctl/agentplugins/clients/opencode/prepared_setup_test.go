package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
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
	completion := request
	completion.VerifyOnly = false
	if err != nil || preview.Activation != domain.ActivationPrepared || !New().CanCompletePreparedSetup(completion, preview) {
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
			completion := request
			completion.VerifyOnly = false
			if err != nil || actual.Activation != domain.ActivationPrepared || New().CanCompletePreparedSetup(completion, actual) {
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
		{"verification request", func(in *domain.ActivationRequest, _ *domain.ActivationOutcome) { in.VerifyOnly = true }},
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
			request.VerifyOnly = false
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

type openCodeMaterializationFixture struct {
	request        domain.ActivationRequest
	env            clients.Env
	config, skills []byte
	calls          []string
}

func newOpenCodeMaterializationFixture(t *testing.T, scope domain.InstallScope) *openCodeMaterializationFixture {
	t.Helper()
	r, env, config, skills := nativeProbeFailureFixture(t)
	r.Client.ExecutablePath = filepath.Join(r.Plan.NativeRegistryRoot, "opencode")
	r.Plan.ActivePath = r.Delivery.ActivePath
	f := &openCodeMaterializationFixture{request: r, env: env, config: config, skills: skills}
	if scope == domain.ScopeUser {
		prepareOpenCodeMaterializationUserScope(t, f)
	}
	qualifyOpenCodeObservation(&f.request, "1.18.34")
	f.env.Runner = skillObservationRunner(func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		name := strings.Join(command.Argv[1:], " ")
		f.calls = append(f.calls, name)
		if command.Dir != f.request.Plan.NativeRegistryRoot || !slices.Contains(command.Env, "OPENCODE_CONFIG_DIR="+f.request.Client.ConfigRoot) {
			t.Fatal("read-only observation changed the selected native profile")
		}
		if scope == domain.ScopeUser && command.Env[len(command.Env)-1] != "OPENCODE_DISABLE_PROJECT_CONFIG=1" {
			t.Fatal("User observation did not explicitly disable project configuration")
		}
		switch name {
		case "debug config":
			return legacyports.CommandResult{Stdout: f.config}, nil
		case "debug skill":
			return legacyports.CommandResult{Stdout: f.skills}, nil
		default:
			t.Fatalf("materialization verification started unexpected command: %v", command.Argv)
			return legacyports.CommandResult{}, nil
		}
	})
	return f
}

func prepareOpenCodeMaterializationUserScope(t *testing.T, f *openCodeMaterializationFixture) {
	t.Helper()
	root := f.request.Plan.NativeRegistryRoot
	skill := filepath.Join(root, "skills", "probe")
	if err := os.MkdirAll(filepath.Dir(skill), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(f.request.PreviousNativeObjects[0].Path, skill); err != nil {
		t.Fatal(err)
	}
	f.request.PreviousNativeObjects = slices.Clone(f.request.PreviousNativeObjects)
	f.request.Delivery.NativeObjects = slices.Clone(f.request.Delivery.NativeObjects)
	f.request.PreviousNativeObjects[0].Path = skill
	f.request.Delivery.NativeObjects[0].Path = skill
	f.request.Client.ConfigRoot = root
	f.request.Plan.Scope = domain.ScopeUser
	f.request.Plan.TargetContext = domain.TargetContext{Scope: domain.ScopeUser, UserConfigRoot: root}
	var catalog []openCodeObservedSkill
	if err := json.Unmarshal(f.skills, &catalog); err != nil {
		t.Fatal(err)
	}
	catalog[0].Location = filepath.Join(skill, "SKILL.md")
	var err error
	f.skills, err = json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
}

func openCodeMaterializationFiles(t *testing.T, request domain.ActivationRequest) map[string]string {
	t.Helper()
	paths := []string{filepath.Join(request.Delivery.ActivePath, OpenCodeProjectionFile),
		request.PreviousNativeObjects[1].Path, filepath.Join(request.PreviousNativeObjects[0].Path, "SKILL.md")}
	result := make(map[string]string, len(paths))
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		result[path] = fmt.Sprintf("%o:%d:%s", info.Mode(), info.ModTime().UnixNano(), body)
	}
	return result
}

func TestOpenCodePreparedMaterializationUsesFreshReadOnlyObservation(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
		t.Run(string(scope), func(t *testing.T) {
			f := newOpenCodeMaterializationFixture(t, scope)
			before := openCodeMaterializationFiles(t, f.request)
			observed, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || observed.Activation != domain.ActivationPrepared || observed.NativeEffect != domain.NativeEffectUnchanged {
				t.Fatalf("fresh read-only observation failed: %+v %v", observed, err)
			}
			actions := slices.Clone(observed.UserActions)
			objects := slices.Clone(observed.NativeObjects)
			if !New().PreparedMaterializationVerified(f.request, observed) {
				t.Fatal("complete fresh materialization was not recognized")
			}
			if !slices.Equal(f.calls, []string{"debug config", "debug skill"}) {
				t.Fatalf("materialization proof invoked MCP startup: %v", f.calls)
			}
			if observed.Activation != domain.ActivationPrepared || observed.ActivationAttested || observed.AuthenticationAttested ||
				!slices.Equal(observed.UserActions, actions) || !slices.Equal(observed.NativeObjects, objects) ||
				!reflect.DeepEqual(before, openCodeMaterializationFiles(t, f.request)) {
				t.Fatal("pure materialization proof changed readiness, ownership or original files")
			}
		})
	}
}

func TestOpenCodePreparedMaterializationRejectsChangedPacket(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*domain.ActivationRequest, *domain.ActivationOutcome)
	}{
		{"acting", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.VerifyOnly = false }},
		{"attested request", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.ActivationComplete = true }},
		{"no marker", func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.UserActions = nil }},
		{"activation attested", func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.ActivationAttested = true }},
		{"authentication attested", func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.AuthenticationAttested = true }},
		{"authoritative negative", func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.AuthoritativeObservation = true }},
		{"unknown ABI", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			qualifyOpenCodeObservation(r, "unknown")
		}},
		{"missing ABI", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.OpenCodeHost = nil }},
		{"unqualified ABI", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			qualifyOpenCodeObservation(r, "1.17.0")
		}},
		{"nonautomatic intent", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.InstallIntent = domain.InstallIntentPrepare
		}},
		{"no MCP", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.Components = r.Plan.Components[:1]
		}},
		{"relative executable", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Client.ExecutablePath = "opencode" }},
		{"no ownership", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.PreviousNativeObjects = nil }},
		{"different ownership", func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.NativeObjects[0].Path += "-foreign" }},
		{"wrong client", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Client.ClientID = "other" }},
		{"wrong plan client", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.ClientID = "other" }},
		{"wrong delivery client", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Delivery.ClientID = "other" }},
		{"wrong scope", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.Scope = domain.ScopeUser }},
		{"wrong native root", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.NativeRegistryRoot += "-foreign"
		}},
		{"wrong user profile", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Client.ConfigRoot += "-foreign" }},
		{"wrong active path", func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.ActivePath += "-foreign" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOpenCodeMaterializationFixture(t, domain.ScopeProject)
			observed, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || !New().PreparedMaterializationVerified(f.request, observed) {
				t.Fatalf("invalid positive materialization fixture: %v", err)
			}
			tc.change(&f.request, &observed)
			if New().PreparedMaterializationVerified(f.request, observed) {
				t.Fatal("changed read-only packet borrowed fresh proof")
			}
		})
	}
}

func changeOpenCodeMaterializationFixture(t *testing.T, f *openCodeMaterializationFixture, variant string) {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(f.config, &document); err != nil {
		t.Fatal(err)
	}
	server := document["mcp"].(map[string]any)["local"].(map[string]any)
	switch variant {
	case "missing config":
		if err := os.Remove(f.request.PreviousNativeObjects[1].Path); err != nil {
			t.Fatal(err)
		}
	case "missing skill":
		if err := os.Remove(filepath.Join(f.request.PreviousNativeObjects[0].Path, "SKILL.md")); err != nil {
			t.Fatal(err)
		}
	case "missing MCP":
		delete(document["mcp"].(map[string]any), "local")
	case "permission":
		document["permission"] = map[string]any{"*": "deny"}
	case "foreign command":
		server["command"] = []string{"/FOREIGN/server"}
	case "remote":
		server["type"] = "remote"
	case "disabled":
		server["enabled"] = false
	case "wrong skill":
		f.skills = []byte("[]")
	}
	var err error
	f.config, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenCodePreparedMaterializationRejectsIncompleteNativeProjection(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
		for _, variant := range []string{"missing config", "missing skill", "missing MCP", "permission", "foreign command", "remote", "disabled", "wrong skill"} {
			t.Run(string(scope)+"/"+variant, func(t *testing.T) {
				f := newOpenCodeMaterializationFixture(t, scope)
				changeOpenCodeMaterializationFixture(t, f, variant)
				observed, _ := New().Activate(t.Context(), f.env, f.request)
				if New().PreparedMaterializationVerified(f.request, observed) || slices.Contains(observed.UserActions, openCodeMCPConfirmationAction) {
					t.Fatalf("incomplete native projection gained materialization proof: %+v", observed)
				}
			})
		}
	}
}

type missingOpenCodeNativeToken struct{ contracttest.OpenCodeV1Host }

func (missingOpenCodeNativeToken) NativeObservationAuthority() *opencodehost.NativePrepared {
	return nil
}

func TestOpenCodeForeignAuthorityStartsNoNativeCommands(t *testing.T) {
	for _, host := range []domain.OpenCodeHostAuthority{contracttest.OpenCodeV1Host{}, missingOpenCodeNativeToken{}} {
		request, env, _, _ := nativeProbeFailureFixture(t)
		request.Plan.OpenCodeHost = host
		env.Runner = skillObservationRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
			t.Fatal("foreign authority launched native discovery")
			return legacyports.CommandResult{}, nil
		})
		actual, err := observeOpenCodeNativeComponents(t.Context(), env, request, preparedOpenCodeObservation())
		if err != nil || actual.Activation != domain.ActivationPrepared {
			t.Fatalf("foreign authority claimed readiness: %+v %v", actual, err)
		}
	}
}
