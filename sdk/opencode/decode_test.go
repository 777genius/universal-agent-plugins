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
