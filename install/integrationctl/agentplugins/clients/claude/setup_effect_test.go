package claude

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func legacyClaudeTrustRequest(t *testing.T) domain.ActivationRequest {
	t.Helper()
	r := newClaudeMarketplaceRequest(t)
	r.Plan.PhysicalArtifactID = "demo-1123456789ab"
	r.Plan.ActivePath = filepath.Join(r.Plan.TargetRoot, r.Plan.PhysicalArtifactID)
	for _, name := range []string{"first", "second"} {
		claudeMarketplaceWrite(t, filepath.Join(r.Plan.ActivePath, "skills", name, "SKILL.md"), "---\nname: "+name+"\ndescription: inert fixture\n---\n")
	}
	envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: r.DeclaredName, Version: "1.0.0"}}
	if err := ProjectClaude(r.Plan.ActivePath, envelope, r.Plan, ""); err != nil {
		t.Fatal(err)
	}
	digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), r.Plan.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	r.Delivery.ActivePath, r.Delivery.ArtifactDigest = r.Plan.ActivePath, digest
	r.Delivery.NativeObjects = []domain.NativeObjectOwnership{{ObjectID: "package:claude:" + r.Plan.PhysicalArtifactID, Kind: "managed_package_directory", LogicalName: r.DeclaredName, Path: r.Plan.ActivePath, ManagedDigest: digest, ProtectionClass: "managed"}}
	r.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), r.Delivery.NativeObjects...)
	r.Replacing = true
	return r
}

func TestLegacyClaudeTrustMutationSurvivesFollowupObservation(t *testing.T) {
	for _, mode := range []string{"ready", "closed-failure", "unclosed-failure", "verify-only"} {
		t.Run(mode, func(t *testing.T) {
			r := legacyClaudeTrustRequest(t)
			r.VerifyOnly = mode == "verify-only"
			failure := errors.New("native list failed after trust")
			calls := 0
			runner := claudeMarketplaceRunnerFunc(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				if calls == 1 {
					body, _ := json.Marshal([]any{currentClaudeSuppression()})
					return legacyports.CommandResult{Stdout: body, NoChildStarted: true}, nil
				}
				if mode != "ready" {
					return legacyports.CommandResult{CleanupComplete: mode == "closed-failure"}, failure
				}
				body, _ := json.Marshal([]any{map[string]any{"id": r.DeclaredName + "@skills-dir", "scope": "project", "enabled": true, "installPath": r.Plan.ActivePath, "projectPath": r.Plan.TargetContext.ProjectRoot}})
				return legacyports.CommandResult{Stdout: body, NoChildStarted: true}, nil
			})
			outcome, err := New().Activate(t.Context(), clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}, r)
			wantEffect, wantActivation, wantCalls := domain.NativeEffectCommitted, domain.ActivationActive, 2
			switch mode {
			case "verify-only":
				wantEffect, wantActivation, wantCalls = domain.NativeEffectUnchanged, domain.ActivationPrepared, 1
			case "closed-failure", "unclosed-failure":
				wantActivation = domain.ActivationFailed
				if mode == "unclosed-failure" {
					wantEffect = domain.NativeEffectUncertain
				}
				if !errors.Is(err, failure) {
					t.Fatalf("native failure cause lost: %v", err)
				}
			}
			if outcome.NativeEffect != wantEffect || outcome.Activation != wantActivation || calls != wantCalls || !reflect.DeepEqual(outcome.NativeObjects, r.Delivery.NativeObjects) {
				t.Fatalf("trust mutation or ownership lost: %+v, %v, calls=%d", outcome, err, calls)
			}
			profile := filepath.Join(r.Client.ConfigRoot, ".claude.json")
			if mode == "verify-only" {
				if _, statErr := os.Stat(profile); !os.IsNotExist(statErr) || err != nil {
					t.Fatalf("read-only observation wrote trust: %v %v", statErr, err)
				}
			} else if trusted, readErr := claudeProjectTrustRecorded(clients.Env{NativeConfig: nativeconfig.New()}, r); !trusted || readErr != nil || mode == "ready" && err != nil {
				t.Fatalf("confirmed trust missing after observation: %v %v %v", trusted, readErr, err)
			}
		})
	}
}

type claudeUncertainSetupFiles struct {
	writes int
}

func (*claudeUncertainSetupFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	snapshot, err := nativeconfig.New().ReadExactFile(path)
	return snapshot.Body, snapshot.Mode, snapshot.Exists, err
}

func (files *claudeUncertainSetupFiles) WriteAtomic(path string, _ []byte, mode os.FileMode) error {
	files.writes++
	if err := os.WriteFile(path, []byte(`{"foreign":"concurrent native preference"}`), mode); err != nil {
		return err
	}
	return errors.New("native preference write outcome uncertain")
}

func (*claudeUncertainSetupFiles) RemoveNoFollow(string) error {
	return errors.New("foreign preference must not be removed")
}

func TestClaudeUncertainPreferenceWriteStopsNativeSetup(t *testing.T) {
	for _, surface := range []string{"marketplace-trust", "legacy-trust", "mcp-approval"} {
		t.Run(surface, func(t *testing.T) {
			r := newClaudeMarketplaceRequest(t)
			if surface == "legacy-trust" {
				r = legacyClaudeTrustRequest(t)
			}
			files := &claudeUncertainSetupFiles{}
			calls := 0
			runner := claudeMarketplaceRunnerFunc(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				rows := []any{map[string]any{"id": "demo@skills-dir", "enabled": true}}
				if surface == "legacy-trust" {
					rows = []any{currentClaudeSuppression()}
				}
				body, _ := json.Marshal(rows)
				return legacyports.CommandResult{Stdout: body, NoChildStarted: true}, nil
			})
			env := clients.Env{Runner: runner, NativeConfig: nativeconfig.NewWithFileIO(files)}
			initial := domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed,
				NativeEffect: domain.NativeEffectUnchanged, NativeObjects: append([]domain.NativeObjectOwnership(nil), r.Delivery.NativeObjects...)}
			var outcome domain.ActivationOutcome
			var err error
			path, wantCalls := filepath.Join(r.Client.ConfigRoot, ".claude.json"), 0
			switch surface {
			case "marketplace-trust":
				outcome, err = finishClaudeMarketplaceTrust(env, r, initial)
			case "legacy-trust":
				wantCalls = 1
				outcome, err = New().Activate(t.Context(), env, r)
			case "mcp-approval":
				wantCalls = 1
				r.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportProjected}}
				path = filepath.Join(r.Plan.TargetAnchor, "settings.local.json")
				outcome, err = finishClaudeMCPActivation(t.Context(), env, r, initial)
			}
			if err == nil || outcome.NativeEffect != domain.NativeEffectUncertain || outcome.Activation != domain.ActivationFailed || calls != wantCalls || files.writes != 1 || !reflect.DeepEqual(outcome.NativeObjects, initial.NativeObjects) {
				t.Fatalf("uncertain setup continued or lost effect: %+v %v, calls=%d writes=%d", outcome, err, calls, files.writes)
			}
			body, readErr := os.ReadFile(path)
			if readErr != nil || string(body) != `{"foreign":"concurrent native preference"}` {
				t.Fatalf("concurrent native preference was changed: %q %v", body, readErr)
			}
		})
	}
}
