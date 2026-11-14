package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyControllerRejectsForeignHomeBeforeLegacyEngine(t *testing.T) {
	t.Parallel()
	controller := legacyController{statePath: filepath.Join(t.TempDir(), ".plugin-kit-ai", "state.json")}
	if err := controller.requireLegacyEngineHome(); err == nil {
		t.Fatal("mismatched legacy state root was accepted")
	}
	// PlanRemove is read-only even if the guard regresses. It must refuse
	// before invoking the old engine, which uses the ambient OS user home.
	_, err := controller.PlanRemove(context.Background(), "never-installed")
	if err == nil || !strings.Contains(err.Error(), "does not match OS user home") {
		t.Fatalf("expected explicit foreign-home refusal, got %v", err)
	}
}

func TestLegacyControllerAcceptsOriginalUserHome(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	controller := legacyController{statePath: filepath.Join(home, ".plugin-kit-ai", "state.json")}
	if err := controller.requireLegacyEngineHome(); err != nil {
		t.Fatalf("original legacy home was rejected: %v", err)
	}
}
