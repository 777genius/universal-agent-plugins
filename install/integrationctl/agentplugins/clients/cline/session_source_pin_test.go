package cline

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type unboundedSessionSourceIO struct {
	nativeconfig.FileIO
	calls int
}

func (files *unboundedSessionSourceIO) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	files.calls++
	return nil, 0, false, errors.New("unbounded custom source must not be read")
}

type recordedBoundedSessionIO struct {
	nativeconfig.FileIO
	body  []byte
	mode  os.FileMode
	calls int
}

func (files *recordedBoundedSessionIO) ReadNoFollowBounded(string, int64) ([]byte, os.FileMode, bool, error) {
	files.calls++
	return bytes.Clone(files.body), files.mode, true, nil
}

func sourceInventoryResult(t *testing.T, path string) legacyports.CommandResult {
	t.Helper()
	body, err := json.Marshal([]map[string]any{{"name": "owned", "path": path, "transportType": "stdio", "disabled": false}})
	if err != nil {
		t.Fatal(err)
	}
	return legacyports.CommandResult{Stdout: body, CleanupComplete: true}
}

func ownedVerificationPlan(t *testing.T) (clients.SessionPlan, string, nativeconfig.FileSnapshot) {
	t.Helper()
	req, path := ownedSessionFixture(t)
	kernel := nativeconfig.New()
	plan, err := New().PlanSession(t.Context(), clients.Env{NativeConfig: kernel}, req)
	if err != nil {
		t.Fatal(err)
	}
	file, err := kernel.ReadExactFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return plan, path, file
}

func TestSessionBoundedReaderRefusesUnsupportedCustomAuthority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent", "mcp.json")
	plan := clients.SessionPlan{Executable: "/native/cline", Directory: filepath.Dir(path),
		Servers: []string{"owned"}, ConfigDigest: strings.Repeat("a", 64),
		Environment: map[string]string{"CLINE_MCP_SETTINGS_PATH": path}}
	files := &unboundedSessionSourceIO{}
	runner := &sessionObservationRunner{}
	err := New().VerifySession(t.Context(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files), Runner: runner}, plan)
	if !errors.Is(err, nativeconfig.ErrBoundedReadUnavailable) || files.calls != 0 || runner.command.Argv != nil {
		t.Fatalf("unsupported custom reader acquired local/native authority: calls=%d err=%v", files.calls, err)
	}
	if _, err := os.Lstat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("refused source created an absent parent: %v", err)
	}
}

func TestSessionBoundedSnapshotMustMatchActualHeldBytes(t *testing.T) {
	plan, path, owned := ownedVerificationPlan(t)
	foreign := bytes.Replace(owned.Body, []byte("owned-probe"), []byte("other-probe"), 1)
	if bytes.Equal(foreign, owned.Body) {
		t.Fatal("fixture did not change the owned executable")
	}
	if err := os.WriteFile(path, foreign, owned.Mode); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	files := &recordedBoundedSessionIO{body: owned.Body, mode: owned.Mode}
	runner := &sessionObservationRunner{result: sourceInventoryResult(t, path)}
	err = New().VerifySession(t.Context(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files), Runner: runner}, plan)
	if !errors.Is(err, nativeconfig.ErrConcurrentChange) || files.calls != 1 || runner.command.Argv != nil {
		t.Fatalf("custom old bytes substituted for the actual held file: calls=%d err=%v", files.calls, err)
	}
	assertUnchangedSessionEntry(t, path, before, foreign)
}

func TestSessionBoundedPinRejectsSameBytesAndTimestampsReplacement(t *testing.T) {
	plan, path, owned := ownedVerificationPlan(t)
	original, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(path)
	parentBefore, err := os.Lstat(parent)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(t.TempDir(), "replacement.json")
	if err := os.WriteFile(replacement, owned.Body, owned.Mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(replacement, original.ModTime(), original.ModTime()); err != nil {
		t.Fatal(err)
	}
	calls := 0
	runner := &sessionObservationRunner{result: sourceInventoryResult(t, path), onRun: func() {
		calls++
		if err := os.Rename(replacement, path); err != nil {
			t.Fatal("real same-filesystem replacement must execute", err)
		}
		if err := os.Chtimes(parent, parentBefore.ModTime(), parentBefore.ModTime()); err != nil {
			t.Fatal(err)
		}
	}}
	err = New().VerifySession(t.Context(), clients.Env{NativeConfig: nativeconfig.New(), Runner: runner}, plan)
	if !errors.Is(err, nativeconfig.ErrConcurrentChange) || calls != 1 {
		t.Fatalf("same-byte replacement passed native verification: calls=%d err=%v", calls, err)
	}
	after, err := os.Lstat(path)
	if err != nil || os.SameFile(original, after) || original.Mode() != after.Mode() ||
		original.Size() != after.Size() || !original.ModTime().Equal(after.ModTime()) {
		t.Fatalf("replacement did not preserve all nonidentity file metadata: %v", err)
	}
	assertUnchangedSessionEntry(t, parent, parentBefore, nil)
	if !bytes.Equal(mustSessionBytes(t, path), owned.Body) {
		t.Fatal("refusal rewrote the foreign replacement bytes")
	}
}

func TestSessionBoundedPinRejectsRetainedParentUnderNewAncestor(t *testing.T) {
	plan, path, owned := ownedVerificationPlan(t)
	parent := filepath.Dir(path)
	ancestor := filepath.Dir(parent)
	beforeParent, err := os.Lstat(parent)
	if err != nil {
		t.Fatal(err)
	}
	beforeFile, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeAncestor, err := os.Lstat(ancestor)
	if err != nil {
		t.Fatal(err)
	}
	moved := ancestor + ".owned-moved"
	t.Cleanup(func() {
		if err := os.RemoveAll(moved); err != nil {
			t.Error(err)
		}
	})
	calls := 0
	runner := &sessionObservationRunner{result: sourceInventoryResult(t, path), onRun: func() {
		calls++
		replaceSessionSourceAncestor(t, ancestor, parent, moved, beforeAncestor.Mode().Perm())
		if err := os.Chtimes(parent, beforeParent.ModTime(), beforeParent.ModTime()); err != nil {
			t.Fatal(err)
		}
	}}
	err = New().VerifySession(t.Context(), clients.Env{NativeConfig: nativeconfig.New(), Runner: runner}, plan)
	if !errors.Is(err, nativeconfig.ErrConcurrentChange) || calls != 1 {
		t.Fatalf("new ancestor with original parent/file was admitted: calls=%d err=%v", calls, err)
	}
	afterAncestor, err := os.Lstat(ancestor)
	if err != nil || os.SameFile(beforeAncestor, afterAncestor) {
		t.Fatalf("ancestor substitution was not established: %v", err)
	}
	assertUnchangedSessionEntry(t, parent, beforeParent, nil)
	assertUnchangedSessionEntry(t, path, beforeFile, owned.Body)
}

func replaceSessionSourceAncestor(t *testing.T, ancestor, parent, moved string, mode os.FileMode) {
	t.Helper()
	if _, err := os.Lstat(moved); !os.IsNotExist(err) {
		t.Fatal("moved fixture destination already exists", err)
	}
	if err := os.Rename(ancestor, moved); err != nil {
		t.Fatal("real ancestor rename must execute", err)
	}
	if err := os.Mkdir(ancestor, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(moved, filepath.Base(parent)), parent); err != nil {
		t.Fatal("original parent must be retained under replacement ancestor", err)
	}
}

func assertUnchangedSessionEntry(t *testing.T, path string, before os.FileInfo, body []byte) {
	t.Helper()
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() ||
		before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("refusal changed original entry identity or metadata: %v", err)
	}
	if body != nil && !bytes.Equal(mustSessionBytes(t, path), body) {
		t.Fatal("refusal changed original bytes")
	}
}
