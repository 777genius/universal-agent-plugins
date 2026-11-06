package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestAgentMachineFailuresAreSanitizedAndTyped(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code string
	}{
		{"recovery", installer.ErrRecoveryRequired, "RECOVERY_REQUIRED"},
		{"security", installer.ErrAssessmentRejected, "SECURITY_REVIEW_BLOCKED"},
		{"target", installer.ErrTargetFactsUnavailable, "TARGET_UNAVAILABLE"},
		{"timeout", context.DeadlineExceeded, "TIMEOUT"},
		{"provider", errors.New("provider token=DO_NOT_EMIT"), "COMMAND_FAILED"},
		{"selection", skillsSelectionError{Code: "SKILL_AMBIGUOUS", Message: "Choose exact --path"}, "SKILL_AMBIGUOUS"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := WriteAgentFailure(&out, test.err); err != nil {
				t.Fatal(err)
			}
			var response struct {
				SchemaVersion int    `json:"schema_version"`
				Result        string `json:"result"`
				Data          struct {
					Error AgentFailure `json:"error"`
				} `json:"data"`
			}
			if err := json.Unmarshal(out.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.SchemaVersion != 1 || response.Result != "failure" || response.Data.Error.Code != test.code {
				t.Fatalf("unexpected response: %s", out.String())
			}
			if strings.Contains(out.String(), "DO_NOT_EMIT") {
				t.Fatal("provider diagnostics escaped sanitized machine output")
			}
		})
	}
}

// Native command contracts intentionally keep empty stdout on repeated
// removal and other pre-existing runtime refusals; typed usage is new.
func TestMachineFailureOutputIsOptInForNewTypedErrors(t *testing.T) {
	for _, test := range []struct {
		err  error
		want bool
	}{
		{CLIUsageError{Code: "TARGET_REQUIRED", Message: "select --target"}, true},
		{skillsSelectionError{Code: "SKILL_NOT_FOUND", Message: "choose an exact Skill"}, true},
		{errors.New("installation \"native-proof\" was not found"), false},
		{errors.New("plugin is not installed for target \"codex\" in user scope; no target was changed"), false},
		{installer.ErrRecoveryRequired, false},
	} {
		if got := ShouldWriteAgentFailure(test.err); got != test.want {
			t.Fatalf("%v: eligibility %v, want %v", test.err, got, test.want)
		}
	}
}

func TestDerivedAgentReadinessDoesNotPromoteAttestations(t *testing.T) {
	ready := usecase.AddResult{Mutated: true, Activation: domain.ActivationOutcome{
		Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired,
		Verification: domain.VerificationInstalled,
	}}
	if state, next := deriveAddReadiness(ready, false); state != "ready" || len(next) != 0 {
		t.Fatalf("verified record: %s %+v", state, next)
	}
	ready.Activation.ActivationAttested = true
	if state, next := deriveAddReadiness(ready, false); state != "needs_action" || len(next) != 1 || next[0].Code != "verify_native" {
		t.Fatalf("attested result must remain distinct: %s %+v", state, next)
	}
	if state, _ := deriveAddReadiness(ready, true); state != "unknown" {
		t.Fatalf("dry-run claimed native completion: %s", state)
	}
	ready.Activation.Activation = domain.ActivationFailed
	if state, _ := deriveAddReadiness(ready, false); state != "failed" {
		t.Fatalf("failed activation claimed readiness: %s", state)
	}
}

func TestDependencyAndRawSkillReadinessDoNotAssumeNativeUse(t *testing.T) {
	for _, test := range []struct{ facts []string; want string }{
		{nil, "ready"},
		{[]string{"activation_prepared"}, "needs_action"},
		{[]string{"auth_pending"}, "needs_action"},
		{[]string{"pending_recovery"}, "needs_action"},
		{[]string{"auth_failed"}, "failed"},
		{[]string{"store_mismatch"}, "failed"},
		{[]string{"manifest_missing"}, "unknown"},
	} {
		if got := deriveDependencyReadiness(test.facts); got != test.want {
			t.Fatalf("facts %v: got %s, want %s", test.facts, got, test.want)
		}
	}
	if got, next := deriveSkillsReadiness("completed"); got != "needs_action" || len(next) == 0 {
		t.Fatalf("raw Skill install is not runtime discovery: %s %+v", got, next)
	}
	if got, _ := deriveSkillsReadiness("partial"); got != "failed" {
		t.Fatalf("partial raw Skills installation: %s", got)
	}
}

func TestExplicitScopeReasonIsStable(t *testing.T) {
	root := NewRoot(App{})
	if got := scopeSelectionReason(root, &options{scope: "user"}); got != "default_user" {
		t.Fatal(got)
	}
	if got := scopeSelectionReason(root, &options{scope: "project", scopeInferred: true}); got != "argumentless_project_manifest" {
		t.Fatal(got)
	}
	if got := scopeSelectionReason(root, &options{scope: "project", projectRoot: "/project"}); got != "explicit" {
		t.Fatal(got)
	}
}
