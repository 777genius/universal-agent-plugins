package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

type geminiInventoryEnvironmentProbe struct {
	command  legacyports.Command
	deadline time.Time
}

func (probe *geminiInventoryEnvironmentProbe) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	probe.command = command
	if deadline, ok := ctx.Deadline(); ok {
		probe.deadline = deadline
	}
	return legacyports.CommandResult{ExitCode: 0, CleanupComplete: true}, nil
}

func TestGeminiInventoryForwardsOnlyExplicitProfileContextEnvironment(t *testing.T) {
	request, _ := geminiProbeFixture(t)
	if err := os.MkdirAll(request.Client.ConfigRoot, 0700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(request.Client.ConfigRoot)
	expected := map[string]string{
		"PATH": filepath.Join(root, "bin"), "HOME": root, "USERPROFILE": root,
		"SYSTEMROOT": filepath.Join(root, "system"), "WINDIR": filepath.Join(root, "windows"),
		"TEMP": filepath.Join(root, "temp"), "TMP": filepath.Join(root, "tmp"),
		"TMPDIR": filepath.Join(root, "tmpdir"), "GEMINI_RESTRICTED_MODE": "false",
		"GEMINI_CLI_TRUST_WORKSPACE": "false", "LANG": "en_US.UTF-8", "LC_ALL": "en_US.UTF-8",
		"TZ":                              "UTC",
		"GEMINI_CLI_SYSTEM_SETTINGS_PATH": filepath.Join(request.Client.ConfigRoot, "inventory-system.json"),
		"GEMINI_CLI_SYSTEM_DEFAULTS_PATH": filepath.Join(request.Client.ConfigRoot, "inventory-defaults.json"),
	}
	for key, value := range expected {
		t.Setenv(key, value)
	}
	for _, key := range []string{"PRIVATE_CANARY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "CODEX_HOME", "GEMINI_CLI_UNKNOWN"} {
		t.Setenv(key, "must-not-be-forwarded")
	}
	t.Setenv("GEMINI_CLI_HOME", filepath.Join(root, "foreign-native-profile"))
	expected["GEMINI_CLI_HOME"] = root
	expected["NO_COLOR"], expected["TERM"] = "1", "dumb"
	probe := &geminiInventoryEnvironmentProbe{}
	before := time.Now()
	if _, err := runGeminiInventory(context.Background(), clients.Env{Runner: probe}, request, "skills", "list", "--all"); err != nil {
		t.Fatal(err)
	}
	after := time.Now()
	if probe.command.Dir != request.Plan.TargetContext.ProjectRoot || probe.command.StdoutLimitBytes != 1<<20 ||
		strings.Join(probe.command.Argv, "\x00") != strings.Join([]string{request.Client.ExecutablePath, "skills", "list", "--all"}, "\x00") ||
		probe.deadline.IsZero() || probe.deadline.Before(before.Add(20*time.Second)) ||
		probe.deadline.After(after.Add(20*time.Second)) {
		t.Fatal("inventory argv, directory, capture bound or finite deadline changed")
	}
	actual := map[string]string{}
	for _, entry := range probe.command.Env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || actual[parts[0]] != "" {
			t.Fatal("malformed or duplicate explicit environment")
		}
		actual[parts[0]] = parts[1]
	}
	if len(actual) != len(expected) {
		t.Fatal("unexpected ambient environment reached native inventory")
	}
	for key, value := range expected {
		if actual[key] != value {
			t.Fatalf("explicit environment binding changed for %s", key)
		}
	}
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
