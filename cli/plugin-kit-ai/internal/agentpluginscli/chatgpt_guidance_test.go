package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestContext7InteractivePreparationChoice(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "offered", true: "requested"}[explicit], func(t *testing.T) {
			f, d, a := context7GuidedFixture(t)
			f.app.Detector = staticDetector{clients: []domain.DetectedClient{fixtureClient(t, domain.ClientChatGPT)}}
			before, _ := json.Marshal(d.bundle)
			args := []string{"add", "context7-alias", "--plain"}
			if explicit {
				args = append(args, "--prepare")
			}
			out, _, err := f.executeInput(true, "\n", args...)
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
		})
	}
}

func emittedRegistrationCommand(t *testing.T, action string) string {
	t.Helper()
	_, command, ok := strings.Cut(action, "Rerun ")
	if !ok {
		t.Fatalf("missing rerun: %s", action)
	}
	command, _, ok = strings.Cut(command, ". Then")
	if !ok {
		t.Fatalf("missing command boundary: %s", action)
	}
	return strings.ReplaceAll(command, "<ID>", fixturePersonalAppID)
}

func TestContext7ResumeEmittedMixedCommandAndPreparedGuidance(t *testing.T) {
	f, d, a := context7GuidedFixture(t)
	before, _ := json.Marshal(d.bundle)
	out, _, err := f.execute(false, "add", "context7-alias", "--target", "kiro,chatgpt", "--prepare", "--format", "json", "--scope", "user", "--plain", "--security-details", "--no-color")
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
	if strings.Contains(out, fixturePersonalAppID) || strings.Contains(out, "copy its plugin_asdk_app_") || strings.Contains(out, "Rerun add") {
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
	for _, text := range []string{"ChatGPT desktop", "new chat", "Remote OAuth and tool calls have not been verified", "including --purge-data"} {
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
			out, _, err := f.executeInput(true, "\n", "add", "context7", "--prepare", "--plain")
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
			out, _, err := f.executeInput(true, "", "add", "context7", "--prepare", "--plain")
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
	prepared, _, err := f.execute(false, "add", "context7", "--target", "chatgpt", "--prepare", "--chatgpt-app-id", fixturePersonalAppID)
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
		if strings.Contains(out, path) || strings.Contains(out, fixturePersonalAppID) || strings.Contains(out, "Rerun add") || !strings.Contains(out, "Remote OAuth and tool calls have not been verified") {
			t.Fatalf("%s public guidance: %s", op, out)
		}
	}
}
