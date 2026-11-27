package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func enterpriseDiagnosticFixture() error {
	return fmt.Errorf("transport https://user:DO_NOT_EMIT@example.test/private?token=DO_NOT_EMIT: %w",
		&domain.InstallPolicyError{Violations: []domain.InstallPolicyViolation{{
			Code: "components.denied", Layer: "ma\x1b[31mchine", PolicyPath: "/managed/policy.yaml",
			Subject: "https://user:DO_NOT_EMIT@example.test/?token=DO_NOT_EMIT", Message: "provider body DO_NOT_EMIT",
		}}})
}

func TestEnterprisePolicyFailuresAreTypedAndSafe(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code string
	}{
		{"denied", enterpriseDiagnosticFixture(), "POLICY_DENIED"},
		{"changed", fmt.Errorf("provider DO_NOT_EMIT: %w", domain.ErrInstallPolicyChanged), "POLICY_CHANGED"},
		{"invalid", &enterprisepolicy.ConfigError{Layer: "machine", Path: "/managed/policy.yaml", Code: "DO_NOT_EMIT"}, "POLICY_INVALID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if !ShouldWriteAgentFailure(test.err) {
				t.Fatal("early typed policy error did not opt into structured output")
			}
			if err := WriteAgentFailure(&out, test.err); err != nil {
				t.Fatal(err)
			}
			failure := classifyAgentFailure(test.err)
			if failure.Code != test.code || len(failure.NextActions) == 0 {
				t.Fatalf("missing policy classification or next action: %+v", failure)
			}
			text := out.String() + ErrorText([]string{"add", "anything", "--no-color"}, &out, test.err)
			if strings.Contains(text, "DO_NOT_EMIT") || strings.Contains(text, "\u001b") {
				t.Fatalf("untrusted policy diagnostics escaped: %s", text)
			}
			if test.name == "denied" && (len(failure.Violations) != 1 || failure.Violations[0].Layer != "machine" || failure.Violations[0].PolicyPath != "/managed/policy.yaml") {
				t.Fatalf("safe rule origin was lost: %+v", failure)
			}
		})
	}
}

func TestEnterprisePolicyFailureOutputRemainsOneJSONDocument(t *testing.T) {
	for _, early := range []bool{true, false} {
		t.Run(fmt.Sprint("early_", early), func(t *testing.T) {
			var out bytes.Buffer
			cause := enterpriseDiagnosticFixture()
			command := &cobra.Command{Use: "fixture", SilenceErrors: true, SilenceUsage: true,
				RunE: func(cmd *cobra.Command, _ []string) error {
					if early {
						return cause
					}
					return errors.Join(cause, WriteAgentFailure(cmd.OutOrStdout(), cause))
				}}
			command.SetOut(&out)
			command.SetArgs([]string{})
			configureEnterprisePolicyFailureOutput(command, &options{format: "json"})
			err := command.ExecuteContext(t.Context())
			if !errors.Is(err, cause) || ShouldWriteAgentFailure(err) != early {
				t.Fatalf("policy cause or output ownership changed: %v", err)
			}
			if ShouldWriteAgentFailure(err) {
				if err := WriteAgentFailure(&out, err); err != nil {
					t.Fatal(err)
				}
			}
			var document any
			decoder := json.NewDecoder(&out)
			if err := decoder.Decode(&document); err != nil {
				t.Fatal(err)
			}
			if err := decoder.Decode(&document); !errors.Is(err, io.EOF) {
				t.Fatalf("policy failure produced a second JSON document: %v", err)
			}
		})
	}
}

func TestEnterprisePolicyFailureOutputPreservesHumanWriter(t *testing.T) {
	var out bytes.Buffer
	cause := enterpriseDiagnosticFixture()
	command := &cobra.Command{Use: "fixture", SilenceErrors: true, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.OutOrStdout() != &out {
				t.Fatal("human writer identity was changed")
			}
			return cause
		}}
	command.SetOut(&out)
	command.SetArgs([]string{})
	configureEnterprisePolicyFailureOutput(command, &options{format: "human"})
	if err := command.ExecuteContext(t.Context()); err != cause || command.OutOrStdout() != &out {
		t.Fatalf("human policy error or writer identity changed: %v", err)
	}
}

type enterprisePolicyBrokenOutput struct{ writes int }

func (output *enterprisePolicyBrokenOutput) Write([]byte) (int, error) {
	output.writes++
	return 0, io.ErrClosedPipe
}

func TestEnterprisePolicyFailureDoesNotRetryBrokenJSON(t *testing.T) {
	output := &enterprisePolicyBrokenOutput{}
	cause := enterpriseDiagnosticFixture()
	command := &cobra.Command{Use: "fixture", SilenceErrors: true, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return errors.Join(cause, WriteAgentFailure(cmd.OutOrStdout(), cause))
		}}
	command.SetOut(output)
	command.SetArgs([]string{})
	configureEnterprisePolicyFailureOutput(command, &options{format: "json"})
	err := command.ExecuteContext(t.Context())
	if !errors.Is(err, cause) || !errors.Is(err, io.ErrClosedPipe) || ShouldWriteAgentFailure(err) || output.writes != 1 {
		t.Fatalf("failed policy result stream allowed another document: err=%v writes=%d", err, output.writes)
	}
}

func TestEnterprisePolicyManagementGuidancePreservesTypedCause(t *testing.T) {
	cause := enterpriseDiagnosticFixture()
	err := managementEnterprisePolicyError(cause)
	var refusal *management.Refusal
	var denied *domain.InstallPolicyError
	if !errors.As(err, &refusal) || !errors.As(err, &denied) || !errors.Is(err, cause) {
		t.Fatalf("safe management boundary lost its typed causes: %v", err)
	}
	if refusal.Code != "POLICY_DENIED" || strings.Contains(err.Error(), "DO_NOT_EMIT") || !strings.Contains(err.Error(), "/managed/policy.yaml") {
		t.Fatalf("management refusal lost safe rule guidance: %+v", refusal)
	}
	out := management.Outcome{State: management.Failed}
	if !managementEnterprisePolicyOutcome(&out, cause) || len(out.Diagnostics) != 1 || out.Diagnostics[0].Code != "POLICY_DENIED" || len(out.Result.NextActions) == 0 {
		t.Fatalf("management outcome lost policy diagnostics: %+v", out)
	}
	body, err := json.Marshal(out)
	if err != nil || strings.Contains(string(body), "DO_NOT_EMIT") {
		t.Fatalf("management outcome leaked private diagnostics: %v %s", err, body)
	}
	other := errors.New("unrelated failure")
	if managementEnterprisePolicyError(other) != other {
		t.Fatal("policy presentation rewrote an unrelated error")
	}
}
