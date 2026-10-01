package vscodeprofile_test

import (
	"bytes"
	"strings"
	"testing"

	vp "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

// Red: lexical bounds run after recursive parse, count delimiters inside strings/comments,
// or permit output growth beyond any stated budget.
func TestDocumentDepthNodeAndOutputBudgets(t *testing.T) {
	id := identity()
	for _, depth := range []int{vp.MaxSettingsDepth, vp.MaxSettingsDepth + 1} {
		body := []byte(`{"opaque":` + strings.Repeat("[", depth-1) + `null` + strings.Repeat("]", depth-1) + `}`)
		req := vp.Request{Settings: body, Identity: id, Action: vp.Install}
		if depth > vp.MaxSettingsDepth {
			refused(t, req)
		} else {
			decoded(t, plan(t, req).Settings)
		}
	}
	// Root, key, array: 3 nodes; scalar elements supply the remainder.
	nodesBody := func(nodes int) []byte { return []byte(`{"opaque":[` + strings.Repeat("0,", nodes-4) + `0]}`) }
	// New registration adds exactly four nodes: selector key/object/location key/bool.
	fitting := plan(t, vp.Request{Settings: nodesBody(vp.MaxSettingsNodes - 4), Identity: id, Action: vp.Install})
	decoded(t, fitting.Settings)
	for _, nodes := range []int{vp.MaxSettingsNodes, vp.MaxSettingsNodes + 1} {
		refused(t, vp.Request{Settings: nodesBody(nodes), Identity: id, Action: vp.Install})
	}
	// An exactly-at-budget document is legal for an exact owned no-op.
	installed := plan(t, vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	exact := append(bytes.Clone(fitting.Settings), []byte(" /* legal comment */")...)
	noOp := plan(t, vp.Request{Settings: exact, Identity: id, Action: vp.Install, Previous: installed.Receipt})
	if noOp.Changed || !bytes.Equal(noOp.Settings, exact) {
		t.Fatal("node-bound no-op rejected/changed")
	}
	byteBound := append([]byte(`{}`), bytes.Repeat([]byte(" "), vp.MaxSettingsBytes-2)...)
	refused(t, vp.Request{Settings: byteBound, Identity: id, Action: vp.Install}) // legal input, oversized output
	over := append(bytes.Clone(byteBound), ' ')
	refused(t, vp.Request{Settings: over, Identity: id, Action: vp.Install})
	ownedAtByteBound := append(bytes.Clone(installed.Settings), bytes.Repeat([]byte(" "), vp.MaxSettingsBytes-len(installed.Settings))...)
	unchanged := plan(t, vp.Request{Settings: ownedAtByteBound, Identity: id, Action: vp.Install, Previous: installed.Receipt})
	if unchanged.Changed || !bytes.Equal(unchanged.Settings, ownedAtByteBound) {
		t.Fatal("exact byte budget not accepted")
	}
	safe := []byte(`{/*` + strings.Repeat("[{", vp.MaxSettingsDepth*2) + `*/"literal":"` + strings.Repeat("[", vp.MaxSettingsNodes) + `",}`)
	decoded(t, plan(t, vp.Request{Settings: safe, Identity: id, Action: vp.Install}).Settings)
}

// Red: malformed or invalid UTF-8 input produces replacement data, partial receipt or panic.
func TestMalformedSnapshotsRefuseOriginalBytes(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`[]`), []byte(`null`), []byte(`{`), []byte(`{} {}`), []byte(`{"x":NaN}`),
		[]byte(`{"x":/*unterminated`), []byte(`{"x":"unterminated`),
		[]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'},
	} {
		refused(t, vp.Request{Settings: body, Identity: identity(), Action: vp.Install})
	}
	empty := plan(t, vp.Request{Identity: identity(), Action: vp.Install})
	decoded(t, empty.Settings)
}

// Red: invalid/missing identity or misrouted enable decisions can be used to mutate settings.
func TestIdentityAndOperationValidation(t *testing.T) {
	id := identity()
	installed := plan(t, vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	for _, action := range []vp.Action{vp.Install, vp.Update, vp.Remove, vp.Repair} {
		refused(t, vp.Request{Settings: installed.Settings, Identity: id, Action: action, Previous: installed.Receipt, EnableSnapshotDigest: vp.SnapshotDigest(installed.Settings)})
	}
	refused(t, vp.Request{Settings: installed.Settings, Identity: id, Action: vp.Action("unknown"), Previous: installed.Receipt})
	for _, p := range []string{"relative", "~/plugin", "/lab/../plugin", "/lab/plugin/", "/lab/plugin\n", "//server/share", `C:relative`, `\\server\share`, `C:\lab/other`, "/lab/\x00plugin"} {
		bad := id
		bad.PluginRoot = p
		refused(t, vp.Request{Settings: []byte(`{}`), Identity: bad, Action: vp.Install})
	}
	for _, field := range []string{"profile", "package", "digest", "length"} {
		bad := id
		switch field {
		case "profile":
			bad.ProfileID = ""
		case "package":
			bad.PackageID = ""
		case "digest":
			bad.PackageDigest = "sha256:bad"
		case "length":
			bad.ProfileID = strings.Repeat("x", vp.MaxIdentityBytes+1)
		}
		refused(t, vp.Request{Settings: []byte(`{}`), Identity: bad, Action: vp.Install})
	}
	win := id
	win.SettingsPath = `C:\lab\profile\settings.json`
	win.PluginRoot = `C:\lab\plugins\通知`
	registered := plan(t, vp.Request{Settings: []byte(`{}`), Identity: win, Action: vp.Install})
	if _, err := vp.VerifyOwned(registered.Settings, win, registered.Receipt); err != nil {
		t.Fatal(err)
	}
	refused(t, vp.Request{Settings: []byte(`{"chat.pluginLocations":{"c:\\lab\\plugins\\通知":false}}`), Identity: win, Action: vp.Install})
}
