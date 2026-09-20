package agentpluginscli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestAddMissingSourceGuidance(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		for _, name := range []string{"add", "install"} {
			for _, format := range []string{"human", "json"} {
				var output bytes.Buffer
				cmd := NewRoot(App{Terminal: terminal, Input: strings.NewReader(""), Output: &output, ErrorOutput: &output})
				cmd.SetArgs([]string{name, "--format", format})
				err := cmd.Execute()
				if err == nil {
					t.Fatal("missing source accepted")
				}
				if format == "json" {
					if err.Error() != "accepts 1 arg(s), received 0" {
						t.Fatalf("JSON error changed: %v", err)
					}
				} else {
					for _, want := range []string{"Provide a plugin name or source", "agentplugins add ./my-plugin", "agentplugins add --help"} {
						if !strings.Contains(err.Error(), want) {
							t.Fatalf("missing %q: %v", want, err)
						}
					}
				}
				if output.Len() != 0 {
					t.Fatalf("argument validation wrote output: %q", output.String())
				}
			}
		}
	}
}

func TestAddFailedLifecycleRecoveryGuidance(t *testing.T) {
	for _, phase := range []string{"activation_failed", "authentication_failed", "verification_failed"} {
		for _, mutated := range []bool{false, true} {
			result := usecase.AddResult{Mutated: mutated}
			result.Activation.LocalActions = []string{"Recover using /private/client/settings, then rerun add."}
			result.Activation.UserActions = []string{"Recover in the selected client, then rerun add."}
			switch phase {
			case "activation_failed":
				result.Activation.Activation = domain.ActivationFailed
			case "authentication_failed":
				result.Activation.Authentication = domain.AuthenticationFailed
			case "verification_failed":
				result.Activation.Verification = domain.VerificationFailed
			}
			var human, public bytes.Buffer
			if err := renderAddResult(&human, "human", domain.PackageEnvelope{}, result, false); err != nil {
				t.Fatal(err)
			}
			if human.String() != "Add: "+phase+"\nNext: "+result.Activation.LocalActions[0]+"\n" {
				t.Fatalf("human result: %q", human.String())
			}
			if err := renderAddResult(&public, "json", domain.PackageEnvelope{}, result, false); err != nil {
				t.Fatal(err)
			}
			assertOutputEnvelopeResult(t, public.Bytes(), "add", outputResultFailure)
			if strings.Contains(public.String(), "/private/") || !strings.Contains(public.String(), result.Activation.UserActions[0]) {
				t.Fatalf("public recovery contract: %s", public.String())
			}
		}
	}
}

func TestAddRecoveryDoesNotOverrideTransactionFailures(t *testing.T) {
	for _, phase := range []usecase.GroupTargetPhase{usecase.GroupTargetManagedRolledBack, usecase.GroupTargetManagedUnknown, usecase.GroupTargetExternalFailed, usecase.GroupTargetExternalPartial} {
		result := usecase.AddResult{Mutated: true, GroupPhase: phase, Activation: domain.ActivationOutcome{Activation: domain.ActivationFailed, LocalActions: []string{"Enable the prepared plugin."}}}
		var output bytes.Buffer
		if err := renderAddResult(&output, "human", domain.PackageEnvelope{}, result, false); err != nil {
			t.Fatal(err)
		}
		if output.String() != "Add: "+string(phase)+"\n" {
			t.Fatalf("transaction recovery implied usable install: %q", output.String())
		}
	}
}

type recoveryFailWriter struct{ writes int }

func (w *recoveryFailWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}
func TestAddRecoveryPropagatesWriterFailure(t *testing.T) {
	result := usecase.AddResult{Activation: domain.ActivationOutcome{Activation: domain.ActivationFailed}}
	err := renderAddResult(&recoveryFailWriter{}, "human", domain.PackageEnvelope{}, result, false)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("writer error lost: %v", err)
	}
}
