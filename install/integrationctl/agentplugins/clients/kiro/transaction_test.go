package kiro

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestKiroFailedRestoreRetainsLastBackup(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kiro")
	active, previous := kiroNativeFixture(t, root, "old", "https://docs.example.test/old")
	if err := applyKiroNativeMutation(root, active, nil, previous); err != nil {
		t.Fatal(err)
	}
	active, desired := kiroNativeFixture(t, root, "new", "https://docs.example.test/new")
	installErr, restoreErr := errors.New("injected install failure"), errors.New("injected restore failure")
	var retained string
	restores := 0
	rename := func(src, dst string) error {
		if strings.HasPrefix(filepath.Base(src), "new-") {
			return installErr
		}
		if strings.HasPrefix(filepath.Base(src), "old-") {
			retained = src
			restores++
			return restoreErr
		}
		return os.Rename(src, dst)
	}
	err := applyKiroNativeMutationWithOps(root, active, previous, desired, rename, os.RemoveAll)
	if !errors.Is(err, installErr) || !errors.Is(err, restoreErr) {
		t.Errorf("both error causes must survive: %v", err)
	}
	if restores != 1 {
		t.Errorf("restore attempts = %d", restores)
	}
	body, readErr := os.ReadFile(filepath.Join(retained, "SKILL.md"))
	if readErr != nil || string(body) != "old\n" {
		t.Errorf("last backup lost at %s: %q, %v", retained, body, readErr)
	}
	if retained == "" || err == nil || !strings.Contains(err.Error(), retained) {
		t.Errorf("retained backup path missing: %v", err)
	}
}

func TestKiroMCPRollbackPreservesConcurrentVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings", "mcp.json")
	original := []byte(`{"mcpServers":{"old":{"url":"https://old.test"}}}`)
	ours := []byte(`{"mcpServers":{"new":{"url":"https://new.test"}}}`)
	foreign := []byte(`{"mcpServers":{"foreign":{"url":"https://foreign.test"}}}`)
	writeTestFile(t, path, string(original))
	file, err := nativeconfig.New().BeginExactFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close MCP transaction: %v", err)
		}
	}()
	if err := file.Apply(ours); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, string(foreign))
	err = rollbackKiroNative(kiroMCPMutation{file: file, body: ours}, nil, nil, nil, true)
	got, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(got, foreign) {
		t.Fatalf("concurrent MCP overwritten: %s, %v (rollback %v)", got, readErr, err)
	}
	if err == nil {
		t.Fatal("concurrent rollback must report uncertainty")
	}
}

func TestKiroRestorePreservesLateEmptyForeignDirectory(t *testing.T) {
	root := t.TempDir()
	backup, target := filepath.Join(root, "old-docs"), filepath.Join(root, "docs")
	writeTestFile(t, filepath.Join(backup, "SKILL.md"), "old\n")
	previous := map[string]domain.NativeObjectOwnership{"docs": {Path: target}}
	calls := 0
	rename := func(src, dst string) error {
		calls++
		if err := os.Mkdir(dst, 0700); err != nil {
			return err
		}
		return shared.RenameDirectoryExclusive(src, dst)
	}
	_, err := rollbackKiroNativeWithOps(kiroMCPMutation{}, previous, map[string]string{"docs": backup}, nil, false, rename, os.RemoveAll)
	if err == nil || calls != 1 {
		t.Fatalf("restore = %v, calls %d", err, calls)
	}
	body, readErr := os.ReadFile(filepath.Join(backup, "SKILL.md"))
	if readErr != nil || string(body) != "old\n" {
		t.Fatalf("backup = %q, %v", body, readErr)
	}
	entries, readErr := os.ReadDir(target)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("foreign target replaced: %v, %v", entries, readErr)
	}
}
