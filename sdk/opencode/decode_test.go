package opencode

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSharedWireFixtures(t *testing.T) {
	b, err := os.ReadFile("../opencode-js/fixtures/wire-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	root := true
	want := []ObservedEvent{
		{Version: Version, Kind: TurnIdleVerified, SessionID: "s1", TurnID: "u1", MessageID: "a1", RootSession: &root},
		{Version: Version, Kind: QuestionAsked, SessionID: "s1", TurnID: "u1", RequestID: "q1", RootSession: &root},
		{Version: Version, Kind: PermissionAsked, SessionID: "s1", TurnID: "u1", RequestID: "p1", RootSession: &root},
		{Version: Version, Kind: TerminalError, SessionID: "s1", TurnID: "u2", RootSession: &root},
		{Version: Version, Kind: Unknown, SessionID: "s1", NativeType: "future.event"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d fixtures", len(rows))
	}
	for i, row := range rows {
		e, err := Decode(row)
		if err != nil || !reflect.DeepEqual(e, want[i]) {
			t.Fatalf("row %d: got %#v, want %#v, err %v", i, e, want[i], err)
		}
	}
}

func TestVersionAndShape(t *testing.T) {
	if _, err := Decode([]byte(strings.Repeat("x", MaxBytes+1))); err == nil {
		t.Fatal("accepted oversized wire")
	}
	for _, input := range []string{
		`{"version":2,"kind":"terminal_error","sessionID":"s","turnID":"u"}`,
		`{"version":1,"kind":"question_asked","sessionID":"s","turnID":"u"}`,
		`{"version":1,"kind":"turn_idle_verified","sessionID":"s","turnID":"u","messageID":"a"} {}`,
		`{"version":1,"kind":"terminal_error","sessionID":42,"turnID":"u"}`,
		`{"version":1,"kind":"terminal_error","sessionID":"s\u0000","turnID":"u","rootSession":true}`,
		`{"version":1,"kind":"terminal_error","sessionID":"s","turnID":"u","messageID":"a"}`,
		`{"version":1,"kind":"terminal_error","sessionID":"s","turnID":"u"}`,
	} {
		if _, err := Decode([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	for _, input := range []string{
		`{"version":1,"kind":"future_kind","additional":{"secret":"ignored"}}`,
		`{"version":1,"kind":"unknown","nativeType":"future.event","newField":true}`,
	} {
		e, err := Decode([]byte(input))
		if err != nil || e.Kind != Unknown {
			t.Fatalf("additive %s: %#v %v", input, e, err)
		}
	}
	child, err := Decode([]byte(`{"version":1,"kind":"turn_idle_verified","sessionID":"s","turnID":"u","messageID":"a","rootSession":false}`))
	if err != nil || child.RootSession == nil || *child.RootSession {
		t.Fatalf("child ancestry lost: %#v %v", child, err)
	}
}

func TestAdditiveNativeProvenance(t *testing.T) {
	for _, generation := range []string{"v1", "v2"} {
		basis, nativeID := "assistant_created_lower_bound", ""
		if generation == "v2" {
			basis, nativeID = "envelope_created", "native-start"
		}
		payload, err := json.Marshal(map[string]any{
			"version": 1, "kind": "permission_asked", "sessionID": "session", "turnID": "native-turn", "requestID": "permission", "rootSession": true,
			"provenance": Provenance{Generation: generation, ObservationID: `["native","canonical"]`, NativeEventID: nativeID, NativeTime: 1790867856742, TimeBasis: basis},
		})
		if err != nil {
			t.Fatal(err)
		}
		e, err := Decode(payload)
		if err != nil || e.Provenance == nil || e.Provenance.NativeTime != 1790867856742 || e.Provenance.TimeBasis != basis {
			t.Fatalf("provenance lost: %#v %v", e, err)
		}
		// The pre-extension v1 consumer's JSON projection ignores additive evidence.
		var old struct {
			Version                      int
			Kind                         Kind
			SessionID, TurnID, RequestID string
			RootSession                  *bool
		}
		if err := json.Unmarshal(payload, &old); err != nil || old.Version != 1 || old.Kind != PermissionAsked || old.TurnID != "native-turn" || old.RootSession == nil || !*old.RootSession {
			t.Fatalf("old projection incompatible: %#v %v", old, err)
		}
	}
	for _, p := range []string{
		`{"generation":"v2","observationID":"x","nativeTime":1,"timeBasis":"envelope_created"}`,
		`{"generation":"v1","observationID":"x","nativeTime":1,"timeBasis":"receive_time"}`,
		`{"generation":"v1","observationID":"x","nativeTime":0,"timeBasis":"assistant_completed"}`,
		`{"generation":"v1","observationID":"x","nativeTime":1,"timeBasis":"assistant_completed"}`,
		`{"generation":"v2","observationID":"x","nativeEventID":"event","nativeTime":1,"timeBasis":"assistant_created_lower_bound"}`,
		`{"generation":"v1","observationID":"x","nativeTime":1.5,"timeBasis":"assistant_completed"}`,
	} {
		if _, err := Decode([]byte(`{"version":1,"kind":"terminal_error","sessionID":"s","turnID":"u","rootSession":true,"provenance":` + p + `}`)); err == nil {
			t.Fatalf("accepted untruthful provenance %s", p)
		}
	}
}

// NativeMessageID is typed evidence; legacy absence is still wire-compatible.
func TestV1TerminalMessageIdentity(t *testing.T) {
	base := `{"version":1,"kind":"terminal_error","sessionID":"s","turnID":"u","rootSession":true`
	if _, err := Decode([]byte(base + `}`)); err != nil {
		t.Fatal(err)
	}
	for _, messageID := range []string{"", "assistant", "invalid\x00", strings.Repeat("a", 257)} {
		p, err := json.Marshal(Provenance{Generation: "v1", ObservationID: "opaque-no-parsing", NativeMessageID: messageID, NativeTime: 1790867856742, TimeBasis: "assistant_created_lower_bound"})
		if err != nil {
			t.Fatal(err)
		}
		e, err := Decode([]byte(base + `,"provenance":` + string(p) + `}`))
		if messageID == "assistant" {
			if err != nil || e.Provenance == nil || e.Provenance.NativeMessageID != messageID {
				t.Fatalf("identity lost: %#v %v", e, err)
			}
		} else if err == nil {
			t.Fatalf("accepted invalid native terminal message %q", messageID)
		}
	}
}
