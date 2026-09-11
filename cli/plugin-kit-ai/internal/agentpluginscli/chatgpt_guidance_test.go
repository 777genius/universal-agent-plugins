package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestContext7InteractivePreparationChoice(t *testing.T) {
	f, d, a := context7GuidedFixture(t)
	f.app.Detector = staticDetector{clients: []domain.DetectedClient{fixtureClient(t, domain.ClientChatGPT)}}
	before, _ := json.Marshal(d.bundle)
	out, _, err := f.executeInput(true, "\n", "add", "context7-alias", "--plain")
	if err == nil || !strings.Contains(err.Error(), "action_required") || !strings.Contains(out, "prepare personal marketplace") {
		t.Fatalf("choice/guidance: %s %v", out, err)
	}
	if a.verifiedCalls != 1 || a.directGitCalls != 0 || a.localCalls != 0 {
		t.Fatalf("acquisition: %+v", a)
	}
	state, _ := f.store.Load()
	after, _ := json.Marshal(d.bundle)
	if len(state.Installations) != 0 || string(before) != string(after) {
		t.Fatal("registration changed state or signed metadata")
	}
	command := emittedRegistrationCommand(t, err.Error())
	if !strings.Contains(command, "context7-alias") {
		t.Fatal(command)
	}
	out, _, err = f.execute(false, strings.Fields(command)...)
	if err != nil {
		t.Fatalf("resume %s: %s %v", command, out, err)
	}
	state, _ = f.store.Load()
	b := onlyCLIClient(state.Installations[0])
	if b.ClientID != "chatgpt" || b.InstallIntent != domain.InstallIntentPrepare || b.Activation != domain.ActivationPrepared {
		t.Fatalf("intent not retained: %+v", b)
	}
}

func emittedRegistrationCommand(t *testing.T, action string) string {
	t.Helper()
	_, command, ok := strings.Cut(action, "3. Run: npx universal-agent-plugins ")
	if !ok {
		t.Fatalf("missing resume command: %s", action)
	}
	command, _, ok = strings.Cut(command, "\n")
	if !ok {
		t.Fatalf("missing command boundary: %s", action)
	}
	return strings.ReplaceAll(command, "<ID>", fixturePersonalAppID)
}

func TestContext7ResumeEmittedMixedCommandAndPreparedGuidance(t *testing.T) {
	f, d, a := context7GuidedFixture(t)
	before, _ := json.Marshal(d.bundle)
	out, _, err := f.execute(false, "add", "context7-alias", "--target", "kiro,chatgpt", "--format", "json", "--scope", "user", "--plain", "--security-details", "--no-color")
	if err == nil {
		t.Fatal("expected registration")
	}
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		t.Fatal(err)
	}
	command := emittedRegistrationCommand(t, response.Data["next_action"].(string))
	for _, flag := range []string{"context7-alias", "kiro,chatgpt", "--format=json", "--scope=user", "--plain=true", "--security-details=true", "--no-color=true"} {
		if !strings.Contains(command, flag) {
			t.Fatalf("lost %s: %s", flag, command)
		}
	}
	out, _, err = f.execute(false, strings.Fields(command)...)
	if err != nil {
		t.Fatalf("emitted command: %s %v", out, err)
	}
	state, _ := f.store.Load()
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 || a.verifiedCalls != 2 {
		t.Fatalf("mixed resume: %+v calls=%d", state, a.verifiedCalls)
	}
	if strings.Contains(out, fixturePersonalAppID) || strings.Contains(out, "copy its asdk_app_") || strings.Contains(out, "Rerun add") {
		t.Fatalf("wrong stage or private ID: %s", out)
	}
	for _, b := range state.Installations[0].Clients {
		if b.InstallIntent != domain.InstallIntentPrepare || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationPackageValid {
			t.Fatalf("false completion: %+v", b)
		}
		if b.ClientID == "chatgpt" {
			if _, err := os.Stat(filepath.Join(b.TargetLocator, ".agents", "plugins", "marketplace.json")); err != nil {
				t.Fatal(err)
			}
			encodedPath, _ := json.Marshal(b.TargetLocator)
			if strings.Contains(out, strings.Trim(string(encodedPath), "\"")) {
				t.Fatalf("public JSON leaked marketplace path: %s", out)
			}
		}
	}
	for _, text := range []string{"personal marketplace", "new chat"} {
		if !strings.Contains(out, text) {
			t.Fatalf("missing %q: %s", text, out)
		}
	}
	after, _ := json.Marshal(d.bundle)
	if string(before) != string(after) {
		t.Fatal("changed signed metadata")
	}
}

func TestContext7InteractivePreparationPolicyGates(t *testing.T) {
	for _, kind := range []string{"signature", "source", "scope", "revoked"} {
		t.Run(kind, func(t *testing.T) {
			f, d, a := context7GuidedFixture(t)
			f.app.Detector = staticDetector{clients: []domain.DetectedClient{fixtureClient(t, domain.ClientChatGPT)}}
			switch kind {
			case "signature":
				d.err = os.ErrPermission
			case "source":
				d.bundle.Snapshot.Distributions[0].Releases[0].PackageSource.Repository = "other/context7"
			case "scope":
				d.bundle.Snapshot.Distributions[0].ReleasePolicies[0].Targets = append(d.bundle.Snapshot.Distributions[0].ReleasePolicies[0].Targets, domain.DirectoryTarget{Client: domain.ClientChatGPT, Scopes: []domain.InstallScope{domain.ScopeProject}, Delivery: "managed"})
			case "revoked":
				d.bundle.Snapshot.Distributions[0].ReleasePolicies[0].Status = domain.ReleaseRevoked
			}
			out, _, err := f.executeInput(true, "\n", "add", "context7", "--plain")
			if err == nil || strings.Contains(out, "prepare personal marketplace") || a.verifiedCalls != 0 {
				t.Fatalf("gate bypass: %s %v calls=%d", out, err, a.verifiedCalls)
			}
			state, _ := f.store.Load()
			if len(state.Installations) != 0 {
				t.Fatal("mutated")
			}
		})
	}
}

func TestContext7InteractiveSelectionOnlyAppliesSelectedIntent(t *testing.T) {
	for _, chooseChatGPT := range []bool{false, true} {
		t.Run(map[bool]string{false: "kiro-only", true: "mixed"}[chooseChatGPT], func(t *testing.T) {
			f, _, a := context7GuidedFixture(t)
			f.app.Detector = staticDetector{clients: []domain.DetectedClient{fixtureClient(t, domain.ClientChatGPT), fixtureClient(t, domain.ClientKiro)}}
			f.app.Prompter = fakePrompter{
				selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
					if len(request.Choices) != 2 {
						t.Fatalf("lost eligible peer: %+v", request)
					}
					ids := []domain.ClientID{domain.ClientKiro}
					if chooseChatGPT {
						ids = append(ids, domain.ClientChatGPT)
					}
					return prompt.TargetSelectionResult{IDs: ids}, nil
				},
				confirmFn: func() (prompt.ConfirmationResult, error) { return prompt.ConfirmationResult{Accepted: true}, nil },
			}
			out, _, err := f.executeInput(true, "", "add", "context7", "--plain")
			if chooseChatGPT {
				if err == nil || !strings.Contains(err.Error(), "action_required") {
					t.Fatalf("%s %v", out, err)
				}
				command := emittedRegistrationCommand(t, err.Error())
				out, _, err = f.execute(false, strings.Fields(command)...)
			}
			if err != nil {
				t.Fatalf("%s %v", out, err)
			}
			state, _ := f.store.Load()
			want := 1
			if chooseChatGPT {
				want = 2
			}
			if len(state.Installations) != 1 || len(state.Installations[0].Clients) != want || a.verifiedCalls != want {
				t.Fatalf("unexpected selected outcome: %+v calls=%d", state, a.verifiedCalls)
			}
			if !chooseChatGPT && state.Installations[0].LocalChatGPTMapping != nil {
				t.Fatal("unselected registration retained")
			}
		})
	}
}

func TestContext7PreparedGuidanceAcrossLifecycle(t *testing.T) {
	f, _, _ := context7GuidedFixture(t)
	prepared, _, err := f.execute(false, "add", "context7", "--target", "chatgpt", "--chatgpt-app-id", fixturePersonalAppID)
	if err != nil {
		t.Fatalf("%s %v", prepared, err)
	}
	state, _ := f.store.Load()
	path := onlyCLIClient(state.Installations[0]).TargetLocator
	if !strings.Contains(prepared, path) || strings.Contains(prepared, fixturePersonalAppID) || strings.Contains(prepared, "Rerun add") {
		t.Fatalf("initial human guidance: %s", prepared)
	}
	for _, op := range []string{"add", "update", "repair"} {
		out, _, err := f.execute(false, op, "context7", "--target", "chatgpt", "--format", "json")
		if err != nil {
			t.Fatalf("%s: %s %v", op, out, err)
		}
		if strings.Contains(out, path) || strings.Contains(out, fixturePersonalAppID) || !strings.Contains(out, "personal marketplace") {
			t.Fatalf("%s public guidance: %s", op, out)
		}
	}
}

func TestContext7HumanPathsAcrossSingleAndMixedLifecycle(t *testing.T) {
	for _, targets := range []string{"chatgpt", "kiro,chatgpt"} {
		t.Run(targets, func(t *testing.T) {
			f, _, _ := context7GuidedFixture(t)
			check := func(out string) {
				t.Helper()
				state, err := f.store.Load()
				if err != nil {
					t.Fatal(err)
				}
				path := ""
				for _, b := range state.Installations[0].Clients {
					if b.ClientID == "chatgpt" {
						path = b.TargetLocator
					}
				}
				if path == "" || !strings.Contains(out, path) || strings.Contains(out, fixturePersonalAppID) {
					t.Fatalf("missing usable path or leaked ID: %s", out)
				}
				if _, err := os.Stat(filepath.Join(path, ".agents", "plugins", "marketplace.json")); err != nil {
					t.Fatal(err)
				}
			}
			out, _, err := f.execute(false, "add", "context7", "--target", targets, "--chatgpt-app-id", fixturePersonalAppID)
			if err != nil {
				t.Fatalf("initial: %s %v", out, err)
			}
			check(out)
			for _, op := range []string{"add", "update", "repair"} {
				t.Run(op, func(t *testing.T) {
					out, _, err := f.execute(false, op, "context7", "--target", targets)
					if err != nil {
						t.Fatalf("%s: %s %v", op, out, err)
					}
					check(out)
					out, _, err = f.execute(false, op, "context7", "--target", targets, "--format", "json")
					if err != nil {
						t.Fatalf("json: %s %v", out, err)
					}
					state, _ := f.store.Load()
					for _, b := range state.Installations[0].Clients {
						encoded, _ := json.Marshal(b.TargetLocator)
						if b.TargetLocator != "" && strings.Contains(out, strings.Trim(string(encoded), "\"")) {
							t.Fatalf("private path in JSON: %s", out)
						}
					}
					if strings.Contains(out, fixturePersonalAppID) {
						t.Fatalf("private ID in JSON: %s", out)
					}
				})
			}

			state, _ := f.store.Load()
			for _, b := range state.Installations[0].Clients {
				if b.ClientID == "chatgpt" {
					if err := os.RemoveAll(b.TargetLocator); err != nil {
						t.Fatal(err)
					}
				}
			}
			out, _, err = f.execute(false, "repair", "context7", "--target", targets)
			if err != nil {
				t.Fatalf("repair missing marketplace: %s %v", out, err)
			}
			check(out)
			f.app.Detector = staticDetector{clients: []domain.DetectedClient{fixtureClient(t, domain.ClientChatGPT)}}
			out, _, err = f.executeInput(true, "\n", "add", "context7-alias", "--plain")
			if err != nil {
				t.Fatalf("retained interactive intent: %s %v", out, err)
			}
			check(out)
		})
	}
}

func TestContext7GuidedSelectionKeepsEligibleCodex(t *testing.T) {
	f, d, _ := context7GuidedFixture(t)
	snapshot := &d.bundle.Snapshot
	policy := &snapshot.Distributions[0].ReleasePolicies[0]
	policy.Targets = append(policy.Targets, domain.DirectoryTarget{Client: domain.ClientCodex, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementRequired})
	policy.CurrentEvidence = append(policy.CurrentEvidence, "materialized/codex")
	evidence := snapshot.Evidence[0]
	evidence.ID, evidence.Client = "materialized/codex", domain.ClientCodex
	snapshot.Evidence = append(snapshot.Evidence, evidence)
	f.app.Detector = staticDetector{clients: []domain.DetectedClient{fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientKiro), fixtureClient(t, domain.ClientChatGPT)}}
	// Prove this peer has ordinary signed eligibility before testing preparation.
	clients := detectedSupportedClients(f.app.Detector.(staticDetector).clients)
	compatible, err := f.app.compatibleDirectoryTargets(context.Background(), "context7", clients)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, client := range compatible {
		if client.ClientID == domain.ClientCodex {
			found = true
		}
	}
	if !found {
		t.Fatalf("fixture Codex is not ordinarily eligible: %+v", compatible)
	}
	f.app.Prompter = fakePrompter{
		selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
			if len(request.Choices) != 3 {
				t.Fatalf("prepare choices: %+v", request)
			}
			return prompt.TargetSelectionResult{IDs: []domain.ClientID{domain.ClientKiro, domain.ClientChatGPT}}, nil
		},
		confirmFn: func() (prompt.ConfirmationResult, error) { return prompt.ConfirmationResult{Accepted: true}, nil },
	}
	out, _, err := f.executeInput(true, "", "add", "context7", "--plain")
	if err == nil || !strings.Contains(err.Error(), "action_required") {
		t.Fatalf("registration: %s %v", out, err)
	}
	command := emittedRegistrationCommand(t, err.Error())
	if strings.Contains(command, "codex") || !strings.Contains(command, "chatgpt,kiro") {
		t.Fatal(command)
	}
	out, _, err = f.execute(false, strings.Fields(command)...)
	if err != nil {
		t.Fatalf("resume %s: %s %v", command, out, err)
	}
	state, _ := f.store.Load()
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("resume state: %+v", state)
	}
}

func TestContext7SingleLifecycleRenderPaths(t *testing.T) {
	for _, noChange := range []bool{false, true} {
		result := usecase.AddResult{
			Plan:       domain.DeliveryPlan{ClientID: domain.ClientChatGPT, InstallIntent: domain.InstallIntentPrepare, ActivePath: "/private/personal-marketplace", DeclaredName: "context7"},
			Activation: domain.ActivationOutcome{Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotRequired, Verification: domain.VerificationPackageValid},
			NoChange:   noChange, Mutated: !noChange,
		}
		for _, op := range []string{"add", "update", "repair"} {
			t.Run(fmt.Sprintf("%s/noChange=%t", op, noChange), func(t *testing.T) {
				for _, format := range []string{"human", "json"} {
					var out bytes.Buffer
					var err error
					switch op {
					case "add":
						err = renderAddResult(&out, format, domain.PackageEnvelope{}, result, false)
					case "update":
						err = renderUpdateResult(&out, format, domain.PackageEnvelope{}, result, false)
					case "repair":
						err = renderRepairResult(&out, format, domain.Installation{}, result, false)
					}
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(out.String(), result.Plan.ActivePath) != (format == "human") {
						t.Fatalf("%s path boundary: %s", format, out.String())
					}
				}
			})
		}
	}
}
