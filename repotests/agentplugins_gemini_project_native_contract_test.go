//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func geminiProjectFactFixture(t *testing.T) (*geminiProjectAttempt, nativeFacts) {
	t.Helper()
	root := t.TempDir()
	f := &geminiProjectAttempt{base: &nativeFixture{Root: root, Home: filepath.Join(root, "home"), Events: filepath.Join(root, "events.jsonl"), Nonce: "inert-nonce"}, data: filepath.Join(root, "owned-data")}
	f.binding.TargetLocator = filepath.Join(root, "owned-native")
	facts := nativeFacts{Nonce: "inert-nonce", Revision: "A-one", CWD: f.binding.TargetLocator,
		Argv: []string{"argument with spaces", f.binding.TargetLocator + "/argument", "${UNKNOWN}"},
		Root: f.binding.TargetLocator, Data: f.data, Literal: "literal  " + f.base.Home,
		Once: f.binding.TargetLocator + "/", Marker: "retained-inert-nonce", PID: 1234, Session: "inert-process-session"}
	return f, facts
}

func geminiProjectFactEvents(t *testing.T, f *geminiProjectAttempt, facts nativeFacts, fault string) {
	t.Helper()
	var body bytes.Buffer
	for index, method := range []string{"initialize", "tools/list", "tools/call"} {
		row := map[string]any{"method": method, "pid": facts.PID, "session": facts.Session, "nonce": facts.Nonce,
			"timestamp": time.Date(2026, 10, 27, 0, 0, index, 0, time.UTC).Format(time.RFC3339Nano), "facts": nil}
		if method == "tools/call" {
			observed := facts
			if fault == "event-facts" {
				observed.Marker = "foreign-event-marker"
			}
			row["facts"] = observed
		}
		if fault == "event-pid" {
			row["pid"] = facts.PID + 1
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		body.Write(encoded)
		body.WriteByte('\n')
	}
	projectNativeRuntimeWrite(t, f.base.Events, body.Bytes(), 0600)
}

func TestGeminiProjectNativeFactsRequireExactHeldNativeEnvExpansionAndEventJoin(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*nativeFacts)
	}{
		{"exact", func(*nativeFacts) {}},
		{"old-literal", func(f *nativeFacts) { f.Literal = "literal ${UNKNOWN} $HOME" }},
		{"old-once", func(f *nativeFacts) { f.Once = f.Root + "/${UNKNOWN}" }},
		{"wrong-home", func(f *nativeFacts) { f.Literal = "literal  /foreign/home" }},
		{"literal-spacing", func(f *nativeFacts) { f.Literal = strings.Replace(f.Literal, "literal  ", "literal ", 1) }},
		{"sibling-once", func(f *nativeFacts) { f.Once = f.Root + "-sibling/" }},
		{"argv-expanded", func(f *nativeFacts) { f.Argv[2] = "" }},
		{"root", func(f *nativeFacts) { f.Root += "-foreign" }},
		{"data", func(f *nativeFacts) { f.Data += "-foreign" }},
		{"cwd", func(f *nativeFacts) { f.CWD += "-foreign" }},
		{"nonce", func(f *nativeFacts) { f.Nonce = "foreign" }},
		{"marker", func(f *nativeFacts) { f.Marker = "foreign" }},
		{"revision", func(f *nativeFacts) { f.Revision = "B-one" }},
		{"pid", func(f *nativeFacts) { f.PID = 0 }},
		{"session", func(f *nativeFacts) { f.Session = "" }},
		{"event-pid", func(*nativeFacts) {}}, {"event-facts", func(*nativeFacts) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, facts := geminiProjectFactFixture(t)
			tc.mutate(&facts)
			geminiProjectFactEvents(t, f, facts, tc.name)
			body, err := json.Marshal(facts)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.validateFacts(string(body)); (err == nil) != (tc.name == "exact") {
				t.Fatal("native env expectation or independent fact/event authority differs", err)
			}
		})
	}
}

func geminiProjectNativeStderrAllowed(body []byte) bool {
	const ripgrep = "Ripgrep is not available. Falling back to GrepTool.\n"
	const terminal = "Warning: Basic terminal detected (TERM=dumb). Visual rendering will be limited. For the best experience, use a terminal emulator with truecolor support.\n" +
		"Warning: 256-color support not detected. Using a terminal with at least 256-color support is recommended for a better visual experience.\n" + ripgrep
	const startupMetrics = terminal + "[STARTUP] Phase 'cleanup_ops' was started but never ended. Skipping metrics.\n"
	const missingStartMark = startupMetrics + "[STARTUP] Cannot measure phase 'cleanup_ops': start mark 'startup:cleanup_ops:start' not found (likely cleared by reset).\n"
	switch string(body) {
	case "", ripgrep, terminal, startupMetrics, missingStartMark:
		return true
	default:
		return false
	}
}

func TestGeminiProjectNativeStderrAdmitsOnlyExactMeasuredTuples(t *testing.T) {
	const observed = `Warning: Basic terminal detected (TERM=dumb). Visual rendering will be limited. For the best experience, use a terminal emulator with truecolor support.
Warning: 256-color support not detected. Using a terminal with at least 256-color support is recommended for a better visual experience.
Ripgrep is not available. Falling back to GrepTool.
`
	if len(observed) != 342 || fmt.Sprintf("%x", sha256.Sum256([]byte(observed))) != "0309ffcd9f9558b019117eab516a089bfe80be8c1a596be48e3794ecf695a953" {
		t.Fatal("independent measured terminal tuple differs")
	}
	for _, body := range []string{"", "Ripgrep is not available. Falling back to GrepTool.\n", observed} {
		if !geminiProjectNativeStderrAllowed([]byte(body)) {
			t.Fatal("exact measured diagnostic tuple refused")
		}
	}
	lines := strings.SplitAfter(observed, "\n")
	for index, body := range []string{
		observed[:len(observed)-1], observed + "\n", "\n" + observed, observed + observed,
		strings.ReplaceAll(observed, "\n", "\r\n"), lines[0], lines[1], lines[0] + lines[1],
		lines[1] + lines[0] + lines[2], lines[2] + lines[0] + lines[1],
		strings.Replace(observed, "TERM=dumb", "TERM=vt100", 1), strings.Replace(observed, "256-color", "truecolor", 1),
		observed + "API request failed\nError: private stack\n", observed + "Not signed in\n",
		"\x1b[31m" + observed, observed + "\x00",
	} {
		if geminiProjectNativeStderrAllowed([]byte(body)) {
			t.Fatalf("unmeasured or failing diagnostic %d admitted", index)
		}
	}
}

func TestGeminiProjectNativeStartupMetricsAdmitsOnlyWholeMeasuredTuple(t *testing.T) {
	const observed = `Warning: Basic terminal detected (TERM=dumb). Visual rendering will be limited. For the best experience, use a terminal emulator with truecolor support.
Warning: 256-color support not detected. Using a terminal with at least 256-color support is recommended for a better visual experience.
Ripgrep is not available. Falling back to GrepTool.
[STARTUP] Phase 'cleanup_ops' was started but never ended. Skipping metrics.
`
	if len(observed) != 419 || fmt.Sprintf("%x", sha256.Sum256([]byte(observed))) != "315efcf74c381fe9f43bc921c84091ca24436a267b3bfcd76b5c705089309c7f" {
		t.Fatal("independent measured startup-metrics tuple differs")
	}
	if !geminiProjectNativeStderrAllowed([]byte(observed)) {
		t.Fatal("exact measured startup-metrics tuple refused")
	}
	lines := strings.SplitAfter(observed, "\n")
	for index, body := range []string{
		lines[3], lines[2] + lines[3], lines[3] + lines[0] + lines[1] + lines[2],
		observed[:len(observed)-1], observed + "\n", "\n" + observed, observed + lines[3], observed + observed,
		strings.ReplaceAll(observed, "\n", "\r\n"), strings.Replace(observed, "cleanup_ops", "config", 1),
		strings.Replace(observed, "Skipping metrics.", "Early cleanup failed:", 1),
		observed + "Early cleanup failed: private error\n", observed + "[STARTUP] unknown metrics\n",
		observed + "API request failed\nError: private stack\n", observed + "Not signed in\n",
		"\x1b[31m" + observed, observed + "\x00",
	} {
		if geminiProjectNativeStderrAllowed([]byte(body)) {
			t.Fatalf("unmeasured startup-metrics diagnostic %d admitted", index)
		}
	}
}

func TestGeminiProjectNativeMissingStartMarkAdmitsOnlyWholeMeasuredTuple(t *testing.T) {
	const observed = `Warning: Basic terminal detected (TERM=dumb). Visual rendering will be limited. For the best experience, use a terminal emulator with truecolor support.
Warning: 256-color support not detected. Using a terminal with at least 256-color support is recommended for a better visual experience.
Ripgrep is not available. Falling back to GrepTool.
[STARTUP] Phase 'cleanup_ops' was started but never ended. Skipping metrics.
[STARTUP] Cannot measure phase 'cleanup_ops': start mark 'startup:cleanup_ops:start' not found (likely cleared by reset).
`
	const suffix = "[STARTUP] Cannot measure phase 'cleanup_ops': start mark 'startup:cleanup_ops:start' not found (likely cleared by reset).\n"
	if len(observed) != 541 || fmt.Sprintf("%x", sha256.Sum256([]byte(observed))) != "4a44629c8b644a0e5629b463e0fc14392bda3ef65c08cca2e0cd8963ff59d3c2" {
		t.Fatal("independent measured missing-start-mark tuple differs")
	}
	if len(suffix) != 122 || fmt.Sprintf("%x", sha256.Sum256([]byte(suffix))) != "2bc542b67a6f3474557140033c8830144135651234852ee38e78b9b4aa32a01a" {
		t.Fatal("independent measured missing-start-mark suffix differs")
	}
	if !geminiProjectNativeStderrAllowed([]byte(observed)) {
		t.Fatal("exact measured missing-start-mark tuple refused")
	}
	lines := strings.SplitAfter(observed, "\n")
	for index, body := range []string{
		suffix, lines[0] + lines[1] + lines[2] + suffix, lines[0] + lines[1] + lines[2] + suffix + lines[3],
		observed[:len(observed)-1], observed + "\n", "\n" + observed, observed + suffix, observed + observed,
		strings.ReplaceAll(observed, "\n", "\r\n"), strings.ReplaceAll(observed, "cleanup_ops", "config"),
		strings.Replace(observed, "startup:cleanup_ops:start", "startup:config:start", 1),
		strings.Replace(observed, "likely cleared by reset", "cleanup failed", 1),
		observed + "Early cleanup failed: private error\n", observed + "[STARTUP] unknown metrics\n",
		observed + "API request failed\nError: private stack\n", observed + "Not signed in\n",
		"\x1b[31m" + observed, observed + "\x00",
	} {
		if geminiProjectNativeStderrAllowed([]byte(body)) {
			t.Fatalf("unmeasured missing-start-mark diagnostic %d admitted", index)
		}
	}
}

// Source-defined JSONL union; genuine runtime evidence must still satisfy every
// ordering, tool, native-session and independent probe-facts constraint.
func geminiProjectStream(body []byte, p *geminiProjectProvider, prompt string, validate func(string) error) (string, error) {
	if len(body) == 0 || len(body) > projectNativeRuntimeBodyLimit || body[len(body)-1] != '\n' {
		return "", fmt.Errorf("Gemini whole JSONL bound")
	}
	stage := 0
	session, assistant := "", ""
	lastTime := time.Time{}
	for _, line := range bytes.Split(body[:len(body)-1], []byte{'\n'}) {
		value, err := projectNativeRuntimeJSON(line)
		if err != nil {
			return "", err
		}
		row, ok := value.(map[string]any)
		if !ok {
			return "", fmt.Errorf("Gemini event object")
		}
		timestamp, ok := row["timestamp"].(string)
		when, err := time.Parse(time.RFC3339Nano, timestamp)
		if !ok || err != nil || when.Before(lastTime) {
			return "", fmt.Errorf("Gemini timestamp")
		}
		lastTime = when
		switch row["type"] {
		case "init":
			if stage != 0 || projectNativeRuntimeKeys(row, "type timestamp session_id model", "") != nil || !projectNativeRuntimeText(row["session_id"], 256) || row["session_id"] == "" || row["model"] != geminiProjectModel {
				return "", fmt.Errorf("Gemini init identity")
			}
			session = row["session_id"].(string)
			stage = 1
		case "message":
			if projectNativeRuntimeKeys(row, "type timestamp role content", "delta") != nil || !projectNativeRuntimeText(row["content"], projectNativeRuntimeBodyLimit) {
				return "", fmt.Errorf("Gemini message fields")
			}
			if delta, exists := row["delta"]; exists {
				if _, ok := delta.(bool); !ok {
					return "", fmt.Errorf("Gemini delta type")
				}
			}
			switch row["role"] {
			case "user":
				if stage != 1 || row["content"] != prompt {
					return "", fmt.Errorf("Gemini initial user message")
				}
				stage = 2
			case "assistant":
				if stage != 4 && stage != 5 {
					return "", fmt.Errorf("Gemini assistant out of order")
				}
				assistant += row["content"].(string)
				if len(assistant) > len(p.sentinel) {
					return "", fmt.Errorf("Gemini unexpected assistant text")
				}
				stage = 5
			default:
				return "", fmt.Errorf("Gemini message role")
			}
		case "tool_use":
			if stage != 2 || projectNativeRuntimeKeys(row, "type timestamp tool_name tool_id parameters", "") != nil || row["tool_name"] != p.tool || row["tool_id"] != p.id || !reflect.DeepEqual(row["parameters"], map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}) {
				return "", fmt.Errorf("Gemini tool use identity/arguments")
			}
			stage = 3
		case "tool_result":
			if stage != 3 || projectNativeRuntimeKeys(row, "type timestamp tool_id status output", "") != nil || row["tool_id"] != p.id || row["status"] != "success" || !projectNativeRuntimeText(row["output"], projectNativeRuntimeBodyLimit) {
				return "", fmt.Errorf("Gemini tool result")
			}
			if err := validate(row["output"].(string)); err != nil {
				return "", err
			}
			stage = 4
		case "result":
			if stage != 5 || assistant != p.sentinel || projectNativeRuntimeKeys(row, "type timestamp status", "stats") != nil || row["status"] != "success" {
				return "", fmt.Errorf("Gemini terminal result")
			}
			if stats, exists := row["stats"]; exists {
				if err := geminiProjectStats(stats); err != nil {
					return "", err
				}
			}
			stage = 6
		default:
			return "", fmt.Errorf("unknown/unsuccessful Gemini event")
		}
	}
	if stage != 6 {
		return "", fmt.Errorf("Gemini incomplete native session")
	}
	return session, nil
}
func geminiProjectStats(value any) error {
	row, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(row, "total_tokens input_tokens output_tokens cached input duration_ms tool_calls models", "") != nil {
		return fmt.Errorf("Gemini stats fields")
	}
	for key, v := range row {
		if key != "models" && !projectNativeRuntimeNumber(v, 0, 1<<40, true) {
			return fmt.Errorf("Gemini stat number")
		}
	}
	models, ok := row["models"].(map[string]any)
	if !ok || len(models) > 8 {
		return fmt.Errorf("Gemini stats models")
	}
	for name, value := range models {
		m, ok := value.(map[string]any)
		if !ok || name == "" || len(name) > 256 || projectNativeRuntimeKeys(m, "total_tokens input_tokens output_tokens cached input", "") != nil {
			return fmt.Errorf("Gemini model stats")
		}
		for _, v := range m {
			if !projectNativeRuntimeNumber(v, 0, 1<<40, true) {
				return fmt.Errorf("Gemini model stat number")
			}
		}
	}
	return nil
}

func geminiProjectContractRequest(p *geminiProjectProvider) map[string]any {
	return map[string]any{"contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "fixture"}}}}, "tools": []any{map[string]any{"functionDeclarations": []any{map[string]any{"name": p.tool, "description": "Inspect only disposable fixture runtime facts.", "parametersJsonSchema": map[string]any{"type": "object", "properties": map[string]any{"nonce": map[string]any{"type": "string"}, "operation": map[string]any{"type": "string"}, "marker": map[string]any{"type": "string"}, "wait_for_previous": map[string]any{"type": "boolean", "description": geminiProjectWaitDescription}}, "required": []any{"nonce"}, "additionalProperties": false}}}}}}
}
func geminiProjectContractBody(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
func TestGeminiProjectNativeSchemaAcceptsOnlyExactVendorWaitProperty(t *testing.T) {
	p := newGeminiProjectProvider("nonce", "private-fixture-key", func(string) error { return nil })
	args := map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}
	// Independent bytes from native artifact 12677916698/provider-request-1.json
	// and pinned tools.ts blob ab2870984aba77e77fc5bacb25009cd47698582e.
	const observedDescription = "Set to true to wait for all previously requested tools in this turn to complete before starting. Set to false (or omit) to run in parallel. Use true when this tool depends on the output of previous tools."
	for _, kind := range []string{"valid", "unknown", "different-wait", "truncated-wait", "missing-description", "wrong-type", "extra-wait-key", "missing-wait", "foreign-tool", "unknown-schema-key", "missing-schema-field"} {
		m := geminiProjectContractRequest(p)
		function := m["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)[0].(map[string]any)
		schema := function["parametersJsonSchema"].(map[string]any)
		properties := schema["properties"].(map[string]any)
		wait := properties["wait_for_previous"].(map[string]any)
		wait["description"] = observedDescription
		switch kind {
		case "unknown":
			properties["extra"] = map[string]any{"type": "string"}
		case "different-wait":
			wait["description"] = "changed"
		case "truncated-wait":
			wait["description"] = "Set to true to wait for all previously requested tools in this turn to complete before starting. Set to false (or omit) to run in parallel. Use true whe."
		case "missing-description":
			delete(wait, "description")
		case "wrong-type":
			wait["type"] = "string"
		case "extra-wait-key":
			wait["extra"] = true
		case "missing-wait":
			delete(properties, "wait_for_previous")
		case "foreign-tool":
			function["name"] = "mcp_uapgeminib_inspect_runtime"
		case "unknown-schema-key":
			schema["extra"] = true
		case "missing-schema-field":
			delete(schema, "required")
		}
		_, err := geminiProjectRequest(geminiProjectContractBody(t, m), p.tool, p.wireID, args, false)
		if (err == nil) != (kind == "valid") {
			t.Fatalf("%s schema: %v", kind, err)
		}
	}
}
func TestGeminiProjectNativeCallPartRequiresMeasuredSignatureAndWireIdentity(t *testing.T) {
	p := newGeminiProjectProvider("nonce", "private-fixture-key", nil)
	if p.id != "mcp_uapgeminia_inspect_runtime__uap_nonce_attempt" || p.wireID != "uap_nonce_attempt" || p.id == p.wireID {
		t.Fatal("source-defined native and wire identities differ")
	}
	args := map[string]any{"nonce": "nonce", "operation": "write", "marker": "retained-nonce"}
	for _, kind := range []string{"valid", "missing-signature", "empty-signature", "foreign-signature", "null-signature", "bool-signature", "object-signature", "unknown-key", "extra-payload", "text-signature", "response-signature", "wrong-role", "missing-role", "native-id", "foreign-id", "foreign-tool", "foreign-arguments", "null-call"} {
		call := map[string]any{"id": "uap_nonce_attempt", "name": "mcp_uapgeminia_inspect_runtime", "args": map[string]any{"nonce": "nonce", "operation": "write", "marker": "retained-nonce"}}
		part := map[string]any{"functionCall": call, "thoughtSignature": "skip_thought_signature_validator"}
		row := map[string]any{"role": "model", "parts": []any{part}}
		switch kind {
		case "missing-signature":
			delete(part, "thoughtSignature")
		case "empty-signature":
			part["thoughtSignature"] = ""
		case "foreign-signature":
			part["thoughtSignature"] = "unreviewed-signature"
		case "null-signature":
			part["thoughtSignature"] = nil
		case "bool-signature":
			part["thoughtSignature"] = true
		case "object-signature":
			part["thoughtSignature"] = map[string]any{"value": "skip_thought_signature_validator"}
		case "unknown-key":
			part["thought"] = true
		case "extra-payload":
			part["text"] = "extra"
		case "text-signature":
			delete(part, "functionCall")
			part["text"] = "extra"
		case "response-signature":
			delete(part, "functionCall")
			part["functionResponse"] = map[string]any{"id": p.wireID, "name": p.tool, "response": map[string]any{"output": "facts"}}
		case "wrong-role":
			row["role"] = "user"
		case "missing-role":
			delete(row, "role")
		case "native-id":
			call["id"] = p.id
		case "foreign-id":
			call["id"] = "foreign"
		case "foreign-tool":
			call["name"] = "mcp_uapgeminib_inspect_runtime"
		case "foreign-arguments":
			call["args"].(map[string]any)["operation"] = "read"
		case "null-call":
			part["functionCall"] = nil
		}
		_, err := geminiProjectContents([]any{row}, p.tool, p.wireID, args)
		if (err == nil) != (kind == "valid") {
			t.Fatalf("%s call part: %v", kind, err)
		}
	}
}

func TestGeminiProjectNativeProviderRequiresExactReturnWrapperAndIdentity(t *testing.T) {
	for _, kind := range []string{"valid", "wrong-id", "wrong-call-id", "native-call-id", "native-response-id", "wrong-wrapper", "wrong-facts", "wrong-route", "wrong-key"} {
		t.Run(kind, func(t *testing.T) {
			p := newGeminiProjectProvider("nonce", "private-fixture-key", func(body string) error {
				if body != "facts" {
					return fmt.Errorf("facts mismatch")
				}
				return nil
			})
			p.server = &httptest.Server{URL: "http://127.0.0.1:12345"}
			send := func(value any) *httptest.ResponseRecorder {
				route := "/v1beta/models/" + geminiProjectModel + ":streamGenerateContent?alt=sse"
				if kind == "wrong-route" {
					route += "&extra=1"
				}
				target := p.server.URL + route
				r, err := http.NewRequestWithContext(context.Background(), "POST", target, bytes.NewReader(geminiProjectContractBody(t, value)))
				if err != nil {
					t.Fatal(err)
				}
				r.RequestURI = target
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("x-goog-api-key", p.key)
				if kind == "wrong-key" {
					r.Header.Set("x-goog-api-key", "other")
				}
				w := httptest.NewRecorder()
				p.serve(w, r)
				return w
			}
			first := send(geminiProjectContractRequest(p))
			if kind == "wrong-route" || kind == "wrong-key" {
				if first.Code != 400 {
					t.Fatal("unadmitted route/key")
				}
				return
			}
			if first.Code != 200 || strings.Contains(first.Body.String(), p.sentinel) || !strings.Contains(first.Body.String(), `"id":"`+p.id+`"`) {
				t.Fatal("premature sentinel")
			}
			id, output := p.wireID, "<untrusted_context>\nfacts\n</untrusted_context>"
			if kind == "wrong-id" {
				id = "wrong"
			}
			if kind == "native-response-id" {
				id = p.id
			}
			if kind == "wrong-wrapper" {
				output = "facts"
			}
			if kind == "wrong-facts" {
				output = strings.ReplaceAll(output, "facts", "wrong")
			}
			m := geminiProjectContractRequest(p)
			callID := p.wireID
			switch kind {
			case "wrong-call-id":
				callID = "wrong"
			case "native-call-id":
				callID = p.id
			}
			m["contents"] = []any{map[string]any{"role": "model", "parts": []any{map[string]any{"functionCall": map[string]any{"id": callID, "name": p.tool, "args": map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}}, "thoughtSignature": "skip_thought_signature_validator"}}}, map[string]any{"role": "user", "parts": []any{map[string]any{"functionResponse": map[string]any{"id": id, "name": p.tool, "response": map[string]any{"output": output}}}}}}
			second := send(m)
			if kind == "valid" {
				if second.Code != 200 || !p.returned || !strings.Contains(second.Body.String(), p.sentinel) {
					t.Fatal("valid inert Gemini exchange rejected")
				}
			} else if second.Code != 400 || p.returned {
				t.Fatal("invalid returned result admitted")
			}
		})
	}
}
func TestGeminiProjectNativeStreamRejectsWrongIdentityOrderAndTerminal(t *testing.T) {
	p := newGeminiProjectProvider("nonce", "private-fixture-key", nil)
	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	for _, kind := range []string{"valid", "wrong-id", "wire-id", "wrong-order", "bad-terminal", "missing-result", "unknown-field"} {
		rows := []map[string]any{{"type": "init", "timestamp": timestamp, "session_id": "native-session", "model": geminiProjectModel}, {"type": "message", "timestamp": timestamp, "role": "user", "content": "fixture"}, {"type": "tool_use", "timestamp": timestamp, "tool_name": p.tool, "tool_id": p.id, "parameters": map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}}, {"type": "tool_result", "timestamp": timestamp, "tool_id": p.id, "status": "success", "output": "facts"}, {"type": "message", "timestamp": timestamp, "role": "assistant", "content": p.sentinel, "delta": true}, {"type": "result", "timestamp": timestamp, "status": "success"}}
		switch kind {
		case "wrong-id":
			rows[3]["tool_id"] = "other"
		case "wire-id":
			rows[2]["tool_id"], rows[3]["tool_id"] = p.wireID, p.wireID
		case "wrong-order":
			rows[2], rows[3] = rows[3], rows[2]
		case "bad-terminal":
			rows[5]["status"] = "error"
		case "missing-result":
			rows = rows[:5]
		case "unknown-field":
			rows[0]["unknown"] = true
		}
		var body bytes.Buffer
		for _, row := range rows {
			_ = json.NewEncoder(&body).Encode(row)
		}
		session, err := geminiProjectStream(body.Bytes(), p, "fixture", func(body string) error {
			if body != "facts" {
				return fmt.Errorf("facts")
			}
			return nil
		})
		if (err == nil) != (kind == "valid") || (kind == "valid" && session != "native-session") {
			t.Fatalf("%s stream: %v", kind, err)
		}
	}
}

func TestGeminiProjectNativePrivateKeyRedactionAndClosedAdmission(t *testing.T) {
	key := "private-token-canary-uap"
	f := &geminiProjectAttempt{secret: key, files: map[string][]byte{}}
	f.keep(t, "native-attempt.stdout.log", []byte("native "+key+" output"))
	f.keep(t, "provider-request-1.json", []byte(`{"text":"`+key+`"}`))
	if err := geminiProjectSecretFree(f.files, []string{key}); err != nil {
		t.Fatal(err)
	}
	for _, body := range f.files {
		if !bytes.Contains(body, []byte("[redacted]")) {
			t.Fatal("private fixture key not redacted")
		}
	}
	for _, files := range []map[string][]byte{{"native-attempt.stderr.log": []byte(key)}, {"runtime-attempt.json": []byte(`{"key":"` + key + `"}`)}, {key + ".log": []byte("safe")}} {
		if err := geminiProjectSecretFree(files, []string{key}); err == nil {
			t.Fatal("known private token canary admitted")
		}
	}
}
