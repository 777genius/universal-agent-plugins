//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type geminiCompleteContractPhase struct {
	stage, scope, prompt, sentinel, operation, wire, native, tool string
	r                                                             *geminiCycleRecipe
	advertised                                                    []*geminiCycleRecipe
	tools                                                         []string
	facts                                                         map[string]any
}

func (f *geminiCompleteContractFixture) phase(stage string, nativeIndex int) geminiCompleteContractPhase {
	proofStage := map[string]string{"installed-user": "add-user", "installed-a": "add-a", "installed-b": "add-b", "restarted-a": "intact-repair-a", "updated-a": "update-a", "repaired-a": "missing-repair-a", "removed-a": "remove-a", "removed-b": "remove-b", "removed-user": "remove-user"}[stage]
	if proofStage == "" && !strings.HasPrefix(stage, "baseline-") {
		proofStage = stage
	}
	scope := "a"
	if strings.HasSuffix(stage, "-user") {
		scope = "user"
	}
	if strings.HasSuffix(stage, "-b") {
		scope = "b"
	}
	view := f.views[proofStage]
	r := view[scope]
	p := geminiCompleteContractPhase{stage: stage, scope: scope, r: r, operation: "none", tools: []string{}, prompt: "Disposable Gemini fixture stage " + stage + ". Call only the preselected installed inspect_runtime tool when requested.", sentinel: "UAP_GEMINI_DONE_" + stage + "_" + r.Nonce}
	if stage == "installed-user" || stage == "installed-a" || stage == "installed-b" {
		p.operation = "write"
	}
	if stage == "restarted-a" || stage == "updated-a" || stage == "repaired-a" {
		p.operation = "read"
	}
	seen := map[string]bool{}
	for _, key := range []string{"user", scope} {
		other := view[key]
		if !seen[key] && other.Binding.ClientBindingID != "" && !other.Removed {
			p.advertised = append(p.advertised, other)
			p.tools = append(p.tools, "mcp_"+other.Name+"_inspect_runtime")
		}
		seen[key] = true
	}
	if p.operation != "none" {
		p.tool = "mcp_" + r.Name + "_inspect_runtime"
		p.wire = "uap_cycle_" + geminiCycleSHA([]byte(stage + r.Nonce))[:24]
		p.native = p.tool + "__" + p.wire
		selected := 0
		for index, advertised := range p.advertised {
			if advertised.Scope == scope {
				selected = index
			}
		}
		pid := 1000 + nativeIndex*2 + selected
		p.facts = map[string]any{"nonce": r.Nonce, "revision": r.Revision, "cwd": r.Active, "argv": []string{"argument with spaces", r.Active + "/argument", "${UNKNOWN}"}, "plugin_root": r.Active, "plugin_data": r.Data, "literal": "literal  " + f.paths["home"], "once": r.Active + "/", "marker": "retained-" + r.Nonce, "pid": pid, "session": fmt.Sprintf("%d-%d", pid, 1793140000000000000+nativeIndex*2+selected)}
	}
	return p
}

func (f *geminiCompleteContractFixture) declaration(tool string) map[string]any {
	properties := map[string]any{"nonce": map[string]any{"type": "string"}, "operation": map[string]any{"type": "string"}, "marker": map[string]any{"type": "string"}, "wait_for_previous": map[string]any{"type": "boolean", "description": "Set to true to wait for all previously requested tools in this turn to complete before starting. Set to false (or omit) to run in parallel. Use true when this tool depends on the output of previous tools."}}
	return map[string]any{"name": tool, "description": "Inspect only disposable fixture runtime facts.", "parametersJsonSchema": map[string]any{"type": "object", "properties": properties, "required": []string{"nonce"}, "additionalProperties": false}}
}

func (f *geminiCompleteContractFixture) provider(p geminiCompleteContractPhase) {
	declarations := []any{}
	for _, name := range p.tools {
		declarations = append(declarations, f.declaration(name))
	}
	contents := []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": p.prompt}}}}
	first := map[string]any{"tools": []any{map[string]any{"functionDeclarations": declarations}}, "contents": contents}
	requests := []map[string]any{first}
	args := map[string]any{"nonce": p.r.Nonce, "operation": p.operation, "marker": "retained-" + p.r.Nonce}
	if p.operation != "none" {
		call := map[string]any{"functionCall": map[string]any{"id": p.wire, "name": p.tool, "args": args}, "thoughtSignature": "skip_thought_signature_validator"}
		returned := map[string]any{"functionResponse": map[string]any{"id": p.wire, "name": p.tool, "response": map[string]any{"output": "<untrusted_context>\n" + string(geminiCompleteContractEncode(f.t, p.facts)) + "\n</untrusted_context>"}}}
		second := map[string]any{"tools": first["tools"], "contents": append(append([]any{}, contents...), map[string]any{"role": "model", "parts": []any{call}}, map[string]any{"role": "user", "parts": []any{returned}})}
		requests = append(requests, second)
	}
	for index, request := range requests {
		f.add("cycle-provider-requests.jsonl", p.stage, map[string]any{"index": index, "stream": f.stream(geminiCompleteContractEncode(f.t, request))})
		part := map[string]any{"text": p.sentinel}
		if p.operation != "none" && index == 0 {
			part = map[string]any{"functionCall": map[string]any{"id": p.native, "name": p.tool, "args": args}}
		}
		response := map[string]any{"candidates": []any{map[string]any{"index": 0, "content": map[string]any{"role": "model", "parts": []any{part}}, "finishReason": "STOP"}}, "usageMetadata": map[string]any{"promptTokenCount": 1, "candidatesTokenCount": 1, "totalTokenCount": 2}}
		body := append(append([]byte("data: "), geminiCompleteContractEncode(f.t, response)...), []byte("\n\n")...)
		f.add("cycle-provider-responses.jsonl", p.stage, map[string]any{"index": index, "stream": f.stream(body)})
	}
	f.add("cycle-provider-receipts.jsonl", p.stage, map[string]any{"scope": p.scope, "operation": p.operation, "tools": p.tools, "native_call_id": p.native, "wire_call_id": p.wire, "prompt": p.prompt, "sentinel": p.sentinel, "requests": len(requests), "responses": len(requests), "issued": p.operation != "none", "returned": p.operation != "none", "error": false})
}

func (f *geminiCompleteContractFixture) native(p geminiCompleteContractPhase, start time.Time, nativeIndex int) ([]byte, []byte) {
	session := "inert-never-executed-native-" + p.stage
	events := []map[string]any{{"type": "init", "timestamp": f.date(start.Add(100 * time.Millisecond)), "session_id": session, "model": "gemini-2.5-flash"}, {"type": "message", "timestamp": f.date(start.Add(200 * time.Millisecond)), "role": "user", "content": p.prompt}}
	if p.operation != "none" {
		args := map[string]any{"nonce": p.r.Nonce, "operation": p.operation, "marker": "retained-" + p.r.Nonce}
		events = append(events, map[string]any{"type": "tool_use", "timestamp": f.date(start.Add(300 * time.Millisecond)), "tool_name": p.tool, "tool_id": p.native, "parameters": args}, map[string]any{"type": "tool_result", "timestamp": f.date(start.Add(400 * time.Millisecond)), "tool_id": p.native, "status": "success", "output": string(geminiCompleteContractEncode(f.t, p.facts))})
	}
	events = append(events, map[string]any{"type": "message", "timestamp": f.date(start.Add(450 * time.Millisecond)), "role": "assistant", "content": p.sentinel, "delta": true}, map[string]any{"type": "result", "timestamp": f.date(start.Add(460 * time.Millisecond)), "status": "success"})
	var body, probe []byte
	for _, event := range events {
		body = append(append(body, geminiCompleteContractEncode(f.t, event)...), '\n')
	}
	for group, recipe := range p.advertised {
		if f.aliasedNonce && p.scope == "a" && len(p.advertised) == 2 && recipe.Scope == "user" {
			continue
		}
		pid := 1000 + nativeIndex*2 + group
		methods := []string{"initialize", "tools/list"}
		if p.operation != "none" && recipe.Scope == p.scope {
			methods = append(methods, "tools/call")
		}
		for index, method := range methods {
			var facts any
			if method == "tools/call" {
				facts = p.facts
			}
			event := map[string]any{"method": method, "pid": pid, "session": fmt.Sprintf("%d-%d", pid, 1793140000000000000+nativeIndex*2+group), "nonce": recipe.Nonce, "timestamp": f.date(start.Add(time.Duration(260+index*20) * time.Millisecond)), "facts": facts}
			probe = append(append(probe, geminiCompleteContractEncode(f.t, event)...), '\n')
		}
	}
	f.add("cycle-native-events.jsonl", p.stage, map[string]any{"scope": p.scope, "session": session, "stream": f.stream(body)})
	return body, probe
}

func (f *geminiCompleteContractFixture) catalog(p geminiCompleteContractPhase) []byte {
	lines := map[string]string{}
	for name, description := range geminiCycleBuiltinDescription {
		lines[name] = name + " [Enabled] [Built-in]\n  Description: " + description + "\n  Location:    " + filepath.Join(filepath.Dir(f.paths["client"]), "builtin", name, "SKILL.md") + "\n"
	}
	proofStage := map[string]string{"installed-user": "add-user", "installed-a": "add-a", "installed-b": "add-b", "updated-a": "update-a", "repaired-a": "missing-repair-a", "removed-a": "remove-a", "removed-b": "remove-b", "removed-user": "remove-user"}[p.stage]
	if strings.HasPrefix(p.stage, "baseline-") {
		proofStage = ""
	} else if proofStage == "" {
		proofStage = p.stage
	}
	view, seen := f.views[proofStage], map[string]bool{}
	for _, scope := range []string{"user", p.scope} {
		if seen[scope] {
			continue
		}
		seen[scope] = true
		r := view[scope]
		name := "foreign-" + scope
		lines[name] = name + " [Disabled]\n  Description: Foreign inert marker\n  Location:    " + r.Profile + "/skills/" + name + "/SKILL.md\n"
		if r.Binding.ClientBindingID != "" && !r.Removed {
			lines[r.Name] = r.Name + " [Enabled]\n  Description: " + r.Revision + "\n  Location:    " + r.SkillPath + "\n"
		}
	}
	names := make([]string, 0, len(lines))
	for name := range lines {
		names = append(names, name)
	}
	sort.Strings(names)
	body := []byte("Discovered Agent Skills:\n")
	for _, name := range names {
		body = append(body, []byte(lines[name])...)
	}
	return body
}

func (f *geminiCompleteContractFixture) observations() {
	nativeIndex := 0
	for index, tag := range []string{"listing:baseline-a", "listing:baseline-b", "listing:baseline-user", "native:baseline-a", "listing:installed-user", "native:installed-user", "listing:installed-a", "native:installed-a", "listing:installed-b", "native:installed-b", "listing:repeat-add-a", "listing:unchanged-update-a", "listing:intact-repair-a", "native:restarted-a", "listing:updated-a", "native:updated-a", "listing:repaired-a", "native:repaired-a", "listing:removed-a", "native:removed-a", "listing:removed-b", "native:removed-b", "listing:removed-user", "native:removed-user"} {
		kind, stage, _ := strings.Cut(tag, ":")
		p := f.phase(stage, nativeIndex)
		start := f.started.Add(time.Duration(index+2) * time.Second)
		end := start.Add(500 * time.Millisecond)
		argv := []string{f.paths["node"], f.paths["client"], "skills", "list", "--all"}
		var stdout, probe []byte
		if kind == "native" {
			argv = []string{f.paths["node"], f.paths["client"], "--prompt", p.prompt, "--approval-mode", "default", "--output-format", "stream-json", "--model", "gemini-2.5-flash"}
			f.provider(p)
			stdout, probe = f.native(p, start, nativeIndex)
			nativeIndex++
		} else {
			stdout = f.catalog(p)
			f.add("cycle-discovery.jsonl", stage, map[string]any{"scope": p.scope, "stdout": f.stream(stdout), "stderr": f.stream([]byte{})})
		}
		f.add("cycle-native-commands.jsonl", tag, map[string]any{"kind": kind, "scope": p.scope, "argv": argv, "cwd": p.r.Project, "exit": 0, "error": false, "context_error": false, "started_at": f.date(start), "finished_at": f.date(end), "elapsed_ms": 500, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure", "stdout": f.stream(stdout), "stderr": f.stream([]byte{})})
		probeTag := stage
		if kind == "listing" {
			probeTag = tag
		}
		f.add("cycle-probe-events.jsonl", probeTag, map[string]any{"scope": p.scope, "stream": f.stream(probe)})
	}
}
