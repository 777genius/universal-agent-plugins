package nativeconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fastClineLockTiming() clineLockTiming {
	return clineLockTiming{
		staleAfter:        80 * time.Millisecond,
		pollInterval:      5 * time.Millisecond,
		acquireTimeout:    time.Second,
		heartbeatInterval: 10 * time.Millisecond,
	}
}

// This encodes the portable populated-directory protocol from Cline's
// settingsLock.ts at cline/cline@4829f08b3f444fc81a5c76243d9ee32da44c9160:
// <settings>.lock.tmp.<token>/owner.<token> is renamed to <settings>.lock.
func TestClineLockUsesOfficialOwnerTokenProtocolAndKeepsSlowHolderLive(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), "cline_mcp_settings.json.lock")
	timing := fastClineLockTiming()
	release, err := lockClineConfigWithTiming(lockDir, timing)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(lockDir)
	if err != nil || len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "owner.") {
		t.Fatalf("lock does not match Cline owner-token protocol: entries=%v err=%v", entries, err)
	}
	ownerPath := filepath.Join(lockDir, entries[0].Name())
	token, err := os.ReadFile(ownerPath)
	if err != nil || entries[0].Name() != "owner."+string(token) {
		t.Fatalf("owner filename/content token mismatch: name=%q token=%q err=%v", entries[0].Name(), token, err)
	}
	if staging, err := filepath.Glob(lockDir + ".tmp.*"); err != nil || len(staging) != 0 {
		t.Fatalf("visible staging directory remained: %v, %v", staging, err)
	}

	// Hold for multiple stale windows. A waiter following Cline's 10s-equivalent
	// stale rule must not rename/reclaim a live agentplugins critical section.
	time.Sleep(2 * timing.staleAfter)
	acquired := make(chan func() error, 1)
	errCh := make(chan error, 1)
	go func() {
		nextRelease, acquireErr := lockClineConfigWithTiming(lockDir, timing)
		if acquireErr != nil {
			errCh <- acquireErr
			return
		}
		acquired <- nextRelease
	}()
	select {
	case nextRelease := <-acquired:
		_ = nextRelease()
		t.Fatal("live Cline-compatible lock was reclaimed")
	case acquireErr := <-errCh:
		t.Fatalf("waiter failed before release: %v", acquireErr)
	case <-time.After(2 * timing.staleAfter):
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	select {
	case nextRelease := <-acquired:
		if err := nextRelease(); err != nil {
			t.Fatal(err)
		}
	case acquireErr := <-errCh:
		t.Fatal(acquireErr)
	case <-time.After(time.Second):
		t.Fatal("waiter did not acquire after release")
	}
}

func TestClineLockRecoversStaleCrashedOwnerByRename(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), "cline_mcp_settings.json.lock")
	if err := os.Mkdir(lockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, "owner.crashed"), []byte("crashed"), 0o600); err != nil {
		t.Fatal(err)
	}
	timing := fastClineLockTiming()
	old := time.Now().Add(-2 * timing.staleAfter)
	if err := os.Chtimes(lockDir, old, old); err != nil {
		t.Fatal(err)
	}
	release, err := lockClineConfigWithTiming(lockDir, timing)
	if err != nil {
		t.Fatal(err)
	}
	if stale, err := filepath.Glob(lockDir + ".stale.*"); err != nil || len(stale) != 0 {
		t.Fatalf("stale rename was not cleaned: %v, %v", stale, err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
}

func TestClineLockReleaseRejectsForeignOwnerToken(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), "cline_mcp_settings.json.lock")
	timing := fastClineLockTiming()
	timing.heartbeatInterval = time.Hour
	release, err := lockClineConfigWithTiming(lockDir, timing)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(lockDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read owner: entries=%v err=%v", entries, err)
	}
	ownerPath := filepath.Join(lockDir, entries[0].Name())
	if err := os.WriteFile(ownerPath, []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := release(); !errors.Is(err, ErrConcurrentChange) {
		t.Fatalf("foreign owner token was not reported as identity loss: %v", err)
	}
	if _, err := os.Lstat(lockDir); err != nil {
		t.Fatalf("foreign lock was removed: %v", err)
	}
}

func TestClineLockReleaseReportsLostDirectory(t *testing.T) {
	lockDir := filepath.Join(t.TempDir(), "cline_mcp_settings.json.lock")
	timing := fastClineLockTiming()
	timing.heartbeatInterval = time.Hour
	release, err := lockClineConfigWithTiming(lockDir, timing)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(lockDir); err != nil {
		t.Fatal(err)
	}
	if err := release(); !errors.Is(err, ErrConcurrentChange) {
		t.Fatalf("lost lock directory was not surfaced: %v", err)
	}
}
