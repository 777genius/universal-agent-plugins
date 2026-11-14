//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestGeminiCycleCompleteTimestampGrammarIsASCIIAndExact(t *testing.T) {
	for _, value := range []string{"2026-10-27T00:00:01Z", "2026-10-27T00:00:01.1Z", "2026-10-27T00:00:01.123456789Z", "2026-10-27T00:00:01+00:00", "2026-10-27T00:00:01-05:30", "2026-10-27T00:00:01+23:59", "0001-01-01T00:00:00Z"} {
		if _, err := geminiCompleteTime(value); err != nil {
			t.Fatal("exact supported timestamp refused", value, err)
		}
	}
	for _, value := range []string{"2026-10-27T00:00:01,1Z", "2026-10-27T00:00:01.0000000009Z", "2026-10-27T00:00:01.٠Z", "2026-10-27T00:00:01.０Z", "2026-10-27T00:00:01+24:00", "2026-10-27T00:00:01+23:60", "2026-10-27T00:00:01+00:60", "2026-10-27T00:00:01+00:99", "0000-01-01T00:00:00Z"} {
		if _, err := geminiCompleteTime(value); err == nil {
			t.Fatal("normalized/truncated timestamp admitted", value)
		}
	}
}

func geminiCompleteContractTimeVariant(t *testing.T, value string, comma bool) string {
	t.Helper()
	base := strings.TrimSuffix(value, "Z")
	clock, fraction, _ := strings.Cut(base, ".")
	if comma {
		if fraction == "" {
			fraction = "000000000"
		}
		base = clock + "," + fraction
	} else {
		base = clock + "." + fraction + strings.Repeat("0", 9-len(fraction)) + "9"
	}
	variant := base + "Z"
	old, err := time.Parse(time.RFC3339Nano, value)
	now, variantErr := time.Parse(time.RFC3339Nano, variant)
	if err != nil || variantErr != nil || !old.Equal(now) {
		t.Fatal("control did not reproduce preserved timestamp normalization", value, variant, err, variantErr)
	}
	return variant
}

func TestGeminiCycleCompleteCoherentNativeProbeAndCommandTimestampVariantsRefuse(t *testing.T) {
	baseline, source := geminiCompleteSyntheticFiles(t)
	for _, kind := range []string{"closure", "command", "native", "probe"} {
		for _, comma := range []bool{false, true} {
			files := geminiCompleteContractCopy(baseline)
			switch kind {
			case "closure":
				row := geminiCompleteContractDecode(t, files["runtime-attempt.json"])
				closure := row["cycle_completion"].(map[string]any)
				closure["started_at"] = geminiCompleteContractTimeVariant(t, closure["started_at"].(string), comma)
				files["runtime-attempt.json"] = geminiCompleteContractEncode(t, row)
			case "command":
				geminiCompleteContractRows(t, files, "cycle-native-commands.jsonl", func(row map[string]any) {
					if row["stage"] == "listing:baseline-a" {
						value := row["value"].(map[string]any)
						value["started_at"] = geminiCompleteContractTimeVariant(t, value["started_at"].(string), comma)
					}
				})
			case "native":
				geminiCompleteContractObservation(t, files, "native:installed-a", "stdout", func(body []byte) []byte {
					var changed []byte
					for index, line := range bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'}) {
						row := geminiCompleteContractDecode(t, line)
						if index == 0 {
							row["timestamp"] = geminiCompleteContractTimeVariant(t, row["timestamp"].(string), comma)
						}
						changed = append(append(changed, geminiCompleteContractEncode(t, row)...), '\n')
					}
					return changed
				})
			case "probe":
				geminiCompleteContractRows(t, files, "cycle-probe-events.jsonl", func(row map[string]any) {
					if row["stage"] != "installed-a" {
						return
					}
					value := row["value"].(map[string]any)
					body := geminiCompleteContractRaw(t, value["stream"])
					var changed []byte
					for index, line := range bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'}) {
						event := geminiCompleteContractDecode(t, line)
						if index == 0 {
							event["timestamp"] = geminiCompleteContractTimeVariant(t, event["timestamp"].(string), comma)
						}
						changed = append(append(changed, geminiCompleteContractEncode(t, event)...), '\n')
					}
					value["stream"] = geminiCompleteContractReceipt(changed)
				})
			}
			if geminiCycleCompleteFiles(files, source) == nil {
				t.Fatal("coherent timestamp variant admitted", kind, comma)
			}
		}
	}
}
