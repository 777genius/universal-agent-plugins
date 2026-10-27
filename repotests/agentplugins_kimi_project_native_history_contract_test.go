//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"net/url"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Exact Kimi2.3.2 wire enum; prose/test labels use American spelling.
const kimiAttemptCanceledWireReason = "cancelled" //nolint:misspell // Exact pinned upstream protocol spelling.

func kimiHistoryEncode(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func kimiHistoryDecode(t *testing.T, body []byte) map[string]any {
	t.Helper()
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		t.Fatal(err)
	}
	row, ok := value.(map[string]any)
	if !ok {
		t.Fatal("contract object required")
	}
	return row
}

// Every record below is independently authored for an inert contract. These
// strings are never executed and no captured native stream is stored in Git.
func kimiSyntheticFailedHistoryFiles(t *testing.T) map[string][]byte {
	t.Helper()
	nonce := strings.Repeat("c", 32)
	installation := "inert-contract-installation"
	physical := domain.ComputePhysicalArtifactID("uap-kimi-native-a", installation)
	root := "/inert/kimi-finite-completion"
	projects := map[string]string{"a": root + "/project", "b": root + "/project-b", "c": root + "/project-c", "user": root + "/outside"}
	client := "/inert/never-executed/kimi"
	active := projects["a"] + "/.uap/managed/clients/kimi/" + physical
	data := projects["a"] + "/.uap/plugin-data/" + physical
	profile := projects["a"] + "/.kimi-code"
	nativeSession := "session_11111111-2222-4333-8444-555555555555"
	callID, approvalID := "uap_"+nonce+"_attempt", "approval_inert-owned"
	prompt := "Call the one installed fixture tool once with the exact nonce and marker; then report completion. " + nonce + "-a"
	marker := "retained-" + nonce
	stamp := "2026-01-02T03:04:07Z"
	args := map[string]any{"nonce": nonce + "-a", "operation": "write", "marker": marker}
	facts := map[string]any{"nonce": nonce + "-a", "revision": "a-one", "cwd": active, "argv": []any{"Kimi owned a", active + "/argument"}, "plugin_root": active, "plugin_data": data, "literal": "", "once": "", "marker": marker, "pid": 17, "session": "17-1767323048000000000"}
	binding := domain.ClientBinding{ClientBindingID: domain.ComputeClientBindingID(installation, "kimi", "project", active), ClientID: "kimi", Scope: "project", TargetLocator: active, PhysicalArtifact: physical,
		NativeProfileRoot: profile, DataReceiptID: "data_" + kimiProjectProofSHA([]byte(strings.Join([]string{installation, physical, "project"}, "\x00")))[:24], Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotChecked, Verification: domain.VerificationInstalled}
	server := kimiProjectExpectedServer(&kimiProjectNativePackage{Revision: "a-one", Data: data, Binding: binding}, "a", root, nonce)
	files := map[string][]byte{}
	receipt := func(label, cwd string, argv []string) {
		files[label+".command.json"] = kimiHistoryEncode(t, map[string]any{"argv": argv, "cwd": cwd, "exit": 0, "error": false, "deadline": false, "cleanup": "ordinary full-descendant supervision; forced termination fails"})
		files[label+".stdout.log"], files[label+".stderr.log"] = []byte{}, []byte{}
	}
	for _, scope := range []string{"a", "b", "c"} {
		receipt("git-"+scope, projects[scope], []string{"/inert/never-executed/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"})
	}
	receipt("version", projects["user"], []string{client, "--version"})
	files["version.stdout.log"] = []byte("2.3.2\n")
	receipt("add-a", projects["a"], []string{"/inert/never-executed/agentplugins", "add", root + "/sources/a", "--no-save", "--target", "kimi", "--format", "json", "--scope", "project", "--project-root", projects["a"]})
	files["add-a.stdout.log"] = kimiHistoryEncode(t, map[string]any{"schema_version": 1, "command": "add", "result": "success", "data": map[string]any{"targets": []any{map[string]any{"target": "kimi", "output": map[string]any{"result": map[string]any{"installation_id": installation, "mutated": true}}}}}})
	receipt("native-attempt", projects["user"], []string{"/inert/never-executed/python", "/inert/checked-source/scripts/project_native_collectors/kimi_attempt.py", "--attempt-request", root + "/attempt-request.json"})
	files[".kimi-code-mcp.json.installed"] = kimiHistoryEncode(t, map[string]any{"mcpServers": map[string]any{"uap-kimi-native-a": server}})
	files["uap-kimi-native-a-SKILL.md.installed"] = []byte("---\nname: uap-kimi-native-a\ndescription: Kimi installed a-one\n---\nUAP_KIMI_a-one_" + nonce + "\n")
	for index, method := range []string{"initialize", "tools/list", "tools/call"} {
		event := map[string]any{"method": method, "pid": 17, "session": facts["session"], "nonce": nonce + "-a", "timestamp": "2026-01-02T03:04:08.00" + strconv.Itoa(index) + "Z", "facts": nil}
		if method == "tools/call" {
			event["facts"] = facts
		}
		files["probe-events.jsonl"] = append(files["probe-events.jsonl"], append(kimiHistoryEncode(t, event), '\n')...)
	}
	first := kimiAttemptContractBody()
	first["prompt_cache_key"] = nativeSession
	first["messages"] = []any{map[string]any{"role": "user", "content": prompt}}
	files["provider-request-1.json"] = kimiHistoryEncode(t, first)
	second := kimiHistoryDecode(t, files["provider-request-1.json"])
	second["messages"] = append(second["messages"].([]any), map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"id": callID, "type": "function", "function": map[string]any{"name": kimiAttemptTool, "arguments": string(kimiHistoryEncode(t, args))}}}}, map[string]any{"role": "tool", "tool_call_id": callID, "content": "Wall time: 0.004 seconds\n" + string(kimiHistoryEncode(t, facts))})
	files["provider-request-2.json"] = kimiHistoryEncode(t, second)
	messages := []any{}
	message := func(id, role string, content []any, metadata any) {
		row := map[string]any{"id": id, "session_id": nativeSession, "role": role, "content": content, "created_at": stamp}
		if id == "msg_inert_result" {
			row["created_at"] = "2026-01-02T03:04:08.003Z"
		}
		if id == "msg_inert_final" {
			row["created_at"] = "2026-01-02T03:04:08.004Z"
		}
		if metadata != nil {
			row["metadata"] = metadata
		}
		messages = append(messages, row)
	}
	message("msg_inert_prompt", "user", []any{map[string]any{"type": "text", "text": prompt}}, map[string]any{"origin": map[string]any{"kind": "user"}})
	reminder := "<system-reminder>\nToday's date is 2026-01-02. The current date is restated in a reminder whenever it changes; rely on the latest such reminder for the current date. DO NOT mention this to the user explicitly.\n</system-reminder>"
	message("msg_inert_date", "user", []any{map[string]any{"type": "text", "text": reminder}}, map[string]any{"origin": map[string]any{"kind": "injection", "variant": "date_change", "disclosure": map[string]any{"kind": "date", "renderGeneration": 1, "localDate": "2026-01-02", "timeZone": "UTC"}}})
	message("msg_inert_call", "assistant", []any{map[string]any{"type": "tool_use", "tool_call_id": callID, "tool_name": kimiAttemptTool, "input": args}}, nil)
	message("msg_inert_result", "tool", []any{map[string]any{"type": "tool_result", "tool_call_id": callID, "output": string(kimiHistoryEncode(t, facts))}}, nil)
	message("msg_inert_final", "assistant", []any{}, nil)
	raw := map[string]any{"kimi-single-tool.stdout.log": "inert synthetic service stream; never executed\n", "kimi-single-tool.stderr.log": ""}
	command := map[string]any{"argv": []string{client, "web", "--no-open", "--host", "127.0.0.1", "--port", "0"}, "cwd": projects["a"], "probe": "kimi-single-tool", "elapsed_seconds": 0.01, "error": nil, "exit": 0,
		"cleanup": map[string]bool{"forced_kill": false, "group_reaped": true, "port_closed": true, "shutdown_acknowledged": true, "sigterm_sent": false}}
	for _, channel := range []string{"stdout", "stderr"} {
		name := "kimi-single-tool." + channel + ".log"
		body := []byte(raw[name].(string))
		command[channel+"_file"] = name
		command[channel+"_bytes"] = len(body)
		command[channel+"_sha256"] = kimiProjectProofSHA(body)
	}
	httpRows := []any{}
	http := func(label, method, path string, auth bool, status, code int, data any) {
		response := map[string]any{"code": code, "msg": "success", "data": data, "request_id": "inert-request-" + label}
		if code != 0 {
			response["msg"] = "inert unauthorized"
		}
		body := kimiHistoryEncode(t, response)
		name := "kimi-single-tool-" + label + ".http.json"
		raw[name] = string(body)
		httpRows = append(httpRows, map[string]any{"authenticated": auth, "error": nil, "label": label, "method": method, "path": path, "response_bytes": len(body), "response_file": name, "response_sha256": kimiProjectProofSHA(body), "status": status})
	}
	http("health", "GET", "/api/v1/healthz", false, 200, 0, map[string]bool{"ok": true})
	http("unauthorized", "GET", "/api/v1/meta", false, 401, 40101, nil)
	http("meta", "GET", "/api/v1/meta", true, 200, 0, map[string]any{"server_version": "2.3.2", "capabilities": map[string]bool{"websocket": true, "file_upload": true, "fs_query": true, "mcp": true, "tasks": true, "terminal": true}, "server_id": "inert-server", "started_at": stamp, "open_in_apps": []any{}, "dangerous_bypass_auth": false, "backend": "v2"})
	workspaces := map[string]string{"a": "wd_project_111111111111", "b": "wd_project-b_222222222222", "c": "wd_project-c_333333333333"}
	for _, scope := range []string{"a", "b", "c"} {
		http("workspace-"+scope, "POST", "/api/v1/workspaces", true, 200, 0, map[string]any{"id": workspaces[scope], "root": projects[scope], "name": "uap-kimi-native-" + scope, "created_at": stamp, "last_opened_at": stamp, "session_count": 0})
		http("trust-before-"+scope, "GET", "/api/v1/workspaces/"+workspaces[scope]+"/trust", true, 200, 0, map[string]bool{"trusted": false})
	}
	for _, scope := range []string{"a", "b"} {
		for _, action := range []string{"set", "after"} {
			method := "GET"
			if action == "set" {
				method = "POST"
			}
			http("trust-"+action+"-"+scope, method, "/api/v1/workspaces/"+workspaces[scope]+"/trust", true, 200, 0, map[string]bool{"trusted": true})
		}
	}
	http("trust-untrusted-c", "GET", "/api/v1/workspaces/"+workspaces["c"]+"/trust", true, 200, 0, map[string]bool{"trusted": false})
	projected := map[string]any{}
	for key, value := range server {
		projected[key] = value
	}
	delete(projected, "env")
	projected["envKeys"] = []string{"PLUGIN_DATA", "PLUGIN_ROOT", "UAP_TEST_EVENTS", "UAP_TEST_NONCE", "UAP_TEST_REVISION", "UAP_TEST_ROOT"}
	for _, scope := range []string{"a", "b", "c", "user"} {
		rows := []any{}
		if scope == "a" {
			rows = append(rows, map[string]any{"name": "uap-kimi-native-a", "config": projected, "source": "global", "origin": profile + "/mcp.json", "mutable": false})
		}
		http("catalog-"+scope, "GET", "/api/v2/mcp/servers?cwd="+url.QueryEscape(projects[scope]), true, 200, 0, rows)
	}
	http("skills-a", "GET", "/api/v1/workspaces/"+workspaces["a"]+"/skills", true, 200, 0, map[string]any{"skills": []any{map[string]any{"name": "uap-kimi-native-a", "description": "Kimi installed a-one", "path": profile + "/skills/uap-kimi-native-a/SKILL.md", "source": "project"}}})
	state := func(initial bool) map[string]any {
		model, title := "uap-kimi-native-model", prompt
		if initial {
			model = ""
			title = ""
		}
		row := map[string]any{"id": nativeSession, "workspace_id": workspaces["a"], "title": title, "created_at": stamp, "updated_at": "2026-01-02T03:04:10Z", "busy": false, "main_turn_active": false, "pending_interaction": "none", "archived": false, "metadata": map[string]any{"cwd": projects["a"]}, "agent_config": map[string]any{"model": model}, "usage": map[string]int{"input_tokens": 0, "output_tokens": 0, "cache_read_tokens": 0, "cache_creation_tokens": 0, "context_tokens": 0}, "permission_rules": []any{}, "message_count": 0, "last_seq": 0}
		if !initial {
			row["last_turn_reason"] = "failed"
			row["last_prompt"] = prompt
		}
		return row
	}
	base := "/api/v1/sessions/" + nativeSession
	http("create-session", "POST", "/api/v1/sessions", true, 200, 0, state(true))
	http("prompt", "POST", base+"/prompts", true, 200, 0, map[string]any{"prompt_id": "msg_inert_prompt", "user_message_id": "msg_inert_prompt", "status": "running", "content": []any{map[string]any{"type": "text", "text": prompt}}, "created_at": stamp})
	http("approvals-0", "GET", base+"/approvals?status=pending", true, 200, 0, map[string]any{"items": []any{map[string]any{"approval_id": approvalID, "session_id": nativeSession, "agent_id": "main", "turn_id": 0, "tool_call_id": callID, "tool_name": kimiAttemptTool, "action": "Approve " + kimiAttemptTool, "tool_input_display": map[string]any{"kind": "generic", "summary": "Approve " + kimiAttemptTool, "detail": args}, "created_at": stamp, "expires_at": "2026-01-03T03:04:07Z"}}})
	http("approve-once", "POST", base+"/approvals/"+approvalID, true, 200, 0, map[string]any{"resolved": true, "resolved_at": stamp})
	for index := range 2 {
		var inFlight any
		if index == 0 {
			inFlight = map[string]any{"turn_id": 0, "assistant_text": "", "thinking_text": "", "running_tools": []any{}}
		}
		http("snapshot-"+strconv.Itoa(index), "GET", base+"/snapshot", true, 200, 0, map[string]any{"as_of_seq": 20 + index, "epoch": "inert-epoch", "session": state(false), "messages": map[string]any{"items": messages, "has_more": false}, "in_flight_turn": inFlight, "pending_approvals": []any{}, "pending_questions": []any{}, "subagents": []any{}})
	}
	reversed := []any{}
	for index := len(messages) - 1; index >= 0; index-- {
		reversed = append(reversed, messages[index])
	}
	http("history", "GET", base+"/messages?page_size=20", true, 200, 0, map[string]any{"items": reversed, "has_more": false})
	http("shutdown", "POST", "/api/v1/shutdown", true, 200, 0, map[string]bool{"ok": true})
	flags := map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}
	research := map[string]any{"schema_version": 1, "client": "kimi", "phase": "installed-single-tool-attempt", "commands": []any{command}, "http_requests": httpRows, "qualification": flags,
		"observations": map[string]any{"native_session_id": nativeSession, "native_turn_id": 0, "provider_call_id": callID, "approval_id": approvalID, "api_request_count": len(httpRows), "snapshot_count": 2, "native_capture_complete": false, "native_history_grammar_frozen": false, "boundary": "Bounded initial capture; no native history or qualification inferred from provider/probe success"}}
	raw["research.json"] = string(kimiHistoryEncode(t, research))
	envelope := append(kimiHistoryEncode(t, map[string]any{"schema_version": 1, "files": raw}), '\n')
	files["python-api.json"], files["native-attempt.stdout.log"] = envelope, envelope
	files["runtime-attempt.json"] = kimiHistoryEncode(t, map[string]any{"client": "kimi", "version": "2.3.2", "phase": "installed-single-tool-attempt", "activation": "prepared", "provider_kind": "scripted-openai-chat-completions", "runtime_attempt_complete": false, "test_failed": true, "vendor_authentication_qualified": false, "real_model_qualified": false, "qualification": flags, "elapsed_ms": 8000, "provider_returned": false, "native_capture_complete": false, "native_session_id": nativeSession, "native_turn_id": 0, "provider_call_id": callID, "binding": binding, "data_locator": data, "acquisition": map[string]any{"runtime_files": []any{map[string]string{"path": client}}}, "source_identity": map[string]any{"installer_base_commit": strings.Repeat("a", 40), "installer_tree": strings.Repeat("b", 40), "installer_source_state": "committed", "installer_patch_sha256": []any{}}})
	return files
}

func kimiHistoryFixtureExpectation(t *testing.T, files map[string][]byte) kimiAttemptHistoryExpectation {
	t.Helper()
	result := kimiHistoryDecode(t, files["runtime-attempt.json"])
	binding := result["binding"].(map[string]any)
	profile := binding["native_profile_root"].(string)
	project, root := filepath.Dir(profile), filepath.Dir(filepath.Dir(profile))
	probe := bytes.Split(bytes.TrimSpace(files["probe-events.jsonl"]), []byte{'\n'})
	facts := kimiHistoryDecode(t, probe[len(probe)-1])["facts"].(map[string]any)
	config := kimiHistoryDecode(t, files[".kimi-code-mcp.json.installed"])
	server := config["mcpServers"].(map[string]any)["uap-kimi-native-a"].(map[string]any)
	call := result["provider_call_id"].(string)
	nonce := strings.TrimSuffix(strings.TrimPrefix(call, "uap_"), "_attempt")
	return kimiAttemptHistoryExpectation{Client: result["acquisition"].(map[string]any)["runtime_files"].([]any)[0].(map[string]any)["path"].(string), Session: result["native_session_id"].(string), Call: call,
		Approval: "approval_inert-owned", Prompt: "Call the one installed fixture tool once with the exact nonce and marker; then report completion. " + nonce + "-a", Sentinel: "UAP_KIMI_DONE_" + nonce,
		Skill: filepath.Join(profile, "skills", "uap-kimi-native-a", "SKILL.md"), Projects: map[string]string{"a": project, "b": filepath.Join(root, "project-b"), "c": filepath.Join(root, "project-c"), "user": filepath.Join(root, "outside")}, Server: server, Facts: facts,
		Started: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Ended: time.Date(2026, 1, 2, 3, 4, 11, 0, time.UTC)}
}

// This modifies only an in-memory derivative. It is explicitly synthetic and
// cannot establish a native outcome, source qualification or release claim.
func kimiSyntheticCompletedHistory(t *testing.T) (map[string][]byte, kimiAttemptHistoryExpectation) {
	t.Helper()
	files := kimiSyntheticFailedHistoryFiles(t)
	expected := kimiHistoryFixtureExpectation(t, files)
	kimiMutateHistoryEnvelope(t, files, func(raw map[string]any) {
		for _, label := range []string{"snapshot-0", "snapshot-1", "history"} {
			name := "kimi-single-tool-" + label + ".http.json"
			response := kimiHistoryDecode(t, []byte(raw[name].(string)))
			data := response["data"].(map[string]any)
			var rows []any
			if label == "history" {
				rows = data["items"].([]any)
			} else {
				data["session"].(map[string]any)["last_turn_reason"] = "completed"
				rows = data["messages"].(map[string]any)["items"].([]any)
			}
			for _, item := range rows {
				row := item.(map[string]any)
				if row["role"] == "assistant" && len(row["content"].([]any)) == 0 {
					row["content"] = []any{map[string]any{"type": "text", "text": expected.Sentinel}}
				}
			}
			raw[name] = string(kimiHistoryEncode(t, response))
		}
	})
	proof, err := kimiAttemptHistory(files["python-api.json"], expected)
	if err != nil {
		t.Fatal("synthetic history fixture refused", err)
	}
	dataProof := kimiHistoryEncode(t, map[string]any{"schema_version": 1, "before": map[string]string{".": "020000000700:directory", ".agentplugins-data-owner.json": "0600:" + strings.Repeat("a", 64)}, "after_without_marker": map[string]string{".": "020000000700:directory", ".agentplugins-data-owner.json": "0600:" + strings.Repeat("a", 64)}, "marker_body": expected.Facts["marker"], "marker_bytes": len(expected.Facts["marker"].(string)), "marker_sha256": kimiProjectProofSHA([]byte(expected.Facts["marker"].(string)))})
	files["native-data-proof.json"] = dataProof
	proof["data_proof_sha256"] = kimiProjectProofSHA(dataProof)
	result := kimiHistoryDecode(t, files["runtime-attempt.json"])
	for _, key := range []string{"runtime_attempt_complete", "native_capture_complete", "native_history_grammar_frozen", "provider_returned"} {
		result[key] = true
	}
	result["test_failed"] = false
	result["probe_facts"], result["native_history"] = expected.Facts, proof
	result["provider_cleanup"] = map[string]any{"shutdown_complete": true, "forced_close": false, "error": false, "requests": 2, "issued": true, "returned": true}
	result["qualification"] = map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": true, "mcp-tools-list": true, "mcp-tools-call": true}
	files["runtime-attempt.json"] = kimiHistoryEncode(t, result)
	return files, expected
}

func kimiMutateHistoryEnvelope(t *testing.T, files map[string][]byte, mutate func(map[string]any)) {
	t.Helper()
	envelope := kimiHistoryDecode(t, files["python-api.json"])
	raw := envelope["files"].(map[string]any)
	mutate(raw)
	research := kimiHistoryDecode(t, []byte(raw["research.json"].(string)))
	for _, item := range research["http_requests"].([]any) {
		row := item.(map[string]any)
		body, exists := raw[row["response_file"].(string)]
		if exists {
			stream := []byte(body.(string))
			row["response_bytes"] = len(stream)
			row["response_sha256"] = kimiProjectProofSHA(stream)
		}
	}
	raw["research.json"] = string(kimiHistoryEncode(t, research))
	body := append(kimiHistoryEncode(t, envelope), '\n')
	files["python-api.json"], files["native-attempt.stdout.log"] = body, body
}

func kimiMutateHTTP(t *testing.T, files map[string][]byte, label string, mutate func(map[string]any)) {
	t.Helper()
	kimiMutateHistoryEnvelope(t, files, func(raw map[string]any) {
		name := "kimi-single-tool-" + label + ".http.json"
		response := kimiHistoryDecode(t, []byte(raw[name].(string)))
		mutate(response["data"].(map[string]any))
		raw[name] = string(kimiHistoryEncode(t, response))
	})
}

func TestKimiSingleToolSyntheticFailedHistoryAndModelFraming(t *testing.T) {
	files := kimiSyntheticFailedHistoryFiles(t)
	if err := kimiAttemptAdmit(files); err != nil {
		t.Fatal("synthetic failed archive refused", err)
	}
	expected := kimiHistoryFixtureExpectation(t, files)
	request := kimiHistoryDecode(t, files["provider-request-2.json"])
	messages := request["messages"].([]any)
	text := messages[len(messages)-1].(map[string]any)["content"].(string)
	if _, err := projectNativeRuntimeJSON([]byte(text)); err == nil {
		t.Fatal("original whole-JSON refusal not reproduced")
	}
	jsonFacts, err := kimiAttemptModelFacts(text)
	if err != nil || !reflect.DeepEqual(kimiHistoryDecode(t, jsonFacts), expected.Facts) {
		t.Fatal("synthetic precise framing/facts join failed", err)
	}
	if _, err := kimiAttemptHistory(files["python-api.json"], expected); err == nil {
		t.Fatal("synthetic failed/empty native assistant promoted")
	}
	result := kimiHistoryDecode(t, files["runtime-attempt.json"])
	if result["runtime_attempt_complete"] != false || result["test_failed"] != true {
		t.Fatal("synthetic failed outcome changed")
	}
}

func TestKimiSingleToolModelFramingIsExactAndBounded(t *testing.T) {
	for _, header := range []string{"Wall time: 0.000 seconds", "Wall time: 0.004 seconds", "Wall time: 60.000 seconds"} {
		if raw, err := kimiAttemptModelFacts(header + "\n{\"owned\":true}"); err != nil || string(raw) != `{"owned":true}` {
			t.Fatal("finite source header refused", err)
		}
	}
	for _, body := range []string{`{"owned":true}`, "Wall time: +0.004 seconds\n{}", "Wall time: -0.004 seconds\n{}", "Wall time: 00.004 seconds\n{}", "Wall time: 0.04 seconds\n{}", "Wall time: 0.0040 seconds\n{}", "Wall time: 0.004 second\n{}", "Wall time: 0.004 seconds\r\n{}", "Wall time: 60.001 seconds\n{}", "Wall time: NaN seconds\n{}", "Wall time: 1e3 seconds\n{}", "Wall time: 0.004 seconds\nWall time: 0.004 seconds\n{}", "Wall time: 0.004 seconds\n{}extra", "Wall time: 0.004 seconds\n{\"a\":1,\"a\":2}", "Wall time: 0.004 seconds\n{\"a\":1e999}", "Wall time: 0.004 seconds\n\xff", "Wall time: 0.004 seconds\n" + strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34), "Wall time: 0.004 seconds\n" + strings.Repeat("x", 1<<20)} {
		if _, err := kimiAttemptModelFacts(body); err == nil {
			t.Fatal("non-source model framing admitted", strconv.Quote(body[:min(len(body), 100)]))
		}
	}
}

func TestKimiSingleToolSyntheticCompletedHistoryHasOnlyThreeMCPFlags(t *testing.T) {
	files, _ := kimiSyntheticCompletedHistory(t)
	if err := kimiAttemptAdmit(files); err != nil {
		t.Fatal("synthetic finite complete contract refused", err)
	}
	for name := range files {
		copyFiles := map[string][]byte{}
		for key, body := range files {
			if key != name {
				copyFiles[key] = body
			}
		}
		if err := kimiAttemptAdmit(copyFiles); err == nil {
			t.Fatal("missing successful member admitted", name)
		}
	}
	failed := kimiSyntheticFailedHistoryFiles(t)
	failed["runtime-attempt.json"] = files["runtime-attempt.json"]
	failed["native-data-proof.json"] = files["native-data-proof.json"]
	if err := kimiAttemptAdmit(failed); err == nil {
		t.Fatal("success flags/proof placed over synthetic failed raw admitted")
	}
}

func TestKimiSingleToolFiniteHistoryRejectsForeignOrIncompleteNativeData(t *testing.T) {
	kinds := []string{"failed", "canceled", "active", "in-flight", "pending-approval", "pending-question", "subagent", "session", "workspace", "cwd", "model", "permission", "epoch", "sequence", "history-order", "history-more", "snapshot-more", "extra-message", "duplicate-id", "stale-time", "future-time", "unknown-message", "unknown-part", "wrong-call", "wrong-tool", "wrong-args", "error-result", "wrong-facts", "missing-sentinel", "wrong-sentinel", "extra-sentinel", "date-metadata", "date-text", "unknown-file", "missing-file", "response-hash", "receipt-path", "receipt-method", "receipt-auth", "unknown-response", "capture-complete"}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			files, expected := kimiSyntheticCompletedHistory(t)
			snapshotMutation := func(data map[string]any) {
				session := data["session"].(map[string]any)
				page := data["messages"].(map[string]any)
				rows := page["items"].([]any)
				switch kind {
				case "failed", "canceled":
					reason := kind
					if kind == "canceled" {
						reason = kimiAttemptCanceledWireReason
					}
					session["last_turn_reason"] = reason
				case "active":
					session["busy"] = true
				case "in-flight":
					data["in_flight_turn"] = map[string]any{"turn_id": 0, "assistant_text": "", "thinking_text": "", "running_tools": []any{}}
				case "pending-approval":
					data["pending_approvals"] = []any{map[string]any{"unapproved": true}}
				case "pending-question":
					data["pending_questions"] = []any{map[string]any{"unapproved": true}}
				case "subagent":
					data["subagents"] = []any{map[string]any{"foreign": true}}
				case "session":
					session["id"] = "session_foreign"
				case "workspace":
					session["workspace_id"] = "foreign"
				case "cwd":
					session["metadata"].(map[string]any)["cwd"] = "/foreign"
				case "model":
					session["agent_config"].(map[string]any)["model"] = "foreign"
				case "permission":
					session["permission_rules"] = []any{map[string]any{"remembered": true}}
				case "epoch":
					data["epoch"] = "foreign"
				case "sequence":
					data["as_of_seq"] = -1
				case "snapshot-more":
					page["has_more"] = true
				case "extra-message":
					page["items"] = append(rows, rows[0])
				case "duplicate-id":
					rows[1].(map[string]any)["id"] = rows[0].(map[string]any)["id"]
				case "stale-time":
					rows[0].(map[string]any)["created_at"] = "2000-01-01T00:00:00Z"
				case "future-time":
					rows[0].(map[string]any)["created_at"] = "2099-01-01T00:00:00Z"
				case "unknown-message":
					rows[2].(map[string]any)["unknown"] = true
				case "unknown-part":
					rows[2].(map[string]any)["content"].([]any)[0].(map[string]any)["unknown"] = true
				case "wrong-call":
					rows[2].(map[string]any)["content"].([]any)[0].(map[string]any)["tool_call_id"] = "foreign"
				case "wrong-tool":
					rows[2].(map[string]any)["content"].([]any)[0].(map[string]any)["tool_name"] = "Bash"
				case "wrong-args":
					rows[2].(map[string]any)["content"].([]any)[0].(map[string]any)["input"].(map[string]any)["nonce"] = "foreign"
				case "error-result":
					rows[3].(map[string]any)["content"].([]any)[0].(map[string]any)["is_error"] = true
				case "wrong-facts":
					rows[3].(map[string]any)["content"].([]any)[0].(map[string]any)["output"] = `{"nonce":"foreign"}`
				case "missing-sentinel":
					rows[4].(map[string]any)["content"] = []any{}
				case "wrong-sentinel":
					rows[4].(map[string]any)["content"] = []any{map[string]any{"type": "text", "text": "foreign"}}
				case "extra-sentinel":
					rows[4].(map[string]any)["content"] = append(rows[4].(map[string]any)["content"].([]any), map[string]any{"type": "text", "text": "extra"})
				case "date-metadata":
					rows[1].(map[string]any)["metadata"].(map[string]any)["origin"].(map[string]any)["disclosure"].(map[string]any)["timeZone"] = "foreign"
				case "date-text":
					rows[1].(map[string]any)["content"] = []any{map[string]any{"type": "text", "text": "foreign"}}
				}
			}
			switch kind {
			case "history-order":
				kimiMutateHTTP(t, files, "history", func(data map[string]any) { rows := data["items"].([]any); rows[0], rows[1] = rows[1], rows[0] })
			case "history-more":
				kimiMutateHTTP(t, files, "history", func(data map[string]any) { data["has_more"] = true })
			case "unknown-file", "missing-file", "response-hash", "receipt-path", "receipt-method", "receipt-auth", "unknown-response", "capture-complete":
				kimiMutateHistoryEnvelope(t, files, func(raw map[string]any) {
					if kind == "unknown-file" {
						raw["unknown.json"] = "{}"
						return
					}
					if kind == "missing-file" {
						delete(raw, "kimi-single-tool-history.http.json")
						return
					}
					research := kimiHistoryDecode(t, []byte(raw["research.json"].(string)))
					row := research["http_requests"].([]any)[0].(map[string]any)
					switch kind {
					case "response-hash":
						row["response_sha256"] = "wrong"
						row["response_file"] = "foreign.json"
					case "receipt-path":
						row["path"] = "/foreign"
					case "receipt-method":
						row["method"] = "DELETE"
					case "receipt-auth":
						row["authenticated"] = true
					case "unknown-response":
						row["unknown"] = true
					case "capture-complete":
						research["observations"].(map[string]any)["native_capture_complete"] = true
					}
					raw["research.json"] = string(kimiHistoryEncode(t, research))
				})
			default:
				kimiMutateHTTP(t, files, "snapshot-1", snapshotMutation)
			}
			if _, err := kimiAttemptHistory(files["python-api.json"], expected); err == nil {
				t.Fatal("foreign/incomplete native history admitted")
			}
		})
	}
}

func TestKimiSingleToolPromptIdentityAndAbsentResponseMetadata(t *testing.T) {
	files, expected := kimiSyntheticCompletedHistory(t)
	for _, label := range []string{"snapshot-0", "snapshot-1"} {
		kimiMutateHTTP(t, files, label, func(data map[string]any) {
			data["session"].(map[string]any)["current_prompt_id"] = "msg_inert_prompt"
		})
	}
	proof, err := kimiAttemptHistory(files["python-api.json"], expected)
	if err != nil {
		t.Fatal("optional own session prompt identity refused", err)
	}
	proof["data_proof_sha256"] = kimiProjectProofSHA(files["native-data-proof.json"])
	result := kimiHistoryDecode(t, files["runtime-attempt.json"])
	result["native_history"] = proof
	files["runtime-attempt.json"] = kimiHistoryEncode(t, result)
	if err := kimiAttemptAdmit(files); err != nil {
		t.Fatal("coherent own-prompt successful archive refused", err)
	}
	for _, kind := range []string{"session-foreign", "session-null", "create-own", "create-null", "in-flight-null", "metadata-user", "metadata-null", "metadata-foreign"} {
		t.Run(kind, func(t *testing.T) {
			files, expected := kimiSyntheticCompletedHistory(t)
			switch kind {
			case "session-foreign", "session-null":
				kimiMutateHTTP(t, files, "snapshot-1", func(data map[string]any) {
					var value any = "foreign"
					if kind == "session-null" {
						value = nil
					}
					data["session"].(map[string]any)["current_prompt_id"] = value
				})
			case "create-own", "create-null":
				kimiMutateHTTP(t, files, "create-session", func(data map[string]any) {
					var value any = "msg_inert_prompt"
					if kind == "create-null" {
						value = nil
					}
					data["current_prompt_id"] = value
				})
			case "in-flight-null":
				kimiMutateHTTP(t, files, "snapshot-0", func(data map[string]any) {
					data["in_flight_turn"].(map[string]any)["current_prompt_id"] = nil
				})
			default:
				kimiMutateHTTP(t, files, "prompt", func(data map[string]any) {
					var metadata any = map[string]any{"origin": map[string]any{"kind": "user"}}
					switch kind {
					case "metadata-null":
						metadata = nil
					case "metadata-foreign":
						metadata = map[string]any{"origin": map[string]any{"kind": "injection"}}
					}
					data["metadata"] = metadata
				})
			}
			// The envelope helper recalculates every HTTP digest and stream alias.
			// Direct semantic validation cannot fail merely on an old proof hash.
			if _, err := kimiAttemptHistory(files["python-api.json"], expected); err == nil {
				t.Fatal("unjoined prompt identity/metadata admitted")
			}
		})
	}
}

func kimiSyntheticBindingRecords(binding map[string]any) {
	binding["package_revision"] = map[string]any{"version": "1.0.0", "tree_digest": "sha256:" + strings.Repeat("a", 64), "manifest_digest": "sha256:" + strings.Repeat("b", 64)}
	binding["affected_surfaces"] = []any{"kimi"}
	binding["native_objects"] = []any{map[string]any{"object_id": "package:kimi:inert", "kind": "managed_package_directory", "path": binding["target_locator"], "protection_class": "managed"}}
	binding["receipts"] = []any{map[string]any{"operation_id": "inert-operation", "sequence": 1, "mutation_type": "directory_swap", "client_binding_id": binding["client_binding_id"], "phase": "committed"}}
}

func TestKimiSingleToolBindingAuthorityIsClosedAndTyped(t *testing.T) {
	files, _ := kimiSyntheticCompletedHistory(t)
	result := kimiHistoryDecode(t, files["runtime-attempt.json"])
	kimiSyntheticBindingRecords(result["binding"].(map[string]any))
	files["runtime-attempt.json"] = kimiHistoryEncode(t, result)
	if err := kimiAttemptAdmit(files); err != nil {
		t.Fatal("independent typed Kimi binding refused", err)
	}
	for _, kind := range []string{"unknown", "authority", "missing", "scalar", "profile-leaf", "profile-parent", "profile-relative", "profile-double-root", "revision-extra", "revision-type", "surface-type", "object-extra", "object-type", "object-modified", "receipt-extra", "receipt-type", "receipt-sequence"} {
		t.Run(kind, func(t *testing.T) {
			files, _ := kimiSyntheticCompletedHistory(t)
			result := kimiHistoryDecode(t, files["runtime-attempt.json"])
			binding := result["binding"].(map[string]any)
			kimiSyntheticBindingRecords(binding)
			switch kind {
			case "unknown":
				binding["unknown"] = true
			case "authority":
				binding["profile_authority"] = map[string]any{}
			case "missing":
				delete(binding, "policy")
			case "scalar":
				binding["updated_at"] = nil
			case "profile-leaf":
				binding["native_profile_root"] = "/inert/project/.foreign"
			case "profile-parent":
				binding["native_profile_root"] = "/inert/project/../project/.kimi-code"
			case "profile-relative":
				binding["native_profile_root"] = "relative/.kimi-code"
			case "profile-double-root":
				binding["native_profile_root"] = "/" + binding["native_profile_root"].(string)
			case "revision-extra":
				binding["package_revision"].(map[string]any)["unknown"] = true
			case "revision-type":
				binding["package_revision"].(map[string]any)["version"] = 1
			case "surface-type":
				binding["affected_surfaces"] = []any{false}
			case "object-extra":
				binding["native_objects"].([]any)[0].(map[string]any)["unknown"] = true
			case "object-type":
				binding["native_objects"].([]any)[0].(map[string]any)["path"] = nil
			case "object-modified":
				binding["native_objects"].([]any)[0].(map[string]any)["user_modified"] = 0
			case "receipt-extra":
				binding["receipts"].([]any)[0].(map[string]any)["unknown"] = true
			case "receipt-type":
				binding["receipts"].([]any)[0].(map[string]any)["phase"] = false
			case "receipt-sequence":
				binding["receipts"].([]any)[0].(map[string]any)["sequence"] = true
			}
			files["runtime-attempt.json"] = kimiHistoryEncode(t, result)
			if err := kimiAttemptAdmit(files); err == nil {
				t.Fatal("untyped or broadened Kimi binding admitted")
			}
		})
	}
}

func TestKimiSingleToolOwnedInventoryRejectsNULAndUTF8Overflow(t *testing.T) {
	for _, name := range []string{"owned\x00name", strings.Repeat("é", 2049)} {
		files, _ := kimiSyntheticCompletedHistory(t)
		data := kimiHistoryDecode(t, files["native-data-proof.json"])
		for _, key := range []string{"before", "after_without_marker"} {
			data[key].(map[string]any)[name] = "0600:" + strings.Repeat("a", 64)
		}
		files["native-data-proof.json"] = kimiHistoryEncode(t, data)
		result := kimiHistoryDecode(t, files["runtime-attempt.json"])
		result["native_history"].(map[string]any)["data_proof_sha256"] = kimiProjectProofSHA(files["native-data-proof.json"])
		files["runtime-attempt.json"] = kimiHistoryEncode(t, result)
		if err := kimiAttemptAdmit(files); err == nil {
			t.Fatal("invalid owned path admitted with coherent data proof", name)
		}
	}
}

func TestKimiSingleToolLateFlagsDataAndProviderClosure(t *testing.T) {
	files, expected := kimiSyntheticCompletedHistory(t)
	for _, kind := range []string{"marker", "marker-sha", "inventory", "unknown-inventory", "missing-owner", "oversized", "extra-field"} {
		row := kimiHistoryDecode(t, files["native-data-proof.json"])
		switch kind {
		case "marker":
			row["marker_body"] = "foreign"
		case "marker-sha":
			row["marker_sha256"] = "foreign"
		case "inventory":
			row["after_without_marker"].(map[string]any)["extra"] = "0600:" + strings.Repeat("a", 64)
		case "unknown-inventory":
			row["before"].(map[string]any)["../foreign"] = "0600:" + strings.Repeat("a", 64)
			row["after_without_marker"] = row["before"]
		case "missing-owner":
			delete(row["before"].(map[string]any), ".agentplugins-data-owner.json")
			row["after_without_marker"] = row["before"]
		case "oversized":
			row["marker_body"] = strings.Repeat("x", 1<<20)
		case "extra-field":
			row["unknown"] = true
		}
		if err := kimiAttemptDataProof(kimiHistoryEncode(t, row), expected.Facts); err == nil {
			t.Fatal("unverified data proof admitted", kind)
		}
	}
	for _, kind := range []string{"forced", "error", "not-closed", "request-count", "not-returned"} {
		row := kimiHistoryDecode(t, files["runtime-attempt.json"])["provider_cleanup"].(map[string]any)
		switch kind {
		case "forced":
			row["forced_close"] = true
		case "error":
			row["error"] = true
		case "not-closed":
			row["shutdown_complete"] = false
		case "request-count":
			row["requests"] = 3
		case "not-returned":
			row["returned"] = false
		}
		if kimiAttemptProviderClosed(row) {
			t.Fatal("incomplete provider cleanup admitted", kind)
		}
	}
	result := kimiHistoryDecode(t, files["runtime-attempt.json"])
	kimiAttemptClearCompletion(result)
	if result["runtime_attempt_complete"] != false || result["native_capture_complete"] != false || result["native_history"] != nil {
		t.Fatal("late failure retained successful proof")
	}
	for _, flag := range result["qualification"].(map[string]bool) {
		if flag {
			t.Fatal("late failure retained qualification")
		}
	}
	p := &kimiAttemptProvider{key: "inert", id: "inert", issued: true, returned: true, requestCount: 2}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	if !kimiAttemptProviderClosed(p.closeOriginal(time.Now())) {
		t.Fatal("ordinary inert provider shutdown refused")
	}
	q := &kimiAttemptProvider{key: "inert", id: "inert", issued: true, returned: true, requestCount: 2}
	if err := q.start(); err != nil {
		t.Fatal(err)
	}
	// An exhausted original origin is failure even when no request is active.
	cleanup := q.closeOriginal(time.Now().Add(-181 * time.Second))
	if kimiAttemptProviderClosed(cleanup) {
		t.Fatal("expired original provider deadline admitted")
	}
}
