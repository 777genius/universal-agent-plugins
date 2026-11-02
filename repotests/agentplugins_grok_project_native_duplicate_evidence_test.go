//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

func grokProjectDuplicateJSON(raw []byte, required, optional string) (map[string]any, error) {
	value, err := projectNativeRuntimeJSON(raw)
	row, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, required, optional) != nil {
		return nil, fmt.Errorf("Grok duplicate closed object differs")
	}
	return row, nil
}

func grokProjectDuplicateParent(files map[string][]byte, title bool) error {
	name, role, model, transport := "grok-provider", "", grokProjectProviderModel, grokProjectProviderTransport
	required := "transport model started requests responses credential_matched failure shutdown_complete forced_close"
	if title {
		name, role, model, transport = "grok-title-provider", "session-title", grokProjectTitleModel, grokProjectTitleTransport
		required += " role"
	}
	row, err := grokProjectDuplicateJSON(files[name+".json"], required, "route_observation")
	if err != nil || len(files[name+".json"]) > 2048 || row["model"] != model || row["transport"] != transport ||
		row["started"] != true || title && row["role"] != role {
		return fmt.Errorf("Grok duplicate held main/title identity differs")
	}
	for _, key := range []string{"started", "credential_matched", "shutdown_complete", "forced_close"} {
		if _, ok := row[key].(bool); !ok {
			return fmt.Errorf("Grok duplicate parent Boolean differs")
		}
	}
	counts := map[string]int64{}
	for _, key := range []string{"requests", "responses"} {
		number, ok := row[key].(json.Number)
		if !ok {
			return fmt.Errorf("Grok duplicate parent integer differs")
		}
		count, err := number.Int64()
		if err != nil || count < 0 || count > 1 {
			return fmt.Errorf("Grok duplicate parent count differs")
		}
		counts[key] = count
		_, present := files[name+map[string]string{"requests": ".request.json", "responses": ".response.sse"}[key]]
		if present != (count == 1) {
			return fmt.Errorf("Grok duplicate parent member join differs")
		}
	}
	if counts["responses"] > counts["requests"] || counts["requests"] > 0 && row["credential_matched"] != true {
		return fmt.Errorf("Grok duplicate parent exchange differs")
	}
	if !title && (counts["requests"] != 1 || counts["responses"] != 1 || row["failure"] != "duplicate" || row["route_observation"] != nil) {
		return fmt.Errorf("Grok duplicate completed first main exchange absent")
	}
	failure, isFailure := row["failure"].(string)
	failures := map[string]bool{"none": true, "listen": true, "serve": true, "deadline": true, "route": true, "credential": true,
		"duplicate": true, "body": true, "request-shape": true, "privacy": true, "response-write": true, "cleanup": true}
	if title && (!isFailure || !failures[failure]) {
		return fmt.Errorf("Grok duplicate title failure differs")
	}
	if observation, present := row["route_observation"]; present {
		if failure != "route" || grokProjectDuplicateRoute(observation) != nil {
			return fmt.Errorf("Grok duplicate parent route observation differs")
		}
	}
	if title && (row["failure"] == "listen" || row["failure"] == "none" &&
		(counts["requests"] != counts["responses"] || row["forced_close"] != false || row["shutdown_complete"] != true) ||
		(row["failure"] == "body" || row["failure"] == "privacy") && counts["requests"] != 0 ||
		map[string]bool{"request-shape": true, "privacy": true, "body": true, "response-write": true}[failure] && counts["responses"] != 0) {
		return fmt.Errorf("Grok duplicate title partial/cleanup state differs")
	}
	if counts["requests"] == 1 && grokProjectDuplicateObject(files[name+".request.json"]) != "captured" {
		return fmt.Errorf("Grok duplicate parent safe request absent")
	}
	if !title {
		value, err := projectNativeRuntimeJSON(files[name+".request.json"])
		request, ok := value.(map[string]any)
		if err != nil || !ok {
			return fmt.Errorf("Grok duplicate first request absent")
		}
		messages, ok := request["messages"].([]any)
		if request["model"] != grokProjectProviderModel || request["stream"] != true || !ok || len(messages) == 0 || len(messages) > 128 {
			return fmt.Errorf("Grok duplicate first request predicate differs")
		}
	}
	if counts["responses"] == 1 {
		response := grokProjectProviderResponse()
		if title {
			response = grokProjectTitleResponse()
			value, err := projectNativeRuntimeJSON(files[name+".request.json"])
			if err != nil || !reflect.DeepEqual(value, grokProjectTitleExpected()) {
				return fmt.Errorf("Grok duplicate title response/request join differs")
			}
		}
		if !bytes.Equal(files[name+".response.sse"], response) {
			return fmt.Errorf("Grok duplicate first response changed")
		}
	}
	return nil
}

func grokProjectDuplicateRoute(value any) error {
	row, ok := value.(map[string]any)
	flags := []string{"peer_loopback", "host_matches", "relative_url", "empty_url_host", "empty_raw_path", "content_type_exact"}
	if !ok || projectNativeRuntimeKeys(row, "method uri peer_loopback host_matches relative_url empty_url_host empty_raw_path content_type_exact content_type_count content_type_kind", "") != nil {
		return fmt.Errorf("Grok duplicate route shape differs")
	}
	all := true
	for _, key := range flags {
		value, ok := row[key].(bool)
		if !ok {
			return fmt.Errorf("Grok duplicate route Boolean differs")
		}
		all = all && value
	}
	choices := map[string]map[string]bool{
		"method":             {"POST": true, "GET": true, "other": true},
		"uri":                {"/": true, "/v1/chat/completions": true, "/chat/completions": true, "/v1/models": true, "/models": true, "/v1/responses": true, "/responses": true, "other": true},
		"content_type_count": {"none": true, "one": true, "multiple": true},
		"content_type_kind":  {"missing": true, "json-bare": true, "json-utf8": true, "other": true},
	}
	for key, allowed := range choices {
		value, ok := row[key].(string)
		if !ok || !allowed[value] {
			return fmt.Errorf("Grok duplicate route enum differs")
		}
	}
	count, kind := row["content_type_count"], row["content_type_kind"]
	if (count == "none") != (kind == "missing") || count == "multiple" && kind != "other" ||
		(kind == "json-bare" || kind == "json-utf8") && count != "one" ||
		row["content_type_exact"] == true && (count != "one" || kind != "json-bare") ||
		row["method"] == "POST" && row["uri"] == "/v1/chat/completions" && all {
		return fmt.Errorf("Grok duplicate route contradiction")
	}
	return nil
}

func grokProjectDuplicateFailedTuple(raw []byte) error {
	value, err := projectNativeRuntimeJSON(raw)
	row, ok := value.(map[string]any)
	if err != nil || !ok || row["client"] != "grok" || row["activation"] != "prepared" || row["runtime_attempt_complete"] != false || row["test_failed"] != true ||
		row["vendor_authentication_qualified"] != false || row["real_model_qualified"] != false {
		return fmt.Errorf("Grok duplicate must remain failed and unqualified")
	}
	flags, ok := row["qualification"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(flags, "project-lifecycle native-restart-removal mcp-initialize mcp-tools-list mcp-tools-call", "") != nil {
		return fmt.Errorf("Grok duplicate qualification shape differs")
	}
	for _, value := range flags {
		if value != false {
			return fmt.Errorf("Grok duplicate cannot qualify any surface")
		}
	}
	return nil
}

func grokProjectDuplicateMembers(files map[string][]byte, client string) error {
	receipt, present := files[grokDuplicateReceiptName]
	raw, captured := files[grokDuplicateRequestName]
	if !present && !captured {
		return nil
	}
	if client != "grok" || !present || len(receipt) > 2048 {
		return fmt.Errorf("Grok duplicate member isolation differs")
	}
	if err := grokProjectDuplicateClosedFiles(files); err != nil {
		return err
	}
	row, err := grokProjectDuplicateJSON(receipt, "schema_version role attempt_index status original_failure response_issued captured_bytes captured_sha256", "")
	if err != nil || row["schema_version"] != json.Number("1") || row["role"] != "main-duplicate-diagnostic" ||
		row["attempt_index"] != json.Number("2") || row["original_failure"] != "duplicate" || row["response_issued"] != false {
		return fmt.Errorf("Grok duplicate receipt identity differs")
	}
	if err := grokProjectDuplicateFailedTuple(files["runtime-attempt.json"]); err != nil {
		return err
	}
	for _, title := range []bool{false, true} {
		if err := grokProjectDuplicateParent(files, title); err != nil {
			return err
		}
	}
	switch row["status"] {
	case "captured":
		if !captured || len(raw) == 0 || len(raw) > 1<<20 || row["captured_bytes"] != json.Number(fmt.Sprint(len(raw))) ||
			row["captured_sha256"] != grokProjectDuplicateSHA(raw) || grokProjectDuplicateObject(raw) != "captured" {
			return fmt.Errorf("Grok duplicate raw/privacy join differs")
		}
	case "body-refused", "json-refused", "privacy-refused", "context-refused":
		if captured || row["captured_bytes"] != json.Number("0") || row["captured_sha256"] != "" {
			return fmt.Errorf("Grok duplicate refusal retained raw")
		}
	default:
		return fmt.Errorf("Grok duplicate status differs")
	}
	return nil
}

func grokProjectDuplicateClosedFiles(files map[string][]byte) error {
	allowed := map[string]bool{"runtime-attempt.json": true, "probe-events.jsonl": true, grokDuplicateReceiptName: true, grokDuplicateRequestName: true}
	for _, prefix := range []string{"grok-provider", "grok-title-provider"} {
		for _, suffix := range []string{".json", ".request.json", ".response.sse"} {
			allowed[prefix+suffix] = true
		}
	}
	for _, label := range []string{"git-a", "git-b", "version", "add-a", "native-inspect", "native-attempt"} {
		for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
			allowed[label+suffix] = true
		}
	}
	total := 0
	for name, body := range files {
		total += len(body)
		if !allowed[name] || len(body) > 1<<20 || grokProjectPrivateCredential(body) {
			return fmt.Errorf("Grok duplicate closed/privacy member differs")
		}
	}
	if len(files) > 80 || total > 16<<20 {
		return fmt.Errorf("Grok duplicate archive bound differs")
	}
	return nil
}
