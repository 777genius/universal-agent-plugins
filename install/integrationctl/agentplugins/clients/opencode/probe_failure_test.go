package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func nativeProbeFailureFixture(t *testing.T) (domain.ActivationRequest, clients.Env, []byte, []byte) {
	t.Helper()
	root := t.TempDir()
	active := filepath.Join(root, ".uap", "managed", "probe")
	skill := filepath.Join(root, ".opencode", "skills", "probe")
	if err := os.MkdirAll(skill, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(active, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: probe\ndescription: Probe\n---\nProbe body\n"), 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := shared.DigestSkillDirectory(skill)
	if err != nil {
		t.Fatal(err)
	}
	kernel := nativeconfig.New()
	paths := nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	receipt, err := kernel.Apply(nativeconfig.Request{Paths: paths, Codec: nativeconfig.CodecOpenCode, Name: "local", Action: nativeconfig.ActionAdd, Server: nativeconfig.Server{Type: "stdio", Command: "/TEST/probe"}})
	if err != nil {
		t.Fatal(err)
	}
	projection := OpenCodeProjection{Version: 1, ConfigPath: receipt.Path, ConfigJSON: paths.JSON, ConfigJSONC: paths.JSONC, PackageRoot: active}
	body, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, OpenCodeProjectionFile), body, 0600); err != nil {
		t.Fatal(err)
	}
	objects := []domain.NativeObjectOwnership{
		{ObjectID: "opencode-skill:probe", Kind: openCodeSkillKind, LogicalName: "probe", Path: skill, SourceRelative: "skills/probe", ManagedDigest: digest, ProtectionClass: "managed"},
		{ObjectID: "opencode-mcp:local", Kind: OpenCodeMCPObjectKind, LogicalName: "local", Path: receipt.Path, ManagedDigest: receipt.Digest, ProtectionClass: "managed"},
	}
	request := domain.ActivationRequest{VerifyOnly: true, Client: domain.DetectedClient{ClientID: domain.ClientOpenCode, ExecutablePath: "/TEST/opencode", ConfigRoot: filepath.Join(root, "user")}, Plan: domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Scope: domain.ScopeProject, InstallIntent: domain.InstallIntentAutomatic, NativeRegistryRoot: root, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: root, UserConfigRoot: filepath.Join(root, "user")}, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "probe"}, {Kind: domain.ComponentMCPServer, Name: "local"}}}, Delivery: domain.StagedDelivery{ClientID: domain.ClientOpenCode, ActivePath: active, NativeObjects: objects}, PreviousNativeObjects: objects}
	qualifyOpenCodeObservation(&request, "1.18.34")
	config, err := os.ReadFile(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	skills, err := json.Marshal([]openCodeObservedSkill{{Name: "probe", Location: filepath.Join(skill, "SKILL.md"), Content: "Probe body\n"}})
	if err != nil {
		t.Fatal(err)
	}
	return request, clients.Env{NativeConfig: kernel}, config, skills
}

// An unknown close/reap/containment failure must stop activation and group
// progress. A proved-closed or never-started failure can remain pending.
func TestNativeProbeFailurePreservesProcessClosureEvidence(t *testing.T) {
	for index, phase := range []string{"debug config", "debug skill", "mcp list"} {
		for _, closure := range []struct {
			name   string
			result legacyports.CommandResult
			unsafe bool
		}{
			{name: "unknown", unsafe: true},
			{name: "closed", result: legacyports.CommandResult{CleanupComplete: true}},
			{name: "not started", result: legacyports.CommandResult{NoChildStarted: true}},
		} {
			t.Run(phase+"/"+closure.name, func(t *testing.T) {
				request, env, config, skills := nativeProbeFailureFixture(t)
				sentinel := errors.New("TEST native containment failure")
				calls := 0
				env.Runner = skillObservationRunner(func(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
					calls++
					if c.Argv[1]+" "+c.Argv[2] == phase {
						return closure.result, sentinel
					}
					output := []byte("┌  MCP Servers\n●  ✓ local connected\n└  1 server(s)\n")
					if c.Argv[2] == "config" {
						output = config
					}
					if c.Argv[2] == "skill" {
						output = skills
					}
					return legacyports.CommandResult{Stdout: output}, nil
				})
				var outcome domain.ActivationOutcome
				var err error
				if phase == "mcp list" {
					// MCP health belongs to the already confirmed acting path.
					request.VerifyOnly = false
					outcome, err = observeOpenCodeNativeComponents(t.Context(), env, request, domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectUnchanged})
				} else {
					outcome, err = New().Activate(t.Context(), env, request)
				}
				if calls != index+1 {
					t.Fatalf("probe continued past failure: calls=%d want=%d err=%v", calls, index+1, err)
				}
				if closure.unsafe {
					if !errors.Is(err, sentinel) || !domain.IsUnclosedClientProbe(err) || outcome.NativeEffect != domain.NativeEffectUncertain || outcome.Activation != domain.ActivationFailed {
						t.Fatalf("lost unclosed process evidence: outcome=%+v err=%v", outcome, err)
					}
				} else if err != nil || outcome.NativeEffect == domain.NativeEffectUncertain || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled {
					t.Fatalf("closed failure must stay installed/pending: outcome=%+v err=%v", outcome, err)
				}
			})
		}
	}
}
