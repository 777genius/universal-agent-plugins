package pluginkitai_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/gemini"
)

// Synthetic native-shape input derived from official Gemini CLI v0.62.0
// (b460678f3db508407554afd604cc9d6635becb2a), NotificationInput/HookInput
// and fireNotificationEvent. This is injected_contract, not native CLI evidence.
const notificationInput = `{"session_id":"fixture-session","transcript_path":"/synthetic/transcript.json","cwd":"/synthetic/project","hook_event_name":"Notification","timestamp":"2026-10-01T00:00:00.000Z","notification_type":"ToolPermission","message":"synthetic permission request","details":{"toolName":"fixture_tool","future":{"nested":[true,null,9007199254740993]}},"future_top_level":{"ignored":true}}`

// Red: public metadata omits the new runtime event, promotes it without native
// qualification, or changes maturity/count for the nine existing Gemini hooks.
func TestGeminiNotificationSupportContract(t *testing.T) {
	stable := map[string]bool{
		"SessionStart": false, "SessionEnd": false, "BeforeModel": false,
		"AfterModel": false, "BeforeToolSelection": false, "BeforeAgent": false,
		"AfterAgent": false, "BeforeTool": false, "AfterTool": false,
	}
	count, observers := 0, 0
	for _, entry := range pluginkitai.Supported() {
		if entry.Platform != "gemini" {
			continue
		}
		count++
		if entry.Status != "runtime_supported" || entry.Carrier != "stdin_json" {
			t.Fatalf("wrong runtime contract for %s", entry.Event)
		}
		if entry.Event == "Notification" {
			observers++
			if entry.Maturity != "beta" || entry.V1Target {
				t.Fatal("unqualified observer entered the stable contract")
			}
		} else {
			seen, exists := stable[string(entry.Event)]
			if !exists || seen || entry.Maturity != "stable" || !entry.V1Target {
				t.Fatalf("existing stable Gemini contract changed for %s", entry.Event)
			}
			stable[string(entry.Event)] = true
		}
	}
	if count != 10 || observers != 1 {
		t.Fatalf("Gemini events=%d observers=%d", count, observers)
	}
}

// Red: a reusable public consumer cannot dispatch GeminiNotification, loses
// native fields/future details, or emits a tool decision instead of neutral {}.
func TestGeminiObserverProcess(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "observer")
	build := exec.Command("go", "build", "-p", "2", "-o", binary, "./testdata/gemini-observer")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build independent public consumer: %v\n%s", err, out)
	}
	type observation struct {
		Kind     string          `json:"kind"`
		Accepted bool            `json:"accepted"`
		Event    json.RawMessage `json:"event"`
	}
	run := func(t *testing.T, selector, input string, extra ...string) (int, string, string) {
		t.Helper()
		cmd := exec.Command(binary, append([]string{selector}, extra...)...)
		cmd.Stdin = strings.NewReader(input)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		code := 0
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatal(err)
			}
			code = exitErr.ExitCode()
		}
		return code, stdout.String(), stderr.String()
	}
	readObservation := func(t *testing.T, selector, input string, extra ...string) observation {
		t.Helper()
		code, stdout, stderr := run(t, selector, input, extra...)
		if code != 0 || stdout != "{}" {
			t.Fatalf("exit=%d, output=%q; want exit 0 and {}", code, stdout)
		}
		var got observation
		// A canonical-name mismatch warning would break this JSON-only record.
		if err := json.Unmarshal([]byte(stderr), &got); err != nil {
			t.Fatalf("expected one synthetic consumer observation: %v", err)
		}
		return got
	}
	t.Run("native fields and neutral response", func(t *testing.T) {
		got := readObservation(t, "GeminiNotification", notificationInput)
		var event gemini.NotificationEvent
		if err := json.Unmarshal(got.Event, &event); err != nil {
			t.Fatal(err)
		}
		if got.Kind != "Notification" || !got.Accepted || event.NotificationType != gemini.NotificationTypeToolPermission ||
			event.SessionID != "fixture-session" || event.Timestamp != "2026-10-01T00:00:00.000Z" ||
			event.HookEventName != "Notification" || event.CWD != "/synthetic/project" || event.TranscriptPath != "/synthetic/transcript.json" ||
			event.Message != "synthetic permission request" || string(event.Details["toolName"]) != `"fixture_tool"` ||
			string(event.Details["future"]) != `{"nested":[true,null,9007199254740993]}` {
			t.Fatal("native fields or future detail values were lost")
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(got.Event, &fields); err != nil {
			t.Fatal(err)
		}
		if _, retained := fields["future_top_level"]; retained {
			t.Fatal("unknown top-level field changed the existing decoder policy")
		}
	})
	t.Run("case folded selector and nil response", func(t *testing.T) {
		got := readObservation(t, "gEmInInOtIfIcAtIoN", notificationInput, "nil")
		if got.Kind != "Notification" || !got.Accepted {
			t.Fatal("wrong dispatch")
		}
	})
	// Red: unknown subtype is rejected, coerced into ToolPermission, or handled
	// as a permission notification by a consumer that knows only ToolPermission.
	t.Run("consumer ignores future subtype", func(t *testing.T) {
		input := strings.Replace(notificationInput, `"ToolPermission"`, `"FutureNotification"`, 1)
		got := readObservation(t, "GeminiNotification", input)
		var event gemini.NotificationEvent
		if err := json.Unmarshal(got.Event, &event); err != nil {
			t.Fatal(err)
		}
		if got.Accepted || event.NotificationType != gemini.NotificationType("FutureNotification") {
			t.Fatal("future subtype was rejected, changed or accepted as permission")
		}
	})
	// Red: new dispatch changes existing argv-first/native-name admission policy.
	// Gemini decoders return the descriptor name, so DTO name mismatch currently
	// neither rejects input nor warns. Trusted consumers must check the DTO name.
	t.Run("wrong native name preserves current decoder policy", func(t *testing.T) {
		input := strings.Replace(notificationInput, `"hook_event_name":"Notification"`, `"hook_event_name":"AfterAgent"`, 1)
		got := readObservation(t, "GeminiNotification", input)
		var event gemini.NotificationEvent
		if err := json.Unmarshal(got.Event, &event); err != nil {
			t.Fatal(err)
		}
		if got.Kind != "Notification" || event.HookEventName != "AfterAgent" {
			t.Fatal("SDK no longer dispatches by trusted argv or loses native name")
		}
	})
	t.Run("existing AfterAgent consumer", func(t *testing.T) {
		input := `{"session_id":"fixture-session","hook_event_name":"AfterAgent","timestamp":"2026-10-01T00:00:01Z","prompt":"synthetic prompt","prompt_response":"synthetic response","stop_hook_active":true}`
		got := readObservation(t, "GeminiAfterAgent", input)
		var event gemini.AfterAgentEvent
		if err := json.Unmarshal(got.Event, &event); err != nil {
			t.Fatal(err)
		}
		if got.Kind != "AfterAgent" || !event.StopHookActive || event.Prompt != "synthetic prompt" || event.PromptResponse != "synthetic response" || event.Timestamp != "2026-10-01T00:00:01Z" {
			t.Fatal("existing AfterAgent behavior changed")
		}
	})
	t.Run("wrong valid selector dispatches by argv", func(t *testing.T) {
		got := readObservation(t, "GeminiAfterAgent", notificationInput)
		var event gemini.AfterAgentEvent
		if err := json.Unmarshal(got.Event, &event); err != nil {
			t.Fatal(err)
		}
		if got.Kind != "AfterAgent" || event.HookEventName != "Notification" {
			t.Fatal("selector policy changed")
		}
	})
	t.Run("bare Notification retains Claude route", func(t *testing.T) {
		code, stdout, stderr := run(t, "Notification", notificationInput)
		if code != 1 || stdout != "" || !strings.Contains(stderr, "no handler registered for claude/Notification") {
			t.Fatal("bare Notification must select Claude, not the registered Gemini observer")
		}
	})
	t.Run("unknown selector", func(t *testing.T) {
		code, stdout, stderr := run(t, "GeminiNotARealEvent", notificationInput)
		if code != 1 || stdout != "" || !strings.Contains(stderr, "unknown invocation") {
			t.Fatal("unknown selector reached a handler")
		}
	})
	// Red: malformed/type/trailing/oversize input reaches a callback or emits {}.
	// This uses real process stdin and the shared size-limited JSON decoder.
	badInputs := []struct{ name, input, diagnostic string }{
		{"malformed", `{"session_id":`, "decode Gemini notification input"},
		{"non object", `[]`, "decode Gemini notification input"},
		{"message type", `{"message":42}`, "decode Gemini notification input"},
		{"subtype type", `{"notification_type":false}`, "decode Gemini notification input"},
		{"base field type", `{"session_id":[]}`, "decode Gemini notification input"},
		{"timestamp type", `{"timestamp":17}`, "decode Gemini notification input"},
		{"native name type", `{"hook_event_name":{}}`, "decode Gemini notification input"},
		{"details type", `{"details":[]}`, "decode Gemini notification input"},
		{"trailing object", notificationInput + ` {}`, "decode Gemini notification input"},
		{"oversize", `{"message":"` + strings.Repeat("x", pluginkitai.MaxPayloadBytes) + `"}`, "exceeds max payload size"},
	}
	for _, tc := range badInputs {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := run(t, "GeminiNotification", tc.input)
			if code != 1 || stdout != "" || !strings.Contains(stderr, tc.diagnostic) {
				t.Fatal("invalid input did not fail under the existing SDK policy")
			}
		})
	}
	// Red: this hook accidentally introduces stricter required-field/null/duplicate
	// parsing than the existing encoding/json-based Gemini decoder contract.
	t.Run("missing fields use zero values", func(t *testing.T) {
		got := readObservation(t, "GeminiNotification", `{}`)
		if got.Accepted || got.Kind != "Notification" {
			t.Fatal("missing subtype was accepted as permission")
		}
	})
	t.Run("duplicate keys retain encoding json policy", func(t *testing.T) {
		got := readObservation(t, "GeminiNotification", `{"message":"first","message":"last","details":null,"notification_type":null}`)
		var event gemini.NotificationEvent
		if err := json.Unmarshal(got.Event, &event); err != nil {
			t.Fatal(err)
		}
		if got.Accepted || event.Message != "last" || event.Details != nil || event.NotificationType != "" {
			t.Fatal("existing duplicate/null decode behavior changed")
		}
	})
}
