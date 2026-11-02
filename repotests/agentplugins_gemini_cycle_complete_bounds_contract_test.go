//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestGeminiCycleCompleteMetadataHasFiniteCanonicalNamesAndArguments(t *testing.T) {
	files, _ := geminiCompleteSyntheticFiles(t)
	for _, kind := range []string{"valid", "dot", "dotdot", "nul", "too-many", "too-large", "node-alias"} {
		t.Run(kind, func(t *testing.T) {
			result := geminiCompleteContractDecode(t, files["runtime-attempt.json"])
			paths, err := geminiCompleteLayout(result["cycle_layout"])
			if err != nil {
				t.Fatal(err)
			}
			identity := result["acquisition"].(map[string]any)
			archive := identity["acquisition"].(map[string]any)
			members := archive["members"].([]any)
			if kind == "dot" || kind == "dotdot" || kind == "nul" {
				name := map[string]string{"dot": ".", "dotdot": "..", "nul": "inert\x00name"}[kind]
				members = append(members, map[string]any{"name": name, "kind": "directory", "bytes": 0, "sha256": ""})
			}
			if kind == "too-many" {
				for len(members) < 513 {
					members = append(members, map[string]any{"name": fmt.Sprintf("inert-directory-%d", len(members)), "kind": "directory", "bytes": 0, "sha256": ""})
				}
			}
			if kind == "too-large" {
				members[0].(map[string]any)["bytes"] = int64(1<<30) + 1
			}
			if kind == "node-alias" {
				members = append(members, map[string]any{"name": "node", "kind": "file", "bytes": 126595440, "sha256": "7fde7b8afa198da66257f42ee2001d874c7355631e6d1579a5fb5ef1f246df4c"})
			}
			archive["members"] = members
			result = geminiCompleteContractDecode(t, geminiCompleteContractEncode(t, result))
			if err := geminiCompletePins(result, paths); (err == nil) != (kind == "valid") {
				t.Fatal("finite pin metadata boundary differs", kind, err)
			}
		})
	}
	for _, arg := range []string{"/inert/client", "", strings.Repeat("x", 4097), "inert\x00argument"} {
		argv := []string{arg}
		row := geminiCompleteContractDecode(t, geminiCompleteContractEncode(t, map[string]any{"argv": argv, "cwd": "/inert/project", "exit": 0, "error": false, "context_error": false, "elapsed_ms": 1, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"}))
		if err := geminiCompleteReceipt(row, argv, "/inert/project", 30000, 0); (err == nil) != (arg == "/inert/client") {
			t.Fatal("held command text bounds differ", err)
		}
	}
	result := geminiCompleteContractDecode(t, files["runtime-attempt.json"])
	paths := result["cycle_layout"].(map[string]any)
	paths["root"] = "/" + strings.Repeat("r", 4096)
	for key, suffix := range map[string]string{"home": "home", "a": "project", "b": "project-B", "outside": "outside"} {
		paths[key] = paths["root"].(string) + "/" + suffix
	}
	if _, err := geminiCompleteLayout(paths); err == nil {
		t.Fatal("coherent oversized canonical layout admitted")
	}
}

func TestGeminiCycleCompleteIntegerFieldsRejectFloatFormsButPreserveTemperature(t *testing.T) {
	baseline, source := geminiCompleteSyntheticFiles(t)
	for _, kind := range []string{"elapsed", "public-elapsed", "probe-bytes", "generation-int", "temperature"} {
		t.Run(kind, func(t *testing.T) {
			files := geminiCompleteContractCopy(baseline)
			switch kind {
			case "elapsed":
				result := geminiCompleteContractDecode(t, files["runtime-attempt.json"])
				result["elapsed_ms"] = json.Number("31000.0")
				files["runtime-attempt.json"] = geminiCompleteContractEncode(t, result)
			case "public-elapsed":
				row := geminiCompleteContractDecode(t, files["add-a.command.json"])
				row["elapsed_ms"] = json.Number("1e0")
				files["add-a.command.json"] = geminiCompleteContractEncode(t, row)
			case "probe-bytes":
				geminiCompleteContractRows(t, files, "cycle-proof.jsonl", func(row map[string]any) {
					if strings.HasPrefix(row["stage"].(string), "source-") {
						entry := row["value"].(map[string]any)["files"].(map[string]any)["bin/probe"].(map[string]any)
						entry["bytes"] = json.Number(entry["bytes"].(json.Number).String() + ".0")
					}
				})
			case "generation-int", "temperature":
				for _, index := range []json.Number{"0", "1"} {
					geminiCompleteContractProvider(t, files, "installed-a", index, func(row map[string]any) {
						if kind == "generation-int" {
							row["generationConfig"] = map[string]any{"topK": json.Number("1.0")}
						} else {
							row["generationConfig"] = map[string]any{"temperature": json.Number("0.5")}
						}
					})
				}
			}
			err := geminiCycleCompleteFiles(files, source)
			if (err == nil) != (kind == "temperature") {
				t.Fatal("integer/finite-float success boundary differs", kind, err)
			}
		})
	}
}

func TestGeminiCycleCompleteUnicodeEscapesAreLossless(t *testing.T) {
	for _, item := range []struct {
		body  string
		valid bool
	}{
		{`{"text":"\ud800"}`, false}, {`{"text":"\udc00"}`, false}, {`{"\ud800":"value"}`, false}, {`{"text":"\ud800\u0061"}`, false},
		{`{"text":"\ud83d\ude00"}`, true}, {`{"text":"\\ud800"}`, true}, {`{"text":"escaped quote: \" and slash: \\"}`, true}, {`{"text":"�😀"}`, true},
	} {
		_, err := geminiCompleteJSON([]byte(item.body))
		if (err == nil) != item.valid {
			t.Fatal("lossless JSON Unicode boundary differs", item.body, err)
		}
	}
}

func TestGeminiCycleCompleteCoherentUnpairedSurrogateSessionRefuses(t *testing.T) {
	f := geminiCompleteContractFixtureNew(t)
	f.publicRecords()
	f.observations()
	f.encodeRows()
	f.metadata()
	facts := geminiCompleteContractProviderFacts(t, f.files, "restarted-a")
	quoted := geminiCompleteContractEncode(t, facts["session"])
	unpaired := append(append([]byte{}, quoted[:len(quoted)-1]...), []byte(`\ud800"`)...)
	encoded := bytes.ReplaceAll(geminiCompleteContractEncode(t, facts), quoted, unpaired)
	phase := f.phase("restarted-a", 4)
	legacy, err := geminiCycleFacts(string(encoded), phase.r, f.paths["home"])
	if err != nil || !strings.HasSuffix(legacy.Session, "�") {
		t.Fatal("control did not reproduce preserved lossy decoder", err)
	}
	geminiCompleteContractProvider(t, f.files, "restarted-a", "1", func(row map[string]any) {
		contents := row["contents"].([]any)
		response := contents[len(contents)-1].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)["response"].(map[string]any)
		response["output"] = "<untrusted_context>\n" + string(encoded) + "\n</untrusted_context>"
	})
	geminiCompleteContractObservation(t, f.files, "native:restarted-a", "stdout", func(body []byte) []byte {
		var changed []byte
		for _, line := range bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'}) {
			row := geminiCompleteContractDecode(t, line)
			if row["type"] == "tool_result" {
				row["output"] = string(encoded)
			}
			changed = append(append(changed, geminiCompleteContractEncode(t, row)...), '\n')
		}
		return changed
	})
	geminiCompleteContractRows(t, f.files, "cycle-probe-events.jsonl", func(row map[string]any) {
		if row["stage"] == "restarted-a" {
			value := row["value"].(map[string]any)
			body := geminiCompleteContractRaw(t, value["stream"])
			value["stream"] = geminiCompleteContractReceipt(bytes.ReplaceAll(body, quoted, unpaired))
		}
	})
	source := geminiCompleteContractDecode(t, geminiCompleteContractEncode(t, f.contractSource()))
	if len(f.files) != 57 || geminiCycleCompleteFiles(f.files, source) == nil {
		t.Fatal("coherently joined unpaired-surrogate session admitted")
	}
}
