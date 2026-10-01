package vscodeprofile_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	vp "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

// Red: absence is treated as install, repair moves identity, or drift produces a new receipt.
func TestRepairRequiresRecordedAbsentSelector(t *testing.T) {
	id := identity()
	installed := plan(t, vp.Request{Settings: []byte(`{"foreign":42}`), Identity: id, Action: vp.Install})
	absent := []byte(`{/* late foreign edit */"foreign":43,"chat.pluginLocations":{"/foreign":false,},}`)
	req := vp.Request{Settings: absent, Identity: id, Previous: installed.Receipt, Action: vp.Repair}
	repaired := plan(t, req)
	if !repaired.Changed || !bytes.Contains(repaired.Settings, []byte(`"/foreign":false`)) || !bytes.Contains(repaired.Settings, []byte("/* late foreign edit */")) {
		t.Fatal("repair lost siblings")
	}
	if *repaired.Receipt != *installed.Receipt {
		t.Fatal("repair changed recorded identity")
	}
	req.Settings = repaired.Settings
	again := plan(t, req)
	if again.Changed || !bytes.Equal(again.Settings, repaired.Settings) {
		t.Fatal("repair not repeatable")
	}
	for _, value := range []string{`null`, `"true"`, `{}`, `[]`, `1`} {
		req.Settings = []byte(`{"chat.pluginLocations":{"/lab/plugins/通知":` + value + `}}`)
		for _, a := range []vp.Action{vp.Install, vp.Update, vp.Repair, vp.Remove, vp.Enable} {
			req.Action = a
			refused(t, req)
		}
	}
	req.Action = vp.Repair
	req.Settings = absent
	req.Previous = nil
	refused(t, req)
	if _, err := vp.VerifyOwned(absent, id, installed.Receipt); !errors.Is(err, vp.ErrConflict) {
		t.Fatal("absent selector verified")
	}
	// Repair of a newly observed false returns no document change, never true.
	req.Previous = installed.Receipt
	req.Settings = bytes.Replace(installed.Settings, []byte("true"), []byte("false"), 1)
	disabled := plan(t, req)
	if disabled.Changed || !disabled.Disabled {
		t.Fatal("repair altered false")
	}
	req.Previous = disabled.Receipt
	req.Settings = absent
	restoredFalse := plan(t, req)
	if !restoredFalse.Disabled || restoredFalse.Receipt.Enabled {
		t.Fatal("absence re-enabled disabled receipt")
	}
	root := decoded(t, restoredFalse.Settings)
	if root["chat.pluginLocations"].(map[string]any)[id.PluginRoot] != false {
		t.Fatal("disabled repair bytes wrong")
	}
}

// Red: any receipt field can be changed without refusal, or a moved selector is adopted.
func TestReceiptBindsFullIdentityAndValue(t *testing.T) {
	id := identity()
	installed := plan(t, vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	changes := []func(*vp.Identity){
		func(i *vp.Identity) { i.SettingsPath = "/lab/other/settings.json" },
		func(i *vp.Identity) { i.ProfileID = "other" },
		func(i *vp.Identity) { i.PluginRoot = "/lab/plugins/moved" },
		func(i *vp.Identity) { i.PackageID = "other" },
		func(i *vp.Identity) { i.PackageDigest = "sha256:" + strings.Repeat("c", 64) },
		func(i *vp.Identity) { i.ProjectionDigest = "sha256:" + strings.Repeat("d", 64) },
	}
	for n, change := range changes {
		changed := id
		change(&changed)
		for _, a := range []vp.Action{vp.Install, vp.Remove, vp.Repair, vp.Enable} {
			refused(t, vp.Request{Settings: installed.Settings, Identity: changed, Previous: installed.Receipt, Action: a})
		}
		if _, err := vp.VerifyOwned(installed.Settings, changed, installed.Receipt); err == nil {
			t.Fatalf("identity field %d unbound", n)
		}
		if n < 4 {
			refused(t, vp.Request{Settings: installed.Settings, Identity: changed, Previous: installed.Receipt, Action: vp.Update})
		}
	}
	mutations := []func(*vp.Receipt){
		func(r *vp.Receipt) { r.Version = "2" }, func(r *vp.Receipt) { r.Selector = "chat.other" },
		func(r *vp.Receipt) { r.Enabled = false }, func(r *vp.Receipt) { r.Digest = "sha256:" + strings.Repeat("0", 64) },
		func(r *vp.Receipt) { r.Identity.ProfileID = "other" },
	}
	for _, mutate := range mutations {
		changed := *installed.Receipt
		mutate(&changed)
		refused(t, vp.Request{Settings: installed.Settings, Identity: id, Previous: &changed, Action: vp.Repair})
	}
	updatedID := id
	updatedID.PackageDigest = "sha256:" + strings.Repeat("c", 64)
	updated := plan(t, vp.Request{Settings: installed.Settings, Identity: updatedID, Previous: installed.Receipt, Action: vp.Update})
	if updated.Changed || updated.Receipt.Identity != updatedID || updated.Receipt.Digest == installed.Receipt.Digest {
		t.Fatal("update did not rebind projection")
	}
}

// Red: aliases/duplicate decoded keys bypass foreign ownership, including during absent repair.
func TestSelectorAmbiguityAndAliases(t *testing.T) {
	id := identity()
	installed := plan(t, vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	bad := []string{
		`{"chat.pluginLocations":{},"chat.\u0070luginLocations":{}}`,
		`{"chat.pluginLocations":{"/lab/plugins/通知":true,"/lab/plugins/\u901a\u77e5":false}}`,
		`{"chat.pluginLocations":{"/foreign":true,"/foreign":false}}`,
		`{"chat.pluginLocations":{"/lab/plugins/./通知":false}}`,
		`{"chat.pluginLocations":{" /lab/plugins/通知 ":true}}`,
		`{"chat.pluginLocations":{"/lab/plugins/通知\ufeff":true}}`,
		`{"chat.pluginLocations":{"/\ud800":true}}`,
		`{"chat.pluginLocations":false}`, `{"chat.pluginLocations":[]}`,
	}
	for _, body := range bad {
		for _, a := range []vp.Action{vp.Install, vp.Update, vp.Remove, vp.Repair} {
			req := vp.Request{Settings: []byte(body), Identity: id, Action: a}
			if a != vp.Install {
				req.Previous = installed.Receipt
			}
			refused(t, req)
		}
	}
	// Native key identity equates paired surrogate escapes with literal Unicode.
	rocket := id
	rocket.PluginRoot = "/lab/plugins/🚀"
	refused(t, vp.Request{Settings: []byte(`{"chat.pluginLocations":{"/lab/plugins/🚀":true,"/lab/plugins/\ud83d\ude80":false}}`), Identity: rocket, Action: vp.Install})
}

// Red: valid paired UTF-16 spelling is rejected or rewrites the native-equivalent owned key.
func TestNativeUnicodeEscapeOwnershipIsByteExact(t *testing.T) {
	id := identity()
	id.PluginRoot = "/lab/plugins/🚀"
	installed := plan(t, vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	escaped := bytes.Replace(installed.Settings, []byte("🚀"), []byte(`\ud83d\ude80`), 1)
	if _, err := vp.VerifyOwned(escaped, id, installed.Receipt); err != nil {
		t.Fatal("native-equivalent Unicode not recognized", err)
	}
	repeat := plan(t, vp.Request{Settings: escaped, Identity: id, Action: vp.Install, Previous: installed.Receipt})
	if repeat.Changed || !bytes.Equal(repeat.Settings, escaped) || *repeat.Receipt != *installed.Receipt {
		t.Fatal("equivalent Unicode spelling churned bytes/digest")
	}
	removed := plan(t, vp.Request{Settings: escaped, Identity: id, Action: vp.Remove, Previous: installed.Receipt})
	if len(decoded(t, removed.Settings)["chat.pluginLocations"].(map[string]any)) != 0 {
		t.Fatal("escaped owned key was not removed")
	}
}
