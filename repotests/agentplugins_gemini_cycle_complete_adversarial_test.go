//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func geminiCompleteContractCopy(files map[string][]byte) map[string][]byte {
	result := map[string][]byte{}
	for name, body := range files {
		result[name] = bytes.Clone(body)
	}
	return result
}

func geminiCompleteContractRows(t *testing.T, files map[string][]byte, member string, mutate func(map[string]any)) {
	t.Helper()
	var body []byte
	for _, line := range bytes.Split(bytes.TrimSuffix(files[member], []byte{'\n'}), []byte{'\n'}) {
		row := geminiCompleteContractDecode(t, line)
		mutate(row)
		body = append(append(body, geminiCompleteContractEncode(t, row)...), '\n')
	}
	files[member] = body
}

func geminiCompleteContractRaw(t *testing.T, value any) []byte {
	t.Helper()
	row, ok := value.(map[string]any)
	if !ok {
		t.Fatal("inert raw receipt object")
	}
	body, err := base64.StdEncoding.DecodeString(row["body"].(string))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func geminiCompleteContractReceipt(body []byte) map[string]any {
	return map[string]any{"bytes": len(body), "sha256": geminiCycleSHA(body), "body": append([]byte{}, body...)}
}

func geminiCompleteContractProvider(t *testing.T, files map[string][]byte, stage string, index json.Number, mutate func(map[string]any)) {
	t.Helper()
	geminiCompleteContractRows(t, files, "cycle-provider-requests.jsonl", func(row map[string]any) {
		value := row["value"].(map[string]any)
		if row["stage"] == stage && value["index"] == index {
			body := geminiCompleteContractDecode(t, geminiCompleteContractRaw(t, value["stream"]))
			mutate(body)
			value["stream"] = geminiCompleteContractReceipt(geminiCompleteContractEncode(t, body))
		}
	})
}

func geminiCompleteContractObservation(t *testing.T, files map[string][]byte, tag, channel string, mutate func([]byte) []byte) {
	t.Helper()
	var changed []byte
	geminiCompleteContractRows(t, files, "cycle-native-commands.jsonl", func(row map[string]any) {
		if row["stage"] == tag {
			value := row["value"].(map[string]any)
			changed = mutate(geminiCompleteContractRaw(t, value[channel]))
			value[channel] = geminiCompleteContractReceipt(changed)
		}
	})
	if changed == nil {
		t.Fatal("inert observation mutation missed stage")
	}
	kind, stage, _ := strings.Cut(tag, ":")
	member, key := "cycle-discovery.jsonl", channel
	if kind == "native" {
		if channel != "stdout" {
			return
		}
		member, key = "cycle-native-events.jsonl", "stream"
	}
	geminiCompleteContractRows(t, files, member, func(row map[string]any) {
		if row["stage"] == stage {
			row["value"].(map[string]any)[key] = geminiCompleteContractReceipt(changed)
		}
	})
}

func geminiCompleteContractProof(t *testing.T, files map[string][]byte, stage string, mutate func(map[string]any, map[string]any)) {
	t.Helper()
	geminiCompleteContractRows(t, files, "cycle-proof.jsonl", func(row map[string]any) {
		if row["stage"] == stage {
			value := row["value"].(map[string]any)
			state := geminiCompleteContractDecode(t, geminiCompleteContractRaw(t, value["state"]))
			mutate(value, state)
			value["state"] = geminiCompleteContractReceipt(geminiCompleteContractEncode(t, state))
		}
	})
}

func TestGeminiCycleCompleteDigestUpdatedSemanticContradictionsRefuse(t *testing.T) {
	baseline, source := geminiCompleteSyntheticFiles(t)
	if err := geminiCycleCompleteFiles(baseline, source); err != nil {
		t.Fatal("independent inert baseline failed before adversarial mutations", err)
	}
	for _, kind := range []string{"public-installation", "public-operation", "public-scope", "public-security", "public-unknown", "repeat-remove", "catalog-foreign", "catalog-revision", "native-sentinel", "native-call-id", "native-error", "native-stderr", "native-session-nul", "provider-call-field", "provider-part-field", "provider-prefix", "provider-wrapper", "provider-facts", "provider-response", "provider-receipt", "probe-nonce", "probe-extra", "probe-replay-consistent", "probe-nul-consistent", "state-identity", "state-group", "receipt-authority", "state-unknown-zero", "nochange-state", "removed-clients", "closure-forced", "closure-missing", "closure-late", "tuple-failed", "tuple-old-mode", "lifecycle-type", "source", "pin", "unknown-member", "jsonl-framing", "nested-private"} {
		t.Run(kind, func(t *testing.T) {
			files := geminiCompleteContractCopy(baseline)
			geminiCompleteContractMutation(t, files, kind)
			if err := geminiCycleCompleteFiles(files, source); err == nil {
				t.Fatal("digest-updated semantic contradiction admitted", kind)
			}
		})
	}
}

func geminiCompleteContractMutation(t *testing.T, files map[string][]byte, kind string) {
	t.Helper()
	if strings.HasPrefix(kind, "public-") {
		row := geminiCompleteContractDecode(t, files["add-a.stdout.log"])
		data := row["data"].(map[string]any)
		result := data["targets"].([]any)[0].(map[string]any)["output"].(map[string]any)["result"].(map[string]any)
		switch kind {
		case "public-installation":
			result["installation_id"] = "foreign"
		case "public-operation":
			data["operation_id"] = "op-" + strings.Repeat("e", 32)
		case "public-scope":
			result["plan"].(map[string]any)["scope"] = "user"
		case "public-security":
			data["security"].(map[string]any)["subject"].(map[string]any)["tree_digest"] = "sha256:" + strings.Repeat("0", 64)
		case "public-unknown":
			result["unobserved"] = false
		}
		files["add-a.stdout.log"] = geminiCompleteContractEncode(t, row)
		return
	}
	if strings.HasPrefix(kind, "provider-") {
		geminiCompleteContractProviderMutation(t, files, kind)
		return
	}
	if strings.HasPrefix(kind, "native-") || strings.HasPrefix(kind, "catalog-") {
		geminiCompleteContractObservationMutation(t, files, kind)
		return
	}
	if strings.HasPrefix(kind, "state-") || strings.HasPrefix(kind, "receipt-") || kind == "nochange-state" || kind == "removed-clients" {
		geminiCompleteContractStateMutation(t, files, kind)
		return
	}
	geminiCompleteContractBoundaryMutation(t, files, kind)
}

func geminiCompleteContractProviderMutation(t *testing.T, files map[string][]byte, kind string) {
	t.Helper()
	if kind == "provider-response" {
		geminiCompleteContractRows(t, files, "cycle-provider-responses.jsonl", func(row map[string]any) {
			if row["stage"] == "installed-a" {
				value := row["value"].(map[string]any)
				value["stream"] = geminiCompleteContractReceipt(append(geminiCompleteContractRaw(t, value["stream"]), []byte("data: {}\n\n")...))
			}
		})
		return
	}
	if kind == "provider-receipt" {
		geminiCompleteContractRows(t, files, "cycle-provider-receipts.jsonl", func(row map[string]any) {
			if row["stage"] == "installed-a" {
				row["value"].(map[string]any)["wire_call_id"] = "foreign"
			}
		})
		return
	}
	geminiCompleteContractProvider(t, files, "installed-a", "1", func(row map[string]any) {
		contents := row["contents"].([]any)
		callPart := contents[len(contents)-2].(map[string]any)["parts"].([]any)[0].(map[string]any)
		response := contents[len(contents)-1].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)["response"].(map[string]any)
		switch kind {
		case "provider-call-field":
			callPart["functionCall"].(map[string]any)["ignored"] = false
		case "provider-part-field":
			callPart["ignored"] = false
		case "provider-prefix":
			contents[0].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"] = "foreign prompt"
		case "provider-wrapper":
			response["output"] = "prefix" + response["output"].(string)
		case "provider-facts":
			text := response["output"].(string)
			response["output"] = strings.Replace(text, `"revision":"A-one"`, `"revision":"A-two"`, 1)
		}
	})
}

func geminiCompleteContractObservationMutation(t *testing.T, files map[string][]byte, kind string) {
	t.Helper()
	if strings.HasPrefix(kind, "catalog-") {
		geminiCompleteContractObservation(t, files, "listing:installed-a", "stdout", func(body []byte) []byte {
			if kind == "catalog-foreign" {
				return bytes.Replace(body, []byte("foreign-a [Disabled]"), []byte("foreign-a [Enabled]"), 1)
			}
			return bytes.Replace(body, []byte("Description: A-one"), []byte("Description: A-two"), 1)
		})
		return
	}
	if kind == "native-stderr" {
		geminiCompleteContractObservation(t, files, "native:installed-a", "stderr", func([]byte) []byte { return []byte("unobserved diagnostic\n") })
		return
	}
	var changedSession any
	geminiCompleteContractObservation(t, files, "native:installed-a", "stdout", func(body []byte) []byte {
		var changed []byte
		for _, line := range bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'}) {
			row := geminiCompleteContractDecode(t, line)
			if kind == "native-sentinel" && row["type"] == "message" && row["role"] == "assistant" {
				row["content"] = "foreign completion"
			}
			if kind == "native-call-id" && row["type"] == "tool_use" {
				row["tool_id"] = "foreign"
			}
			if kind == "native-error" && row["type"] == "result" {
				row["status"] = "error"
			}
			if kind == "native-session-nul" && row["type"] == "init" {
				changedSession = row["session_id"].(string) + "\x00"
				row["session_id"] = changedSession
			}
			changed = append(append(changed, geminiCompleteContractEncode(t, row)...), '\n')
		}
		return changed
	})
	if kind == "native-session-nul" {
		geminiCompleteContractRows(t, files, "cycle-native-events.jsonl", func(row map[string]any) {
			if row["stage"] == "installed-a" {
				row["value"].(map[string]any)["session"] = changedSession
			}
		})
	}
}

func geminiCompleteContractStateMutation(t *testing.T, files map[string][]byte, kind string) {
	t.Helper()
	stage := "add-a"
	if kind == "nochange-state" {
		stage = "repeat-add-a"
	}
	if kind == "removed-clients" {
		stage = "remove-a"
	}
	geminiCompleteContractProof(t, files, stage, func(value, state map[string]any) {
		i := state["installations"].([]any)[0].(map[string]any)
		switch kind {
		case "state-identity":
			i["installation_id"] = "foreign"
		case "state-group":
			i["operation_group_id"] = "foreign"
		case "state-unknown-zero":
			i["needs_rebind"] = false
		case "nochange-state":
			stamp, err := time.Parse(time.RFC3339Nano, i["updated_at"].(string))
			if err != nil {
				t.Fatal(err)
			}
			i["updated_at"] = stamp.Add(time.Nanosecond).UTC().Format(time.RFC3339Nano)
		case "removed-clients":
			i["clients"] = map[string]any{"foreign": value["binding"]}
		case "receipt-authority":
			b := value["binding"].(map[string]any)
			b["receipts"].([]any)[0].(map[string]any)["environment_authority"] = map[string]any{"environment_id": "foreign"}
			for _, binding := range i["clients"].(map[string]any) {
				binding.(map[string]any)["receipts"].([]any)[0].(map[string]any)["environment_authority"] = map[string]any{"environment_id": "foreign"}
			}
		}
	})
}

func geminiCompleteContractBoundaryMutation(t *testing.T, files map[string][]byte, kind string) {
	t.Helper()
	if kind == "repeat-remove" {
		files["repeat-remove-a.stderr.log"] = []byte("changed refusal\n")
		return
	}
	if kind == "unknown-member" {
		files["unobserved.json"] = []byte("{}")
		return
	}
	if kind == "jsonl-framing" {
		files["cycle-proof.jsonl"] = bytes.TrimSuffix(files["cycle-proof.jsonl"], []byte{'\n'})
		return
	}
	if strings.HasPrefix(kind, "probe-") {
		if kind == "probe-replay-consistent" || kind == "probe-nul-consistent" {
			geminiCompleteContractReplayProcess(t, files, kind == "probe-nul-consistent")
			return
		}
		geminiCompleteContractRows(t, files, "cycle-probe-events.jsonl", func(row map[string]any) {
			if row["stage"] == "installed-a" {
				value := row["value"].(map[string]any)
				body := geminiCompleteContractRaw(t, value["stream"])
				if kind == "probe-extra" {
					body = append(body, []byte("{}\n")...)
				} else {
					body = bytes.ReplaceAll(body, []byte(`"nonce":"`), []byte(`"nonce":"foreign-`))
				}
				value["stream"] = geminiCompleteContractReceipt(body)
			}
		})
		return
	}
	row := geminiCompleteContractDecode(t, files["runtime-attempt.json"])
	closure := row["cycle_completion"].(map[string]any)
	switch kind {
	case "closure-forced":
		closure["provider"].(map[string]any)["forced_close"] = true
	case "closure-missing":
		delete(row, "cycle_completion")
	case "closure-late":
		closure["finished_at"] = "9999-01-01T00:00:00Z"
	case "tuple-failed":
		row["test_failed"] = true
	case "tuple-old-mode":
		row["capture_mode"] = "gemini-complete-cycle-first-capture"
	case "lifecycle-type":
		row["qualification"].(map[string]any)["mcp-tools-call"] = 1
	case "source":
		row["source_identity"].(map[string]any)["installer_tree"] = strings.Repeat("f", 40)
	case "pin":
		row["acquisition"].(map[string]any)["node"].(map[string]any)["sha256"] = strings.Repeat("f", 64)
	case "nested-private":
		row["cycle_completion"].(map[string]any)["hidden"] = `{"nested":"uap-fixture-` + strings.Repeat("a", 48) + `"}`
	}
	files["runtime-attempt.json"] = geminiCompleteContractEncode(t, row)
}

func geminiCompleteContractProviderFacts(t *testing.T, files map[string][]byte, stage string) map[string]any {
	t.Helper()
	var result map[string]any
	geminiCompleteContractProvider(t, files, stage, "1", func(row map[string]any) {
		contents := row["contents"].([]any)
		response := contents[len(contents)-1].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)["response"].(map[string]any)
		body := strings.TrimSuffix(strings.TrimPrefix(response["output"].(string), "<untrusted_context>\n"), "\n</untrusted_context>")
		result = geminiCompleteContractDecode(t, []byte(body))
	})
	if result == nil {
		t.Fatal("inert returned facts required")
	}
	return result
}

func geminiCompleteContractReplayProcess(t *testing.T, files map[string][]byte, nul bool) {
	t.Helper()
	old := geminiCompleteContractProviderFacts(t, files, "installed-a")
	facts := geminiCompleteContractProviderFacts(t, files, "restarted-a")
	if nul {
		old = map[string]any{"pid": facts["pid"], "session": facts["session"].(string) + "\x00"}
	}
	facts["pid"], facts["session"] = old["pid"], old["session"]
	encoded := geminiCompleteContractEncode(t, facts)
	geminiCompleteContractProvider(t, files, "restarted-a", "1", func(row map[string]any) {
		contents := row["contents"].([]any)
		response := contents[len(contents)-1].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)["response"].(map[string]any)
		response["output"] = "<untrusted_context>\n" + string(encoded) + "\n</untrusted_context>"
	})
	geminiCompleteContractObservation(t, files, "native:restarted-a", "stdout", func(body []byte) []byte {
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
	geminiCompleteContractRows(t, files, "cycle-probe-events.jsonl", func(row map[string]any) {
		if row["stage"] != "restarted-a" {
			return
		}
		value := row["value"].(map[string]any)
		body := geminiCompleteContractRaw(t, value["stream"])
		var changed []byte
		for _, line := range bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'}) {
			event := geminiCompleteContractDecode(t, line)
			if event["nonce"] == facts["nonce"] {
				event["pid"], event["session"] = old["pid"], old["session"]
				if event["method"] == "tools/call" {
					event["facts"] = facts
				}
			}
			changed = append(append(changed, geminiCompleteContractEncode(t, event)...), '\n')
		}
		value["stream"] = geminiCompleteContractReceipt(changed)
	})
}

func TestGeminiCycleCompleteOptionalStatsJoinSelectedModelAndCallCount(t *testing.T) {
	baseline, source := geminiCompleteSyntheticFiles(t)
	for _, kind := range []string{"valid", "foreign-model", "wrong-count", "float-root", "float-model", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			files := geminiCompleteContractCopy(baseline)
			geminiCompleteContractObservation(t, files, "native:installed-a", "stdout", func(body []byte) []byte {
				var changed []byte
				for _, line := range bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'}) {
					row := geminiCompleteContractDecode(t, line)
					if row["type"] == "result" {
						model := "gemini-2.5-flash"
						if kind == "foreign-model" {
							model = "foreign-model"
						}
						stats := geminiCompleteContractDecode(t, []byte(`{"total_tokens":4,"input_tokens":2,"output_tokens":2,"cached":0,"input":2,"duration_ms":100,"tool_calls":1,"models":{}}`))
						stats["models"] = map[string]any{model: map[string]any{"total_tokens": 4, "input_tokens": 2, "output_tokens": 2, "cached": 0, "input": 2}}
						if kind == "wrong-count" {
							stats["tool_calls"] = 0
						}
						if kind == "float-root" {
							stats["duration_ms"] = json.Number("100.0")
						}
						if kind == "float-model" {
							stats["models"].(map[string]any)[model].(map[string]any)["input_tokens"] = json.Number("2e0")
						}
						if kind == "unknown" {
							stats["extra"] = false
						}
						row["stats"] = stats
					}
					changed = append(append(changed, geminiCompleteContractEncode(t, row)...), '\n')
				}
				return changed
			})
			err := geminiCycleCompleteFiles(files, source)
			if (err == nil) != (kind == "valid") {
				t.Fatal("optional native stats/model/tool count predicate differs", err)
			}
		})
	}
}

func TestGeminiCycleCompleteDistinctScopeNoncesPreventCoherentProbeMapCollapse(t *testing.T) {
	f := geminiCompleteContractFixtureNonce(t, true)
	f.publicRecords()
	f.observations()
	f.encodeRows()
	f.metadata()
	p := f.phase("installed-a", 2)
	allowed := map[string]*geminiCycleRecipe{}
	for _, r := range p.advertised {
		allowed[r.Nonce] = r
	}
	if len(p.advertised) != 2 || len(allowed) != 1 {
		t.Fatal("coherent nonce alias fixture did not collapse advertised scope map")
	}
	var segment []byte
	for _, row := range f.rows["cycle-probe-events.jsonl"] {
		if row["stage"] == "installed-a" {
			value := geminiCompleteContractDecode(t, geminiCompleteContractEncode(t, row["value"]))
			segment = geminiCompleteContractRaw(t, value["stream"])
		}
	}
	if len(bytes.Split(bytes.TrimSuffix(segment, []byte{'\n'}), []byte{'\n'})) != 3 {
		t.Fatal("coherent alias control did not omit user initialize/list events")
	}
	facts, err := geminiCycleFacts(string(geminiCompleteContractEncode(t, p.facts)), p.r, f.paths["home"])
	if err != nil {
		t.Fatal(err)
	}
	phase := &geminiCyclePhase{r: p.r, operation: p.operation, facts: facts}
	start := f.started.Add(9 * time.Second)
	if groups, err := geminiCycleProbeSegment(segment, allowed, phase, f.paths["home"], start, start.Add(500*time.Millisecond), map[string]bool{}, map[int64]bool{}); err != nil || len(groups) != 1 {
		t.Fatal("control failed to reproduce preserved map-key collapse", err)
	}
	source := geminiCompleteContractDecode(t, geminiCompleteContractEncode(t, f.contractSource()))
	if len(f.files) != 57 || geminiCycleCompleteFiles(f.files, source) == nil {
		t.Fatal("coherent full evidence with aliased scope nonces admitted")
	}
}
