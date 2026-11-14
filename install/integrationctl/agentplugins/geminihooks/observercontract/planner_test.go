package observercontract_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	gh "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/geminihooks"
)

type settingsDocument struct {
	Hooks map[string][]struct {
		Hooks []struct {
			Name    string `json:"name"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func hookCommand(t *testing.T, settings []byte, hook gh.HookSpec) string {
	t.Helper()
	var doc settingsDocument
	if err := json.Unmarshal(settings, &doc); err != nil {
		t.Fatal(err)
	}
	for _, group := range doc.Hooks[hook.Event] {
		for _, entry := range group.Hooks {
			if entry.Name == hook.Name {
				if entry.Timeout != hook.Timeout {
					t.Fatalf("native timeout changed: %d, want %d", entry.Timeout, hook.Timeout)
				}
				return entry.Command
			}
		}
	}
	t.Fatal("requested native hook missing")
	return ""
}

func plannedCommand(t *testing.T, shell gh.Shell, hook gh.HookSpec) string {
	t.Helper()
	result := planRequest(t, gh.Request{Shell: shell, Operation: gh.Install, Hooks: []gh.HookSpec{hook}})
	return hookCommand(t, result.Desired, hook)
}

func planRequest(t *testing.T, req gh.Request) gh.Result {
	t.Helper()
	result, err := gh.Plan(req)
	if err != nil || result.Conflict {
		t.Fatalf("Plan conflict=%v: %v", result.Conflict, err)
	}
	if req.Operation != gh.Remove {
		if err = gh.VerifyOwned(result.Desired, result.Receipt); err != nil {
			t.Fatal("actual command bytes failed ownership:", err)
		}
	}
	return result
}

func assertConflict(t *testing.T, req gh.Request) {
	t.Helper()
	original := append([]byte(nil), req.Settings...)
	result, err := gh.Plan(req)
	if !errors.Is(err, gh.ErrConflict) || !result.Conflict || result.Receipt != nil ||
		!bytes.Equal(result.Desired, original) || !bytes.Equal(req.Settings, original) {
		t.Fatalf("invalid input mutated settings or returned ownership: %+v %v", result, err)
	}
}

func TestZeroObserverRenderingUnchanged(t *testing.T) {
	for shell, want := range map[gh.Shell]string{
		gh.Bash:       `exec -- '/tool path/a'"'"'b' 'space inside' 'literal$'`,
		gh.PowerShell: `& '/tool path/a''b' 'space inside' 'literal$'`,
	} {
		t.Run(string(shell), func(t *testing.T) {
			hook := gh.HookSpec{Event: "BeforeTool", Name: "generic", Argv: []string{"/tool path/a'b", "space inside", "literal$"}}
			if got := plannedCommand(t, shell, hook); got != want {
				t.Fatalf("zero-value command=%q, want original bytes %q", got, want)
			}
			if got, err := gh.RenderArgv(shell, hook.Argv); err != nil || got != want {
				t.Fatalf("public generic renderer changed: %q %v", got, err)
			}
		})
	}
}

func TestObserverUnsupportedEventsBeforeMutation(t *testing.T) {
	for _, shell := range []gh.Shell{gh.Bash, gh.PowerShell} {
		valid := gh.HookSpec{Event: "AfterAgent", Name: "valid", Argv: []string{"fixture"}, Observer: true}
		base := planRequest(t, gh.Request{Shell: shell, Operation: gh.Install, Hooks: []gh.HookSpec{valid}})
		for _, event := range []string{"BeforeTool", "AfterTool", "BeforeAgent", "SessionStart", "SessionEnd", "PreCompress", "BeforeModel", "AfterModel", "BeforeToolSelection", "MadeUp", ""} {
			t.Run(string(shell)+"/"+event, func(t *testing.T) {
				invalid := valid
				invalid.Event, invalid.Name = event, "invalid"
				for _, op := range []gh.Operation{gh.Install, gh.Update} {
					assertConflict(t, gh.Request{Settings: base.Desired, Shell: shell, Operation: op,
						Hooks: []gh.HookSpec{valid, invalid}, Previous: base.Receipt})
				}
			})
		}
	}
}

func TestObserverRetainsLiteralValidation(t *testing.T) {
	for _, shell := range []gh.Shell{gh.Bash, gh.PowerShell} {
		for _, arg := range []string{"$VAR", "${VAR:-default}", "${arbitrary name}", "x\x00y", "x\ny", "x\ry", string([]byte{0xff})} {
			hook := gh.HookSpec{Event: "Notification", Name: "observer", Argv: []string{"fixture", arg}, Observer: true}
			assertConflict(t, gh.Request{Shell: shell, Operation: gh.Install, Hooks: []gh.HookSpec{hook}})
		}
	}
	for _, argv := range [][]string{nil, {""}, {"fixture", ""}, {"fixture", `double"quote`}} {
		hook := gh.HookSpec{Event: "AfterAgent", Name: "observer", Argv: argv, Observer: true}
		assertConflict(t, gh.Request{Shell: gh.PowerShell, Operation: gh.Install, Hooks: []gh.HookSpec{hook}})
	}
}

func TestObserverOwnershipLifecycle(t *testing.T) {
	for _, shell := range []gh.Shell{gh.Bash, gh.PowerShell} {
		for _, event := range []string{"AfterAgent", "Notification"} {
			t.Run(string(shell)+"/"+event, func(t *testing.T) { assertLifecycle(t, shell, event) })
		}
	}
}

func assertLifecycle(t *testing.T, shell gh.Shell, event string) {
	t.Helper()
	input := []byte(`{"hooksConfig":{"enabled":false},"security":{"keep":true},"hooks":{"AfterAgent":[{"hooks":[{"name":"foreign","command":"echo keep"}]}]}}`)
	hook := gh.HookSpec{Event: event, Name: "observer", Argv: []string{"/fixture path/observer", "apostrophe's", "literal$"}, Timeout: 2500}
	base := planRequest(t, gh.Request{Settings: input, Shell: shell, Operation: gh.Install, Hooks: []gh.HookSpec{hook}})
	hook.Observer = true
	updated := planRequest(t, gh.Request{Settings: base.Desired, Shell: shell, Operation: gh.Update, Hooks: []gh.HookSpec{hook}, Previous: base.Receipt})
	if updated.NoOp || hookCommand(t, base.Desired, hook) == hookCommand(t, updated.Desired, hook) || reflect.DeepEqual(base.Receipt, updated.Receipt) {
		t.Fatal("observer opt-in did not change actual command and digest")
	}
	// The receipt remains JSON-portable and binds the resulting native bytes.
	body, err := json.Marshal(updated.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	var receipt gh.Receipt
	if err = json.Unmarshal(body, &receipt); err != nil {
		t.Fatal(err)
	}
	for _, op := range []gh.Operation{gh.Install, gh.Update} {
		repeat := planRequest(t, gh.Request{Settings: updated.Desired, Shell: shell, Operation: op, Hooks: []gh.HookSpec{hook}, Previous: &receipt})
		if !repeat.NoOp || !bytes.Equal(repeat.Desired, updated.Desired) || !reflect.DeepEqual(repeat.Receipt, &receipt) {
			t.Fatal("observer repeat changed bytes or receipt")
		}
	}
	assertDrift(t, shell, updated, hook)
	assertOptOut(t, shell, base, updated, hook)
	hook.Argv, hook.Timeout = []string{"/new observer", "fixed"}, 3500
	changed := planRequest(t, gh.Request{Settings: updated.Desired, Shell: shell, Operation: gh.Update, Hooks: []gh.HookSpec{hook}, Previous: &receipt})
	if changed.NoOp || reflect.DeepEqual(changed.Receipt, &receipt) {
		t.Fatal("observer update did not rebind command bytes")
	}
	_ = hookCommand(t, changed.Desired, hook)
	removed := planRequest(t, gh.Request{Settings: changed.Desired, Shell: shell, Operation: gh.Remove, Previous: changed.Receipt})
	if removed.Receipt != nil || bytes.Contains(removed.Desired, []byte("/new observer")) {
		t.Fatal("removal retained observer command or receipt")
	}
	assertForeign(t, removed.Desired)
}

func assertOptOut(t *testing.T, shell gh.Shell, base gh.Result, updated gh.Result, hook gh.HookSpec) {
	t.Helper()
	hook.Observer = false
	result := planRequest(t, gh.Request{Settings: updated.Desired, Shell: shell, Operation: gh.Update, Hooks: []gh.HookSpec{hook}, Previous: updated.Receipt})
	if result.NoOp || !bytes.Equal(result.Desired, base.Desired) || !reflect.DeepEqual(result.Receipt, base.Receipt) {
		t.Fatal("opting out did not restore the original generic bytes and digest")
	}
}

func assertDrift(t *testing.T, shell gh.Shell, result gh.Result, hook gh.HookSpec) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(result.Desired, &doc); err != nil {
		t.Fatal(err)
	}
	groups := doc["hooks"].(map[string]any)[hook.Event].([]any)
	entry := groups[len(groups)-1].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	entry["command"] = entry["command"].(string) + " "
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(gh.VerifyOwned(body, result.Receipt), gh.ErrConflict) {
		t.Fatal("observer command-byte drift verified")
	}
	for _, op := range []gh.Operation{gh.Update, gh.Remove} {
		assertConflict(t, gh.Request{Settings: body, Shell: shell, Operation: op, Hooks: []gh.HookSpec{hook}, Previous: result.Receipt})
	}
}

func assertForeign(t *testing.T, settings []byte) {
	t.Helper()
	var doc settingsDocument
	if err := json.Unmarshal(settings, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Hooks["AfterAgent"]) != 1 || len(doc.Hooks["Notification"]) != 0 ||
		len(doc.Hooks["AfterAgent"][0].Hooks) != 1 || doc.Hooks["AfterAgent"][0].Hooks[0].Command != "echo keep" ||
		!bytes.Contains(settings, []byte(`"hooksConfig":{"enabled":false}`)) || !bytes.Contains(settings, []byte(`"security":{"keep":true}`)) {
		t.Fatal("removal changed foreign policy or hooks:", string(settings))
	}
}
