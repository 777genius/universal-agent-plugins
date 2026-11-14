package cursorhooks_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	ch "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/cursorhooks"
)

// Red condition: Go float/string re-encoding would round numbers, replace lone
// surrogates or normalize Unicode/escape lexemes in unrelated config entries.
func TestForeignLexemesAndArrayOrder(t *testing.T) {
	foreignEntry := `{"command":"foreign-\u4e2d","opaque":1.234567890123456789e+200,"text":"\ud800","keys":{"\ud800":1,"\ufffd":2}}`
	foreignRoot := `"opaque":{"n":90071992547409931234567890,"pair":"\ud83d\ude00","literal":"ü é","zero":-0}`
	body := []byte("{\n  \"version\": 1, " + foreignRoot + `,"hooks":{"future":[` + foreignEntry + `],"stop":[{"command":"first"},{"command":"last"}]}}`)
	original := bytes.Clone(body)
	i := mustPlan(t, request(body, ch.Install, nil))
	if !bytes.Equal(body, original) {
		t.Fatal("successful planning mutated caller bytes")
	}
	for _, token := range []string{foreignEntry, foreignRoot} {
		if !bytes.Contains(i.Desired, []byte(token)) {
			t.Fatalf("foreign lexeme lost: %s", token)
		}
	}
	// Move the intact owner between its foreign siblings. Ownership is not
	// based on its original index, and update/remove must retain sibling order.
	var doc struct {
		Hooks map[string][]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(i.Desired, &doc); err != nil {
		t.Fatal(err)
	}
	owned := string(doc.Hooks["stop"][2])
	moved := []byte(strings.Replace(string(i.Desired), `{"command":"first"},{"command":"last"},`+owned,
		`{"command":"first"},`+owned+`,{"command":"last"}`, 1))
	if err := ch.VerifyOwned(moved, i.Receipt); err != nil {
		t.Fatal(err)
	}
	update := request(moved, ch.Update, i.Receipt)
	update.Specs[0].Selector = "/TEST owned/control/updated.json"
	u := mustPlan(t, update)
	r := mustPlan(t, request(u.Desired, ch.Remove, u.Receipt))
	if !bytes.Contains(r.Desired, []byte(`"stop":[{"command":"first"},{"command":"last"}]`)) {
		t.Fatal("foreign stop sibling order changed")
	}
	for _, token := range []string{foreignEntry, foreignRoot} {
		if !bytes.Contains(r.Desired, []byte(token)) {
			t.Fatal("foreign lexeme changed during update/remove")
		}
	}
}

// Red condition: JSONC, duplicate escaped keys, alternate versions or nested
// event maps would silently acquire ownership under an incompatible grammar.
func TestStrictGrammarAndNativeKeyIdentity(t *testing.T) {
	for _, body := range []string{
		" ", `{}`, `[]`, `{"version":2}`, `{"version":"1"}`, `{"version":1.1}`,
		`{"version":1,"hooks":null}`, `{"version":1,"hooks":{"stop":{}}}`,
		`{"version":1,"hooks":{"future":false}}`, `{"version":1,}`,
		`{/*TEST*/"version":1}`, `{"version":1,"version":1}`,
		`{"version":1,"\u0076ersion":1}`,
		`{"version":1,"opaque":{"x":1,"\u0078":2}}`,
		`{"version":1,"opaque":{"😀":1,"\ud83d\ude00":2}}`,
		`{"version":1,"hooks":{"stop":[],"\u0073top":[]}}`,
		`{"version":1} {"version":1}`,
	} {
		refused(t, request([]byte(body), ch.Install, nil), nil)
	}
	// Future native fields/entries stay opaque. Distinct native surrogate keys
	// must not collapse into the replacement character as in Go unmarshal.
	valid := []byte(`{"version":1,"opaque":{"\ud800":1,"\ufffd":2},"hooks":{"future":[null,7,"opaque",{"type":"future"}]}}`)
	i := mustPlan(t, request(valid, ch.Install, nil))
	if !bytes.Contains(i.Desired, []byte(`"\ud800":1,"\ufffd":2`)) {
		t.Fatal("distinct Unicode keys collapsed")
	}
	badUTF8 := append([]byte(`{"version":1,"text":"`), 0xff)
	badUTF8 = append(badUTF8, []byte(`"}`)...)
	refused(t, request(badUTF8, ch.Install, nil), nil)
}

// Red condition found in the initial implementation: requiring the token "1"
// rejects valid strict JSON number spellings of native version 1. This check
// also catches rewriting the version token when inserting the owned entry.
func TestNativeVersionOneNumberLexemes(t *testing.T) {
	for _, token := range []string{"1", "1.0", "1e0", "1.00E+00"} {
		body := []byte(`{"version":` + token + `,"hooks":{}}`)
		i := mustPlan(t, request(body, ch.Install, nil))
		if !bytes.Contains(i.Desired, []byte(`"version":`+token)) {
			t.Fatal("version number lexeme changed")
		}
	}
}

// Red condition: whitespace or escape-only changes would needlessly rewrite a
// valid owner; a large-number lexeme edit in the remainder would pass Repair.
func TestCanonicalOwnerAndRemainderProof(t *testing.T) {
	i := mustPlan(t, request([]byte(`{"version":1,"opaque":9007199254740993}`), ch.Install, nil))
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, i.Desired, "", "  "); err != nil {
		t.Fatal(err)
	}
	body := []byte(strings.Replace(formatted.String(), "cursor-event", `\u0063ursor-event`, 1))
	r := mustPlan(t, request(body, ch.Update, i.Receipt))
	if !r.NoOp || !bytes.Equal(r.Desired, body) || r.Receipt.EntryDigest != i.Receipt.EntryDigest {
		t.Fatal("canonical owner or byte-exact no-op failed")
	}
	removed := mustPlan(t, request(body, ch.Remove, i.Receipt))
	fixed := mustPlan(t, request(removed.Desired, ch.Repair, i.Receipt))
	if fixed.Receipt.RemainderDigest != i.Receipt.RemainderDigest {
		t.Fatal("whitespace changed remainder proof")
	}
	drift := []byte(strings.Replace(string(removed.Desired), "9007199254740993", "9007199254740992", 1))
	refused(t, request(drift, ch.Repair, i.Receipt), ch.ErrAbsenceUnproven)
	// The entire file may be restored only when the prior proof had no foreign
	// remainder; deleting a foreign-bearing file is already refused above.
	empty := mustPlan(t, request(nil, ch.Install, nil))
	mustPlan(t, request(nil, ch.Repair, empty.Receipt))
	stripped := []byte(`{"version":1}`)
	mustPlan(t, request(stripped, ch.Repair, empty.Receipt))
}

// Red condition: recursive parsing would traverse a 30,000-level input first,
// or oversized/node-heavy inputs/output would escape the bounded public API.
func TestResourceBounds(t *testing.T) {
	owned := mustPlan(t, request(nil, ch.Install, nil))
	for _, body := range [][]byte{
		[]byte(strings.Repeat(" ", ch.MaxDocumentBytes+1)),
		[]byte(`{"version":1,"opaque":` + strings.Repeat("[", 30000) + `0` + strings.Repeat("]", 30000) + `}`),
		[]byte(`{"version":1,"opaque":` + strings.Repeat("[", 64) + `0` + strings.Repeat("]", 64) + `}`),
		[]byte(`{"version":1,"opaque":[` + strings.Repeat("0,", 65531) + `0]}`),
	} {
		refused(t, request(body, ch.Install, nil), nil)
		if err := ch.VerifyOwned(body, owned.Receipt); !errors.Is(err, ch.ErrConflict) {
			t.Fatal("verification accepted out-of-budget input")
		}
	}
	// Exact boundaries: output must also fit, so use no-op for a max-byte file
	// and install with enough structural headroom for the fixed owned entry.
	depth := []byte(`{"version":1,"opaque":` + strings.Repeat("[", 63) + `0` + strings.Repeat("]", 63) + `}`)
	mustPlan(t, request(depth, ch.Install, nil))
	nodes := []byte(`{"version":1,"opaque":[` + strings.Repeat("0,", 65517) + `0]}`)
	mustPlan(t, request(nodes, ch.Install, nil))
	i := mustPlan(t, request(nil, ch.Install, nil))
	maxBytes := append(bytes.Clone(i.Desired), bytes.Repeat([]byte{' '}, ch.MaxDocumentBytes-len(i.Desired))...)
	r := mustPlan(t, request(maxBytes, ch.Install, i.Receipt))
	if !r.NoOp || !bytes.Equal(maxBytes, r.Desired) {
		t.Fatal("max-byte no-op changed bytes")
	}
	full := append([]byte(`{"version":1}`), bytes.Repeat([]byte{' '}, ch.MaxDocumentBytes-len(`{"version":1}`))...)
	refused(t, request(full, ch.Install, nil), nil)
}
