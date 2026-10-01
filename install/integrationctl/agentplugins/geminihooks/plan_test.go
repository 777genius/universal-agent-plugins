package geminihooks_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	gh "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/geminihooks"
	"github.com/tailscale/hujson"
)

// Red condition: installing or reconciling owned groups loses foreign data,
// adopts an unreceipted command, or overwrites any edited own group.
const foreign = `{
 // Native policy is independent from the event map.
 "hooksConfig":{"enabled":false,"disabled":["foreign"],"notifications":false},
 "security":{"folderTrust":{"enabled":true}},
 "unknown":{"array":[null,1.25,{"str":"// literal"}]},
 "hooks":{"enabled":true,"disabled":["legacy"],"notifications":{"old":true},
 "FutureEvent":{"future":42},
 "BeforeTool":[{"matcher":"read_file","hooks":[{"type":"command","name":"foreign","command":"echo foreign","env":{"K":"V"}}],"unknown":7}],
 "AfterAgent":[{"hooks":[{"type":"command","name":"sibling","command":"echo sibling"}],"sequential":true}]}}
`

func specs() []gh.HookSpec {
	return []gh.HookSpec{
		{Event: "AfterAgent", Name: "acme.finish", Argv: []string{"/opt/acme tools/agent", "event", "finish"}, Timeout: 2500},
		{Event: "Notification", Name: "acme.permission", Argv: []string{"/opt/acme tools/agent", "event", "permission"}, Matcher: "ToolPermission", Timeout: 2500},
	}
}
func plan(t *testing.T, settings []byte, op gh.Operation, hooks []gh.HookSpec, previous *gh.Receipt) gh.Result {
	t.Helper()
	r, err := gh.Plan(gh.Request{Settings: settings, Shell: gh.Bash, Operation: op, Hooks: hooks, Previous: previous})
	if err != nil {
		t.Fatal(err)
	}
	if r.Conflict {
		t.Fatal("unexpected conflict")
	}
	return r
}
func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	b, err := hujson.Standardize(b)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err = json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func encode(t *testing.T, m map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func groups(m map[string]any, event string) []any { return m["hooks"].(map[string]any)[event].([]any) }
func ownGroup(m map[string]any, event string) map[string]any {
	g := groups(m, event)
	return g[len(g)-1].(map[string]any)
}
func conflict(t *testing.T, req gh.Request) {
	t.Helper()
	before := append([]byte(nil), req.Settings...)
	r, err := gh.Plan(req)
	if !errors.Is(err, gh.ErrConflict) || !r.Conflict || r.Receipt != nil || !bytes.Equal(r.Desired, req.Settings) || !bytes.Equal(before, req.Settings) {
		t.Fatalf("conflict: err=%v conflict=%v receipt=%v originalBytes=%v inputUnchanged=%v", err, r.Conflict, r.Receipt != nil, bytes.Equal(r.Desired, req.Settings), bytes.Equal(before, req.Settings))
	}
}

func TestLifecyclePreservesForeignPolicyAndSiblingEdits(t *testing.T) {
	input := []byte(foreign)
	saved := append([]byte(nil), input...)
	installed := plan(t, input, gh.Install, specs(), nil)
	if installed.NoOp || !bytes.Equal(input, saved) {
		t.Fatal("install no-op or mutated caller bytes")
	}
	before, after := decode(t, input), decode(t, installed.Desired)
	for _, k := range []string{"hooksConfig", "security", "unknown"} {
		if !reflect.DeepEqual(before[k], after[k]) {
			t.Fatalf("lost %s", k)
		}
	}
	bHooks, aHooks := before["hooks"].(map[string]any), after["hooks"].(map[string]any)
	for _, k := range []string{"enabled", "disabled", "notifications", "FutureEvent", "BeforeTool"} {
		if !reflect.DeepEqual(bHooks[k], aHooks[k]) {
			t.Fatalf("lost foreign hooks.%s", k)
		}
	}
	if len(aHooks) != len(bHooks)+1 || len(groups(after, "AfterAgent")) != 2 || len(groups(after, "Notification")) != 1 {
		t.Fatal("installed extra events/groups")
	}
	if err := gh.VerifyOwned(installed.Desired, installed.Receipt); err != nil {
		t.Fatal(err)
	}
	repeat := plan(t, installed.Desired, gh.Install, specs(), installed.Receipt)
	if !repeat.NoOp || !bytes.Equal(repeat.Desired, installed.Desired) {
		t.Fatal("repeat must preserve bytes")
	}
	after["newSibling"] = map[string]any{"keep": true}
	groups(after, "AfterAgent")[0].(map[string]any)["newField"] = "edited foreign"
	siblingEdited := encode(t, after)
	if err := gh.VerifyOwned(siblingEdited, installed.Receipt); err != nil {
		t.Fatalf("sibling revoked ownership: %v", err)
	}
	repeat = plan(t, siblingEdited, gh.Update, specs(), installed.Receipt)
	if !repeat.NoOp || !bytes.Equal(repeat.Desired, siblingEdited) {
		t.Fatal("foreign edits should be no-op")
	}
	changed := specs()
	changed[0].Argv[0] = "/opt/new helper"
	changed[1].Timeout = 3500
	updated := plan(t, siblingEdited, gh.Update, changed, installed.Receipt)
	if updated.NoOp || reflect.DeepEqual(updated.Receipt, installed.Receipt) {
		t.Fatal("update did not bind changed groups")
	}
	removed := plan(t, updated.Desired, gh.Remove, nil, updated.Receipt)
	final := decode(t, removed.Desired)
	if len(groups(final, "AfterAgent")) != 1 || len(groups(final, "Notification")) != 0 {
		t.Fatal("remove did not delete only owned singleton groups")
	}
	if !reflect.DeepEqual(groups(final, "AfterAgent")[0], groups(after, "AfterAgent")[0]) || !reflect.DeepEqual(final["newSibling"], after["newSibling"]) {
		t.Fatal("remove lost edited siblings")
	}
	for _, k := range []string{"hooksConfig", "security", "unknown"} {
		if !reflect.DeepEqual(final[k], before[k]) {
			t.Fatalf("remove lost %s", k)
		}
	}
	if removed.Receipt != nil {
		t.Fatal("remove retained receipt")
	}
}

func TestFullGroupDriftAndCrossEventCollisionReject(t *testing.T) {
	installed := plan(t, []byte(foreign), gh.Install, specs(), nil)
	edits := map[string]func(map[string]any){
		"group unknown": func(g map[string]any) { g["unknown"] = true },
		"matcher":       func(g map[string]any) { g["matcher"] = "different" },
		"sequential":    func(g map[string]any) { g["sequential"] = true },
		"name":          func(g map[string]any) { g["hooks"].([]any)[0].(map[string]any)["name"] = "renamed" },
		"command":       func(g map[string]any) { g["hooks"].([]any)[0].(map[string]any)["command"] = "echo edited" },
		"timeout":       func(g map[string]any) { g["hooks"].([]any)[0].(map[string]any)["timeout"] = 17 },
		"env":           func(g map[string]any) { g["hooks"].([]any)[0].(map[string]any)["env"] = map[string]any{"K": "V"} },
		"hook unknown":  func(g map[string]any) { g["hooks"].([]any)[0].(map[string]any)["extra"] = 42 },
		"type":          func(g map[string]any) { g["hooks"].([]any)[0].(map[string]any)["type"] = "runtime" },
		"extra hook":    func(g map[string]any) { g["hooks"] = append(g["hooks"].([]any), map[string]any{"name": "other"}) },
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			m := decode(t, installed.Desired)
			edit(ownGroup(m, "AfterAgent"))
			b := encode(t, m)
			if !errors.Is(gh.VerifyOwned(b, installed.Receipt), gh.ErrConflict) {
				t.Fatal("edited own group verified")
			}
			for _, op := range []gh.Operation{gh.Update, gh.Remove} {
				conflict(t, gh.Request{Settings: b, Shell: gh.Bash, Operation: op, Hooks: specs(), Previous: installed.Receipt})
			}
		})
	}
	for _, event := range []string{"AfterAgent", "BeforeTool", "Notification", "FutureArrayEvent"} {
		t.Run("collision "+event, func(t *testing.T) {
			m := decode(t, installed.Desired)
			h := m["hooks"].(map[string]any)
			extra := map[string]any{"hooks": []any{map[string]any{"type": "command", "name": "acme.finish", "command": "echo foreign"}}}
			if h[event] == nil {
				h[event] = []any{extra}
			} else {
				h[event] = append(h[event].([]any), extra)
			}
			b := encode(t, m)
			conflict(t, gh.Request{Settings: b, Shell: gh.Bash, Operation: gh.Update, Hooks: specs(), Previous: installed.Receipt})
			if gh.VerifyOwned(b, installed.Receipt) == nil {
				t.Fatal("duplicate verified")
			}
		})
	}
	m := decode(t, installed.Desired)
	g := ownGroup(m, "AfterAgent")
	m["hooks"].(map[string]any)["AfterAgent"] = groups(m, "AfterAgent")[:1]
	m["hooks"].(map[string]any)["BeforeTool"] = append(groups(m, "BeforeTool"), g)
	if gh.VerifyOwned(encode(t, m), installed.Receipt) == nil {
		t.Fatal("event move verified")
	}
}

func TestNoAdoptionReceiptValidationAndFreshSettings(t *testing.T) {
	installed := plan(t, nil, gh.Install, specs(), nil)
	for _, op := range []gh.Operation{gh.Install, gh.Update, gh.Remove} {
		conflict(t, gh.Request{Settings: installed.Desired, Shell: gh.Bash, Operation: op, Hooks: specs()})
	}
	for _, edit := range []func(*gh.Receipt){
		func(r *gh.Receipt) { r.Version++ }, func(r *gh.Receipt) { r.Groups = nil }, func(r *gh.Receipt) { r.Groups[0].Digest = "sha256:bad" },
		func(r *gh.Receipt) { r.Groups[0].Event = "BeforeAgent" }, func(r *gh.Receipt) { r.Groups = append(r.Groups, r.Groups[0]) },
	} {
		r := *installed.Receipt
		r.Groups = append([]gh.OwnedGroup(nil), r.Groups...)
		edit(&r)
		if gh.VerifyOwned(installed.Desired, &r) == nil {
			t.Fatal("invalid receipt verified")
		}
		conflict(t, gh.Request{Settings: installed.Desired, Shell: gh.Bash, Operation: gh.Update, Hooks: specs(), Previous: &r})
	}
	b, _ := json.Marshal(installed.Receipt)
	for _, secret := range []string{"/opt/acme", "ToolPermission", "command", "settings"} {
		if strings.Contains(string(b), secret) {
			t.Fatal("receipt retains source data")
		}
	}
	if gh.VerifyOwned(installed.Desired, nil) == nil {
		t.Fatal("nil receipt verified")
	}
	// Receipt ordering and object key ordering do not define ownership.
	m := decode(t, installed.Desired)
	if err := gh.VerifyOwned(encode(t, m), installed.Receipt); err != nil {
		t.Fatal(err)
	}
	h := specs()[:1]
	shrunk := plan(t, installed.Desired, gh.Update, h, installed.Receipt)
	if len(shrunk.Receipt.Groups) != 1 || len(groups(decode(t, shrunk.Desired), "Notification")) != 0 {
		t.Fatal("update left retired own hook")
	}
}

func TestMalformedAndAmbiguousSettingsConflict(t *testing.T) {
	for _, input := range []string{" ", "null", "[]", `{"hooks":null}`, `{"hooks":[],"x":1}`, `{"hooks":{"AfterAgent":{}}}`, `{"hooks":{},"hooks":{}}`, `{"x":{"a":1,"a":2}}`, `{"x":[1,]}`, `{"x":1,}`, `{"x":/* unterminated}`, `{"x":NaN}`} {
		t.Run(input, func(t *testing.T) {
			conflict(t, gh.Request{Settings: []byte(input), Shell: gh.Bash, Operation: gh.Install, Hooks: specs()})
		})
	}
	conflict(t, gh.Request{Settings: bytes.Repeat([]byte(" "), gh.MaxSettingsBytes+1), Shell: gh.Bash, Operation: gh.Install, Hooks: specs()})
	conflict(t, gh.Request{Shell: gh.Bash, Operation: "repair", Hooks: specs()})
	for _, edit := range []func([]gh.HookSpec){
		func(h []gh.HookSpec) { h[0].Event = "MadeUp" }, func(h []gh.HookSpec) { h[0].Name = "" }, func(h []gh.HookSpec) { h[1].Name = h[0].Name },
		func(h []gh.HookSpec) { h[0].Argv = nil }, func(h []gh.HookSpec) { h[0].Timeout = -1 }, func(h []gh.HookSpec) { h[0].Name = "$OWNER" },
		func(h []gh.HookSpec) { h[0].Matcher = "${MATCH}" }, func(h []gh.HookSpec) { h[0].Argv[0] = "${BINARY}" },
	} {
		h := specs()
		edit(h)
		conflict(t, gh.Request{Shell: gh.Bash, Operation: gh.Install, Hooks: h})
	}
}

// Independent neutral UAP caller: different name, binary, event and matcher;
// no application policy, state, or notification constants participate.
func TestIndependentCaller(t *testing.T) {
	h := []gh.HookSpec{{Event: "BeforeTool", Name: "example.audit", Argv: []string{"/usr/local/audit", "--fixed", "百分比%"}, Matcher: "write_file", Timeout: 1000}}
	r := plan(t, nil, gh.Install, h, nil)
	if err := gh.VerifyOwned(r.Desired, r.Receipt); err != nil {
		t.Fatal(err)
	}
	m := decode(t, r.Desired)
	if len(m["hooks"].(map[string]any)) != 1 {
		t.Fatal("extra event placeholders")
	}
	g := ownGroup(m, "BeforeTool")
	if g["matcher"] != "write_file" || len(g["hooks"].([]any)) != 1 {
		t.Fatal("wrong native singleton")
	}
	plan(t, r.Desired, gh.Remove, nil, r.Receipt)
}

// Red condition: sibling schema edits revoke valid ownership, or reconciling
// several groups changes foreign ordering, comments or the caller's bytes.
func TestOpaqueSiblingsCommentsAndMultipleOwnedGroups(t *testing.T) {
	h := specs()
	h = append(h, gh.HookSpec{Event: "AfterAgent", Name: "acme.second", Argv: []string{"another binary", "fixed"}})
	r := plan(t, []byte(foreign), gh.Install, h, nil)
	m := decode(t, r.Desired)
	hooks := m["hooks"].(map[string]any)
	hooks["BeforeTool"] = map[string]any{"opaque": []any{false, nil}}
	hooks["AfterAgent"] = append([]any{17, map[string]any{"future": true}}, hooks["AfterAgent"].([]any)...)
	settings := encode(t, m)
	m = decode(t, settings)
	if err := gh.VerifyOwned(settings, r.Receipt); err != nil {
		t.Fatal("foreign schema edit revoked own groups:", err)
	}
	repeat := plan(t, settings, gh.Update, h, r.Receipt)
	if !repeat.NoOp || !bytes.Equal(repeat.Desired, settings) {
		t.Fatal("opaque sibling edit changed no-op")
	}
	reversed := *r.Receipt
	reversed.Groups = append([]gh.OwnedGroup(nil), r.Receipt.Groups...)
	reversed.Groups[0], reversed.Groups[2] = reversed.Groups[2], reversed.Groups[0]
	if err := gh.VerifyOwned(settings, &reversed); err != nil {
		t.Fatal("receipt order matters:", err)
	}
	changed := append([]gh.HookSpec(nil), h...)
	changed[0].Timeout = 123
	updated := plan(t, settings, gh.Update, changed, &reversed)
	after := decode(t, updated.Desired)
	if !reflect.DeepEqual(groups(m, "AfterAgent")[:3], groups(after, "AfterAgent")[:3]) {
		t.Fatal("foreign order changed")
	}
	removed := plan(t, updated.Desired, gh.Remove, nil, updated.Receipt)
	if len(groups(decode(t, removed.Desired), "AfterAgent")) != 3 {
		t.Fatal("multi-own removal dropped sibling")
	}
	// Comments around foreign groups and at root survive all three operations.
	input := []byte(`{/* root */"hooks":{"AfterAgent":[/* foreign */{"hooks":[{"name":"sibling","command":"echo '//literal'"}]}/* closing */]}}`)
	saved := append([]byte(nil), input...)
	installed := plan(t, input, gh.Install, specs(), nil)
	updated = plan(t, installed.Desired, gh.Update, changed[:2], installed.Receipt)
	removed = plan(t, updated.Desired, gh.Remove, nil, updated.Receipt)
	for _, b := range [][]byte{installed.Desired, updated.Desired, removed.Desired} {
		for _, c := range []string{"/* root */", "/* foreign */", "/* closing */", "//literal"} {
			if !bytes.Contains(b, []byte(c)) {
				t.Fatalf("lost %s in %s", c, b)
			}
		}
	}
	if !bytes.Equal(saved, input) {
		t.Fatal("planner mutated caller bytes")
	}
}
