package vscodeprofile_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	vp "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

// Independent test oracle: remove JSONC trivia/trailing commas, then delegate
// grammar/value decoding to encoding/json. No hujson or production helper is
// used. This checks native-compatible JSONC grammar, not a native Code launch.
func decoded(t *testing.T, body []byte) map[string]any {
	t.Helper()
	stripped := normalizeJSONC(body)
	var root map[string]any
	dec := json.NewDecoder(bytes.NewReader(stripped))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		t.Fatal("invalid native-compatible JSONC", err, string(body))
	}
	if !json.Valid(stripped) {
		t.Fatal("invalid/multiple JSONC frames")
	}
	return root
}

// Red: whole-file serialization changes foreign bytes/large numbers/Unicode or drops comments on removal.
func TestJSONCPreservationAcrossLifecycle(t *testing.T) {
	original := []byte(`{
 // prefix comment
 "unknown": { "\ud800": "opaque", "duplicate":1, "duplicate":2, },
 "huge":900719925474099312345678901234567890,
 "exponent":1E+999,
 "unicode":"é / é / 🚀 / 通知",
 "string":"/* literal */ // ,} ] \\\"",
 "list":[1,2,],
 "chat.pluginLocations":{
   /* sibling comment */ "/foreign": false, // sibling after
 },
 /* end comment */
}
`)
	id := identity()
	installed := plan(t, vp.Request{Settings: original, Identity: id, Action: vp.Install})
	parsed := decoded(t, installed.Settings)
	if parsed["chat.pluginLocations"].(map[string]any)[id.PluginRoot] != true {
		t.Fatal("native setting wrong")
	}
	assertForeignLexemes(t, installed.Settings)
	// A foreign edit after install must not invalidate entry ownership.
	edited := bytes.Replace(installed.Settings, []byte("// prefix comment"), []byte("// late edit"), 1)
	if _, err := vp.VerifyOwned(edited, id, installed.Receipt); err != nil {
		t.Fatal(err)
	}
	repeated := plan(t, vp.Request{Settings: edited, Identity: id, Previous: installed.Receipt, Action: vp.Install})
	if repeated.Changed || !bytes.Equal(repeated.Settings, edited) {
		t.Fatal("no-op not exact")
	}
	removed := plan(t, vp.Request{Settings: edited, Identity: id, Previous: installed.Receipt, Action: vp.Remove})
	decoded(t, removed.Settings)
	assertForeignLexemes(t, removed.Settings)
	if !bytes.Contains(removed.Settings, []byte("// late edit")) || !bytes.Contains(removed.Settings, []byte("// sibling after")) {
		t.Fatal("foreign comments removed")
	}
	if bytes.Contains(removed.Settings, []byte(`"/lab/plugins/通知"`)) {
		t.Fatal("owned selector survived remove")
	}
	if parsed["huge"] != json.Number("900719925474099312345678901234567890") {
		t.Fatal("large number rounded")
	}
}

func assertForeignLexemes(t *testing.T, body []byte) {
	t.Helper()
	for _, lexeme := range []string{`"\ud800": "opaque"`, `"duplicate":1, "duplicate":2`, `900719925474099312345678901234567890`, `1E+999`, `é / é / 🚀 / 通知`, `/* sibling comment */`, `// sibling after`, `/* end comment */`} {
		if !bytes.Contains(body, []byte(lexeme)) {
			t.Fatal("foreign lexeme lost", lexeme)
		}
	}
}

// Red: remove loses trivia or a neighbor, or accepted true-to-false causes refusal/enable.
func TestRemoveOnlyOwnedSelectorKeepsAllComments(t *testing.T) {
	id := identity()
	installed := plan(t, vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	for _, position := range []string{"first", "middle", "last", "only"} {
		owned := `/*before*/"/lab/plugins/通知"/*name*/:/*value*/false/*after*/`
		foreign := `"/foreign":true`
		var members string
		switch position {
		case "first":
			members = owned + "," + foreign
		case "middle":
			members = foreign + "," + owned + `,"/other":false`
		case "last":
			members = foreign + "," + owned
		case "only":
			members = owned
		}
		body := []byte(fmt.Sprintf(`{"chat.pluginLocations":{%s,/*tail*/},"unknown":999}`, members))
		removed := plan(t, vp.Request{Settings: body, Identity: id, Previous: installed.Receipt, Action: vp.Remove})
		root := decoded(t, removed.Settings)
		locations := root["chat.pluginLocations"].(map[string]any)
		if _, ok := locations[id.PluginRoot]; ok {
			t.Fatal("owned key retained")
		}
		if position != "only" && locations["/foreign"] != true {
			t.Fatal("foreign key changed")
		}
		for _, comment := range []string{"/*before*/", "/*name*/", "/*value*/", "/*after*/", "/*tail*/"} {
			if !bytes.Contains(removed.Settings, []byte(comment)) {
				t.Fatal("comment deleted", position, comment)
			}
		}
	}
}

// Red: hash/bytes depend on formatting or map order, or input/receipt alias output mutations.
func TestDeterministicDigestsAndNoInputAliasing(t *testing.T) {
	id := identity()
	body := []byte(`{"a":1}`)
	req := vp.Request{Settings: body, Identity: id, Action: vp.Install}
	first := plan(t, req)
	second := plan(t, req)
	if !bytes.Equal(first.Settings, second.Settings) || *first.Receipt != *second.Receipt || first.BeforeDigest != vp.SnapshotDigest(body) || first.AfterDigest != vp.SnapshotDigest(first.Settings) {
		t.Fatal("nondeterministic result")
	}
	reformatted := append([]byte("/* different trivia */"), first.Settings...)
	repeated := plan(t, vp.Request{Settings: reformatted, Identity: id, Previous: first.Receipt, Action: vp.Install})
	if *repeated.Receipt != *first.Receipt {
		t.Fatal("foreign bytes bound to ownership digest")
	}
	saved := *first.Receipt
	repeated.Settings[0] = ' '
	repeated.Receipt.Enabled = false
	if reformatted[0] != '/' || *first.Receipt != saved {
		t.Fatal("output aliased input")
	}
}

func normalizeJSONC(body []byte) []byte {
	stripped := bytes.Clone(body)
	stripComments(stripped)
	stripTrailingCommas(stripped)
	return stripped
}

func stripComments(stripped []byte) {
	for i := 0; i < len(stripped); i++ {
		if stripped[i] == '"' {
			for i++; i < len(stripped); i++ {
				if stripped[i] == '\\' {
					i++
				} else if stripped[i] == '"' {
					break
				}
			}
		} else if i+1 < len(stripped) && stripped[i] == '/' && (stripped[i+1] == '/' || stripped[i+1] == '*') {
			start := i
			line := stripped[i+1] == '/'
			i += 2
			for i < len(stripped) {
				if line && stripped[i] == '\n' {
					break
				}
				if !line && i+1 < len(stripped) && stripped[i] == '*' && stripped[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			for j := start; j < i; j++ {
				if stripped[j] != '\n' && stripped[j] != '\r' {
					stripped[j] = ' '
				}
			}
			i--
		}
	}
}

func stripTrailingCommas(stripped []byte) {
	for i := 0; i < len(stripped); i++ {
		if stripped[i] == '"' {
			for i++; i < len(stripped); i++ {
				if stripped[i] == '\\' {
					i++
				} else if stripped[i] == '"' {
					break
				}
			}
		} else if stripped[i] == ',' {
			j := i + 1
			for j < len(stripped) && bytes.ContainsRune([]byte(" \t\r\n"), rune(stripped[j])) {
				j++
			}
			if j < len(stripped) && (stripped[j] == '}' || stripped[j] == ']') {
				stripped[i] = ' '
			}
		}
	}
}
