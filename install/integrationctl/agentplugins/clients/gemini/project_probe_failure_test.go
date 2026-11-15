package gemini

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

type geminiFailingProbe struct{ result legacyports.CommandResult }

func (r geminiFailingProbe) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return r.result, context.DeadlineExceeded
}

func geminiProbeFixture(t *testing.T) (domain.ActivationRequest, nativeconfig.Kernel) {
	t.Helper()
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(project, ".gemini")
	skill := filepath.Join(root, "skills", "audit")
	if err := os.MkdirAll(skill, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: audit\ndescription: audit marker\n---\nLocal marker.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := shared.DigestSkillDirectory(skill)
	if err != nil {
		t.Fatal(err)
	}
	object := domain.NativeObjectOwnership{ObjectID: GeminiSkillObjectKind + ":audit", Kind: GeminiSkillObjectKind, LogicalName: "audit", Path: skill, ManagedDigest: digest, ProtectionClass: "managed"}
	request := domain.ActivationRequest{VerifyOnly: true, Client: domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: filepath.Join(user, ".gemini"), ExecutablePath: filepath.Join(user, "gemini")}, Plan: domain.DeliveryPlan{Scope: domain.ScopeProject, NativeRegistryRoot: root, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project}, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "audit", Support: domain.SupportNative}}}, Delivery: domain.StagedDelivery{NativeObjects: []domain.NativeObjectOwnership{object}}}
	return request, nativeconfig.New()
}

func TestGeminiCompletionPropagatesUnclosedProbeMetadata(t *testing.T) {
	for _, test := range []struct {
		name   string
		result legacyports.CommandResult
		unsafe bool
	}{{"unknown", legacyports.CommandResult{}, true}, {"closed", legacyports.CommandResult{CleanupComplete: true}, false}, {"not-started", legacyports.CommandResult{NoChildStarted: true}, false}} {
		t.Run(test.name, func(t *testing.T) {
			request, kernel := geminiProbeFixture(t)
			outcome, err := completeGeminiProject(context.Background(), clients.Env{NativeConfig: kernel, Runner: geminiFailingProbe{test.result}}, request, domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectUnchanged})
			if domain.IsUnclosedClientProbe(err) != test.unsafe {
				t.Fatalf("lost cleanup metadata: %v", err)
			}
			if test.unsafe && outcome.NativeEffect != domain.NativeEffectUncertain {
				t.Fatal("unclosed process did not retain uncertain effect")
			}
			if !test.unsafe && err != nil {
				t.Fatalf("closed unavailable probe should remain prepared: %v", err)
			}
		})
	}
	request, kernel := geminiProbeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := completeGeminiProject(ctx, clients.Env{NativeConfig: kernel}, request, domain.ActivationOutcome{Verification: domain.VerificationInstalled})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("parent cancellation lost: %v", err)
	}
}

type geminiTrustProbe struct {
	calls  int
	result legacyports.CommandResult
}

func (probe *geminiTrustProbe) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	probe.calls++
	return probe.result, context.DeadlineExceeded
}

func geminiConfirmedTrustFixture(t *testing.T) (domain.ActivationRequest, nativeconfig.Kernel) {
	t.Helper()
	request, kernel := geminiProbeFixture(t)
	request.VerifyOnly = false
	if err := os.MkdirAll(request.Client.ConfigRoot, 0700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(filepath.Dir(request.Client.ConfigRoot), "foreign-project")
	body, err := json.Marshal(map[string]string{foreign: "DO_NOT_TRUST"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(request.Client.ConfigRoot, "trustedFolders.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	return request, kernel
}

func TestGeminiTrustMutationSurvivesFailedDiscovery(t *testing.T) {
	t.Setenv("GEMINI_RESTRICTED_MODE", "")
	t.Setenv("GEMINI_CLI_TRUST_WORKSPACE", "")
	for _, test := range []struct {
		name    string
		closed  bool
		initial domain.NativeEffectState
		want    domain.NativeEffectState
	}{
		{"closed", true, domain.NativeEffectUnchanged, domain.NativeEffectCommitted},
		{"unclosed", false, domain.NativeEffectUnchanged, domain.NativeEffectUncertain},
		{"prior-uncertain", true, domain.NativeEffectUncertain, domain.NativeEffectUncertain},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, kernel := geminiConfirmedTrustFixture(t)
			probe := &geminiTrustProbe{result: legacyports.CommandResult{CleanupComplete: test.closed}}
			outcome, err := completeGeminiProject(t.Context(), clients.Env{NativeConfig: kernel, Runner: probe}, request, domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, NativeEffect: test.initial})
			if outcome.NativeEffect != test.want || outcome.Activation != domain.ActivationPrepared || probe.calls != 1 || domain.IsUnclosedClientProbe(err) != !test.closed || test.closed && err != nil {
				t.Fatalf("trust effect lost after native failure: %+v, %v, calls=%d", outcome, err, probe.calls)
			}
			body, readErr := os.ReadFile(filepath.Join(request.Client.ConfigRoot, "trustedFolders.json"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			_, mapping, decodeErr := geminiTrustDocument(body)
			foreign := filepath.Join(filepath.Dir(request.Client.ConfigRoot), "foreign-project")
			if decodeErr != nil || len(mapping) != 2 || mapping[request.Plan.TargetContext.ProjectRoot] != "TRUST_FOLDER" || mapping[foreign] != "DO_NOT_TRUST" {
				t.Fatalf("selected or foreign trust changed unexpectedly: %s, %v", body, decodeErr)
			}
		})
	}
}

type geminiUncertainTrustFiles struct {
	writes int
	cause  error
}

func (*geminiUncertainTrustFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	snapshot, err := nativeconfig.New().ReadExactFile(path)
	return snapshot.Body, snapshot.Mode, snapshot.Exists, err
}

func (files *geminiUncertainTrustFiles) WriteAtomic(path string, _ []byte, mode os.FileMode) error {
	files.writes++
	return errors.Join(os.WriteFile(path, []byte(`{"foreign":"late native preference"}`), mode), files.cause)
}

func (*geminiUncertainTrustFiles) RemoveNoFollow(string) error {
	return errors.New("foreign trust preference must not be removed")
}

func TestGeminiUncertainTrustWriteStopsNativeDiscovery(t *testing.T) {
	t.Setenv("GEMINI_RESTRICTED_MODE", "")
	t.Setenv("GEMINI_CLI_TRUST_WORKSPACE", "")
	request, _ := geminiConfirmedTrustFixture(t)
	files := &geminiUncertainTrustFiles{cause: errors.New("native trust write outcome uncertain")}
	probe := &geminiTrustProbe{result: legacyports.CommandResult{CleanupComplete: true}}
	outcome, err := completeGeminiProject(t.Context(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files), Runner: probe}, request, domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectCommitted})
	var observed *shared.NativeEffectError
	if !errors.As(err, &observed) || observed.Effect != domain.NativeEffectUncertain || !errors.Is(err, files.cause) || outcome.NativeEffect != domain.NativeEffectUncertain || probe.calls != 0 || files.writes != 1 {
		t.Fatalf("uncertain trust setup continued or lost recovery state: %+v, %v, calls=%d writes=%d", outcome, err, probe.calls, files.writes)
	}
	body, readErr := os.ReadFile(filepath.Join(request.Client.ConfigRoot, "trustedFolders.json"))
	if readErr != nil || string(body) != `{"foreign":"late native preference"}` {
		t.Fatalf("late foreign trust preference was replaced: %q, %v", body, readErr)
	}
}
