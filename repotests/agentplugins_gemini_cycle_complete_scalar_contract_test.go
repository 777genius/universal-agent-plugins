//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"testing"
)

func TestGeminiCycleCompleteNestedUnicodeScalarsAreLossless(t *testing.T) {
	for _, control := range []struct {
		text  string
		valid bool
	}{
		{`{"opaque":"\ud800"}`, false},
		{`{"opaque":"\udc00"}`, false},
		{`{"\ud800":"ordinary"}`, false},
		{`{"opaque":"\ud83d\ude00"}`, true},
		{`{"opaque":"�😀"}`, true},
		{`ordinary literal \ud800`, true},
		{`{"incomplete":"\ud800`, true},
	} {
		for _, shape := range []string{"value", "key", "array", "jsonl", "recursive"} {
			var value any = map[string]any{"text": control.text}
			switch shape {
			case "key":
				value = map[string]any{control.text: "ordinary"}
			case "array":
				value = []any{control.text}
			case "recursive":
				for count := 0; count < 3; count++ {
					value = map[string]any{"text": string(geminiCompleteContractEncode(t, value))}
				}
			}
			body := geminiCompleteContractEncode(t, value)
			if shape == "jsonl" {
				body = append(append([]byte("{\"ordinary\":\"value\"}\n"), body...), '\n')
			}
			if geminiCompleteScalars(body) != control.valid {
				t.Fatal("nested scalar meaning or ordinary text changed", shape, control.text)
			}
		}
	}
	body := []byte(`{"opaque":"ordinary"}`)
	for _, budget := range []geminiCyclePrivacyBudget{{text: len(body) - 1, nodes: 16384}, {text: 4 * len(body), nodes: 0}} {
		if budget.completeScalarJSON(body, 0) {
			t.Fatal("scalar traversal bypassed a shared resource limit")
		}
	}
	budget := geminiCyclePrivacyBudget{text: 4 * len(body), nodes: 16384}
	if budget.completeScalarJSON(body, 65) {
		t.Fatal("scalar traversal bypassed the original depth limit")
	}
}

func TestGeminiCycleCompleteCoherentNestedProviderUnicodeRefuses(t *testing.T) {
	baseline, source := geminiCompleteSyntheticFiles(t)
	for _, control := range []struct {
		name, text string
		valid      bool
	}{
		{"high", `{"opaque":"\ud800"}`, false},
		{"low", `{"opaque":"\udc00"}`, false},
		{"key", `{"\ud800":"ordinary"}`, false},
		{"jsonl", "ordinary prefix\n{\"opaque\":\"\\ud800\"}\n", false},
		{"pair", `{"opaque":"\ud83d\ude00"}`, true},
		{"literal", `ordinary literal \ud800`, true},
	} {
		t.Run(control.name, func(t *testing.T) {
			files := geminiCompleteContractCopy(baseline)
			geminiCompleteContractProvider(t, files, "baseline-a", json.Number("0"), func(row map[string]any) {
				contents := row["contents"].([]any)
				row["contents"] = append([]any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": control.text}}}}, contents...)
				if geminiCyclePrivateCredential(geminiCompleteContractEncode(t, row)) {
					t.Fatal("control did not isolate preserved lossy-decoder gap")
				}
			})
			if len(files) != 57 || (geminiCycleCompleteFiles(files, source) == nil) != control.valid {
				t.Fatal("coherent nested Unicode success boundary differs")
			}
		})
	}
}
