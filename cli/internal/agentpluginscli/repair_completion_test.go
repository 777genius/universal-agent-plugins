package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestRepairCompletionNeverClaimsSuccessAfterNativeError(t *testing.T) {
	nativeErr := errors.New("native transition was not verified")
	installation := domain.Installation{DeclaredName: "owned"}
	partial := usecase.AddResult{Mutated: true, Activation: domain.ActivationOutcome{
		Activation: domain.ActivationPrepared, NativeEffect: domain.NativeEffectUncertain,
	}}
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			var out bytes.Buffer
			err := renderRepairCompletion(&out, format, installation, partial, nativeErr)
			if !errors.Is(err, nativeErr) {
				t.Fatalf("native error was swallowed: %v", err)
			}
			if format == "human" {
				if out.Len() != 0 {
					t.Fatalf("failed repair printed success guidance: %q", out.String())
				}
				return
			}
			var envelope struct {
				Command string `json:"command"`
				Result  string `json:"result"`
				Data    struct {
					Result usecase.AddResult `json:"result"`
				} `json:"data"`
			}
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Command != "repair" || envelope.Result != "failure" || !envelope.Data.Result.Mutated ||
				envelope.Data.Result.Activation.Activation != domain.ActivationPrepared {
				t.Fatalf("failed repair mislabeled or lost its public partial outcome: %+v", envelope)
			}
			var public struct {
				Data struct {
					Result struct {
						Activation map[string]json.RawMessage `json:"activation"`
					} `json:"result"`
				} `json:"data"`
			}
			if err := json.Unmarshal(out.Bytes(), &public); err != nil {
				t.Fatal(err)
			}
			activation := public.Data.Result.Activation
			if len(activation) != 4 {
				t.Fatalf("repair exposed private activation evidence: %v", activation)
			}
			for _, key := range []string{"activation", "authentication", "policy", "verification"} {
				if _, present := activation[key]; !present {
					t.Fatalf("repair activation lost public field %q: %v", key, activation)
				}
			}
		})
	}
}

func TestRepairCompletionPreservesOperationAndWriterErrors(t *testing.T) {
	nativeErr := errors.New("native transition was not verified")
	writerErr := errors.New("repair output unavailable")
	err := renderRepairCompletion(targetNoticeWriter{err: writerErr}, "json", domain.Installation{}, usecase.AddResult{Mutated: true}, nativeErr)
	if !errors.Is(err, nativeErr) || !errors.Is(err, writerErr) {
		t.Fatalf("repair lost the operation or output error: %v", err)
	}
}

func TestRepairCompletionKeepsSuccessfulJSONContract(t *testing.T) {
	var out bytes.Buffer
	if err := renderRepairCompletion(&out, "json", domain.Installation{DeclaredName: "owned"}, usecase.AddResult{NoChange: true}, nil); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Command string `json:"command"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil || envelope.Command != "repair" || envelope.Result != "success" {
		t.Fatalf("successful repair envelope changed: %+v, %v", envelope, err)
	}
}
