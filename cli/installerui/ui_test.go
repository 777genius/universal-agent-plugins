package installerui

import (
	"context"
	"errors"
	"io"
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
	if err != nil || !got.Accepted || got.Cancelled || len(got.IDs) != 1 || got.IDs[0] != "codex" {
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
	if err != nil || !got.Cancelled || got.Accepted {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	ui, _ = New(Config{Input: strings.NewReader(""), Output: io.Discard})
	_, err = ui.SelectOne(context.Background(), SelectRequest{Title: "Action", Options: []Option{{ID: "install", Label: "Install"}}})
	if !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF, got %v", err)
	}
}
