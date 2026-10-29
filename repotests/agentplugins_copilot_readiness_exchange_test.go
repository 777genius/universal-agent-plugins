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

func (s *copilotReadinessStream) messages(p *copilotProjectProvider, start, end time.Time) error {
	return s.messagesFor(copilotReadinessPrompt, p, start, end)
}
func (s *copilotReadinessStream) messagesFor(prompt string, p *copilotProjectProvider, start, end time.Time) error {
	user := s.data[7]
	if user["content"] != prompt || user["delivery"] != "idle" || !reflect.DeepEqual(user["supportedNativeDocumentMimeTypes"], []any{}) || !copilotReadinessID(user["parentAgentTaskId"]) {
		return fmt.Errorf("Copilot readiness original user message")
	}
	transformed, ok := user["transformedContent"].(string)
	const prefix = "<current_datetime>"
	suffix := "</current_datetime>\n\n" + prompt
	if !ok || !strings.HasPrefix(transformed, prefix) || !strings.HasSuffix(transformed, suffix) {
		return fmt.Errorf("Copilot readiness transformed user message")
	}
	stamp := strings.TrimSuffix(strings.TrimPrefix(transformed, prefix), suffix)
	when, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil || when.Before(start) || when.After(end) {
		return fmt.Errorf("Copilot readiness transformed user timestamp")
	}
	messageIDs := map[any]bool{}
	for _, i := range []int{7, 12, 22} {
		id := s.data[i]["messageId"]
		if !copilotReadinessID(id) || messageIDs[id] || id == s.session {
			return fmt.Errorf("Copilot readiness message identity")
		}
		messageIDs[id] = true
	}
	if !copilotReadinessID(user["interactionId"]) || user["interactionId"] == s.session {
		return fmt.Errorf("Copilot readiness interaction identity")
	}
	for i, data := range s.data {
		turn := "0"
		if i > 16 {
			turn = "1"
		}
		if value, has := data["turnId"]; has && value != turn {
			return fmt.Errorf("Copilot readiness exact native turn")
		}
		if value, has := data["interactionId"]; has && value != user["interactionId"] {
			return fmt.Errorf("Copilot readiness interaction join")
		}
		if value, has := data["model"]; has && value != "uap-fixture-model" {
			return fmt.Errorf("Copilot readiness selected scripted model")
		}
	}
	for _, i := range []int{12, 22} {
		if !copilotReadinessValues(s.data[i], map[string]any{"originatingMessageId": user["messageId"], "rte": false, "apiCallId": "chatcmpl-" + p.nonce}) {
			return fmt.Errorf("Copilot readiness assistant message origin")
		}
	}
	if s.data[12]["content"] != "" || s.data[22]["content"] != p.sentinel || !reflect.DeepEqual(s.data[22]["toolRequests"], []any{}) || s.data[19]["messageId"] != s.data[22]["messageId"] || s.data[20]["messageId"] != s.data[22]["messageId"] || s.data[20]["deltaContent"] != p.sentinel {
		return fmt.Errorf("Copilot readiness final sentinel/no additional call")
	}
	return nil
}
func (s *copilotReadinessStream) tool(p *copilotProjectProvider, validate func(string) error) (string, error) {
	return s.toolFor("uapcopilota", map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}, p, validate)
}
func (s *copilotReadinessStream) toolFor(server string, args map[string]any, p *copilotProjectProvider, validate func(string) error) (string, error) {
	delta := s.data[10]
	input, ok := delta["inputDelta"].(string)
	parsed, err := projectNativeRuntimeJSON([]byte(input))
	if !ok || err != nil || !reflect.DeepEqual(parsed, args) || !copilotReadinessValues(delta, map[string]any{"toolCallId": p.id, "toolName": p.tool, "toolType": "function"}) {
		return "", fmt.Errorf("Copilot readiness sole call delta")
	}
	want := map[string]any{"toolCallId": p.id, "name": p.tool, "arguments": args, "type": "function", "toolTitle": "inspect_runtime", "mcpServerName": server, "mcpToolName": "inspect_runtime"}
	if !reflect.DeepEqual(s.data[12]["toolRequests"], []any{want}) {
		return "", fmt.Errorf("Copilot readiness sole assistant tool request")
	}
	if !copilotReadinessValues(s.data[13], map[string]any{"toolCallId": p.id, "toolName": p.tool, "arguments": args, "toolTitle": "inspect_runtime", "mcpServerName": server, "mcpConfigServerName": server, "mcpTransport": "stdio", "mcpConfigSource": "workspace", "mcpToolName": "inspect_runtime"}) || !copilotReadinessValues(s.data[15], map[string]any{"toolCallId": p.id, "rte": false, "success": true}) {
		return "", fmt.Errorf("Copilot readiness execution authority")
	}
	result, err := copilotReadinessObject(s.data[15]["result"], "content detailedContent contents")
	if err != nil {
		return "", err
	}
	body, ok := result["content"].(string)
	if !ok || result["detailedContent"] != body || !reflect.DeepEqual(result["contents"], []any{map[string]any{"type": "text", "text": body}}) || validate(body) != nil {
		return "", fmt.Errorf("Copilot readiness three exact tool results/facts")
	}
	telemetry := s.data[15]["toolTelemetry"].(map[string]any)["metrics"].(map[string]any)
	if telemetry["mcp_result_content_bytes"] != json.Number(fmt.Sprint(len(body))) {
		return "", fmt.Errorf("Copilot readiness exact tool result byte count")
	}
	return body, nil
}
func (s *copilotReadinessStream) history(p *copilotProjectProvider, facts string) error {
	return s.historyFor("write", p, facts)
}
func (s *copilotReadinessStream) historyFor(operation string, p *copilotProjectProvider, facts string) error {
	if len(p.requests) != 2 || !p.returned || p.err != nil {
		return fmt.Errorf("Copilot readiness ordinary two-request provider exchange")
	}
	first, err := copilotProjectRequest(p.requests[0], p.tool)
	if err != nil {
		return err
	}
	second, err := copilotProjectRequest(p.requests[1], p.tool)
	if err != nil {
		return err
	}
	a, b := first["messages"].([]any), second["messages"].([]any)
	if len(a) != 2 || len(b) != 4 || !reflect.DeepEqual(a, b[:2]) || !reflect.DeepEqual(a[1], map[string]any{"role": "user", "content": s.data[7]["transformedContent"]}) {
		return fmt.Errorf("Copilot readiness transformed prompt/history prefix")
	}
	system, err := copilotReadinessObject(a[0], "role content")
	if err != nil || system["role"] != "system" || !projectNativeRuntimeText(system["content"], copilotProjectBodyLimit) {
		return fmt.Errorf("Copilot readiness system history")
	}
	delete(first, "messages")
	delete(second, "messages")
	parameters := map[string]any{"type": "object", "properties": map[string]any{"marker": map[string]any{"type": "string"}, "nonce": map[string]any{"type": "string"}, "operation": map[string]any{"type": "string"}}, "required": []any{"nonce"}, "additionalProperties": false}
	expected := map[string]any{"model": "uap-fixture-model", "temperature": json.Number("0"), "top_p": json.Number("0.95"), "frequency_penalty": json.Number("0"), "presence_penalty": json.Number("0"), "tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": p.tool, "description": "Inspect only disposable fixture runtime facts.", "parameters": parameters}}}, "stream": true, "stream_options": map[string]any{"include_usage": true}}
	if !reflect.DeepEqual(first, second) || !reflect.DeepEqual(first, expected) {
		return fmt.Errorf("Copilot readiness provider parameter/declaration drift")
	}
	args, _ := json.Marshal(map[string]any{"nonce": p.nonce, "operation": operation, "marker": p.marker})
	assistant := map[string]any{"role": "assistant", "content": nil, "refusal": nil, "tool_calls": []any{map[string]any{"id": p.id, "type": "function", "function": map[string]any{"name": p.tool, "arguments": string(args)}}}}
	if !reflect.DeepEqual(b[2], assistant) || !reflect.DeepEqual(b[3], map[string]any{"role": "tool", "tool_call_id": p.id, "content": facts}) {
		return fmt.Errorf("Copilot readiness exact call/result history delta")
	}
	return nil
}
func (f *copilotProjectAttempt) readiness(body []byte, p *copilotProjectProvider) (string, error) {
	log, err := projectNativeRuntimeRead(f.base.Events, copilotProjectBodyLimit)
	if err != nil {
		return "", err
	}
	return f.readinessBytes(body, log, p)
}
func (f *copilotProjectAttempt) readinessBytes(body, log []byte, p *copilotProjectProvider) (string, error) {
	s, err := copilotReadinessDecode(body, f.nativeStarted, f.nativeEnded)
	if err != nil {
		return "", err
	}
	for _, check := range []func() error{func() error { return s.metadata(f) }, s.telemetry, func() error { return s.messages(p, f.nativeStarted, f.nativeEnded) }} {
		if err := check(); err != nil {
			return "", err
		}
	}
	facts, err := s.tool(p, func(body string) error { return f.validateFactsLog(body, log) })
	if err != nil {
		return "", err
	}
	if err := s.history(p, facts); err != nil {
		return "", err
	}
	lines := bytes.Split(bytes.TrimSuffix(log, []byte{'\n'}), []byte{'\n'})
	if len(lines) != 3 || s.session == f.facts.Session || s.session == p.id {
		return "", fmt.Errorf("Copilot readiness three independent probe events/session")
	}
	for i, method := range []string{"initialize", "tools/list", "tools/call"} {
		v, err := projectNativeRuntimeJSON(lines[i])
		row, ok := v.(map[string]any)
		if err != nil || !ok || row["method"] != method {
			return "", fmt.Errorf("Copilot readiness probe method order")
		}
		stamp, ok := row["timestamp"].(string)
		when, err := time.Parse(time.RFC3339Nano, stamp)
		if !ok || err != nil || when.Before(f.nativeStarted) || when.After(f.nativeEnded) {
			return "", fmt.Errorf("Copilot readiness probe command interval")
		}
	}
	return s.session, nil
}
