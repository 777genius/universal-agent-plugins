//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Source b8dd3a1, toolResultRender.ts blob2bf13ed32a098c16c9b8a4094c49f93e569f58d0.
// Only model-visible MCP text carries this header. Native history/probe output
// remains a whole unframed JSON document and must not use this decoder.
var kimiAttemptWallTime = regexp.MustCompile(`^Wall time: (0|[1-9]\d?)\.\d{3} seconds$`)
var kimiAttemptWorkspaceID = regexp.MustCompile(`^wd_[a-z0-9._-]+_[0-9a-f]{12}$`)
var kimiAttemptCallID = regexp.MustCompile(`^uap_([0-9a-f]{32})_attempt$`)
var kimiAttemptPhysicalID = regexp.MustCompile(`^uap-kimi-native-a-[0-9a-f]{12}$`)
var kimiAttemptInventoryValue = regexp.MustCompile(`^0[0-7]{1,12}:(directory|[0-9a-f]{64})$`)

func kimiAttemptModelFacts(body string) ([]byte, error) {
	if body == "" || len(body) > projectNativeRuntimeBodyLimit {
		return nil, fmt.Errorf("Kimi model tool result bound differs")
	}
	header, raw, ok := strings.Cut(body, "\n")
	if !ok || !kimiAttemptWallTime.MatchString(header) {
		return nil, fmt.Errorf("Kimi model tool result framing differs")
	}
	seconds, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(header, "Wall time: "), " seconds"), 64)
	if err != nil || seconds < 0 || seconds > 60 {
		return nil, fmt.Errorf("Kimi model tool duration exceeds original service bound")
	}
	if _, err := projectNativeRuntimeJSON([]byte(raw)); err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

type kimiAttemptHistoryExpectation struct {
	Client, Session, Call, Approval, Prompt, PromptID, Sentinel, Skill string
	Projects                                                           map[string]string
	Server, Facts                                                      map[string]any
	Started, Ended                                                     time.Time
}

func kimiHistoryObject(value any, required, optional string) (map[string]any, error) {
	row, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(row, required, optional) != nil {
		return nil, fmt.Errorf("Kimi finite history object differs")
	}
	return row, nil
}

func kimiHistoryStringObject(value any, required, optional, except string) (map[string]any, error) {
	row, err := kimiHistoryObject(value, required, optional)
	if err != nil {
		return nil, err
	}
	for key, value := range row {
		if key == except {
			continue
		}
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("Kimi finite binding string differs")
		}
	}
	return row, nil
}

func kimiHistoryBindingRecord(value any, native bool) error {
	if native {
		row, err := kimiHistoryStringObject(value, "object_id kind protection_class", "logical_name path source_relative before_digest managed_digest user_modified", "user_modified")
		if err != nil {
			return err
		}
		if value, exists := row["user_modified"]; exists {
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("Kimi finite binding modified type differs")
			}
		}
		return nil
	}
	row, err := kimiHistoryStringObject(value, "operation_id sequence mutation_type client_binding_id phase", "data_receipt_id operation_group_id active_path staging_path backup_path before_digest after_digest", "sequence")
	if err != nil {
		return err
	}
	if _, err := kimiHistoryInteger(row["sequence"], -1<<63, 1<<63-1); err != nil {
		return err
	}
	return nil
}

func kimiHistoryBinding(value any) error {
	// Only the fixed Kimi add's observed authority carriers are admitted.
	row, err := kimiHistoryObject(value, "client_binding_id client_id scope target_locator physical_artifact_id materialization activation authentication policy verification data_receipt_id native_profile_root updated_at", "package_revision affected_surfaces native_objects receipts")
	if err != nil {
		return err
	}
	for key, value := range row {
		switch key {
		case "package_revision":
			if _, err := kimiHistoryStringObject(value, "tree_digest manifest_digest", "version", ""); err != nil {
				return err
			}
		case "affected_surfaces", "native_objects", "receipts":
			values, ok := value.([]any)
			if !ok {
				return fmt.Errorf("Kimi finite binding list differs")
			}
			for _, value := range values {
				if key == "affected_surfaces" {
					if _, ok := value.(string); !ok {
						return fmt.Errorf("Kimi finite binding surface differs")
					}
				} else if err := kimiHistoryBindingRecord(value, key == "native_objects"); err != nil {
					return err
				}
			}
		default:
			if _, ok := value.(string); !ok {
				return fmt.Errorf("Kimi finite binding scalar differs")
			}
		}
	}
	return nil
}

func kimiHistoryInteger(value any, low, high int64) (int64, error) {
	if !projectNativeRuntimeNumber(value, float64(low), float64(high), true) {
		return 0, fmt.Errorf("Kimi finite history integer differs")
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, fmt.Errorf("Kimi finite history integer encoding differs")
	}
	result, err := number.Int64()
	return result, err
}

func kimiHistoryInstant(value any, started, ended time.Time) (time.Time, error) {
	text, ok := value.(string)
	if !ok || len(text) > 64 {
		return time.Time{}, fmt.Errorf("Kimi finite history timestamp differs")
	}
	stamp, err := time.Parse(time.RFC3339Nano, text)
	if err != nil || stamp.Before(started) || stamp.After(ended) {
		return time.Time{}, fmt.Errorf("Kimi finite history timestamp outside original run")
	}
	return stamp, nil
}

func kimiHistoryEmpty(value any) bool {
	rows, ok := value.([]any)
	return ok && len(rows) == 0
}

func kimiHistorySuccess(body []byte) (any, error) {
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return nil, err
	}
	row, err := kimiHistoryObject(value, "code msg data request_id", "details stack")
	if err != nil || row["code"] != json.Number("0") || row["msg"] != "success" || !projectNativeRuntimeText(row["request_id"], 256) || row["request_id"] == "" || row["details"] != nil || row["stack"] != nil {
		return nil, fmt.Errorf("Kimi finite history success envelope differs")
	}
	return row["data"], nil
}

func kimiHistoryModel(initial bool) string {
	if initial {
		return ""
	}
	return kimiAttemptModel
}

func kimiHistorySession(value any, expected kimiAttemptHistoryExpectation, workspace string, terminal, initial bool) error {
	row, err := kimiHistoryObject(value, "id workspace_id title created_at updated_at busy metadata agent_config usage permission_rules message_count last_seq", "main_turn_active pending_interaction last_turn_reason archived archived_at current_prompt_id last_prompt")
	if err != nil || row["id"] != expected.Session || row["workspace_id"] != workspace || !projectNativeRuntimeText(row["title"], 4096) || (row["title"] != "" && row["title"] != expected.Prompt) || !reflect.DeepEqual(row["metadata"], map[string]any{"cwd": expected.Projects["a"]}) || !reflect.DeepEqual(row["agent_config"], map[string]any{"model": kimiHistoryModel(initial)}) || !kimiHistoryEmpty(row["permission_rules"]) {
		return fmt.Errorf("Kimi finite history session authority differs")
	}
	created, err := kimiHistoryInstant(row["created_at"], expected.Started, expected.Ended)
	if err != nil {
		return err
	}
	if _, err := kimiHistoryInstant(row["updated_at"], created, expected.Ended); err != nil {
		return err
	}
	for _, name := range []string{"message_count", "last_seq"} {
		if _, err := kimiHistoryInteger(row[name], 0, 1<<40); err != nil {
			return err
		}
	}
	if _, ok := row["busy"].(bool); !ok {
		return fmt.Errorf("Kimi finite history busy state differs")
	}
	for _, name := range []string{"main_turn_active", "archived"} {
		if value, exists := row[name]; exists {
			if _, ok := value.(bool); !ok || name == "archived" && value != false {
				return fmt.Errorf("Kimi finite history session flag differs")
			}
		}
	}
	if value, exists := row["pending_interaction"]; exists && value != "none" {
		return fmt.Errorf("Kimi finite history additional interaction")
	}
	if value, exists := row["last_turn_reason"]; exists && value != "completed" {
		return fmt.Errorf("Kimi finite history failed or canceled native turn")
	}
	if value, exists := row["last_prompt"]; exists && value != expected.Prompt {
		return fmt.Errorf("Kimi finite history last prompt differs")
	}
	if _, exists := row["archived_at"]; exists {
		return fmt.Errorf("Kimi finite history archived session")
	}
	if value, exists := row["current_prompt_id"]; exists && (initial || expected.PromptID == "" || value != expected.PromptID) {
		return fmt.Errorf("Kimi finite history current prompt identity differs")
	}
	usage, err := kimiHistoryObject(row["usage"], "input_tokens output_tokens cache_read_tokens cache_creation_tokens context_tokens", "total_cost_usd context_limit turn_count")
	if err != nil {
		return err
	}
	for name, value := range usage {
		if !projectNativeRuntimeNumber(value, 0, 1<<40, name != "total_cost_usd") {
			return fmt.Errorf("Kimi finite history usage differs")
		}
	}
	if terminal && (row["busy"] != false || row["main_turn_active"] != false || row["pending_interaction"] != "none" || row["last_turn_reason"] != "completed" || row["archived"] != false) {
		return fmt.Errorf("Kimi finite history native completion absent")
	}
	return nil
}

func kimiHistoryMessages(value any, expected kimiAttemptHistoryExpectation, promptID string) ([]any, error) {
	page, err := kimiHistoryObject(value, "items has_more", "")
	if err != nil || page["has_more"] != false {
		return nil, fmt.Errorf("Kimi finite complete native history absent")
	}
	rows, ok := page["items"].([]any)
	if !ok || len(rows) < 4 || len(rows) > 5 {
		return nil, fmt.Errorf("Kimi finite native history message count differs")
	}
	seen := map[string]bool{}
	last := expected.Started
	for _, raw := range rows {
		row, err := kimiHistoryObject(raw, "id session_id role content created_at", "metadata")
		if err != nil || !projectNativeRuntimeText(row["id"], 256) || row["id"] == "" || row["session_id"] != expected.Session {
			return nil, fmt.Errorf("Kimi finite native history message identity differs")
		}
		id := row["id"].(string)
		if seen[id] {
			return nil, fmt.Errorf("Kimi finite native history duplicate message")
		}
		seen[id] = true
		stamp, err := kimiHistoryInstant(row["created_at"], last, expected.Ended)
		if err != nil {
			return nil, err
		}
		last = stamp
	}
	first := rows[0].(map[string]any)
	if first["id"] != promptID || first["role"] != "user" || !reflect.DeepEqual(first["content"], []any{map[string]any{"type": "text", "text": expected.Prompt}}) || !reflect.DeepEqual(first["metadata"], map[string]any{"origin": map[string]any{"kind": "user"}}) {
		return nil, fmt.Errorf("Kimi finite native history accepted prompt differs")
	}
	index := 1
	if len(rows) == 5 {
		row := rows[index].(map[string]any)
		metadata, err := kimiHistoryObject(row["metadata"], "origin", "")
		if err != nil {
			return nil, err
		}
		origin, err := kimiHistoryObject(metadata["origin"], "kind variant disclosure", "")
		if err != nil || origin["kind"] != "injection" || origin["variant"] != "date_change" {
			return nil, fmt.Errorf("Kimi finite native history injection differs")
		}
		disclosure, err := kimiHistoryObject(origin["disclosure"], "kind renderGeneration localDate timeZone", "")
		stamp, stampErr := kimiHistoryInstant(row["created_at"], expected.Started, expected.Ended)
		if err != nil || stampErr != nil || disclosure["kind"] != "date" || disclosure["renderGeneration"] != json.Number("1") || disclosure["timeZone"] != "UTC" || disclosure["localDate"] != stamp.UTC().Format("2006-01-02") {
			return nil, fmt.Errorf("Kimi finite native history date disclosure differs")
		}
		text := "<system-reminder>\nToday's date is " + stamp.UTC().Format("2006-01-02") + ". The current date is restated in a reminder whenever it changes; rely on the latest such reminder for the current date. DO NOT mention this to the user explicitly.\n</system-reminder>"
		if row["role"] != "user" || !reflect.DeepEqual(row["content"], []any{map[string]any{"type": "text", "text": text}}) {
			return nil, fmt.Errorf("Kimi finite native history date text differs")
		}
		index++
	}
	args := map[string]any{"nonce": expected.Facts["nonce"], "operation": "write", "marker": expected.Facts["marker"]}
	call := rows[index].(map[string]any)
	if call["role"] != "assistant" || len(call) != 5 || !reflect.DeepEqual(call["content"], []any{map[string]any{"type": "tool_use", "tool_call_id": expected.Call, "tool_name": kimiAttemptTool, "input": args}}) {
		return nil, fmt.Errorf("Kimi finite native history owned call differs")
	}
	result := rows[index+1].(map[string]any)
	parts, ok := result["content"].([]any)
	if result["role"] != "tool" || len(result) != 5 || !ok || len(parts) != 1 {
		return nil, fmt.Errorf("Kimi finite native history owned result absent")
	}
	part, err := kimiHistoryObject(parts[0], "type tool_call_id output", "is_error")
	if err != nil || part["type"] != "tool_result" || part["tool_call_id"] != expected.Call || (len(part) == 4 && part["is_error"] != false) {
		return nil, fmt.Errorf("Kimi finite native history tool error or call differs")
	}
	output, ok := part["output"].(string)
	if !ok {
		return nil, fmt.Errorf("Kimi finite native history result text differs")
	}
	facts, err := projectNativeRuntimeJSON([]byte(output))
	if err != nil || !reflect.DeepEqual(facts, expected.Facts) {
		return nil, fmt.Errorf("Kimi finite native history result/probe join differs")
	}
	final := rows[index+2].(map[string]any)
	if final["role"] != "assistant" || len(final) != 5 || !reflect.DeepEqual(final["content"], []any{map[string]any{"type": "text", "text": expected.Sentinel}}) {
		return nil, fmt.Errorf("Kimi finite native history final sentinel absent")
	}
	return rows, nil
}

func kimiHistoryPresentPromptDiffers(row map[string]any, promptID string) bool {
	value, exists := row["current_prompt_id"]
	return exists && value != promptID
}

func kimiHistoryInFlight(value any, expected kimiAttemptHistoryExpectation, promptID string) error {
	if value == nil {
		return nil
	}
	row, err := kimiHistoryObject(value, "turn_id assistant_text thinking_text running_tools", "current_prompt_id")
	text, textOK := row["assistant_text"].(string)
	if err != nil || row["turn_id"] != json.Number("0") || !textOK || !strings.HasPrefix(expected.Sentinel, text) || row["thinking_text"] != "" || kimiHistoryPresentPromptDiffers(row, promptID) {
		return fmt.Errorf("Kimi finite native history in-flight turn differs")
	}
	tools, ok := row["running_tools"].([]any)
	if !ok || len(tools) > 1 {
		return fmt.Errorf("Kimi finite native history running tool count differs")
	}
	for _, raw := range tools {
		tool, err := kimiHistoryObject(raw, "tool_call_id name", "args")
		if err != nil || tool["tool_call_id"] != expected.Call || tool["name"] != kimiAttemptTool || tool["args"] != nil && !reflect.DeepEqual(tool["args"], map[string]any{"nonce": expected.Facts["nonce"], "operation": "write", "marker": expected.Facts["marker"]}) {
			return fmt.Errorf("Kimi finite native history foreign running tool")
		}
	}
	return nil
}

// The observer remains capture-only. This independent pure reader joins its
// closed raw service/HTTP receipts to the already validated provider/probe.
func kimiAttemptHistory(body []byte, expected kimiAttemptHistoryExpectation) (map[string]any, error) {
	refused := func() (map[string]any, error) { return nil, fmt.Errorf("Kimi finite native history capture refused") }
	if expected.Started.IsZero() || expected.Ended.Before(expected.Started) || expected.Ended.Sub(expected.Started) > 180*time.Second || expected.Client == "" || len(expected.Projects) != 4 || !kimiAttemptCacheKey.MatchString(expected.Session) || expected.Call == "" || expected.Approval == "" || expected.Sentinel == "" || expected.Facts == nil {
		return refused()
	}
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return nil, err
	}
	envelope, err := kimiHistoryObject(value, "schema_version files", "")
	if err != nil || envelope["schema_version"] != json.Number("1") {
		return refused()
	}
	rawFiles, ok := envelope["files"].(map[string]any)
	if !ok || len(rawFiles) < 4 || len(rawFiles) > 64 {
		return refused()
	}
	files := map[string][]byte{}
	for name, value := range rawFiles {
		text, ok := value.(string)
		if !ok || name == "." || name == ".." || strings.ContainsAny(name, "/\\") || len(text) > projectNativeRuntimeBodyLimit {
			return refused()
		}
		files[name] = []byte(text)
	}
	researchValue, err := projectNativeRuntimeJSON(files["research.json"])
	if err != nil {
		return nil, err
	}
	research, err := kimiHistoryObject(researchValue, "schema_version client phase commands http_requests observations qualification", "")
	if err != nil || research["schema_version"] != json.Number("1") || research["client"] != "kimi" || research["phase"] != "installed-single-tool-attempt" {
		return refused()
	}
	flags, err := kimiHistoryObject(research["qualification"], "project-lifecycle native-restart-removal mcp-initialize mcp-tools-list mcp-tools-call", "")
	if err != nil {
		return nil, err
	}
	for _, flag := range flags {
		if flag != false {
			return refused()
		}
	}
	observation, err := kimiHistoryObject(research["observations"], "native_session_id native_turn_id provider_call_id approval_id api_request_count snapshot_count native_capture_complete native_history_grammar_frozen boundary", "")
	if err != nil || observation["native_session_id"] != expected.Session || observation["native_turn_id"] != json.Number("0") || observation["provider_call_id"] != expected.Call || observation["approval_id"] != expected.Approval || observation["native_capture_complete"] != false || observation["native_history_grammar_frozen"] != false || observation["boundary"] != "Bounded initial capture; no native history or qualification inferred from provider/probe success" {
		return refused()
	}
	snapshotCount, err := kimiHistoryInteger(observation["snapshot_count"], 1, 10)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{"research.json": true}
	readStream := func(row map[string]any, channel string) error {
		name, ok := row[channel+"_file"].(string)
		if !ok || used[name] || name != "kimi-single-tool."+channel+".log" {
			return fmt.Errorf("Kimi finite native service stream differs")
		}
		stream, exists := files[name]
		if !exists || row[channel+"_bytes"] != json.Number(strconv.Itoa(len(stream))) || row[channel+"_sha256"] != kimiProjectProofSHA(stream) || channel == "stderr" && len(stream) != 0 {
			return fmt.Errorf("Kimi finite native service stream join differs")
		}
		used[name] = true
		return nil
	}
	commands, ok := research["commands"].([]any)
	if !ok || len(commands) != 1 {
		return refused()
	}
	command, err := kimiHistoryObject(commands[0], "argv cwd probe elapsed_seconds error exit cleanup stdout_file stdout_bytes stdout_sha256 stderr_file stderr_bytes stderr_sha256", "")
	cleanup := map[string]any{"forced_kill": false, "group_reaped": true, "port_closed": true, "shutdown_acknowledged": true, "sigterm_sent": false}
	if err != nil || command["probe"] != "kimi-single-tool" || command["cwd"] != expected.Projects["a"] || command["error"] != nil || command["exit"] != json.Number("0") || !projectNativeRuntimeNumber(command["elapsed_seconds"], 0, 60, false) || !reflect.DeepEqual(command["argv"], []any{expected.Client, "web", "--no-open", "--host", "127.0.0.1", "--port", "0"}) || !reflect.DeepEqual(command["cleanup"], cleanup) {
		return refused()
	}
	for _, channel := range []string{"stdout", "stderr"} {
		if err := readStream(command, channel); err != nil {
			return nil, err
		}
	}
	httpRows, ok := research["http_requests"].([]any)
	if !ok || len(httpRows) > 60 || observation["api_request_count"] != json.Number(strconv.Itoa(len(httpRows))) {
		return refused()
	}
	responses := map[string]any{}
	responseBodies := map[string][]byte{}
	labels := []string{}
	for _, raw := range httpRows {
		row, err := kimiHistoryObject(raw, "authenticated error label method path response_bytes response_file response_sha256 status", "")
		if err != nil {
			return nil, err
		}
		label, labelOK := row["label"].(string)
		name, nameOK := row["response_file"].(string)
		stream, exists := files[name]
		if !labelOK || !nameOK || used[name] || name != "kimi-single-tool-"+label+".http.json" || !exists || row["error"] != nil || row["response_bytes"] != json.Number(strconv.Itoa(len(stream))) || row["response_sha256"] != kimiProjectProofSHA(stream) {
			return refused()
		}
		if _, exists := responses[label]; exists {
			return refused()
		}
		used[name] = true
		responseBodies[label] = stream
		labels = append(labels, label)
		if label == "unauthorized" {
			value, err := projectNativeRuntimeJSON(stream)
			if err != nil {
				return nil, err
			}
			negative, err := kimiHistoryObject(value, "code msg data request_id", "details stack")
			if err != nil || negative["code"] != json.Number("40101") || negative["data"] != nil || !projectNativeRuntimeText(negative["msg"], 1024) || !projectNativeRuntimeText(negative["request_id"], 256) {
				return refused()
			}
			responses[label] = nil
		} else {
			value, err := kimiHistorySuccess(stream)
			if err != nil {
				return nil, err
			}
			responses[label] = value
		}
	}
	if len(used) != len(files) {
		return refused()
	}
	return kimiAttemptHistoryResponses(httpRows, labels, responses, responseBodies, snapshotCount, expected)
}

func kimiAttemptHistoryResponses(httpRows []any, labels []string, responses map[string]any, bodies map[string][]byte, snapshotCount int64, expected kimiAttemptHistoryExpectation) (map[string]any, error) {
	refused := func() (map[string]any, error) {
		return nil, fmt.Errorf("Kimi finite native history response join refused")
	}
	base := "/api/v1/sessions/" + expected.Session
	type endpoint struct {
		method, path string
		auth         bool
		status       int
	}
	endpoints := map[string]endpoint{"health": {"GET", "/api/v1/healthz", false, 200}, "unauthorized": {"GET", "/api/v1/meta", false, 401}, "meta": {"GET", "/api/v1/meta", true, 200}, "create-session": {"POST", "/api/v1/sessions", true, 200}, "prompt": {"POST", base + "/prompts", true, 200}, "approve-once": {"POST", base + "/approvals/" + expected.Approval, true, 200}, "history": {"GET", base + "/messages?page_size=20", true, 200}, "shutdown": {"POST", "/api/v1/shutdown", true, 200}}
	order := []string{"health", "unauthorized", "meta"}
	workspaces := map[string]string{}
	for _, scope := range []string{"a", "b", "c"} {
		row, err := kimiHistoryObject(responses["workspace-"+scope], "id root name created_at last_opened_at session_count", "")
		id, ok := row["id"].(string)
		if err != nil || !ok || len(id) > 256 || !kimiAttemptWorkspaceID.MatchString(id) || row["root"] != expected.Projects[scope] || row["name"] != "uap-kimi-native-"+scope || row["session_count"] != json.Number("0") {
			return refused()
		}
		for _, key := range []string{"created_at", "last_opened_at"} {
			if _, err := kimiHistoryInstant(row[key], expected.Started, expected.Ended); err != nil {
				return nil, err
			}
		}
		workspaces[scope] = id
		endpoints["workspace-"+scope] = endpoint{"POST", "/api/v1/workspaces", true, 200}
		endpoints["trust-before-"+scope] = endpoint{"GET", "/api/v1/workspaces/" + id + "/trust", true, 200}
		order = append(order, "workspace-"+scope, "trust-before-"+scope)
		if !reflect.DeepEqual(responses["trust-before-"+scope], map[string]any{"trusted": false}) {
			return refused()
		}
	}
	if workspaces["a"] == workspaces["b"] || workspaces["a"] == workspaces["c"] || workspaces["b"] == workspaces["c"] {
		return refused()
	}
	for _, scope := range []string{"a", "b"} {
		for _, action := range []string{"set", "after"} {
			label := "trust-" + action + "-" + scope
			method := "GET"
			if action == "set" {
				method = "POST"
			}
			endpoints[label] = endpoint{method, "/api/v1/workspaces/" + workspaces[scope] + "/trust", true, 200}
			order = append(order, label)
			if !reflect.DeepEqual(responses[label], map[string]any{"trusted": true}) {
				return refused()
			}
		}
	}
	endpoints["trust-untrusted-c"] = endpoint{"GET", "/api/v1/workspaces/" + workspaces["c"] + "/trust", true, 200}
	order = append(order, "trust-untrusted-c")
	if !reflect.DeepEqual(responses["trust-untrusted-c"], map[string]any{"trusted": false}) {
		return refused()
	}
	for _, scope := range []string{"a", "b", "c", "user"} {
		label := "catalog-" + scope
		endpoints[label] = endpoint{"GET", "/api/v2/mcp/servers?cwd=" + url.QueryEscape(expected.Projects[scope]), true, 200}
		order = append(order, label)
		rows, ok := responses[label].([]any)
		if !ok || scope != "a" && len(rows) != 0 || scope == "a" && len(rows) != 1 {
			return refused()
		}
		if scope == "a" {
			row, err := kimiHistoryObject(rows[0], "name config source origin mutable", "")
			if err != nil || row["name"] != "uap-kimi-native-a" || row["source"] != "global" || row["mutable"] != false || row["origin"] != filepath.Join(expected.Projects["a"], ".kimi-code", "mcp.json") {
				return refused()
			}
			projected := map[string]any{}
			for key, value := range expected.Server {
				projected[key] = value
			}
			delete(projected, "env")
			projected["envKeys"] = []any{"PLUGIN_DATA", "PLUGIN_ROOT", "UAP_TEST_EVENTS", "UAP_TEST_NONCE", "UAP_TEST_REVISION", "UAP_TEST_ROOT"}
			if !reflect.DeepEqual(row["config"], projected) {
				return refused()
			}
		}
	}
	endpoints["skills-a"] = endpoint{"GET", "/api/v1/workspaces/" + workspaces["a"] + "/skills", true, 200}
	order = append(order, "skills-a", "create-session", "prompt")
	skills, err := kimiHistoryObject(responses["skills-a"], "skills", "")
	if err != nil {
		return nil, err
	}
	skillRows, ok := skills["skills"].([]any)
	if !ok || len(skillRows) > 512 {
		return refused()
	}
	owned := 0
	for _, raw := range skillRows {
		row, err := kimiHistoryObject(raw, "name description path source", "type disable_model_invocation scopes")
		name, ok := row["name"].(string)
		if err != nil || !ok {
			return refused()
		}
		if strings.HasPrefix(name, "uap-kimi-native-") {
			owned++
			if name != "uap-kimi-native-a" || row["description"] != "Kimi installed a-one" || row["path"] != expected.Skill || row["source"] != "project" {
				return refused()
			}
		}
	}
	if owned != 1 {
		return refused()
	}
	if !reflect.DeepEqual(responses["health"], map[string]any{"ok": true}) || !reflect.DeepEqual(responses["shutdown"], map[string]any{"ok": true}) {
		return refused()
	}
	meta, err := kimiHistoryObject(responses["meta"], "server_version capabilities server_id started_at open_in_apps dangerous_bypass_auth backend", "experimental_flags features web_title")
	if err != nil || meta["server_version"] != "2.3.2" || meta["backend"] != "v2" || meta["dangerous_bypass_auth"] != false {
		return refused()
	}
	if err := kimiHistorySession(responses["create-session"], expected, workspaces["a"], false, true); err != nil {
		return nil, err
	}
	prompt, err := kimiHistoryObject(responses["prompt"], "prompt_id user_message_id status content created_at", "")
	promptID, ok := prompt["prompt_id"].(string)
	if err != nil || !ok || promptID == "" || len(promptID) > 256 || prompt["user_message_id"] != promptID || prompt["status"] != "running" && prompt["status"] != "queued" && prompt["status"] != "blocked" || !reflect.DeepEqual(prompt["content"], []any{map[string]any{"type": "text", "text": expected.Prompt}}) {
		return refused()
	}
	if _, err := kimiHistoryInstant(prompt["created_at"], expected.Started, expected.Ended); err != nil {
		return nil, err
	}
	// Fixed request supplies no client metadata; pinned prompts.ts projection
	// therefore omits response metadata. Session prompt identity is optional,
	// but any later present string must name this exact accepted prompt.
	expected.PromptID = promptID
	approvalCount := 0
	for index := range 20 {
		label := "approvals-" + strconv.Itoa(index)
		value, exists := responses[label]
		if !exists {
			break
		}
		approvalCount++
		endpoints[label] = endpoint{"GET", base + "/approvals?status=pending", true, 200}
		order = append(order, label)
		pending, err := kimiHistoryObject(value, "items", "")
		items, ok := pending["items"].([]any)
		if err != nil || !ok || len(items) > 1 {
			return refused()
		}
		if len(items) == 1 {
			row, err := kimiHistoryObject(items[0], "approval_id session_id agent_id turn_id tool_call_id tool_name action tool_input_display created_at expires_at", "")
			action := "Approve " + kimiAttemptTool
			args := map[string]any{"nonce": expected.Facts["nonce"], "operation": "write", "marker": expected.Facts["marker"]}
			if err != nil || row["approval_id"] != expected.Approval || row["session_id"] != expected.Session || row["agent_id"] != "main" || row["turn_id"] != json.Number("0") || row["tool_call_id"] != expected.Call || row["tool_name"] != kimiAttemptTool || row["action"] != action || !reflect.DeepEqual(row["tool_input_display"], map[string]any{"kind": "generic", "summary": action, "detail": args}) {
				return refused()
			}
			created, err := kimiHistoryInstant(row["created_at"], expected.Started, expected.Ended)
			expires, expiresErr := time.Parse(time.RFC3339Nano, fmt.Sprint(row["expires_at"]))
			if err != nil || expiresErr != nil || !expires.After(created) {
				return refused()
			}
			break
		}
	}
	if approvalCount == 0 {
		return refused()
	}
	lastPending, err := kimiHistoryObject(responses["approvals-"+strconv.Itoa(approvalCount-1)], "items", "")
	if err != nil || len(lastPending["items"].([]any)) != 1 {
		return refused()
	}
	order = append(order, "approve-once")
	resolved, err := kimiHistoryObject(responses["approve-once"], "resolved resolved_at", "")
	if err != nil || resolved["resolved"] != true {
		return refused()
	}
	if _, err := kimiHistoryInstant(resolved["resolved_at"], expected.Started, expected.Ended); err != nil {
		return nil, err
	}
	var terminal map[string]any
	capturedSnapshots := []map[string]any{}
	epoch := ""
	lastSeq := int64(-1)
	for index := range snapshotCount {
		label := "snapshot-" + strconv.FormatInt(index, 10)
		endpoints[label] = endpoint{"GET", base + "/snapshot", true, 200}
		order = append(order, label)
		snapshot, err := kimiHistoryObject(responses[label], "as_of_seq epoch session messages in_flight_turn pending_approvals pending_questions subagents", "")
		if err != nil || !kimiHistoryEmpty(snapshot["pending_approvals"]) || !kimiHistoryEmpty(snapshot["pending_questions"]) || !kimiHistoryEmpty(snapshot["subagents"]) {
			return refused()
		}
		seq, err := kimiHistoryInteger(snapshot["as_of_seq"], 0, 1<<40)
		newEpoch, ok := snapshot["epoch"].(string)
		if err != nil || seq < lastSeq || !ok || newEpoch == "" || len(newEpoch) > 256 || epoch != "" && epoch != newEpoch {
			return refused()
		}
		epoch, lastSeq = newEpoch, seq
		last := index == snapshotCount-1
		if err := kimiHistorySession(snapshot["session"], expected, workspaces["a"], last, false); err != nil {
			return nil, err
		}
		if err := kimiHistoryInFlight(snapshot["in_flight_turn"], expected, promptID); err != nil {
			return nil, err
		}
		if last && snapshot["in_flight_turn"] != nil || !last && snapshot["in_flight_turn"] == nil {
			return refused()
		}
		terminal = snapshot
		capturedSnapshots = append(capturedSnapshots, snapshot)
	}
	order = append(order, "history", "shutdown")
	if !reflect.DeepEqual(labels, order) {
		return refused()
	}
	for index, raw := range httpRows {
		row := raw.(map[string]any)
		endpoint, exists := endpoints[labels[index]]
		if !exists || row["method"] != endpoint.method || row["path"] != endpoint.path || row["authenticated"] != endpoint.auth || row["status"] != json.Number(strconv.Itoa(endpoint.status)) {
			return refused()
		}
	}
	rows, err := kimiHistoryMessages(terminal["messages"], expected, promptID)
	if err != nil {
		return nil, err
	}
	for _, snapshot := range capturedSnapshots[:len(capturedSnapshots)-1] {
		if err := kimiHistoryPartialMessages(snapshot["messages"], rows, expected.Sentinel); err != nil {
			return nil, err
		}
	}
	history, err := kimiHistoryObject(responses["history"], "items has_more", "")
	if err != nil || history["has_more"] != false {
		return refused()
	}
	descending, ok := history["items"].([]any)
	if !ok || len(descending) != len(rows) {
		return refused()
	}
	for index, row := range rows {
		if !reflect.DeepEqual(row, descending[len(rows)-1-index]) {
			return refused()
		}
	}
	return map[string]any{"native_session_id": expected.Session, "native_turn_id": json.Number("0"), "provider_call_id": expected.Call, "approval_id": expected.Approval, "prompt_id": promptID, "terminal_reason": "completed", "message_count": json.Number(strconv.Itoa(len(rows))), "snapshot_count": json.Number(strconv.FormatInt(snapshotCount, 10)), "history_sha256": kimiProjectProofSHA(bodies["history"]), "terminal_snapshot_sha256": kimiProjectProofSHA(bodies["snapshot-"+strconv.FormatInt(snapshotCount-1, 10)]), "probe_pid": expected.Facts["pid"], "probe_session": expected.Facts["session"], "started_at": expected.Started.UTC().Format(time.RFC3339Nano), "ended_at": expected.Ended.UTC().Format(time.RFC3339Nano)}, nil
}

func kimiHistoryPartialMessages(value any, terminal []any, sentinel string) error {
	page, err := kimiHistoryObject(value, "items has_more", "")
	rows, ok := page["items"].([]any)
	if err != nil || !ok || len(rows) == 0 || len(rows) > len(terminal) || page["has_more"] != false {
		return fmt.Errorf("Kimi finite intermediate history page differs")
	}
	for index, raw := range rows {
		if reflect.DeepEqual(raw, terminal[index]) {
			continue
		}
		row, err := kimiHistoryObject(raw, "id session_id role content created_at", "")
		if err != nil || index != len(terminal)-1 || row["role"] != "assistant" {
			return fmt.Errorf("Kimi finite intermediate history prefix differs")
		}
		content, ok := row["content"].([]any)
		if !ok || len(content) > 1 {
			return fmt.Errorf("Kimi finite intermediate assistant differs")
		}
		if len(content) == 1 {
			part, err := kimiHistoryObject(content[0], "type text", "")
			text, ok := part["text"].(string)
			if err != nil || !ok || part["type"] != "text" || !strings.HasPrefix(sentinel, text) {
				return fmt.Errorf("Kimi finite intermediate sentinel differs")
			}
		}
		copyRow := map[string]any{}
		for key, value := range row {
			copyRow[key] = value
		}
		copyRow["content"] = terminal[index].(map[string]any)["content"]
		if !reflect.DeepEqual(copyRow, terminal[index]) {
			return fmt.Errorf("Kimi finite intermediate message identity differs")
		}
	}
	return nil
}

func kimiAttemptProbeJoin(body []byte, facts map[string]any, started, ended time.Time) error {
	if len(body) == 0 || len(body) > projectNativeRuntimeBodyLimit || !bytes.HasSuffix(body, []byte{'\n'}) {
		return fmt.Errorf("Kimi proof probe stream bound differs")
	}
	lines := bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'})
	if len(lines) < 3 || len(lines) > 6 {
		return fmt.Errorf("Kimi proof probe count differs")
	}
	last := started
	for index, line := range lines {
		value, err := projectNativeRuntimeJSON(line)
		if err != nil {
			return err
		}
		row, err := kimiHistoryObject(value, "method pid session nonce timestamp facts", "")
		if err != nil || row["pid"] != facts["pid"] || row["session"] != facts["session"] || row["nonce"] != facts["nonce"] {
			return fmt.Errorf("Kimi proof probe identity differs")
		}
		stamp, err := kimiHistoryInstant(row["timestamp"], last, ended)
		if err != nil {
			return err
		}
		last = stamp
		method := "tools/list"
		if index == 0 {
			method = "initialize"
		}
		if index == len(lines)-1 {
			method = "tools/call"
		}
		if row["method"] != method || index != len(lines)-1 && row["facts"] != nil || index == len(lines)-1 && !reflect.DeepEqual(row["facts"], facts) {
			return fmt.Errorf("Kimi proof probe method/facts differs")
		}
	}
	return nil
}

func kimiAttemptProviderClosed(value any) bool {
	body, err := json.Marshal(value)
	if err != nil {
		return false
	}
	parsed, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return false
	}
	row, err := kimiHistoryObject(parsed, "shutdown_complete forced_close error requests issued returned", "")
	return err == nil && row["shutdown_complete"] == true && row["forced_close"] == false && row["error"] == false && row["requests"] == json.Number("2") && row["issued"] == true && row["returned"] == true
}

func kimiAttemptClearCompletion(evidence map[string]any) {
	evidence["runtime_attempt_complete"], evidence["native_capture_complete"], evidence["native_history_grammar_frozen"] = false, false, false
	evidence["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	delete(evidence, "native_history")
}

// A successful label/proof over failed raw bytes is refused here as well as in
// the live fixture. No receiver-provided native outcome is an admission token.
func kimiAttemptSuccessfulFiles(files map[string][]byte, result map[string]any) error {
	refused := func() error { return fmt.Errorf("Kimi successful raw completion/proof differs") }
	if result["native_capture_complete"] != true || result["native_history_grammar_frozen"] != true || result["provider_returned"] != true || result["activation"] != "prepared" || result["version"] != "2.3.2" || result["phase"] != "installed-single-tool-attempt" || result["provider_kind"] != "scripted-openai-chat-completions" || !kimiAttemptProviderClosed(result["provider_cleanup"]) {
		return refused()
	}
	required := map[string]bool{"runtime-attempt.json": true, "probe-events.jsonl": true, "python-api.json": true, "native-data-proof.json": true, ".kimi-code-mcp.json.installed": true, "uap-kimi-native-a-SKILL.md.installed": true, "provider-request-1.json": true, "provider-request-2.json": true}
	commands := map[string]map[string]any{}
	for _, label := range []string{"git-a", "git-b", "git-c", "version", "add-a", "native-attempt"} {
		for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
			required[label+suffix] = true
		}
		value, err := projectNativeRuntimeJSON(files[label+".command.json"])
		if err != nil {
			return err
		}
		row, err := kimiHistoryObject(value, "argv cwd exit error deadline cleanup", "")
		if err != nil || row["exit"] != json.Number("0") || row["error"] != false || row["deadline"] != false || row["cleanup"] != "ordinary full-descendant supervision; forced termination fails" {
			return refused()
		}
		argv, ok := row["argv"].([]any)
		if !ok || len(argv) == 0 || len(argv) > 32 {
			return refused()
		}
		for _, arg := range argv {
			if !projectNativeRuntimeText(arg, 4096) || arg == "" {
				return refused()
			}
		}
		if !projectNativeRuntimeText(row["cwd"], 4096) || row["cwd"] == "" {
			return refused()
		}
		commands[label] = row
	}
	if len(required) != len(files) {
		return refused()
	}
	for name := range required {
		if _, exists := files[name]; !exists {
			return refused()
		}
		if len(files[name]) == 0 && !strings.HasSuffix(name, ".stdout.log") && !strings.HasSuffix(name, ".stderr.log") {
			return refused()
		}
	}
	if !bytes.Equal(files["python-api.json"], files["native-attempt.stdout.log"]) || len(files["native-attempt.stderr.log"]) != 0 || string(files["version.stdout.log"]) != "2.3.2\n" || len(files["version.stderr.log"]) != 0 {
		return refused()
	}
	identity, err := kimiHistoryObject(result["source_identity"], "installer_base_commit installer_tree installer_source_state installer_patch_sha256", "")
	if err != nil || identity["installer_source_state"] != "committed" || !kimiHistoryEmpty(identity["installer_patch_sha256"]) {
		return refused()
	}
	for _, key := range []string{"installer_base_commit", "installer_tree"} {
		text, ok := identity[key].(string)
		if !ok || len(text) != 40 || strings.Trim(text, "0123456789abcdef") != "" {
			return refused()
		}
	}
	proof, err := kimiHistoryObject(result["native_history"], "native_session_id native_turn_id provider_call_id approval_id prompt_id terminal_reason message_count snapshot_count history_sha256 terminal_snapshot_sha256 probe_pid probe_session started_at ended_at data_proof_sha256", "")
	if err != nil || proof["terminal_reason"] != "completed" || proof["native_turn_id"] != json.Number("0") {
		return refused()
	}
	started, startErr := time.Parse(time.RFC3339Nano, fmt.Sprint(proof["started_at"]))
	ended, endErr := time.Parse(time.RFC3339Nano, fmt.Sprint(proof["ended_at"]))
	if startErr != nil || endErr != nil || ended.Before(started) || ended.Sub(started) > 180*time.Second || !projectNativeRuntimeNumber(result["elapsed_ms"], float64(ended.Sub(started).Milliseconds()), 180000, true) {
		return refused()
	}
	call, callOK := result["provider_call_id"].(string)
	parts := kimiAttemptCallID.FindStringSubmatch(call)
	session, sessionOK := result["native_session_id"].(string)
	approval, approvalOK := proof["approval_id"].(string)
	if !callOK || len(parts) != 2 || !sessionOK || !approvalOK || approval == "" || proof["native_session_id"] != session || proof["provider_call_id"] != call || result["native_turn_id"] != json.Number("0") {
		return refused()
	}
	if err := kimiHistoryBinding(result["binding"]); err != nil {
		return err
	}
	bindingBody, err := json.Marshal(result["binding"])
	if err != nil {
		return err
	}
	var binding domain.ClientBinding
	decoder := json.NewDecoder(bytes.NewReader(bindingBody))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&binding) != nil || binding.ClientID != "kimi" || binding.Scope != "project" || binding.Activation != domain.ActivationPrepared || binding.Authentication != domain.AuthenticationNotChecked || binding.Verification != domain.VerificationInstalled || binding.Materialization != domain.MaterializationMaterialized || !kimiAttemptPhysicalID.MatchString(binding.PhysicalArtifact) || filepath.Base(binding.NativeProfileRoot) != ".kimi-code" || filepath.Clean(binding.NativeProfileRoot) != binding.NativeProfileRoot {
		return refused()
	}
	project := filepath.Dir(binding.NativeProfileRoot)
	root := filepath.Dir(project)
	active := filepath.Join(project, ".uap", "managed", "clients", "kimi", binding.PhysicalArtifact)
	data := filepath.Join(project, ".uap", "plugin-data", binding.PhysicalArtifact)
	if !filepath.IsAbs(project) || filepath.Clean(project) != project || binding.TargetLocator != active || result["data_locator"] != data {
		return refused()
	}
	add, err := kimiProjectObject(files["add-a.stdout.log"])
	if err != nil || add["schema_version"] != json.Number("1") || add["command"] != "add" || add["result"] != "success" {
		return refused()
	}
	addData, ok := add["data"].(map[string]any)
	if !ok {
		return refused()
	}
	targets, ok := addData["targets"].([]any)
	if !ok || len(targets) != 1 {
		return refused()
	}
	target, ok := targets[0].(map[string]any)
	if !ok || target["target"] != "kimi" {
		return refused()
	}
	output, ok := target["output"].(map[string]any)
	if !ok {
		return refused()
	}
	public, ok := output["result"].(map[string]any)
	if !ok || public["mutated"] != true {
		return refused()
	}
	installation, ok := public["installation_id"].(string)
	if !ok || installation == "" || binding.PhysicalArtifact != domain.ComputePhysicalArtifactID("uap-kimi-native-a", installation) || binding.ClientBindingID != domain.ComputeClientBindingID(installation, "kimi", "project", active) || binding.DataReceiptID != "data_"+kimiProjectProofSHA([]byte(strings.Join([]string{installation, binding.PhysicalArtifact, "project"}, "\x00")))[:24] {
		return refused()
	}
	facts, err := kimiHistoryObject(result["probe_facts"], "nonce revision cwd argv plugin_root plugin_data literal once marker pid session", "")
	if err != nil {
		return err
	}
	want := map[string]any{"nonce": parts[1] + "-a", "revision": "a-one", "cwd": active, "argv": []any{"Kimi owned a", active + "/argument"}, "plugin_root": active, "plugin_data": data, "literal": "", "once": "", "marker": "retained-" + parts[1], "pid": facts["pid"], "session": facts["session"]}
	pid, err := kimiHistoryInteger(facts["pid"], 1, 1<<30)
	probeSession, ok := facts["session"].(string)
	if err != nil || !ok || !strings.HasPrefix(probeSession, strconv.FormatInt(pid, 10)+"-") || !reflect.DeepEqual(facts, want) || proof["probe_pid"] != facts["pid"] || proof["probe_session"] != facts["session"] {
		return refused()
	}
	if err := kimiAttemptProbeJoin(files["probe-events.jsonl"], facts, started, ended); err != nil {
		return err
	}
	packageFixture := &kimiProjectNativePackage{Revision: "a-one", Data: data, Binding: binding}
	server := kimiProjectExpectedServer(packageFixture, "a", root, parts[1])
	installed, err := kimiProjectObject(files[".kimi-code-mcp.json.installed"])
	if err != nil || !reflect.DeepEqual(installed, map[string]any{"mcpServers": map[string]any{"uap-kimi-native-a": server}}) {
		return refused()
	}
	skill := "---\nname: uap-kimi-native-a\ndescription: Kimi installed a-one\n---\nUAP_KIMI_a-one_" + parts[1] + "\n"
	if string(files["uap-kimi-native-a-SKILL.md.installed"]) != skill {
		return refused()
	}
	first, err := kimiAttemptRequest(files["provider-request-1.json"])
	if err != nil {
		return err
	}
	second, err := kimiAttemptRequest(files["provider-request-2.json"])
	if err != nil {
		return err
	}
	before := first["messages"].([]any)
	after := second["messages"].([]any)
	if first["prompt_cache_key"] != session || second["prompt_cache_key"] != session || len(after) != len(before)+2 || !reflect.DeepEqual(before, after[:len(before)]) {
		return refused()
	}
	for _, raw := range before {
		row := raw.(map[string]any)
		if row["role"] == "tool" || row["tool_calls"] != nil {
			return refused()
		}
	}
	assistant := after[len(before)].(map[string]any)
	tool := after[len(before)+1].(map[string]any)
	calls, ok := assistant["tool_calls"].([]any)
	if assistant["role"] != "assistant" || !ok || len(calls) != 1 || tool["role"] != "tool" || tool["tool_call_id"] != call {
		return refused()
	}
	owned := calls[0].(map[string]any)
	function := owned["function"].(map[string]any)
	args, err := projectNativeRuntimeJSON([]byte(function["arguments"].(string)))
	if err != nil || owned["id"] != call || function["name"] != kimiAttemptTool || !reflect.DeepEqual(args, map[string]any{"nonce": facts["nonce"], "operation": "write", "marker": facts["marker"]}) {
		return refused()
	}
	text, ok := tool["content"].(string)
	if !ok {
		return refused()
	}
	rawFacts, err := kimiAttemptModelFacts(text)
	if err != nil {
		return err
	}
	returned, err := projectNativeRuntimeJSON(rawFacts)
	if err != nil || !reflect.DeepEqual(returned, facts) {
		return refused()
	}
	expected := kimiAttemptHistoryExpectation{Client: fmt.Sprint(commands["version"]["argv"].([]any)[0]), Session: session, Call: call, Approval: approval,
		Prompt: "Call the one installed fixture tool once with the exact nonce and marker; then report completion. " + parts[1] + "-a", Sentinel: "UAP_KIMI_DONE_" + parts[1], Skill: filepath.Join(binding.NativeProfileRoot, "skills", "uap-kimi-native-a", "SKILL.md"),
		Projects: map[string]string{"a": project, "b": filepath.Join(root, "project-b"), "c": filepath.Join(root, "project-c"), "user": filepath.Join(root, "outside")}, Server: server, Facts: facts, Started: started, Ended: ended}
	if !filepath.IsAbs(expected.Client) || !reflect.DeepEqual(commands["version"]["argv"], []any{expected.Client, "--version"}) || commands["version"]["cwd"] != expected.Projects["user"] {
		return refused()
	}
	for _, scope := range []string{"a", "b", "c"} {
		argv := commands["git-"+scope]["argv"].([]any)
		if len(argv) != 5 || !filepath.IsAbs(fmt.Sprint(argv[0])) || !reflect.DeepEqual(argv[1:], []any{"-c", "core.hooksPath=/dev/null", "init", "--quiet"}) || commands["git-"+scope]["cwd"] != expected.Projects[scope] {
			return refused()
		}
	}
	addArgv := commands["add-a"]["argv"].([]any)
	if len(addArgv) != 12 || !filepath.IsAbs(fmt.Sprint(addArgv[0])) || !reflect.DeepEqual(addArgv[1:], []any{"add", filepath.Join(root, "sources", "a"), "--no-save", "--target", "kimi", "--format", "json", "--scope", "project", "--project-root", project}) || commands["add-a"]["cwd"] != project {
		return refused()
	}
	nativeArgv := commands["native-attempt"]["argv"].([]any)
	if len(nativeArgv) != 4 || !filepath.IsAbs(fmt.Sprint(nativeArgv[0])) || !strings.HasSuffix(fmt.Sprint(nativeArgv[1]), "/scripts/project_native_collectors/kimi_attempt.py") || nativeArgv[2] != "--attempt-request" || nativeArgv[3] != filepath.Join(root, "attempt-request.json") || commands["native-attempt"]["cwd"] != expected.Projects["user"] {
		return refused()
	}
	verified, err := kimiAttemptHistory(files["python-api.json"], expected)
	if err != nil {
		return refused()
	}
	if err := kimiAttemptDataProof(files["native-data-proof.json"], facts); err != nil {
		return err
	}
	verified["data_proof_sha256"] = kimiProjectProofSHA(files["native-data-proof.json"])
	if !reflect.DeepEqual(verified, proof) {
		return refused()
	}
	return nil
}

// Only path/digest inventories already held for this owned plugin-data root are
// exported; unknown profile/file contents never enter this proof.
func kimiAttemptDataProof(body []byte, facts map[string]any) error {
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return err
	}
	row, err := kimiHistoryObject(value, "schema_version before after_without_marker marker_body marker_bytes marker_sha256", "")
	marker, ok := row["marker_body"].(string)
	if err != nil || row["schema_version"] != json.Number("1") || !ok || marker != facts["marker"] || len(marker) > 256 || row["marker_bytes"] != json.Number(strconv.Itoa(len(marker))) || row["marker_sha256"] != kimiProjectProofSHA([]byte(marker)) || !reflect.DeepEqual(row["before"], row["after_without_marker"]) {
		return fmt.Errorf("Kimi observed marker/data proof differs")
	}
	inventory, ok := row["before"].(map[string]any)
	if !ok || len(inventory) == 0 || len(inventory) > 4096 || inventory[".agentplugins-data-owner.json"] == nil || inventory["native-marker.txt"] != nil {
		return fmt.Errorf("Kimi owned data inventory closure differs")
	}
	for name, raw := range inventory {
		text, ok := raw.(string)
		if !ok || !kimiAttemptInventoryValue.MatchString(text) || len(name) > 4096 || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") || filepath.IsAbs(name) || filepath.ToSlash(filepath.Clean(name)) != name || name == ".." || strings.HasPrefix(name, "../") || len(strings.Split(name, "/")) > 32 {
			return fmt.Errorf("Kimi owned data inventory entry differs")
		}
		if name == "." && !strings.HasSuffix(text, ":directory") {
			return fmt.Errorf("Kimi owned data root differs")
		}
		for parent := filepath.Dir(name); parent != "."; parent = filepath.Dir(parent) {
			if !strings.HasSuffix(fmt.Sprint(inventory[parent]), ":directory") {
				return fmt.Errorf("Kimi owned data parent closure differs")
			}
		}
	}
	if inventory["."] == nil || strings.HasSuffix(fmt.Sprint(inventory[".agentplugins-data-owner.json"]), ":directory") {
		return fmt.Errorf("Kimi owned data root/owner missing")
	}
	return nil
}
