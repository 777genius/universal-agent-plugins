//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCopilotCycleNoCallDeclarationNeverFiltersForeignTools(t *testing.T) {
	_, p, _ := copilotReadinessActual(t)
	for _, kind := range []string{"absent", "empty", "null", "foreign", "required", "duplicate"} {
		value, err := projectNativeRuntimeJSON(p.requests[0])
		if err != nil {
			t.Fatal(err)
		}
		row := value.(map[string]any)
		delete(row, "tools")
		switch kind {
		case "empty":
			row["tools"] = []any{}
		case "null":
			row["tools"] = nil
		case "foreign":
			row["tools"] = []any{map[string]any{"type": "function"}}
		case "required":
			row["tool_choice"] = "required"
		case "duplicate":
			row["tools"] = []any{}
		}
		raw, _ := json.Marshal(row)
		if kind == "duplicate" {
			raw = append([]byte(`{"tools":[],`), raw[1:]...)
		}
		_, err = copilotCycleRequest(raw, "")
		if (err == nil) != (kind == "absent" || kind == "empty") {
			t.Fatal("no-call declaration widened", kind, err)
		}
	}
	c := copilotCycleInert(t)
	r := c.recipes["a"]
	for _, args := range [][]string{c.nativeArgs("installed-a", r), c.nativeArgs("empty-a", nil)} {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--available-tools") || !strings.Contains(joined, "--excluded-tools="+strings.ReplaceAll(strings.TrimPrefix(copilotReadinessDisabled, "Disabled tools: "), ", ", ",")) {
			t.Fatal("native absence uses filtered availability")
		}
	}
}

func TestCopilotCycleProviderActualHandlerKeepsOneOrTwoExactExchanges(t *testing.T) {
	_, old, _ := copilotReadinessActual(t)
	for _, empty := range []bool{false, true} {
		tool := "uapcopilota-inspect_runtime"
		if empty {
			tool = ""
		}
		p := newCopilotCycleProvider(old.nonce, "inert", tool, "read", func(string) error { return nil })
		p.url = "http://127.0.0.1:39999"
		first, _ := projectNativeRuntimeJSON(old.requests[0])
		row := first.(map[string]any)
		if empty {
			delete(row, "tools")
		}
		raw, _ := json.Marshal(row)
		recorder := httptest.NewRecorder()
		request := copilotCycleInertRequest(t, p.url, raw)
		request.Header.Set("Content-Type", "application/json")
		p.serve(recorder, request)
		if recorder.Code != 200 || len(p.p.requests) != 1 || len(p.responses) != 1 || p.p.err != nil || p.p.returned != empty {
			t.Fatal("inert first provider exchange refused", p.p.err)
		}
		if !empty {
			second, _ := projectNativeRuntimeJSON(old.requests[1])
			m := second.(map[string]any)
			messages := m["messages"].([]any)
			arguments, _ := json.Marshal(p.arguments())
			messages[2] = map[string]any{"role": "assistant", "content": nil, "refusal": nil, "tool_calls": []any{map[string]any{"id": p.p.id, "type": "function", "function": map[string]any{"name": tool, "arguments": string(arguments)}}}}
			messages[3] = map[string]any{"role": "tool", "tool_call_id": p.p.id, "content": "inert result"}
			body, _ := json.Marshal(m)
			recorder = httptest.NewRecorder()
			request = copilotCycleInertRequest(t, p.url, body)
			request.Header.Set("Content-Type", "application/json")
			p.serve(recorder, request)
			if recorder.Code != 200 || len(p.responses) != 2 || !p.p.returned || p.p.err != nil {
				t.Fatal("inert matching returned read exchange refused", p.p.err)
			}
		}
		recorder = httptest.NewRecorder()
		request = copilotCycleInertRequest(t, p.url, raw)
		request.Header.Set("Content-Type", "application/json")
		p.serve(recorder, request)
		if recorder.Code != 400 || p.p.err == nil {
			t.Fatal("extra provider turn accepted")
		}
	}
}

func TestCopilotCyclePrivacyBeforeBase64RefusesEscapedCanaryAndBounds(t *testing.T) {
	for _, raw := range [][]byte{[]byte("uap-copilot-private-test"), []byte(`{"value":"uap-copilot-private-test"}`), []byte(`{"value":"uap-copilot-\u0070rivate-test"}`), bytes.Repeat([]byte{'x'}, (1<<20)+1), {0xff}} {
		if !copilotCyclePrivate(raw) {
			t.Fatal("private/unbounded raw accepted")
		}
	}
	for _, raw := range [][]byte{nil, []byte("ordinary fixture bytes\n"), []byte(`{"safe":"fixture"}`)} {
		if copilotCyclePrivate(raw) {
			t.Fatal("safe raw refused")
		}
	}
	stream := copilotCycleStream(t, nil)
	raw, _ := json.Marshal(stream)
	if !bytes.Contains(raw, []byte(`"body":""`)) {
		t.Fatal("empty stream became null")
	}
}

func TestCopilotCycleOriginalForeignIdentityCannotBeReplacedWithSameBytes(t *testing.T) {
	c := copilotCycleInert(t)
	path := filepath.Join(c.f.base.Home, ".copilot", "skills", "uap-foreign-user", "SKILL.md")
	raw, err := projectNativeRuntimeRead(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	replacement := path + ".replacement"
	projectNativeRuntimeWrite(t, replacement, raw, 0600)
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if c.authority() == nil {
		t.Fatal("same-byte foreign replacement adopted")
	}
}

func TestCopilotCycleCaptureAndLateFailureCannotQualify(t *testing.T) {
	files := map[string][]byte{}
	for _, label := range copilotCyclePublic {
		for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
			files[label+suffix] = []byte{}
		}
	}
	for _, name := range copilotCycleAggregates {
		files[name] = []byte{}
	}
	if !copilotCycleCapture(files) {
		t.Fatal("complete member set refused")
	}
	delete(files, "git-a.stdout.log")
	files["foreign"] = nil
	if copilotCycleCapture(files) {
		t.Fatal("same-count foreign member accepted")
	}
	for _, flags := range [][6]bool{{true, false, true, true, true, true}, {false, false, true, true, true, true}, {true, true, true, true, true, true}, {true, false, false, true, true, true}, {true, false, true, false, true, true}, {true, false, true, true, false, true}, {true, false, true, true, true, false}} {
		got := copilotCycleComplete(flags[0], flags[1], flags[2], flags[3], flags[4], flags[5], time.Second)
		if got != (flags == [6]bool{true, false, true, true, true, true}) {
			t.Fatal("late failed/unfinished closure accepted")
		}
	}
	for _, elapsed := range []time.Duration{-1, 180 * time.Second, 181 * time.Second} {
		if copilotCycleComplete(true, false, true, true, true, true, elapsed) {
			t.Fatal("original total deadline widened")
		}
	}
}

func copilotCycleInertRequest(t *testing.T, url string, body []byte) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), "POST", url+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return request
}
