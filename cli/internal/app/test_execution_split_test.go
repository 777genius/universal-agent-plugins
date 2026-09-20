package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestResolveFixturePathDefaultsToPlatformEventFixture(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	got := resolveFixturePath(root, "", "claude", "Stop")
	want := filepath.Join(root, "fixtures", "claude", "Stop.json")
	if got != want {
		t.Fatalf("resolveFixturePath() = %q, want %q", got, want)
	}
}

func TestExecuteRuntimeTestCommandRejectsMissingCommand(t *testing.T) {
	t.Parallel()

	_, _, _, err := executeRuntimeTestCommand(context.Background(), t.TempDir(), nil, nil)
	if err == nil || err.Error() != "missing command" {
		t.Fatalf("err = %v, want missing command", err)
	}
}
