package cursorhooks_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	ch "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/cursorhooks"
)

func request(body []byte, op ch.Operation, receipt *ch.Receipt) ch.Request {
	return ch.Request{
		Document: body, Operation: op, Previous: receipt, Shell: ch.LinuxUserShell32212,
		Specs:              []ch.HookSpec{{Executable: "/TEST owned/bin/notify", Selector: "/TEST owned/control/hooks-binding.json"}},
		ExecutableVerified: true,
	}
}

func mustPlan(t *testing.T, req ch.Request) ch.Result {
	t.Helper()
	r, err := ch.Plan(req)
	if err != nil || r.Conflict {
		t.Fatalf("plan: %v", err)
	}
	return r
}

func refused(t *testing.T, req ch.Request, reason error) {
	t.Helper()
	before := bytes.Clone(req.Document)
	r, err := ch.Plan(req)
	if !errors.Is(err, ch.ErrConflict) || !r.Conflict || r.NoOp || r.Receipt != nil {
		t.Fatalf("expected ownership refusal: result=%+v error=%v", r, err)
	}
	if reason != nil && !errors.Is(err, reason) {
		t.Fatalf("expected %v, got %v", reason, err)
	}
	if !bytes.Equal(before, req.Document) || !bytes.Equal(r.Desired, before) {
		t.Fatal("refusal mutated or rewrote original bytes")
	}
}

func commandEntry(t *testing.T, body []byte) json.RawMessage {
	t.Helper()
	var doc struct {
		Hooks map[string][]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(body, &doc); err != nil || len(doc.Hooks["stop"]) != 1 {
		t.Fatalf("expected one stop entry: %v", err)
	}
	return doc.Hooks["stop"][0]
}

// Red condition: Gemini-shaped groups, millisecond timeout or configurable
// policy fields would reach Cursor, or repetition would append a duplicate.
func TestFixedNativeInstallRepeatUpdateRemove(t *testing.T) {
	installed := mustPlan(t, request(nil, ch.Install, nil))
	var entry map[string]any
	if err := json.Unmarshal(commandEntry(t, installed.Desired), &entry); err != nil {
		t.Fatal(err)
	}
	if len(entry) != 4 || entry["type"] != "command" || entry["timeout"] != float64(5) || entry["failClosed"] != false {
		t.Fatalf("wrong native grammar: %#v", entry)
	}
	want, err := ch.RenderArgv(ch.LinuxUserShell32212, []string{
		"/TEST owned/bin/notify", "cursor-event", "stop", "--binding", "/TEST owned/control/hooks-binding.json",
	})
	if err != nil || entry["command"] != want {
		t.Fatalf("fixed invocation: %v", err)
	}
	if err := ch.VerifyOwned(installed.Desired, installed.Receipt); err != nil {
		t.Fatal(err)
	}
	repeat := mustPlan(t, request(installed.Desired, ch.Install, installed.Receipt))
	if !repeat.NoOp || !bytes.Equal(repeat.Desired, installed.Desired) {
		t.Fatal("repeated install must be byte-exact")
	}
	update := request(repeat.Desired, ch.Update, repeat.Receipt)
	update.Specs[0].Executable = "/TEST owned/bin/notify-v2"
	refused(t, ch.Request{
		Document: update.Document, Previous: update.Previous, Specs: update.Specs,
		Shell: update.Shell, Operation: ch.Install, ExecutableVerified: true,
	}, nil)
	updated := mustPlan(t, update)
	if updated.NoOp || updated.Receipt.EntryDigest == installed.Receipt.EntryDigest {
		t.Fatal("update did not replace command and ownership proof")
	}
	if err := ch.VerifyOwned(updated.Desired, updated.Receipt); err != nil {
		t.Fatal(err)
	}
	remove := request(updated.Desired, ch.Remove, updated.Receipt)
	remove.Specs, remove.ExecutableVerified, remove.Shell = nil, false, ""
	removed := mustPlan(t, remove)
	if removed.Receipt != nil || strings.Contains(string(removed.Desired), "cursor-event") || !json.Valid(removed.Desired) {
		t.Fatal("remove retained owned command or discarded document")
	}
	refused(t, request(removed.Desired, ch.Remove, updated.Receipt), ch.ErrAbsenceUnproven)
}

// Red condition: a same-command unreceipted entry would be adopted, a duplicate
// would be mistaken for the owner, or moving the entry would escape detection.
func TestCollisionAndEventDrift(t *testing.T) {
	i := mustPlan(t, request(nil, ch.Install, nil))
	entry := string(commandEntry(t, i.Desired))
	for _, body := range []string{
		`{"version":1,"hooks":{"stop":[` + entry + `,` + entry + `]}}`,
		`{"version":1,"hooks":{"stop":[` + entry + `],"future":[` + entry + `]}}`,
		`{"version":1,"hooks":{"future":[` + entry + `]}}`,
	} {
		for _, op := range []ch.Operation{ch.Update, ch.Remove, ch.Repair} {
			refused(t, request([]byte(body), op, i.Receipt), nil)
		}
		if err := ch.VerifyOwned([]byte(body), i.Receipt); !errors.Is(err, ch.ErrConflict) {
			t.Fatal("verification accepted duplicate/moved owner")
		}
	}
	refused(t, request(i.Desired, ch.Install, nil), nil)
	changed := strings.Replace(entry, `"timeout":5`, `"timeout":9`, 1)
	refused(t, request([]byte(`{"version":1,"hooks":{"stop":[`+changed+`]}}`), ch.Install, nil), nil)

	// A desired new command already exists elsewhere; updating the old owner
	// must preserve both entries and report the collision.
	newSpec := request(nil, ch.Install, nil)
	newSpec.Specs[0].Executable = "/TEST owned/bin/notify-v2"
	next := mustPlan(t, newSpec)
	body := []byte(`{"version":1,"hooks":{"stop":[` + entry + `],"future":[` + string(commandEntry(t, next.Desired)) + `]}}`)
	update := request(body, ch.Update, i.Receipt)
	update.Specs = newSpec.Specs
	refused(t, update, nil)
}

// Red condition: checking just command/known fields would overwrite unknown
// owned-field edits, timeout changes, policy additions or changed commands.
func TestFullEntryDrift(t *testing.T) {
	i := mustPlan(t, request(nil, ch.Install, nil))
	for _, change := range []struct{ old, new string }{
		{`"timeout":5`, `"timeout":5000`},
		{`"timeout":5`, `"timeout":5.0`},
		{`"failClosed":false`, `"failClosed":true`},
		{`"type":"command"`, `"type":"prompt"`},
		{`"type":"command"`, `"type":"command","extension":{"opaque":9007199254740993,"text":"\ud800"}`},
		{`"type":"command"`, `"type":"command","loop_limit":null`},
		{`"type":"command"`, `"type":"command","matcher":".*","env":{"TEST":"synthetic"}`},
		{`/TEST owned/bin/notify`, `/TEST owned/bin/edited`},
	} {
		body := []byte(strings.Replace(string(i.Desired), change.old, change.new, 1))
		for _, op := range []ch.Operation{ch.Install, ch.Update, ch.Remove, ch.Repair} {
			refused(t, request(body, op, i.Receipt), nil)
		}
		if err := ch.VerifyOwned(body, i.Receipt); !errors.Is(err, ch.ErrConflict) {
			t.Fatal("verification accepted changed full entry")
		}
	}
}

// Red condition: a deleted command plus arbitrary foreign edits would be
// mistaken for proven absence, or Repair would accept a new desired spec.
func TestNarrowRepairAndForeignEdits(t *testing.T) {
	foreign := []byte(`{"version":1,"keep":{"n":9007199254740993,"text":"\ud800"},"hooks":{"future":[{"command":"foreign"}]}}`)
	i := mustPlan(t, request(foreign, ch.Install, nil))
	removed := mustPlan(t, request(i.Desired, ch.Remove, i.Receipt))
	repair := request(removed.Desired, ch.Repair, i.Receipt)
	repair.Specs = nil
	fixed := mustPlan(t, repair)
	if err := ch.VerifyOwned(fixed.Desired, fixed.Receipt); err != nil {
		t.Fatal(err)
	}
	if fixed.Receipt.EntryDigest != i.Receipt.EntryDigest || fixed.Receipt.RemainderDigest != i.Receipt.RemainderDigest {
		t.Fatal("repair changed fixed specification or remainder")
	}
	repeat := mustPlan(t, request(fixed.Desired, ch.Repair, fixed.Receipt))
	if !repeat.NoOp || !bytes.Equal(repeat.Desired, fixed.Desired) {
		t.Fatal("intact repair rewrote config")
	}
	repair.Specs = []ch.HookSpec{{Executable: "/TEST other/bin", Selector: i.Receipt.Spec.Selector}}
	refused(t, repair, nil)
	for _, body := range [][]byte{
		nil,
		[]byte(strings.Replace(string(removed.Desired), "9007199254740993", "9007199254740992", 1)),
		[]byte(strings.Replace(string(removed.Desired), `"foreign"`, `"changed"`, 1)),
	} {
		refused(t, request(body, ch.Repair, i.Receipt), ch.ErrAbsenceUnproven)
	}
	for _, op := range []ch.Operation{ch.Install, ch.Update, ch.Remove} {
		refused(t, request(removed.Desired, op, i.Receipt), ch.ErrAbsenceUnproven)
	}

	// Legitimate foreign changes do not revoke an intact owner's authority.
	drift := []byte(strings.Replace(string(i.Desired), `"foreign"`, `"legitimate edit"`, 1))
	if err := ch.VerifyOwned(drift, i.Receipt); err != nil {
		t.Fatal(err)
	}
	refused(t, request(drift, ch.Repair, i.Receipt), ch.ErrAbsenceUnproven)
	for _, op := range []ch.Operation{ch.Update, ch.Remove} {
		r := mustPlan(t, request(drift, op, i.Receipt))
		if !strings.Contains(string(r.Desired), `"legitimate edit"`) {
			t.Fatal("ordinary lifecycle lost legitimate foreign edit")
		}
		if op == ch.Update && (!r.NoOp || r.Receipt.RemainderDigest == i.Receipt.RemainderDigest) {
			t.Fatal("no-op update did not refresh the verified remainder proof")
		}
	}
}

// Red condition: missing receipt/version/digest or unverified executable facts
// would grant authority, or foreign bytes would leak into the persisted proof.
func TestReceiptAuthorityAndHostAttestation(t *testing.T) {
	i := mustPlan(t, request([]byte(`{"version":1,"foreign":"TEST private opaque"}`), ch.Install, nil))
	encoded, err := json.Marshal(i.Receipt)
	if err != nil || bytes.Contains(encoded, []byte("TEST private opaque")) {
		t.Fatal("receipt retained foreign config")
	}
	for _, edit := range []func(*ch.Receipt){
		func(r *ch.Receipt) { r.Version = 2 },
		func(r *ch.Receipt) { r.Event = "future" },
		func(r *ch.Receipt) { r.EntryDigest = strings.Repeat("0", 71) },
		func(r *ch.Receipt) { r.RemainderDigest = "" },
		func(r *ch.Receipt) { r.Spec.Executable = "/TEST changed/bin" },
		func(r *ch.Receipt) { r.Shell = ch.WindowsUnqualified },
	} {
		r := *i.Receipt
		edit(&r)
		refused(t, request(i.Desired, ch.Update, &r), nil)
		if err := ch.VerifyOwned(i.Desired, &r); !errors.Is(err, ch.ErrConflict) {
			t.Fatal("invalid receipt accepted")
		}
	}
	for _, op := range []ch.Operation{ch.Install, ch.Update, ch.Remove, ch.Repair} {
		if op != ch.Install {
			refused(t, request(i.Desired, op, nil), nil)
		}
		if op != ch.Remove {
			req := request(i.Desired, op, i.Receipt)
			req.ExecutableVerified = false
			refused(t, req, nil)
		}
	}
}
