package installerui

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestSelectManyDefaultsAndOpaqueIDs(t *testing.T) {
	var out strings.Builder
	ui, err := New(Config{Input: strings.NewReader("\n"), Output: &out})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ui.SelectMany(context.Background(), SelectRequest{
		Title: "Targets", Options: []Option{{ID: "claude", Label: "Claude"}, {ID: "codex", Label: "Codex"}}, Defaults: []string{"codex"},
	})
	if err != nil || !got.Accepted || got.Cancelled || len(got.IDs) != 1 || got.IDs[0] != "codex" { //nolint:misspell // Verify the existing public result field.
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if !strings.Contains(out.String(), "Codex") {
		t.Fatalf("output=%q", out.String())
	}
}

func TestCancellationAndEOF(t *testing.T) {
	ui, err := New(Config{Input: strings.NewReader("cancel\n"), Output: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ui.SelectOne(context.Background(), SelectRequest{Title: "Action", Options: []Option{{ID: "install", Label: "Install"}}})
	if err != nil || !got.Cancelled || got.Accepted { //nolint:misspell // Verify the existing public result field.
		t.Fatalf("got=%+v err=%v", got, err)
	}
	ui, _ = New(Config{Input: strings.NewReader(""), Output: io.Discard})
	_, err = ui.SelectOne(context.Background(), SelectRequest{Title: "Action", Options: []Option{{ID: "install", Label: "Install"}}})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF, got %v", err)
	}
}

// The opt-in facade must not tighten the older public line API.
func TestLegacySelectionRows(t *testing.T) {
	for _, tc := range []struct {
		name, input    string
		defaults, want []string
		cancel         bool
	}{
		{name: "answer-order", input: "2,1\n", want: []string{"none", "all"}},
		{name: "duplicate-defaults", input: "\n", defaults: []string{"all", "all"}, want: []string{"all"}},
		{name: "opaque-all", input: "all\n", want: []string{"all"}},
		{name: "opaque-none", input: "none\n", want: []string{"none"}},
		{name: "cancel-case", input: "Cancel\n", cancel: true},
		{name: "q-case", input: "Q\n", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, err := New(Config{Input: strings.NewReader(tc.input), Output: io.Discard})
			if err != nil {
				t.Fatal(err)
			}
			got, err := u.SelectMany(context.Background(), SelectRequest{Title: "Legacy", Options: []Option{{"all", "All ID"}, {"none", "None ID"}}, Defaults: tc.defaults})
			if err != nil || !reflect.DeepEqual(got.IDs, tc.want) || got.Cancelled != tc.cancel || got.Accepted == tc.cancel {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
}
