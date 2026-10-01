package geminihooks_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	gh "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/geminihooks"
)

// Red: native JSON.parse distinguishes the edit, but VerifyOwned or a public
// update/remove accepts it. Go decoding alone is not a native-string oracle.
func TestNativeSurrogateOwnedEditConflict(t *testing.T) {
	for _, field := range []string{"name", "command", "matcher"} {
		t.Run(field, func(t *testing.T) {
			h := specs()[:1]
			switch field {
			case "name":
				h[0].Name = "other.\ufffd"
			case "command":
				h[0].Argv = []string{"/opt/other/helper", "\ufffd"}
			case "matcher":
				h[0].Matcher = "\ufffd"
			}
			r := plan(t, nil, gh.Install, h, nil)
			for _, surrogate := range []string{`\ud800`, `\ud801`, `\udfff`} {
				edited := bytes.Replace(r.Desired, []byte("\ufffd"), []byte(surrogate), 1)
				if bytes.Equal(edited, r.Desired) {
					t.Fatal("fixture did not edit the owned string")
				}
				node, err := exec.LookPath("node")
				if err != nil {
					t.Fatal(err)
				}
				input, err := json.Marshal([]string{string(r.Desired), string(edited), field})
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(node, "-e", `let s='';process.stdin.on('data',b=>s+=b);process.stdin.on('end',()=>{const [a,b,k]=JSON.parse(s);const get=x=>{const g=JSON.parse(x).hooks.AfterAgent[0];return k==='matcher'?g.matcher:g.hooks[0][k]};const x=get(a),y=get(b);if(x===y||!x.includes('\ufffd')||![0xd800,0xd801,0xdfff].some(c=>y.includes(String.fromCharCode(c))))process.exit(1);console.log('native strings differ')})`)
				cmd.Stdin = bytes.NewReader(input)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("native comparison: %v: %s", err, out)
				}
				if !errors.Is(gh.VerifyOwned(edited, r.Receipt), gh.ErrConflict) {
					t.Error("edited owned native string verified")
				}
				for _, op := range []gh.Operation{gh.Update, gh.Remove} {
					t.Run(surrogate+"/"+string(op), func(t *testing.T) {
						conflict(t, gh.Request{Settings: edited, Shell: gh.Bash, Operation: op, Hooks: h, Previous: r.Receipt})
					})
				}
			}
		})
	}
}

// Red: unrelated native-valid surrogate keys/names are conflated or changed.
func TestNativeSurrogateForeignControl(t *testing.T) {
	opaque := `"opaque" : {"\ud800":1,"\ud801":2,"\udfff":"\ud800","\ud83d\ude00":3,"�":4}`
	foreignGroup := `{"hooks":[{"name":"other.\ud800","command":"echo \ud801"}]}`
	input := []byte(`{` + opaque + `,"hooks":{"AfterAgent":[` + foreignGroup + `]}}`)
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "-e", `let s='';process.stdin.on('data',b=>s+=b);process.stdin.on('end',()=>{const o=JSON.parse(s).opaque;if(Object.keys(o).length!==5||o['\ud800']!==1||o['\ud801']!==2||o['\udfff']!=='\ud800'||o['😀']!==3||o['�']!==4)process.exit(1);console.log('native foreign keys are distinct')})`)
	cmd.Stdin = bytes.NewReader(input)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("native foreign-key control: %v: %s", err, out)
	}
	h := specs()[:1]
	h[0].Name = "other.\ufffd"
	r := plan(t, input, gh.Install, h, nil)
	if err := gh.VerifyOwned(r.Desired, r.Receipt); err != nil {
		t.Fatal(err)
	}
	h[0].Timeout++
	u := plan(t, r.Desired, gh.Update, h, r.Receipt)
	d := plan(t, u.Desired, gh.Remove, nil, u.Receipt)
	for _, b := range [][]byte{r.Desired, u.Desired, d.Desired} {
		if !bytes.Contains(b, []byte(opaque)) || !bytes.Contains(b, []byte(foreignGroup)) {
			t.Fatal("changed opaque native strings")
		}
	}
	for _, keys := range []string{`"\ud800":1,"\uD800":2`, `"\ud83d\ude00":1,"😀":2`, `"a":1,"\u0061":2`, `"\n":1,"\u000a":2`, `"\\ud800":1,"\u005cud800":2`} {
		conflict(t, gh.Request{Settings: []byte(`{"opaque":{` + keys + `}}`), Shell: gh.Bash, Operation: gh.Install, Hooks: h})
	}
	// Paired escapes and literal Unicode denote the same owned native string.
	h[0].Name = "other.😀"
	r = plan(t, nil, gh.Install, h, nil)
	escaped := bytes.ReplaceAll(r.Desired, []byte("😀"), []byte(`\ud83d\ude00`))
	if err := gh.VerifyOwned(escaped, r.Receipt); err != nil {
		t.Fatal("equivalent paired native string revoked ownership:", err)
	}
	repeat := plan(t, escaped, gh.Update, h, r.Receipt)
	if !repeat.NoOp || !bytes.Equal(repeat.Desired, escaped) {
		t.Fatal("equivalent native spelling changed bytes")
	}
}

func TestSettingsOutputBudgetConflict(t *testing.T) {
	// This input fits (root + opaque key/array + 65,533 scalars); adding
	// a valid owned group would exceed the supported document node budget.
	input := []byte(`{"opaque":[` + strings.Repeat("0,", 65532) + "0]}")
	conflict(t, gh.Request{Settings: input, Shell: gh.Bash, Operation: gh.Install, Hooks: specs()})
}

// Red: small but structurally costly documents pass public planning/ownership.
// Limits are observable document budgets, not a test of scanner internals.
func TestSettingsStructuralBudgets(t *testing.T) {
	installed := plan(t, nil, gh.Install, specs(), nil)
	for name, body := range map[string][]byte{
		"depth30000":       []byte(`{"opaque":` + strings.Repeat("[", 30000) + "0" + strings.Repeat("]", 30000) + "}"),
		"depth30000 owned": []byte(`{"opaque":` + strings.Repeat("[", 30000) + "0" + strings.Repeat("]", 30000) + "}"),
		"depth65":          []byte(`{"opaque":` + strings.Repeat("[", 64) + "0" + strings.Repeat("]", 64) + "}"),
		"nodes65537":       []byte(`{"opaque":[` + strings.Repeat("0,", 65501) + "0]}"),
		"bytes":            bytes.Repeat([]byte(" "), gh.MaxSettingsBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if name != "bytes" && name != "depth30000" {
				body = append(append([]byte(nil), body[:len(body)-1]...), append([]byte(","), installed.Desired[1:]...)...)
			}
			if !errors.Is(gh.VerifyOwned(body, installed.Receipt), gh.ErrConflict) {
				t.Error("over-budget document verified")
			}
			for _, op := range []gh.Operation{gh.Install, gh.Update, gh.Remove} {
				t.Run(string(op), func(t *testing.T) {
					previous := installed.Receipt
					if name == "depth30000" && op == gh.Install {
						previous = nil // Exact independent review reproduction.
					}
					conflict(t, gh.Request{Settings: body, Shell: gh.Bash, Operation: op, Hooks: specs(), Previous: previous})
				})
			}
		})
	}
	// Remove accepts exactly 65,536 nodes (33 in the fixed two-hook document,
	// plus opaque key/array and 65,501 scalar values)
	// and depth 64. Existing owned groups must not mask the budget boundaries.
	for name, value := range map[string]string{
		"depth64":    strings.Repeat("[", 63) + "0" + strings.Repeat("]", 63),
		"nodes65536": "[" + strings.Repeat("0,", 65500) + "0]",
	} {
		t.Run(name+" valid", func(t *testing.T) {
			body := append([]byte(`{"opaque":`+value+","), installed.Desired[1:]...)
			if err := gh.VerifyOwned(body, installed.Receipt); err != nil {
				t.Fatal(err)
			}
			plan(t, body, gh.Remove, nil, installed.Receipt)
		})
	}
}

func TestStructuralDelimiterStringCommentControl(t *testing.T) {
	noise := strings.Repeat("[{]}", 30000)
	quoted, err := json.Marshal(noise + `"\\// /* */`)
	if err != nil {
		t.Fatal(err)
	}
	input := []byte("{/* " + noise + " */\n// " + noise + "\n\"opaque\":" + string(quoted) + "}")
	r := plan(t, input, gh.Install, specs(), nil)
	if err := gh.VerifyOwned(r.Desired, r.Receipt); err != nil {
		t.Fatal(err)
	}
	h := specs()
	h[0].Timeout++
	u := plan(t, r.Desired, gh.Update, h, r.Receipt)
	d := plan(t, u.Desired, gh.Remove, nil, u.Receipt)
	for _, b := range [][]byte{r.Desired, u.Desired, d.Desired} {
		if !bytes.Contains(b, quoted) || !bytes.Contains(b, []byte("/* "+noise+" */")) || !bytes.Contains(b, []byte("// "+noise+"\n")) {
			t.Fatal("delimiter control changed opaque bytes/comments")
		}
	}
}
