//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"
)

const clineMeasuredNonce = "c8e15147031de0ef6cf68dc33626ead8"
const clineMeasuredProject = "/home/runner/work/_temp/project-runtime-cline/tmp/TestAgentpluginsClineNativeProjectRuntimeAttempt420864893/001/project"

func clineMeasuredBytes(t *testing.T, name, digest string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/cline-3.0.73-installed-acp/" + name)
	if err != nil || clineAttemptSHA(raw) != digest {
		t.Fatal("exact current-head measured Cline transcript differs", name, err)
	}
	return raw
}

func clineMeasuredTranscript(t *testing.T) ([]map[string]any, []byte) {
	t.Helper()
	raw := clineMeasuredBytes(t, "native-acp.jsonl", "1a4e9a20fe5958b9232bd8d3f1e46c42382c66c61807a91ee133bb39b2a3c8af")
	frames := make([]map[string]any, 0, 10)
	for _, line := range bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'}) {
		value, err := projectNativeRuntimeJSON(line)
		if err != nil {
			t.Fatal(err)
		}
		frame, ok := value.(map[string]any)
		if !ok {
			t.Fatal("measured frame object required")
		}
		frames = append(frames, frame)
	}
	return frames, raw
}

func clineMeasuredAttempt() *clineProjectAttempt {
	p := &clineProjectProvider{nonce: clineMeasuredNonce, callID: "uap-cline-call-" + clineMeasuredNonce,
		marker: "retained-" + clineMeasuredNonce, prompt: "UAP_CLINE_PROMPT_" + clineMeasuredNonce, issued: true}
	return &clineProjectAttempt{base: &nativeFixture{Project: clineMeasuredProject, Nonce: clineMeasuredNonce},
		provider: p, evidence: map[string]any{}}
}

func TestClineProjectNativeMeasuredGrammarReplaysActualInstalledExchange(t *testing.T) {
	_, raw := clineMeasuredTranscript(t)
	f := clineMeasuredAttempt()
	var output, transcript bytes.Buffer
	if err := f.acp(&output, bytes.NewReader(raw), &transcript); err != nil {
		t.Fatal("actual current-head native grammar refused", err)
	}
	if !bytes.Equal(transcript.Bytes(), raw) || f.evidence["native_session_id"] != "1793077736317_iF4rU_cli" {
		t.Fatal("actual transcript or native-generated session changed")
	}
	events := clineMeasuredBytes(t, "probe-events.jsonl", "40f6f77054109fc93e141d1721efd9d50a32676c2631eff1323670fbf1c97284")
	start, err := time.Parse(time.RFC3339Nano, "2026-10-27T05:08:55.580929395Z")
	if err != nil {
		t.Fatal(err)
	}
	end, err := time.Parse(time.RFC3339Nano, "2026-10-27T05:08:56.878857167Z")
	if err != nil {
		t.Fatal(err)
	}
	if err := clineAttemptEvents(events, clineMeasuredNonce, f.nativeFacts, start, end); err != nil {
		t.Fatal("actual independent probe/native facts did not join", err)
	}
	clineMeasuredOutgoing(t, f, output.Bytes())
}

func clineMeasuredOutgoing(t *testing.T, f *clineProjectAttempt, raw []byte) {
	t.Helper()
	session := "1793077736317_iF4rU_cli"
	want := []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": 1, "clientCapabilities": map[string]any{}, "clientInfo": map[string]string{"name": "uap-native-research", "version": "1.0.0"}}},
		{"jsonrpc": "2.0", "id": 2, "method": "session/new", "params": map[string]any{"cwd": clineMeasuredProject, "mcpServers": []any{}}},
		{"jsonrpc": "2.0", "id": 3, "method": "session/prompt", "params": map[string]any{"sessionId": session, "prompt": []any{map[string]string{"type": "text", "text": f.provider.prompt + " Call only " + clineAttemptTool + " with the exact supplied harmless nonce/operation/marker. Do not call built-in tools or any other MCP server."}}}},
		{"jsonrpc": "2.0", "id": 0, "result": map[string]any{"outcome": map[string]string{"outcome": "selected", "optionId": "allow_once"}}},
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	if len(lines) != len(want) {
		t.Fatal("actual grammar emitted extra client authority")
	}
	for index, expected := range want {
		body, err := json.Marshal(expected)
		if err != nil {
			t.Fatal(err)
		}
		actual, err := projectNativeRuntimeJSON(lines[index])
		if err != nil {
			t.Fatal(err)
		}
		value, err := projectNativeRuntimeJSON(body)
		if err != nil || !reflect.DeepEqual(actual, value) {
			t.Fatal("native replay changed exact client operation", index, err)
		}
	}
}

func clineMeasuredUpdate(frames []map[string]any, index int) map[string]any {
	return frames[index]["params"].(map[string]any)["update"].(map[string]any)
}

func clineMeasuredIdentityMutation(frames []map[string]any, kind string) bool {
	switch kind {
	case "agent":
		frames[0]["result"].(map[string]any)["agentInfo"].(map[string]any)["name"] = "foreign"
	case "session":
		frames[3]["params"].(map[string]any)["sessionId"] = "foreign"
	case "tool":
		clineMeasuredUpdate(frames, 2)["title"] = "run_commands"
	case "call":
		clineMeasuredUpdate(frames, 4)["toolCallId"] = "foreign"
	case "arguments":
		frames[3]["params"].(map[string]any)["toolCall"].(map[string]any)["rawInput"].(map[string]any)["operation"] = "read"
	case "missing-sentinel":
		clineMeasuredUpdate(frames, 7)["content"].(map[string]any)["text"] = "foreign"
	case "unknown-key":
		frames[6]["extra"] = true
	case "unknown-method":
		frames[7]["method"] = "session/unknown"
	case "failed-tool":
		clineMeasuredUpdate(frames, 6)["rawOutput"].(map[string]any)["isError"] = true
	default:
		return false
	}
	return true
}

func clineMeasuredOrderMutation(frames []map[string]any, kind string) []map[string]any {
	switch kind {
	case "duplicate-permission":
		return slices.Insert(frames, 4, frames[3])
	case "missing-pending":
		return slices.Delete(frames, 2, 3)
	case "early-progress":
		frames[3], frames[4] = frames[4], frames[3]
	case "duplicate-progress":
		return slices.Insert(frames, 5, frames[4])
	case "missing-progress":
		return slices.Delete(frames, 4, 5)
	case "early-completion":
		frames[5], frames[6] = frames[6], frames[5]
	case "duplicate-completion":
		return slices.Insert(frames, 7, frames[6])
	case "missing-completion":
		return slices.Delete(frames, 6, 7)
	case "early-message":
		frames[6], frames[7] = frames[7], frames[6]
	case "missing-info":
		return slices.Delete(frames, 8, 9)
	case "duplicate-info":
		return slices.Insert(frames, 9, frames[8])
	case "missing-result":
		return frames[:9]
	}
	return frames
}

func TestClineProjectNativeMeasuredGrammarRejectsDifferentialHostileFrames(t *testing.T) {
	for _, kind := range []string{"agent", "session", "tool", "call", "arguments", "duplicate-permission", "missing-pending", "early-progress", "duplicate-progress", "missing-progress", "early-completion", "duplicate-completion", "missing-completion", "early-message", "missing-sentinel", "missing-info", "duplicate-info", "missing-result", "unknown-key", "unknown-method", "failed-tool"} {
		t.Run(kind, func(t *testing.T) {
			frames, _ := clineMeasuredTranscript(t)
			if !clineMeasuredIdentityMutation(frames, kind) {
				frames = clineMeasuredOrderMutation(frames, kind)
			}
			var input, output, transcript bytes.Buffer
			for _, frame := range frames {
				body, err := json.Marshal(frame)
				if err != nil {
					t.Fatal(err)
				}
				input.Write(body)
				input.WriteByte('\n')
			}
			if err := clineMeasuredAttempt().acp(&output, &input, &transcript); err == nil {
				t.Fatal("hostile measured transcript was accepted", kind)
			}
		})
	}
}

func TestClineProjectNativeMeasuredGrammarCannotJoinChangedProbeFacts(t *testing.T) {
	_, raw := clineMeasuredTranscript(t)
	f := clineMeasuredAttempt()
	var output, transcript bytes.Buffer
	if err := f.acp(&output, bytes.NewReader(raw), &transcript); err != nil {
		t.Fatal("positive measured transcript refused before changed probe check", err)
	}
	events := clineMeasuredBytes(t, "probe-events.jsonl", "40f6f77054109fc93e141d1721efd9d50a32676c2631eff1323670fbf1c97284")
	f.nativeFacts["plugin_data"] = "/foreign/data"
	if err := clineAttemptEvents(events, clineMeasuredNonce, f.nativeFacts, time.Date(2026, 10, 27, 5, 8, 55, 0, time.UTC), time.Date(2026, 10, 27, 5, 8, 57, 0, time.UTC)); err == nil {
		t.Fatal("measured native/probe mismatch was accepted")
	}
}
