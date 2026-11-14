package vscodeprofile_test

import (
	"bytes"
	"strings"
	"testing"

	vp "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

func identity() vp.Identity {
	return vp.Identity{SettingsPath: "/lab/profile/settings.json", ProfileID: "test-profile", PluginRoot: "/lab/plugins/通知", PackageID: "notifications", PackageDigest: "sha256:" + strings.Repeat("a", 64), ProjectionDigest: "sha256:" + strings.Repeat("b", 64)}
}
func plan(t *testing.T, req vp.Request) vp.Result {
	t.Helper()
	r, err := vp.Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func refused(t *testing.T, req vp.Request) {
	t.Helper()
	before := bytes.Clone(req.Settings)
	r, err := vp.Plan(req)
	if err == nil || r.Receipt != nil || r.Changed || !bytes.Equal(r.Settings, before) || !bytes.Equal(req.Settings, before) {
		t.Fatalf("unsafe refusal: result=%+v err=%v", r, err)
	}
}

// Red: foreign identical true is adopted, false is overwritten, or repeat/remove churns bytes.
func TestPublicLifecycleAndForeignChoices(t *testing.T) {
	id := identity()
	for _, value := range []string{"true", "false"} {
		refused(t, vp.Request{Settings: []byte(`{"chat.pluginLocations":{"/lab/plugins/通知":` + value + `}}`), Identity: id, Action: vp.Install})
	}
	original := []byte("{\n // foreign\n \"editor.fontSize\":17,\n}\n")
	req := vp.Request{Settings: original, Identity: id, Action: vp.Install}
	installed := plan(t, req)
	if !installed.Changed || installed.Receipt == nil {
		t.Fatal("install did not record ownership")
	}
	if !bytes.Equal(req.Settings, original) {
		t.Fatal("input mutated")
	}
	if _, err := vp.VerifyOwned(installed.Settings, id, installed.Receipt); err != nil {
		t.Fatal(err)
	}
	req.Settings, req.Previous = installed.Settings, installed.Receipt
	repeated := plan(t, req)
	if repeated.Changed || !bytes.Equal(repeated.Settings, installed.Settings) || *repeated.Receipt != *installed.Receipt {
		t.Fatal("repeat churn")
	}
	req.Action = vp.Remove
	removed := plan(t, req)
	if removed.Receipt != nil || !removed.Changed || !bytes.Contains(removed.Settings, []byte("// foreign")) || !bytes.Contains(removed.Settings, []byte(`"editor.fontSize":17`)) {
		t.Fatal("remove lost foreign settings")
	}
	req.Settings = removed.Settings
	again := plan(t, req)
	if again.Changed || !bytes.Equal(again.Settings, removed.Settings) {
		t.Fatal("repeat removal changed settings")
	}
	refused(t, vp.Request{Settings: removed.Settings, Identity: id, Action: vp.Update, Previous: installed.Receipt})
}

// Red: ordinary lifecycle re-enables a native-disabled owned selector, or enable ignores snapshot drift.
func TestStickyDisabledAndExplicitEnable(t *testing.T) {
	id := identity()
	installed := plan(t, vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	disabled := bytes.Replace(installed.Settings, []byte("true"), []byte("false"), 1)
	req := vp.Request{Settings: disabled, Identity: id, Previous: installed.Receipt, Action: vp.Install}
	for _, a := range []vp.Action{vp.Install, vp.Update, vp.Repair} {
		req.Action = a
		r := plan(t, req)
		if r.Changed || !r.Disabled || r.Receipt.Enabled || !bytes.Equal(r.Settings, disabled) {
			t.Fatal("disabled choice lost", a)
		}
		req.Previous = r.Receipt
	}
	if v, err := vp.VerifyOwned(disabled, id, req.Previous); err != nil || !v.Disabled {
		t.Fatal(v, err)
	}
	req.Identity.ProjectionDigest = "sha256:" + strings.Repeat("c", 64)
	req.Action = vp.Update
	updated := plan(t, req)
	if !updated.Disabled || updated.Receipt.Identity != req.Identity || updated.Changed {
		t.Fatal("update lost disabled state or identity")
	}
	req.Previous = updated.Receipt
	req.Action = vp.Enable
	refused(t, req)
	req.EnableSnapshotDigest = vp.SnapshotDigest(disabled)
	req.Settings = append(bytes.Clone(disabled), ' ')
	refused(t, req)
	req.Settings = disabled
	enabled := plan(t, req)
	if !enabled.Changed || enabled.Disabled || !enabled.Receipt.Enabled {
		t.Fatal("explicit enable failed")
	}
	if _, err := vp.VerifyOwned(enabled.Settings, req.Identity, enabled.Receipt); err != nil {
		t.Fatal(err)
	}
	// Native false-to-true without the explicit enable transaction is drift.
	refused(t, vp.Request{Settings: enabled.Settings, Identity: req.Identity, Previous: updated.Receipt, Action: vp.Install})
}
