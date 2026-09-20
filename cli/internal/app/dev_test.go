package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPluginServiceDevOnceRunsRenderValidateAndFixture(t *testing.T) {
	restorePluginTestHelpers(t)
	dir := runtimeTestProjectRoot(t, "claude")
	writeBootstrapProjectFile(t, dir, filepath.Join("fixtures", "claude", "Stop.json"), `{"session_id":"s","cwd":"/tmp","hook_event_name":"Stop"}`)

	var svc PluginService
	var updates []PluginDevUpdate
	summary, err := svc.Dev(context.Background(), PluginDevOptions{
		Root:     dir,
		Platform: "claude",
		Event:    "Stop",
		Once:     true,
	}, func(update PluginDevUpdate) {
		updates = append(updates, update)
	})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Cycles != 1 || !summary.LastPassed {
		t.Fatalf("summary = %+v", summary)
	}
	if len(updates) != 1 {
		t.Fatalf("updates = %d", len(updates))
	}
	output := strings.Join(updates[0].Lines, "\n")
	for _, want := range []string{
		"Cycle 1 [initial]",
		"Generate: wrote",
		"Validate: ok",
		"PASS claude/Stop",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestPluginServiceDevWatchRerunsOnFixtureChange(t *testing.T) {
	restorePluginTestHelpers(t)
	dir := runtimeTestProjectRoot(t, "claude")
	fixturePath := filepath.Join(dir, "fixtures", "claude", "Stop.json")
	writeBootstrapProjectFile(t, dir, filepath.Join("fixtures", "claude", "Stop.json"), `{"session_id":"s","cwd":"/tmp","hook_event_name":"Stop"}`)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var svc PluginService
	updatesCh := make(chan PluginDevUpdate, 4)
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.Dev(ctx, PluginDevOptions{
			Root:     dir,
			Platform: "claude",
			Event:    "Stop",
			Interval: 25 * time.Millisecond,
		}, func(update PluginDevUpdate) {
			updatesCh <- update
			if update.Cycle >= 2 {
				cancel()
			}
		})
		errCh <- err
	}()

	first := receiveDevUpdate(t, updatesCh)
	if first.Cycle != 1 {
		t.Fatalf("first cycle = %d", first.Cycle)
	}
	if err := os.WriteFile(fixturePath, []byte(`{"session_id":"s2","cwd":"/tmp","hook_event_name":"Stop"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	second := receiveDevUpdate(t, updatesCh)
	if second.Cycle != 2 {
		t.Fatalf("second cycle = %d", second.Cycle)
	}
	output := strings.Join(second.Lines, "\n")
	if !strings.Contains(output, "Cycle 2 [watch]") {
		t.Fatalf("output = %s", output)
	}
	if !strings.Contains(output, "fixtures/claude/Stop.json") {
		t.Fatalf("output = %s", output)
	}
	if err := receiveDevError(t, errCh); err != nil {
		t.Fatal(err)
	}
}

func receiveDevUpdate(t *testing.T, updates <-chan PluginDevUpdate) PluginDevUpdate {
	t.Helper()
	select {
	case update := <-updates:
		return update
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dev update")
		return PluginDevUpdate{}
	}
}

func receiveDevError(t *testing.T, errors <-chan error) error {
	t.Helper()
	select {
	case err := <-errors:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dev completion")
		return nil
	}
}

func TestResolveDevPlatformGeminiRequestedReturnsRuntimeGuidance(t *testing.T) {
	t.Parallel()
	_, err := resolveDevPlatform("/unused", "gemini")
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{
		"Gemini uses its dedicated runtime gate instead",
		"plugin-kit-ai generate .",
		"plugin-kit-ai validate . --platform gemini --strict",
		"plugin-kit-ai inspect . --target gemini",
		"plugin-kit-ai capabilities --mode runtime --platform gemini",
		"make test-gemini-runtime",
		"gemini extensions link .",
		"make test-gemini-runtime-live",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q:\n%s", want, err)
		}
	}
}
