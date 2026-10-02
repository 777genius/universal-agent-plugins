package pluginkitai_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/cursor"
)

// Synthetic documented native stop input, not native Cursor qualification.
const cursorInput = `{"conversation_id":"TEST_CONVERSATION","generation_id":"TEST_GENERATION","hook_event_name":"stop","cursor_version":"TEST_BUILD","workspace_roots":["/TEST/workspace"],"model":"TEST_MODEL","model_id":"TEST_MODEL_ID","model_params":[{"id":"temperature","value":"0.5"}],"user_email":null,"transcript_path":"/TEST/transcript","status":"completed","loop_count":0,"future":{"ignored":true}}`

// Red: the independent Go1.22 public consumer cannot compile or resolve the
// beta stop registrar/runner, or the ordinary engine steals a legacy invocation.
func buildCursorConsumer(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"main.go", "go.mod"} {
		b, err := os.ReadFile(filepath.Join("testdata/cursor-observer", name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "go.mod" {
			root, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			b = append(b, []byte("\nreplace github.com/777genius/plugin-kit-ai/sdk => "+filepath.ToSlash(root)+"\n")...)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(dir, "observer")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := []string{"build", "-p=2", "-o", binary}
	if cursorConsumerRace {
		args = append(args, "-race")
	}
	args = append(args, ".")
	cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin/go"), args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("public module build: %v\n%s", err, out)
	}
	return binary
}

type cursorRecord struct {
	Kind   string           `json:"kind"`
	Event  cursor.StopEvent `json:"event"`
	Clean  bool             `json:"clean"`
	Code   int              `json:"code"`
	Stacks string           `json:"stacks,omitempty"`
}

func cursorRecords(t *testing.T, b string) []cursorRecord {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(b))
	var records []cursorRecord
	for dec.More() {
		var r cursorRecord
		if err := dec.Decode(&r); err != nil {
			t.Fatalf("private diagnostics or invalid record: %q: %v", b, err)
		}
		records = append(records, r)
	}
	return records
}

func runCursorConsumer(t *testing.T, binary, selector, mode, input string) (int, string, []cursorRecord) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, selector, mode)
	cmd.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0")
	cmd.Stdin = strings.NewReader(input)
	var out, diag bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diag
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		code = exit.ExitCode()
	}
	if ctx.Err() != nil {
		t.Fatal("consumer failed bounded process cleanup")
	}
	return code, out.String(), cursorRecords(t, diag.String())
}

// Red: writable stdout lacks exactly one fixed response on size/decode/panic/
// callback/cancellation failures, unsafe facts reach callback, or IO leaks after
// return. Old process ReadAll cannot recover a deadline while stdin stays open.
func TestCursorObserverProcess(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("inherited pipe deadlines require Linux/macOS; other native transports unqualified")
	}
	binary := buildCursorConsumer(t)
	cases := []struct {
		name, mode, input string
		callback          bool
	}{
		{"valid", "normal", cursorInput, true},
		{"injected owned pipes", "owned", cursorInput, true},
		{"nil", "nil", cursorInput, true},
		{"simple registrar", "simple", cursorInput, true},
		{"panic", "panic", cursorInput, true},
		{"callback error", "error", cursorInput, true},
		{"cooperative cancellation", "cooperative", cursorInput, true},
		{"canceled context", "canceled", cursorInput, false},
		{"malformed", "normal", `{"conversation_id":`, false},
		{"oversize", "normal", strings.Repeat("x", pluginkitai.MaxPayloadBytes+1), false},
		{"trailing value", "normal", cursorInput + ` {}`, false},
		{"non object", "normal", `[]`, false},
		{"empty IDs", "normal", strings.Replace(cursorInput, "TEST_CONVERSATION", "", 1), false},
		{"control ID", "normal", strings.Replace(cursorInput, "TEST_GENERATION", `bad\nID`, 1), false},
		{"bounded ID", "normal", strings.Replace(cursorInput, "TEST_GENERATION", strings.Repeat("x", 257), 1), false},
		{"wrong event", "normal", strings.Replace(cursorInput, `"hook_event_name":"stop"`, `"hook_event_name":"preToolUse"`, 1), false},
		{"negative loop", "normal", strings.Replace(cursorInput, `"loop_count":0`, `"loop_count":-1`, 1), false},
		{"overflow loop", "normal", strings.Replace(cursorInput, `"loop_count":0`, `"loop_count":2147483648`, 1), false},
		{"status scalar", "normal", strings.Replace(cursorInput, `"status":"completed"`, `"status":false`, 1), false},
		{"param scalar", "normal", strings.Replace(cursorInput, `"value":"0.5"`, `"value":0.5`, 1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out, records := runCursorConsumer(t, binary, "CursorStop", tc.mode, tc.input)
			want := 1
			if tc.callback {
				want++
			}
			if code != 0 || out != "{}\n" || len(records) != want || !records[len(records)-1].Clean || records[len(records)-1].Code != 0 {
				t.Fatalf("exit=%d stdout=%q records=%+v", code, out, records)
			}
			if tc.callback && records[0].Kind != "cursor" {
				t.Fatal("wrong registrar dispatch")
			}
		})
	}
	t.Run("native fields and pointer presence", func(t *testing.T) {
		_, _, records := runCursorConsumer(t, binary, "cUrSoRsToP", "normal", cursorInput)
		e := records[0].Event
		if e.ConversationID != "TEST_CONVERSATION" || e.GenerationID != "TEST_GENERATION" || e.CursorVersion != "TEST_BUILD" || e.HookEventName != "stop" || len(e.WorkspaceRoots) != 1 || e.Model != "TEST_MODEL" || e.ModelID != "TEST_MODEL_ID" || len(e.ModelParams) != 1 || e.ModelParams[0].Value != "0.5" || e.UserEmail != nil || e.TranscriptPath == nil || *e.TranscriptPath != "/TEST/transcript" || e.LoopCount == nil || *e.LoopCount != 0 || e.Status != cursor.StopStatusCompleted {
			t.Fatalf("lost native fields: %+v", e)
		}
	})
	// Red: future/missing/null statuses become completed or are rejected despite
	// being valid unknown facts; omitted loop/email presence becomes native zero.
	for _, status := range []string{`"FutureStatus"`, `null`, `"aborted"`, `"error"`} {
		t.Run("status "+status, func(t *testing.T) {
			input := strings.Replace(cursorInput, `"completed"`, status, 1)
			code, out, records := runCursorConsumer(t, binary, "CursorStop", "normal", input)
			if code != 0 || out != "{}\n" || len(records) != 2 || records[0].Event.Status != cursor.StopStatus(strings.Trim(status, `"`)) && (status != `null` || records[0].Event.Status != "") {
				t.Fatal("status changed or rejected")
			}
		})
	}
	t.Run("missing optional fields", func(t *testing.T) {
		input := `{"conversation_id":"TEST_C","generation_id":"TEST_G","hook_event_name":"stop"}`
		_, out, records := runCursorConsumer(t, binary, "CursorStop", "normal", input)
		if out != "{}\n" || len(records) != 2 || records[0].Event.Status != "" || records[0].Event.LoopCount != nil || records[0].Event.TranscriptPath != nil {
			t.Fatal("omitted fields lost presence")
		}
	})
	t.Run("open stdin without EOF", func(t *testing.T) { cursorOpenPipe(t, binary, "", "normal", false) })
	t.Run("valid frame without EOF", func(t *testing.T) { cursorOpenPipe(t, binary, cursorInput, "normal", false) })
	t.Run("oversize without EOF", func(t *testing.T) {
		cursorOpenPipe(t, binary, strings.Repeat("x", pluginkitai.MaxPayloadBytes+1), "normal", true)
	})
	// Red: cancellation during a blocked read waits for the original deadline.
	t.Run("cancel open stdin", func(t *testing.T) { cursorOpenPipe(t, binary, "", "cancel-reading", true) })
	// Red: the bounded cleanup observation admits an intentionally live goroutine,
	// omits its failure stacks, or waits indefinitely rather than reporting a leak.
	t.Run("leaking cleanup probe", func(t *testing.T) {
		start := time.Now()
		code, out, records := runCursorConsumer(t, binary, "CursorStop", "leak-probe", cursorInput)
		if code != 0 || out != "{}\n" || len(records) != 2 || records[0].Kind != "cursor" || records[1].Clean || records[1].Code != 0 || !strings.Contains(records[1].Stacks, "[select (no cases)]") || time.Since(start) > time.Second {
			t.Fatalf("leak observation: exit=%d stdout=%q records=%+v", code, out, records)
		}
	})
}

func cursorOpenPipe(t *testing.T, binary, input, mode string, oversize bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "CursorStop", mode)
	cmd.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()
	cmd.Stdin = r
	var out, diag bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diag
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if input != "" {
		if _, err := w.Write([]byte(input)); err != nil {
			t.Fatal(err)
		}
	}
	// The writer remains open through Wait; no EOF can accidentally satisfy read.
	if err := cmd.Wait(); err != nil {
		t.Fatalf("no-EOF process: %v, %q", err, diag.String())
	}
	records := cursorRecords(t, diag.String())
	if out.String() != "{}\n" || len(records) != 1 || !records[0].Clean || time.Since(start) > time.Second {
		t.Fatalf("no-EOF response/cleanup: %q %+v", out.String(), records)
	}
	if oversize && time.Since(start) > 250*time.Millisecond {
		t.Fatal("size limit waited for EOF/deadline")
	}
}
