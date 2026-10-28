//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

func (c *geminiCompleteCapture) phase(stage string) (*geminiCyclePhase, []*geminiCycleRecipe, error) {
	view := c.Proof.Views[geminiCompleteProofStage(stage)]
	scope := geminiCycleScope(stage)
	r := view[scope]
	if r == nil {
		return nil, nil, geminiCompleteRefusal()
	}
	q := &geminiCyclePhase{stage: stage, scope: scope, r: r, operation: "none", tools: []string{}, prompt: "Disposable Gemini fixture stage " + stage + ". Call only the preselected installed inspect_runtime tool when requested.", sentinel: "UAP_GEMINI_DONE_" + stage + "_" + r.Nonce}
	if stage == "installed-user" || stage == "installed-a" || stage == "installed-b" {
		q.operation = "write"
	}
	if stage == "restarted-a" || stage == "updated-a" || stage == "repaired-a" {
		q.operation = "read"
	}
	advertised, seen := []*geminiCycleRecipe{}, map[string]bool{}
	for _, key := range []string{"user", scope} {
		other := view[key]
		if !seen[key] && other != nil && other.Binding.ClientBindingID != "" && !other.Removed {
			q.tools = append(q.tools, "mcp_"+other.Name+"_inspect_runtime")
			advertised = append(advertised, other)
		}
		seen[key] = true
	}
	if q.operation != "none" {
		if r.Removed || r.Binding.ClientBindingID == "" {
			return nil, nil, geminiCompleteRefusal()
		}
		q.tool = "mcp_" + r.Name + "_inspect_runtime"
		q.wireID = "uap_cycle_" + geminiCycleSHA([]byte(stage + r.Nonce))[:24]
		q.id = q.tool + "__" + q.wireID
	}
	return q, advertised, nil
}

func (c *geminiCompleteCapture) observations() error {
	orders := map[string][]string{"cycle-native-commands.jsonl": geminiCompleteObservations, "cycle-native-events.jsonl": geminiCycleNative, "cycle-discovery.jsonl": geminiCycleListings, "cycle-provider-receipts.jsonl": geminiCycleNative, "cycle-provider-requests.jsonl": geminiCycleNative, "cycle-provider-responses.jsonl": geminiCycleNative}
	probeOrder := make([]string, 0, 24)
	for _, stage := range geminiCompleteObservations {
		probeOrder = append(probeOrder, strings.TrimPrefix(stage, "native:"))
	}
	orders["cycle-probe-events.jsonl"] = probeOrder
	for name, order := range orders {
		repeat := name == "cycle-provider-requests.jsonl" || name == "cycle-provider-responses.jsonl"
		rows, err := geminiCompleteRows(c.Files[name], order, repeat)
		want := len(order)
		if repeat {
			want = 16
		}
		if err != nil || len(rows) != want {
			return geminiCompleteRefusal()
		}
		c.Rows[name] = rows
	}
	previous := c.Started
	for _, row := range c.Rows["cycle-native-commands.jsonl"] {
		start, end, err := c.observation(row)
		if err != nil || start.Before(previous) {
			return geminiCompleteRefusal()
		}
		previous = end
	}
	return nil
}

func (c *geminiCompleteCapture) observation(row geminiCompleteRow) (time.Time, time.Time, error) {
	value := row.Value
	start, startErr := geminiCompleteTime(value["started_at"])
	end, endErr := geminiCompleteTime(value["finished_at"])
	if projectNativeRuntimeKeys(value, "kind scope argv cwd exit error context_error started_at finished_at elapsed_ms cleanup stdout stderr", "") != nil || startErr != nil || endErr != nil || start.Before(c.Started) || end.Before(start) || end.After(c.Finished) || !geminiCompleteIntegerIs(value["elapsed_ms"], end.Sub(start).Milliseconds()) {
		return start, end, geminiCompleteRefusal()
	}
	kind, stage, ok := strings.Cut(row.Stage, ":")
	scope := geminiCycleScope(stage)
	argv := []string{c.Paths["node"], c.Paths["client"], "skills", "list", "--all"}
	limit := int64(10000)
	if kind == "native" {
		q, _, err := c.phase(stage)
		if err != nil {
			return start, end, err
		}
		argv = []string{c.Paths["node"], c.Paths["client"], "--prompt", q.prompt, "--approval-mode", "default", "--output-format", "stream-json", "--model", geminiProjectModel}
		limit = 60000
	}
	receipt := map[string]any{}
	for _, name := range []string{"argv", "cwd", "exit", "error", "context_error", "elapsed_ms", "cleanup"} {
		receipt[name] = value[name]
	}
	if !ok || kind != "native" && kind != "listing" || value["kind"] != kind || value["scope"] != scope || geminiCompleteReceipt(receipt, argv, geminiCompleteScopePath(c.Paths, scope), limit, 0) != nil {
		return start, end, geminiCompleteRefusal()
	}
	for _, channel := range []string{"stdout", "stderr"} {
		if _, err := geminiCompleteStream(value[channel]); err != nil {
			return start, end, err
		}
	}
	return start, end, nil
}

func (c *geminiCompleteCapture) providerStreams(name, stage string) ([][]byte, error) {
	var result [][]byte
	for _, row := range c.Rows[name] {
		if row.Stage != stage {
			continue
		}
		if projectNativeRuntimeKeys(row.Value, "index stream", "") != nil || !geminiCompleteIntegerIs(row.Value["index"], int64(len(result))) {
			return nil, geminiCompleteRefusal()
		}
		body, err := geminiCompleteStream(row.Value["stream"])
		if err != nil {
			return nil, err
		}
		result = append(result, body)
	}
	return result, nil
}

func (c *geminiCompleteCapture) replayProvider(stage string, receipt map[string]any) (*geminiCyclePhase, error) {
	q, _, err := c.phase(stage)
	if err != nil {
		return nil, err
	}
	requests, err := c.providerStreams("cycle-provider-requests.jsonl", stage)
	if err != nil {
		return nil, err
	}
	responses, err := c.providerStreams("cycle-provider-responses.jsonl", stage)
	want := 1
	if q.operation != "none" {
		want = 2
	}
	if err != nil || len(requests) != want || len(responses) != want {
		return nil, geminiCompleteRefusal()
	}
	for index, body := range requests {
		q.requests = append(q.requests, body)
		parts, err := geminiCycleRequest(body, q)
		if err != nil {
			return nil, err
		}
		if geminiCompleteRequestIntegers(body) != nil {
			return nil, geminiCompleteRefusal()
		}
		inner, err := geminiCycleHistory(parts, q, index)
		if err != nil || geminiCompleteProviderPrompt(q.initialContents, q.prompt) != nil {
			return nil, geminiCompleteRefusal()
		}
		if index == 1 {
			if _, err := geminiCompleteJSON([]byte(inner)); err != nil {
				return nil, err
			}
			q.facts, err = geminiCycleFacts(inner, q.r, c.Paths["home"])
			if err != nil {
				return nil, err
			}
			if !geminiCompleteText(q.facts.Session, 256, true) {
				return nil, geminiCompleteRefusal()
			}
		}
		if err := geminiCompleteResponse(responses[index], q, index); err != nil {
			return nil, err
		}
	}
	if !geminiCompleteEqual(receipt, map[string]any{"scope": q.scope, "operation": q.operation, "tools": q.tools, "native_call_id": q.id, "wire_call_id": q.wireID, "prompt": q.prompt, "sentinel": q.sentinel, "requests": want, "responses": want, "issued": q.operation != "none", "returned": q.operation != "none", "error": false}) {
		return nil, geminiCompleteRefusal()
	}
	return q, nil
}

func geminiCompleteProviderPrompt(contents []any, prompt string) error {
	if len(contents) == 0 {
		return geminiCompleteRefusal()
	}
	row, ok := contents[len(contents)-1].(map[string]any)
	parts, pok := row["parts"].([]any)
	if !ok || !pok || row["role"] != "user" || len(parts) == 0 {
		return geminiCompleteRefusal()
	}
	part, ok := parts[len(parts)-1].(map[string]any)
	if !ok || !reflect.DeepEqual(part, map[string]any{"text": prompt}) {
		return geminiCompleteRefusal()
	}
	return nil
}

func geminiCompleteResponse(body []byte, q *geminiCyclePhase, index int) error {
	if !bytes.HasPrefix(body, []byte("data: ")) || !bytes.HasSuffix(body, []byte("\n\n")) {
		return geminiCompleteRefusal()
	}
	inner := body[6 : len(body)-2]
	if !bytes.Equal(inner, bytes.TrimSpace(inner)) || bytes.ContainsAny(inner, "\r\n") {
		return geminiCompleteRefusal()
	}
	row, err := geminiCompleteJSON(inner)
	part := map[string]any{"text": q.sentinel}
	if q.operation != "none" && index == 0 {
		part = map[string]any{"functionCall": map[string]any{"id": q.id, "name": q.tool, "args": q.arguments()}}
	}
	want := map[string]any{"candidates": []any{map[string]any{"index": 0, "content": map[string]any{"role": "model", "parts": []any{part}}, "finishReason": "STOP"}}, "usageMetadata": map[string]int{"promptTokenCount": 1, "candidatesTokenCount": 1, "totalTokenCount": 2}}
	if err != nil || !geminiCompleteEqual(row, want) {
		return geminiCompleteRefusal()
	}
	return nil
}

func (c *geminiCompleteCapture) protocol() error {
	phases := map[string]*geminiCyclePhase{}
	for _, row := range c.Rows["cycle-provider-receipts.jsonl"] {
		q, err := c.replayProvider(row.Stage, row.Value)
		if err != nil {
			return err
		}
		phases[row.Stage] = q
	}
	sessions, probeSessions, probePIDs := map[string]bool{}, map[string]bool{}, map[int64]bool{}
	nativeIndex, listingIndex := 0, 0
	for index, command := range c.Rows["cycle-native-commands.jsonl"] {
		kind, stage, _ := strings.Cut(command.Stage, ":")
		q := phases[stage]
		if kind != "native" {
			q = nil
		}
		start, end, err := c.observation(command)
		if err != nil {
			return err
		}
		if kind == "native" {
			if err := c.nativeStream(command.Value, c.Rows["cycle-native-events.jsonl"][nativeIndex], q, start, end, sessions); err != nil {
				return err
			}
			nativeIndex++
		} else {
			if err := c.catalog(command.Value, c.Rows["cycle-discovery.jsonl"][listingIndex], stage); err != nil {
				return err
			}
			listingIndex++
		}
		_, advertised, err := c.phase(stage)
		if err != nil {
			return err
		}
		allowed := map[string]*geminiCycleRecipe{}
		for _, r := range advertised {
			allowed[r.Nonce] = r
		}
		probe := c.Rows["cycle-probe-events.jsonl"][index]
		body, streamErr := geminiCompleteStream(probe.Value["stream"])
		if projectNativeRuntimeKeys(probe.Value, "scope stream", "") != nil || probe.Value["scope"] != geminiCycleScope(stage) || streamErr != nil {
			return geminiCompleteRefusal()
		}
		if err := geminiCompleteJSONLines(body); err != nil {
			return err
		}
		groups, err := geminiCycleProbeSegment(body, allowed, q, c.Paths["home"], start, end, probeSessions, probePIDs)
		if err != nil {
			return err
		}
		for _, group := range groups {
			if !geminiCompleteText(group.session, 256, true) {
				return geminiCompleteRefusal()
			}
			pid, _ := group.pid.Int64()
			probeSessions[group.session], probePIDs[pid] = true, true
		}
	}
	return nil
}

func (c *geminiCompleteCapture) nativeStream(command map[string]any, event geminiCompleteRow, q *geminiCyclePhase, start, end time.Time, sessions map[string]bool) error {
	if q == nil || event.Stage != q.stage || projectNativeRuntimeKeys(event.Value, "scope session stream", "") != nil || event.Value["scope"] != q.scope {
		return geminiCompleteRefusal()
	}
	stdout, err := geminiCompleteStream(command["stdout"])
	raw, rawErr := geminiCompleteStream(event.Value["stream"])
	stderr, stderrErr := geminiCompleteStream(command["stderr"])
	if err != nil || rawErr != nil || stderrErr != nil || !bytes.Equal(stdout, raw) || !geminiProjectNativeStderrAllowed(stderr) || geminiCompleteJSONLines(raw) != nil {
		return geminiCompleteRefusal()
	}
	session, err := geminiCycleStream(raw, q, c.Paths["home"], start, end)
	if err != nil || !geminiCompleteText(session, 256, true) || sessions[session] || event.Value["session"] != session || geminiCompleteStats(raw, q.operation != "none") != nil {
		return geminiCompleteRefusal()
	}
	sessions[session] = true
	return nil
}

func geminiCompleteStats(raw []byte, call bool) error {
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	if len(lines) == 0 {
		return geminiCompleteRefusal()
	}
	result, err := geminiCompleteJSON(lines[len(lines)-1])
	if err != nil {
		return err
	}
	value, exists := result["stats"]
	if !exists {
		return nil
	}
	stats, ok := value.(map[string]any)
	models, mok := stats["models"].(map[string]any)
	want := int64(0)
	if call {
		want = 1
	}
	if !ok || !mok || len(models) != 1 || models[geminiProjectModel] == nil || !geminiCompleteIntegerIs(stats["tool_calls"], want) {
		return geminiCompleteRefusal()
	}
	for name, value := range stats {
		if name != "models" && !geminiCompleteInteger(value, 0, 1<<40) {
			return geminiCompleteRefusal()
		}
	}
	model, ok := models[geminiProjectModel].(map[string]any)
	if !ok {
		return geminiCompleteRefusal()
	}
	for _, value := range model {
		if !geminiCompleteInteger(value, 0, 1<<40) {
			return geminiCompleteRefusal()
		}
	}
	return nil
}

func geminiCompleteRequestIntegers(body []byte) error {
	row, err := geminiCompleteJSON(body)
	if err != nil {
		return err
	}
	value, exists := row["generationConfig"]
	if !exists {
		return nil
	}
	generation, ok := value.(map[string]any)
	if !ok {
		return geminiCompleteRefusal()
	}
	for name, bounds := range map[string][2]int64{"topK": {0, 2048}, "candidateCount": {1, 8}, "maxOutputTokens": {1, 1 << 20}, "seed": {-(1 << 31), 1 << 31}} {
		if value, exists := generation[name]; exists && !geminiCompleteInteger(value, bounds[0], bounds[1]) {
			return geminiCompleteRefusal()
		}
	}
	if value, exists := generation["thinkingConfig"]; exists {
		thinking, ok := value.(map[string]any)
		if !ok {
			return geminiCompleteRefusal()
		}
		if value, exists := thinking["thinkingBudget"]; exists && !geminiCompleteInteger(value, -1, 32768) {
			return geminiCompleteRefusal()
		}
	}
	return nil
}

func (c *geminiCompleteCapture) catalog(command map[string]any, event geminiCompleteRow, stage string) error {
	scope := geminiCycleScope(stage)
	if event.Stage != stage || projectNativeRuntimeKeys(event.Value, "scope stdout stderr", "") != nil || event.Value["scope"] != scope {
		return geminiCompleteRefusal()
	}
	streams := map[string][]byte{}
	for _, name := range []string{"stdout", "stderr"} {
		body, err := geminiCompleteStream(command[name])
		other, otherErr := geminiCompleteStream(event.Value[name])
		if err != nil || otherErr != nil || !bytes.Equal(body, other) {
			return geminiCompleteRefusal()
		}
		streams[name] = body
	}
	want := map[string]geminiCycleSkillRow{}
	for name, description := range geminiCycleBuiltinDescription {
		want[name] = geminiCycleSkillRow{Description: description, Path: filepath.Join(filepath.Dir(c.Paths["client"]), "builtin", name, "SKILL.md"), Enabled: true, Builtin: true}
	}
	view, seen := c.Proof.Views[geminiCompleteProofStage(stage)], map[string]bool{}
	for _, key := range []string{"user", scope} {
		if seen[key] {
			continue
		}
		seen[key] = true
		r := view[key]
		want["foreign-"+key] = geminiCycleSkillRow{Description: "Foreign inert marker", Path: r.Profile + "/skills/foreign-" + key + "/SKILL.md"}
		if r.Binding.ClientBindingID != "" && !r.Removed {
			want[r.Name] = geminiCycleSkillRow{Description: r.Revision, Path: r.SkillPath, Enabled: true}
		}
	}
	_, advertised, err := c.phase(stage)
	if err != nil || !geminiCycleListingStderr(streams["stderr"], scope, advertised) {
		return geminiCompleteRefusal()
	}
	return geminiCycleSkills(streams["stdout"], want)
}
