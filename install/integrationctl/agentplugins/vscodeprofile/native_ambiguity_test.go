package vscodeprofile_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	vp "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

func conflictWithoutMutation(t *testing.T, req vp.Request) {
	t.Helper()
	before := bytes.Clone(req.Settings)
	r, err := vp.Plan(req)
	digest := vp.SnapshotDigest(before)
	if !errors.Is(err, vp.ErrConflict) || r.Changed || r.Receipt != nil || !bytes.Equal(r.Settings, before) || !bytes.Equal(req.Settings, before) || r.BeforeDigest != digest || r.AfterDigest != digest {
		t.Errorf("unsafe refusal: %+v err=%v", r, err)
	}
}

func installRepairVerifyConflict(t *testing.T, id vp.Identity, body, ambiguous []byte) {
	t.Helper()
	owned := plan(t, vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	conflictWithoutMutation(t, vp.Request{Settings: body, Identity: id, Action: vp.Install})
	for _, enabled := range []bool{true, false} {
		previous := owned.Receipt
		if !enabled {
			disabled := bytes.Replace(owned.Settings, []byte("true"), []byte("false"), 1)
			previous = plan(t, vp.Request{Settings: disabled, Identity: id, Action: vp.Install, Previous: previous}).Receipt
		}
		conflictWithoutMutation(t, vp.Request{Settings: body, Identity: id, Action: vp.Repair, Previous: previous})
		input := bytes.Clone(ambiguous)
		if _, err := vp.VerifyOwned(input, id, previous); !errors.Is(err, vp.ErrConflict) || !bytes.Equal(input, ambiguous) {
			t.Errorf("ambiguous native selector verified or mutated: %v", err)
		}
	}
}

// Red: native CR ends // and exposes false/true or duplicates, while the planner
// treats those members as absent trivia and issues or verifies ownership.
func TestBareCRLineCommentsRefuseNativeHiddenMembers(t *testing.T) {
	id := identity()
	for _, value := range []string{"false", "true"} {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/nested=%v", value, nested), func(t *testing.T) {
				body := []byte("{// TEST CR terminator\r\"chat.pluginLocations\":{\"/lab/plugins/通知\":" + value + "},\n\"foreign\":17}")
				ambiguous := bytes.Replace(body, []byte(`"foreign":17`), []byte(`"chat.pluginLocations":{"/lab/plugins/通知":true},"foreign":17`), 1)
				if nested {
					body = []byte("{\"chat.pluginLocations\":{// TEST CR terminator\r\"/lab/plugins/通知\":" + value + ",\n\"/foreign\":false},\"foreign\":17}")
					ambiguous = bytes.Replace(body, []byte(`"/foreign":false`), []byte(`"/lab/plugins/通知":true,"/foreign":false`), 1)
				}
				// Independent CR/LF trivia removal plus encoding/json must see it.
				if decoded(t, body)["chat.pluginLocations"].(map[string]any)[id.PluginRoot] != (value == "true") {
					t.Fatal("native-compatible grammar did not expose the CR-hidden member")
				}
				installRepairVerifyConflict(t, id, body, ambiguous)
			})
		}
	}
}

// Red: native file-URI drive paths escape collision comparison, adopting a
// foreign true/false or repairing beside it. Receipt booleans cannot excuse it.
func TestLeadingSeparatorDriveAliasesRefuseForeignChoices(t *testing.T) {
	id := identity()
	id.SettingsPath, id.PluginRoot = `C:\lab\profile\settings.json`, `C:\lab\plugins\通知`
	for _, key := range []string{"/C:/lab/plugins/通知", `\C:\lab\plugins\通知`, "\ufeff/c:/LAB\\plugins/other/../通知\u2003"} {
		for _, value := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", key, value), func(t *testing.T) {
				body, _ := json.Marshal(map[string]any{"chat.pluginLocations": map[string]bool{key: value}})
				ambiguous, _ := json.Marshal(map[string]any{"chat.pluginLocations": map[string]bool{key: value, id.PluginRoot: true}})
				installRepairVerifyConflict(t, id, body, ambiguous)
			})
		}
	}
	bad := id
	bad.PluginRoot = `\C:\lab\plugins\通知`
	conflictWithoutMutation(t, vp.Request{Settings: []byte(`{}`), Identity: bad, Action: vp.Install})
}

// Red: a broad CR ban rejects ordinary CR whitespace/block trivia/escaped text,
// or accepted CRLF/LF comment edits change foreign bytes or sticky false.
func TestUnambiguousCRTriviaPreservesExactDisabledSnapshot(t *testing.T) {
	id := identity()
	for _, trivia := range []string{"// TEST comment\r\n", "// TEST comment\n", "/* TEST // CR\r comment */\r", "\r"} {
		body := []byte("{" + trivia + `"foreign":"// escaped \r",` + "\r\"chat.pluginLocations\":{}}")
		owned := plan(t, vp.Request{Settings: body, Identity: id, Action: vp.Install})
		if !bytes.Contains(owned.Settings, []byte(trivia)) || decoded(t, owned.Settings)["foreign"] != "// escaped \r" {
			t.Fatal("safe foreign trivia/text changed")
		}
		disabled := bytes.Replace(owned.Settings, []byte("true"), []byte("false"), 1)
		repeat := plan(t, vp.Request{Settings: disabled, Identity: id, Action: vp.Install, Previous: owned.Receipt})
		if repeat.Changed || !repeat.Disabled || repeat.Receipt.Enabled || !bytes.Equal(repeat.Settings, disabled) {
			t.Fatal("safe CR trivia lost sticky disabled snapshot")
		}
		if v, err := vp.VerifyOwned(disabled, id, repeat.Receipt); err != nil || !v.Disabled {
			t.Fatal("safe CR snapshot not verified disabled", v, err)
		}
	}
	for _, suffix := range []string{"// TEST bare CR\r", "// TEST repeated CR\r\r\n"} {
		body := []byte(`{}` + suffix)
		conflictWithoutMutation(t, vp.Request{Settings: body, Identity: id, Action: vp.Install})
	}
}

// Red: comparison renames/adopts a distinct foreign URI path, changes the exact
// requested receipt key, or an absence repair re-enables a recorded false.
func TestDistinctDriveURIPathsKeepExactReceiptAndDisabledRepair(t *testing.T) {
	id := identity()
	id.SettingsPath, id.PluginRoot = `C:\lab\profile\settings.json`, `C:\lab\plugins\通知`
	body := []byte(`{"chat.pluginLocations":{"/D:/lab/plugins/通知":false,"\\C:\\lab\\plugins\\other":true}}`)
	owned := plan(t, vp.Request{Settings: body, Identity: id, Action: vp.Install})
	if owned.Receipt.Identity != id || !bytes.Contains(owned.Settings, body[1:len(body)-2]) {
		t.Fatal("foreign spelling or exact receipt identity changed")
	}
	// Change only the exact owned key, leaving the foreign true unchanged.
	key, _ := json.Marshal(id.PluginRoot)
	disabled := bytes.Replace(owned.Settings, append(bytes.Clone(key), []byte(":true")...), append(bytes.Clone(key), []byte(":false")...), 1)
	observed := plan(t, vp.Request{Settings: disabled, Identity: id, Action: vp.Install, Previous: owned.Receipt})
	if observed.Changed || !observed.Disabled || observed.Receipt.Identity != id || !bytes.Equal(observed.Settings, disabled) {
		t.Fatal("disabled exact key or receipt changed")
	}
	repaired := plan(t, vp.Request{Settings: body, Identity: id, Action: vp.Repair, Previous: observed.Receipt})
	locations := decoded(t, repaired.Settings)["chat.pluginLocations"].(map[string]any)
	if !repaired.Disabled || *repaired.Receipt != *observed.Receipt || locations[id.PluginRoot] != false || locations["/D:/lab/plugins/通知"] != false || locations[`\C:\lab\plugins\other`] != true {
		t.Fatal("disabled repair adopted/renamed foreign spelling or lost receipt")
	}
}
