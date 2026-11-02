//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func geminiCycleMeasured(t *testing.T) (*geminiCyclePhase, map[string]string, map[string][]byte) {
	t.Helper()
	root := filepath.Join("testdata", "gemini-cycle-2ff")
	raw, err := os.ReadFile(filepath.Join(root, "context.json"))
	if err != nil {
		t.Fatal(err)
	}
	if geminiCycleSHA(raw) != "3b0a25307fe88be15270a83aeb78bfdca1aeea7ee0b2cf1f55d07a1aab046715" {
		t.Fatal("actual2ff context source receipt changed")
	}
	var value struct {
		Source                                                      string `json:"source_commit"`
		Tree                                                        string `json:"source_tree"`
		Home, Active, Data, Nonce, Revision, Tool, Prompt, Sentinel string
		NativeID                                                    string `json:"native_id"`
		WireID                                                      string `json:"wire_id"`
		Files                                                       map[string]string
	}
	if json.Unmarshal(raw, &value) != nil || value.Source != "2ff752a5909c96333786cfe80454fd8bf7be3a3b" || value.Tree != "9a98d09f6ec25d3d22b137309035bf213f284839" {
		t.Fatal("exact genuine measured context")
	}
	files := map[string][]byte{}
	for name, sha := range value.Files {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || geminiCycleSHA(body) != sha {
			t.Fatal("actual2ff raw fixture SHA mismatch", name)
		}
		files[name] = body
	}
	r := &geminiCycleRecipe{Scope: "a", Name: "uapgeminia", Nonce: value.Nonce, Revision: value.Revision, Active: value.Active, Data: value.Data}
	q := &geminiCyclePhase{operation: "write", r: r, tool: value.Tool, id: value.NativeID, wireID: value.WireID, prompt: value.Prompt, sentinel: value.Sentinel, tools: []string{value.Tool}}
	return q, map[string]string{"home": value.Home}, files
}
func geminiCycleClone(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	value, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value.(map[string]any)
}
func geminiCycleEncoded(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func geminiCycleReplayFirst(t *testing.T, q *geminiCyclePhase, raw []byte) {
	t.Helper()
	q.requests = [][]byte{raw}
	parts, err := geminiCycleRequest(raw, q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := geminiCycleHistory(parts, q, 0); err != nil {
		t.Fatal(err)
	}
}
func TestGeminiCycleActual2ffRequestsNativeFramesAndReturnedFactsReplayUnchanged(t *testing.T) {
	q, meta, files := geminiCycleMeasured(t)
	geminiCycleReplayFirst(t, q, files["provider-request-1.json"])
	q.requests = append(q.requests, files["provider-request-2.json"])
	parts, err := geminiCycleRequest(files["provider-request-2.json"], q)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := geminiCycleHistory(parts, q, 1)
	if err != nil {
		t.Fatal(err)
	}
	q.facts, err = geminiCycleFacts(inner, q.r, meta["home"])
	if err != nil {
		t.Fatal(err)
	}
	end := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	session, err := geminiCycleStream(files["native-attempt.stdout.log"], q, meta["home"], time.Unix(0, 0), end)
	if err != nil || session != "7d3bbbe8-2e08-4906-866e-90ae12ca97e7" {
		t.Fatal("actual6 frames do not match native/wire/facts", err)
	}
	events := bytes.Split(bytes.TrimSuffix(files["probe-events.jsonl"], []byte{'\n'}), []byte{'\n'})
	if len(events) != 3 {
		t.Fatal("actual3 probe methods")
	}
	for index, line := range events {
		value := geminiCycleClone(t, line)
		if value["method"] != []string{"initialize", "tools/list", "tools/call"}[index] {
			t.Fatal("actual probe order")
		}
		if index == 2 {
			facts, err := geminiCycleFacts(string(geminiCycleEncoded(t, value["facts"])), q.r, meta["home"])
			if err != nil || !reflect.DeepEqual(facts, q.facts) {
				t.Fatal("actual provider/native/probe fact join")
			}
		}
	}
}
func TestGeminiCycleDeclarationsAreExactSourceGuidedZeroOneOrTwo(t *testing.T) {
	q, _, files := geminiCycleMeasured(t)
	first := geminiCycleClone(t, files["provider-request-1.json"])
	original := first["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)[0]
	for _, count := range []int{0, 1, 2} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			value := geminiCycleClone(t, files["provider-request-1.json"])
			items := []any{}
			names := []string{}
			for index := range count {
				declaration := geminiCycleClone(t, geminiCycleEncoded(t, original))
				name := q.tool
				if index == 1 {
					name = "mcp_uapgeminiu_inspect_runtime"
				}
				declaration["name"] = name
				items = append(items, declaration)
				names = append(names, name)
			}
			value["tools"] = []any{map[string]any{"functionDeclarations": items}}
			q.tools = names
			q.requests = [][]byte{geminiCycleEncoded(t, value)}
			if _, err := geminiCycleRequest(q.requests[0], q); err != nil {
				t.Fatal("exact group count refused", err)
			}
		})
	}
	for _, kind := range []string{"omitted", "null", "empty-group-array", "duplicate", "builtin", "extra-group", "description", "schema", "wait"} {
		t.Run(kind, func(t *testing.T) {
			q, _, files := geminiCycleMeasured(t)
			value := geminiCycleClone(t, files["provider-request-1.json"])
			group := value["tools"].([]any)[0].(map[string]any)
			decl := group["functionDeclarations"].([]any)[0].(map[string]any)
			switch kind {
			case "omitted":
				delete(value, "tools")
			case "null":
				value["tools"] = nil
			case "empty-group-array":
				value["tools"] = []any{}
			case "duplicate":
				group["functionDeclarations"] = []any{decl, decl}
			case "builtin":
				decl["name"] = "run_shell_command"
			case "extra-group":
				value["tools"] = []any{group, group}
			case "description":
				decl["description"] = "foreign"
			case "schema":
				decl["parametersJsonSchema"].(map[string]any)["additionalProperties"] = true
			case "wait":
				decl["parametersJsonSchema"].(map[string]any)["properties"].(map[string]any)["wait_for_previous"] = map[string]any{"type": "string"}
			}
			body := geminiCycleEncoded(t, value)
			q.requests = [][]byte{body}
			if _, err := geminiCycleRequest(body, q); err == nil {
				t.Fatal("foreign/unknown declaration framing accepted")
			}
		})
	}
}
func TestGeminiCycleReturnedHistoryCannotChangePrefixSignatureRoleIDOrArguments(t *testing.T) {
	for _, kind := range []string{"native-id", "other-id", "signature", "call-role", "response-role", "args", "prefix", "extra-call", "extra-text", "wrapper"} {
		t.Run(kind, func(t *testing.T) {
			q, _, files := geminiCycleMeasured(t)
			geminiCycleReplayFirst(t, q, files["provider-request-1.json"])
			value := geminiCycleClone(t, files["provider-request-2.json"])
			rows := value["contents"].([]any)
			model := rows[1].(map[string]any)
			callPart := model["parts"].([]any)[0].(map[string]any)
			call := callPart["functionCall"].(map[string]any)
			responseRow := rows[2].(map[string]any)
			response := responseRow["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)
			switch kind {
			case "native-id":
				call["id"] = q.id
				response["id"] = q.id
			case "other-id":
				call["id"] = "other"
				response["id"] = "other"
			case "signature":
				callPart["thoughtSignature"] = "other"
			case "call-role":
				model["role"] = "user"
			case "response-role":
				responseRow["role"] = "model"
			case "args":
				call["args"].(map[string]any)["operation"] = "read"
			case "prefix":
				rows[0].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"] = "changed"
			case "extra-call":
				model["parts"] = append(model["parts"].([]any), callPart)
			case "extra-text":
				model["parts"] = append(model["parts"].([]any), map[string]any{"text": "extra"})
			case "wrapper":
				response["response"].(map[string]any)["output"] = "<untrusted_context>\n<untrusted_context>\n{}\n</untrusted_context>\n</untrusted_context>"
			}
			body := geminiCycleEncoded(t, value)
			q.requests = append(q.requests, body)
			parts, err := geminiCycleRequest(body, q)
			if err == nil {
				_, err = geminiCycleHistory(parts, q, 1)
			}
			if err == nil {
				t.Fatal("changed returned history accepted")
			}
		})
	}
}
func TestGeminiCyclePrivacyClosesNestedStringsKeysJSONLAndResourceBounds(t *testing.T) {
	marker := "uap-gemini-private-canary"
	for _, raw := range [][]byte{[]byte(marker), geminiCycleEncoded(t, map[string]any{"nested": `{"text":"\u0075ap-gemini-private-canary"}`}), []byte(`{"ok":true}` + "\n" + `{"nested":"{\"key\":\"\u0075ap-gemini-private-canary\"}"}` + "\n"), []byte(`{"key":1e999}`), []byte(`{"key":` + strings.Repeat("9", 5000) + `,"secret":"\u0075ap-gemini-private-canary"}`), bytes.Repeat([]byte("x"), (1<<20)+1), {0xff}} {
		if !geminiCyclePrivateCredential(raw) {
			t.Fatal("private/bound refusal escaped")
		}
	}
	for _, raw := range [][]byte{[]byte("1e999" + strings.Repeat("a", 59)), []byte("1e999 1e999"), []byte(`{"ordinary":"1e999` + strings.Repeat("a", 59) + `"}`)} {
		if geminiCyclePrivateCredential(raw) {
			t.Fatal("ordinary complete-document numeric prefix text refused")
		}
	}
}

func TestGeminiCycleSourceGuidedEmptyNativeStreamHasNoToolAuthority(t *testing.T) {
	q := &geminiCyclePhase{operation: "none", prompt: "inert empty prompt", sentinel: "INERT_EMPTY_SENTINEL"}
	stamp := "2026-10-27T00:00:01Z"
	frames := []map[string]any{{"type": "init", "timestamp": stamp, "session_id": "inert-fresh-session", "model": geminiProjectModel}, {"type": "message", "timestamp": stamp, "role": "user", "content": q.prompt}, {"type": "message", "timestamp": stamp, "role": "assistant", "content": q.sentinel}, {"type": "result", "timestamp": stamp, "status": "success"}}
	encode := func(rows []map[string]any) []byte {
		var raw []byte
		for _, row := range rows {
			raw = append(append(raw, geminiCycleEncoded(t, row)...), '\n')
		}
		return raw
	}
	start, end := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 27, 0, 1, 0, 0, time.UTC)
	if _, err := geminiCycleStream(encode(frames), q, "/inert/home", start, end); err != nil {
		t.Fatal("source-guided inert empty frames refused", err)
	}
	for _, kind := range []string{"tool", "model", "prompt", "sentinel", "extra", "timestamp", "role"} {
		t.Run(kind, func(t *testing.T) {
			rows := make([]map[string]any, 0, len(frames))
			for _, row := range frames {
				rows = append(rows, geminiCycleClone(t, geminiCycleEncoded(t, row)))
			}
			switch kind {
			case "tool":
				rows = append(rows[:2], append([]map[string]any{{"type": "tool_use", "timestamp": stamp, "tool_name": "mcp_foreign_inspect_runtime", "tool_id": "foreign", "parameters": map[string]any{}}}, rows[2:]...)...)
			case "model":
				rows[0]["model"] = "foreign"
			case "prompt":
				rows[1]["content"] = "foreign"
			case "sentinel":
				rows[2]["content"] = "foreign"
			case "extra":
				rows = append(rows, rows[3])
			case "timestamp":
				rows[2]["timestamp"] = "2030-01-01T00:00:00Z"
			case "role":
				rows[2]["role"] = "model"
			}
			if _, err := geminiCycleStream(encode(rows), q, "/inert/home", start, end); err == nil {
				t.Fatal("empty/native strictness escaped")
			}
		})
	}
}
func TestGeminiCycleActualNativeIDCannotBeReplacedConsistentlyByHTTPWireID(t *testing.T) {
	q, meta, files := geminiCycleMeasured(t)
	geminiCycleReplayFirst(t, q, files["provider-request-1.json"])
	q.requests = append(q.requests, files["provider-request-2.json"])
	parts, err := geminiCycleRequest(files["provider-request-2.json"], q)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := geminiCycleHistory(parts, q, 1)
	if err != nil {
		t.Fatal(err)
	}
	q.facts, err = geminiCycleFacts(inner, q.r, meta["home"])
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.ReplaceAll(files["native-attempt.stdout.log"], []byte(q.id), []byte(q.wireID))
	if _, err := geminiCycleStream(changed, q, meta["home"], time.Unix(0, 0), time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("native stream accepted both events using wire ID")
	}
}
func TestGeminiCycleSemanticPrivacyBindsTheUnchangedActualHeldKeyNamespace(t *testing.T) {
	c := geminiCycleInert(t)
	c.f.secret = "uap-fixture-" + strings.Repeat("a", 48)
	escaped := `{"credential":"\u0075ap-fixture-` + strings.Repeat("a", 48) + `"}`
	key := `{"\u0075ap-fixture-` + strings.Repeat("a", 48) + `":true}`
	for _, raw := range [][]byte{[]byte(escaped), geminiCycleEncoded(t, map[string]any{"nested": escaped}), []byte(key), geminiCycleEncoded(t, map[string]any{"nested": key}), []byte("{}\n" + escaped + "\n")} {
		redacted := bytes.ReplaceAll(raw, []byte(c.f.secret), []byte("[redacted]"))
		if bytes.Contains(raw, []byte(c.f.secret)) || !geminiCyclePrivateCredential(redacted) {
			t.Fatal("JSON-encoded actual held key escaped semantic refusal")
		}
	}
	literal := geminiCycleEncoded(t, map[string]any{"credential": c.f.secret})
	redacted := bytes.ReplaceAll(literal, []byte(c.f.secret), []byte("[redacted]"))
	if geminiCyclePrivateCredential(redacted) {
		t.Fatal("exact literal held-key redaction became an invented credential")
	}
}
