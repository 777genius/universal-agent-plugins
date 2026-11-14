package vscodeprofile_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	vp "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

// Red: legal EOF comments refuse lifecycle operations, change foreign suffix
// bytes/values, or expose an internal terminator in a mutation or exact no-op.
func TestEOFCommentPublicLifecycle(t *testing.T) {
	id := identity()
	for _, suffix := range []string{"//", " \t// TEST EOF 通知 /* } [ \\", "\r\n// TEST EOF \"quotes\" //"} {
		t.Run(suffix, func(t *testing.T) {
			original := []byte(`{"foreign":17,"chat.pluginLocations":{"/foreign":false}}` + suffix)
			installed := plan(t, vp.Request{Settings: original, Identity: id, Action: vp.Install})
			if !installed.Changed || installed.Receipt == nil || !installed.Receipt.Enabled {
				t.Fatal("valid EOF install did not record enabled ownership")
			}
			checkEOFForeign(t, installed.Settings, []byte(suffix))
			if decoded(t, installed.Settings)["chat.pluginLocations"].(map[string]any)[id.PluginRoot] != true {
				t.Fatal("independent decode did not find enabled registration")
			}
			if installed.BeforeDigest != vp.SnapshotDigest(original) || installed.AfterDigest != vp.SnapshotDigest(installed.Settings) {
				t.Fatal("install digest includes synthetic bytes")
			}
			disabled := bytes.Replace(installed.Settings, []byte(`"/lab/plugins/通知":true`), []byte(`"/lab/plugins/通知":false`), 1)
			saved := bytes.Clone(disabled)
			v, err := vp.VerifyOwned(disabled, id, installed.Receipt)
			if err != nil || !v.Disabled || !bytes.Equal(disabled, saved) {
				t.Fatal("read-only verification refused or altered EOF snapshot", v, err)
			}
			repeated := plan(t, vp.Request{Settings: disabled, Identity: id, Action: vp.Install, Previous: installed.Receipt})
			if repeated.Changed || !repeated.Disabled || repeated.Receipt == nil || repeated.Receipt.Enabled || !bytes.Equal(repeated.Settings, saved) || repeated.BeforeDigest != vp.SnapshotDigest(saved) || repeated.AfterDigest != repeated.BeforeDigest {
				t.Fatal("disabled repeat changed bytes/state/digests")
			}
			checkEOFForeign(t, repeated.Settings, []byte(suffix))
			locations := decoded(t, repeated.Settings)["chat.pluginLocations"].(map[string]any)
			if locations[id.PluginRoot] != false {
				t.Fatal("independent decode lost disabled value")
			}
			removed := plan(t, vp.Request{Settings: disabled, Identity: id, Action: vp.Remove, Previous: repeated.Receipt})
			if !removed.Changed || removed.Receipt != nil {
				t.Fatal("EOF removal did not remove ownership")
			}
			checkEOFForeign(t, removed.Settings, []byte(suffix))
			if _, present := decoded(t, removed.Settings)["chat.pluginLocations"].(map[string]any)[id.PluginRoot]; present {
				t.Fatal("owned key survived removal")
			}
			again := plan(t, vp.Request{Settings: removed.Settings, Identity: id, Action: vp.Remove, Previous: repeated.Receipt})
			if again.Changed || !bytes.Equal(again.Settings, removed.Settings) {
				t.Fatal("absent removal exposed a terminator")
			}
		})
	}
}

func checkEOFForeign(t *testing.T, body, suffix []byte) {
	t.Helper()
	root := decoded(t, body) // Independent trivia scanner plus encoding/json.
	if root["foreign"] != json.Number("17") || !bytes.Contains(body, []byte(`"foreign":17`)) {
		t.Fatal("foreign value/lexeme changed")
	}
	if locations, ok := root["chat.pluginLocations"].(map[string]any); ok {
		if locations["/foreign"] != false || !bytes.Contains(body, []byte(`"/foreign":false`)) {
			t.Fatal("foreign disabled location changed")
		}
	}
	if !bytes.HasSuffix(body, suffix) {
		t.Fatal("exact foreign EOF suffix changed")
	}
}

// Red: synthetic bytes count against a legal ceiling, or allow original/real
// output bytes past it; bounded verification/no-op/removal must stay lossless.
func TestEOFCommentOriginalAndOutputByteBudgets(t *testing.T) {
	id := identity()
	base := []byte(`{"foreign":17,"chat.pluginLocations":{"/foreign":false}}//`)
	seed := plan(t, vp.Request{Settings: base, Identity: id, Action: vp.Install})
	growth := len(seed.Settings) - len(base)
	pad := func(body []byte, size int) []byte {
		return append(bytes.Clone(body), bytes.Repeat([]byte("x"), size-len(body))...)
	}
	fitInput := pad(base, vp.MaxSettingsBytes-growth)
	fit := plan(t, vp.Request{Settings: fitInput, Identity: id, Action: vp.Install})
	if len(fit.Settings) != vp.MaxSettingsBytes || !fit.Changed || fit.Receipt == nil {
		t.Fatal("exact real output byte limit not accepted")
	}
	fitSuffix := bytes.Index(fitInput, []byte("//"))
	if fitSuffix < 0 {
		t.Fatal("exact-limit fixture is missing its EOF comment")
	}
	checkEOFForeign(t, fit.Settings, fitInput[fitSuffix:])
	refused(t, vp.Request{Settings: pad(base, len(fitInput)+1), Identity: id, Action: vp.Install})
	disabled := bytes.Replace(seed.Settings, []byte(`"/lab/plugins/通知":true`), []byte(`"/lab/plugins/通知":false`), 1)
	bounded := pad(disabled, vp.MaxSettingsBytes)
	noOp := plan(t, vp.Request{Settings: bounded, Identity: id, Action: vp.Install, Previous: seed.Receipt})
	if noOp.Changed || !noOp.Disabled || noOp.Receipt.Enabled || !bytes.Equal(noOp.Settings, bounded) || noOp.BeforeDigest != vp.SnapshotDigest(bounded) || noOp.AfterDigest != noOp.BeforeDigest {
		t.Fatal("exact-limit EOF disabled no-op changed")
	}
	if v, err := vp.VerifyOwned(bounded, id, noOp.Receipt); err != nil || !v.Disabled {
		t.Fatal("exact-limit EOF verification refused", v, err)
	}
	removed := plan(t, vp.Request{Settings: bounded, Identity: id, Action: vp.Remove, Previous: noOp.Receipt})
	if !removed.Changed || removed.Receipt != nil || len(removed.Settings) >= len(bounded) {
		t.Fatal("bounded EOF removal failed")
	}
	boundedSuffix := bytes.Index(bounded, []byte("//"))
	if boundedSuffix < 0 {
		t.Fatal("bounded fixture is missing its EOF comment")
	}
	checkEOFForeign(t, removed.Settings, bounded[boundedSuffix:])
	oversized := append(bytes.Clone(bounded), 'x')
	refused(t, vp.Request{Settings: oversized, Identity: id, Action: vp.Remove, Previous: noOp.Receipt})
	if _, err := vp.VerifyOwned(oversized, id, noOp.Receipt); !errors.Is(err, vp.ErrConflict) {
		t.Fatal("oversized original verified", err)
	}
}

// Red: an EOF terminator repairs incomplete syntax or reinterprets comment-like
// string/block text; malformed inputs must refuse with exact original bytes.
func TestEOFCommentKeepsNativeSyntaxBoundaries(t *testing.T) {
	for _, input := range []string{
		`{"foreign":17// EOF`, `{"foreign":// EOF`, `{"foreign":[17// EOF`,
		`{}/* // EOF`, `{"foreign":"// EOF`, "{}// TEST bare CR\r",
	} {
		body := []byte(input)
		r, err := vp.Plan(vp.Request{Settings: body, Identity: identity(), Action: vp.Install})
		if !errors.Is(err, vp.ErrConflict) || r.Changed || r.Receipt != nil || !bytes.Equal(r.Settings, body) || r.BeforeDigest != vp.SnapshotDigest(body) || r.AfterDigest != r.BeforeDigest {
			t.Fatal("malformed/ambiguous EOF syntax accepted or changed", input, err)
		}
	}
	for _, suffix := range []string{"", "// TEST EOF"} {
		body := []byte(`{"foreign":17,"literal":"// /* */ \\ \"","chat.pluginLocations":{"/foreign":false}}/* // block CR ` + "\r" + ` */` + suffix)
		installed := plan(t, vp.Request{Settings: body, Identity: identity(), Action: vp.Install})
		checkEOFForeign(t, installed.Settings, []byte("/* // block CR \r */"+suffix))
		if decoded(t, installed.Settings)["literal"] != decoded(t, body)["literal"] || !bytes.Contains(installed.Settings, []byte(`"literal":"// /* */ \\ \""`)) {
			t.Fatal("comment-like string literal changed")
		}
	}
}
