//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

func copilotCyclePrompt(stage string) string {
	return "Report only the disposable fixture challenge for phase " + stage + "."
}

// A no-call exchange is the text-only projection of the already measured graph.
// It keeps the same closed event families and does not admit tool events.
var copilotCycleNoCallIndices = []int{3, 4, 5, 6, 7, 8, 9, 19, 20, 21, 22, 23, 24, 25, 26}

func copilotCycleNoCallDecode(body []byte, start, end time.Time) (*copilotReadinessStream, error) {
	if len(body) > copilotProjectBodyLimit || !bytes.HasSuffix(body, []byte{'\n'}) || start.IsZero() || end.Before(start) || end.Sub(start) > 60*time.Second {
		return nil, fmt.Errorf("Copilot cycle no-call stream/interval")
	}
	lines := bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'})
	if len(lines) != 15 {
		return nil, fmt.Errorf("Copilot cycle no-call finite graph")
	}
	s := &copilotReadinessStream{}
	seen := map[any]bool{}
	last := start
	for i, line := range lines {
		original := copilotCycleNoCallIndices[i]
		keys := "type data id timestamp parentId"
		ephemeral := original < 7 || original == 9 || original >= 19 && original <= 21 || original == 23 || original == 25
		if ephemeral {
			keys += " ephemeral"
		}
		if i == 14 {
			keys = "type timestamp sessionId exitCode usage"
		}
		value, err := projectNativeRuntimeJSON(line)
		row, shape := copilotReadinessObject(value, keys)
		if err != nil || shape != nil {
			return nil, fmt.Errorf("Copilot cycle no-call closed event")
		}
		stamp, ok := row["timestamp"].(string)
		when, err := time.Parse(time.RFC3339Nano, stamp)
		if !ok || err != nil || when.Before(last) || when.After(end) {
			return nil, fmt.Errorf("Copilot cycle no-call event interval")
		}
		last = when
		s.rows = append(s.rows, row)
		if i == 14 {
			if row["type"] != "result" || !copilotReadinessID(row["sessionId"]) || seen[row["sessionId"]] || row["exitCode"] != json.Number("0") {
				return nil, fmt.Errorf("Copilot cycle no-call terminal identity")
			}
			s.session = row["sessionId"].(string)
			continue
		}
		if row["type"] != copilotReadinessGrammar[original][0] || !copilotReadinessID(row["id"]) || seen[row["id"]] || !copilotReadinessID(row["parentId"]) || ephemeral && row["ephemeral"] != true {
			return nil, fmt.Errorf("Copilot cycle no-call event order/identity")
		}
		seen[row["id"]] = true
		data, err := copilotReadinessObject(row["data"], copilotReadinessGrammar[original][1])
		if err != nil {
			return nil, err
		}
		s.data = append(s.data, data)
	}
	external := map[any]bool{}
	for _, i := range []int{0, 5, 9} {
		parent := s.rows[i]["parentId"]
		if seen[parent] || external[parent] || parent == s.session {
			return nil, fmt.Errorf("Copilot cycle no-call external identity")
		}
		external[parent] = true
	}
	for i := 1; i <= 4; i++ {
		if s.rows[i]["parentId"] != s.rows[0]["parentId"] {
			return nil, fmt.Errorf("Copilot cycle no-call startup parent")
		}
	}
	for _, edge := range [][2]int{{6, 5}, {7, 5}, {8, 5}, {11, 10}, {12, 10}, {13, 12}} {
		if s.rows[edge[0]]["parentId"] != s.rows[edge[1]]["id"] {
			return nil, fmt.Errorf("Copilot cycle no-call parent edge")
		}
	}
	if s.rows[10]["parentId"] != s.rows[9]["parentId"] {
		return nil, fmt.Errorf("Copilot cycle no-call response parent")
	}
	return s, nil
}

func (s *copilotReadinessStream) noCallMessages(prompt string, p *copilotProjectProvider, start, end time.Time) error {
	user := s.data[4]
	transformed, ok := user["transformedContent"].(string)
	prefix, suffix := "<current_datetime>", "</current_datetime>\n\n"+prompt
	if user["content"] != prompt || user["delivery"] != "idle" || !reflect.DeepEqual(user["supportedNativeDocumentMimeTypes"], []any{}) || !copilotReadinessID(user["parentAgentTaskId"]) || !ok || !strings.HasPrefix(transformed, prefix) || !strings.HasSuffix(transformed, suffix) {
		return fmt.Errorf("Copilot cycle no-call user authority")
	}
	when, err := time.Parse(time.RFC3339Nano, strings.TrimSuffix(strings.TrimPrefix(transformed, prefix), suffix))
	if err != nil || when.Before(start) || when.After(end) {
		return fmt.Errorf("Copilot cycle no-call transformed time")
	}
	if !copilotReadinessID(user["messageId"]) || !copilotReadinessID(user["interactionId"]) || user["messageId"] == s.session || user["interactionId"] == s.session || !copilotReadinessID(s.data[10]["messageId"]) || s.data[10]["messageId"] == user["messageId"] || s.data[10]["messageId"] == s.session {
		return fmt.Errorf("Copilot cycle no-call message identity")
	}
	for _, row := range s.data {
		if turn, has := row["turnId"]; has && turn != "0" {
			return fmt.Errorf("Copilot cycle no-call turn")
		}
		if id, has := row["interactionId"]; has && id != user["interactionId"] {
			return fmt.Errorf("Copilot cycle no-call interaction")
		}
		if model, has := row["model"]; has && model != "uap-fixture-model" {
			return fmt.Errorf("Copilot cycle no-call model")
		}
	}
	if !copilotReadinessValues(s.data[10], map[string]any{"originatingMessageId": user["messageId"], "rte": false, "apiCallId": "chatcmpl-" + p.nonce, "content": p.sentinel, "toolRequests": []any{}}) || s.data[7]["messageId"] != s.data[10]["messageId"] || s.data[8]["messageId"] != s.data[10]["messageId"] || s.data[8]["deltaContent"] != p.sentinel {
		return fmt.Errorf("Copilot cycle no-call sole final message")
	}
	row := s.data[9]
	if !copilotReadinessNatural(row["dispatchDurationMs"], 60000) || row["outcome"] != "success" || row["editClassifierVersion"] != json.Number("1") || row["containsBuiltInFileEditRequest"] != false || !reflect.DeepEqual(s.data[11], map[string]any{"model": "uap-fixture-model", "isByok": true, "result": "success"}) {
		return fmt.Errorf("Copilot cycle no-call bounded telemetry")
	}
	return copilotCycleUsage(s.rows[14]["usage"])
}

func copilotCycleUsage(value any) error {
	usage, err := copilotReadinessObject(value, "premiumRequests totalApiDurationMs sessionDurationMs codeChanges")
	if err != nil || usage["premiumRequests"] != json.Number("0") || !copilotReadinessNatural(usage["totalApiDurationMs"], 60000) || !copilotReadinessNatural(usage["sessionDurationMs"], 60000) || !reflect.DeepEqual(usage["codeChanges"], map[string]any{"linesAdded": json.Number("0"), "linesRemoved": json.Number("0"), "filesModified": []any{}}) {
		return fmt.Errorf("Copilot cycle terminal usage/code changes")
	}
	return nil
}

func (s *copilotReadinessStream) noCallHistory(p *copilotProjectProvider) error {
	if p.err != nil || !p.returned || len(p.requests) != 1 {
		return fmt.Errorf("Copilot cycle no-call provider exchange")
	}
	m, err := copilotCycleRequest(p.requests[0], "")
	if err != nil {
		return err
	}
	messages := m["messages"].([]any)
	if len(messages) != 2 || !reflect.DeepEqual(messages[1], map[string]any{"role": "user", "content": s.data[4]["transformedContent"]}) {
		return fmt.Errorf("Copilot cycle no-call prompt history")
	}
	system, err := copilotReadinessObject(messages[0], "role content")
	if err != nil || system["role"] != "system" || !projectNativeRuntimeText(system["content"], copilotProjectBodyLimit) {
		return fmt.Errorf("Copilot cycle no-call system history")
	}
	delete(m, "messages")
	if tools, exists := m["tools"]; exists && reflect.DeepEqual(tools, []any{}) {
		delete(m, "tools")
	}
	want := map[string]any{"model": "uap-fixture-model", "temperature": json.Number("0"), "top_p": json.Number("0.95"), "frequency_penalty": json.Number("0"), "presence_penalty": json.Number("0"), "stream": true, "stream_options": map[string]any{"include_usage": true}}
	if !reflect.DeepEqual(m, want) {
		return fmt.Errorf("Copilot cycle no-call exact model parameters")
	}
	return nil
}
