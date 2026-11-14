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
				Result string `json:"result"`
				Data struct {
					Result usecase.AddResult `json:"result"`
				} `json:"data"`
			}
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Command != "repair" || envelope.Result != "failure" || !envelope.Data.Result.Mutated ||
				envelope.Data.Result.Activation.NativeEffect != domain.NativeEffectUncertain {
				t.Fatalf("failed repair mislabeled or lost its partial native effect: %+v", envelope)
			}
		})
	}
}

func TestRepairCompletionKeepsSuccessfulJSONContract(t *testing.T) {
	var out bytes.Buffer
	if err := renderRepairCompletion(&out, "json", domain.Installation{DeclaredName: "owned"}, usecase.AddResult{NoChange: true}, nil); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Command string `json:"command"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil || envelope.Command != "repair" || envelope.Result != "success" {
		t.Fatalf("successful repair envelope changed: %+v, %v", envelope, err)
	}
}
