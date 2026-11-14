package cline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestSessionNativeInventoryRejectsMissingFileIOBeforePathAccess(t *testing.T) {
	// No fixture file is created: the explicit missing-IO authority must
	// refuse before trying to resolve even the selected directory.
	path := filepath.Join(t.TempDir(), "absent", "mcp.json")
	plan := clients.SessionPlan{Executable: "/native/cline", Directory: filepath.Dir(path),
		Servers: []string{"owned"}, ConfigDigest: strings.Repeat("a", 64),
		Environment: map[string]string{"CLINE_MCP_SETTINGS_PATH": path}}
	runner := &sessionObservationRunner{}
	err := New().VerifySession(t.Context(), clients.Env{Runner: runner}, plan)
	if err == nil || !strings.Contains(err.Error(), "native config file IO is required") || runner.command.Argv != nil {
		t.Fatalf("missing FileIO probed local selected source or native runner: %v, %+v", err, runner.command)
	}
}

func TestSessionNativeInventoryRequiresExactSource(t *testing.T) {
	good := `[{"name":"owned","path":"/project/mcp.json","transportType":"stdio","disabled":false}]`
	if err := verifySessionInventory([]byte(good), "/project/mcp.json", []string{"owned"}); err != nil {
		t.Fatal(err)
	}
	// A native client that reports an empty effective approval list is safe;
	// inherited/nonempty tool approvals must never qualify an inspected session.
	emptyApprovals := strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":[]`, 1)
	if err := verifySessionInventory([]byte(emptyApprovals), "/project/mcp.json", []string{"owned"}); err != nil {
		t.Fatal(err)
	}
	bad := []string{"[]", "null", "help", good + "{}", strings.Replace(good, "owned", "foreign", 1),
		strings.Replace(good, "/project/mcp.json", "/user/mcp.json", 1), strings.Replace(good, "false", "true", 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"disabled":false`, 1),
		strings.Replace(good, `"stdio"`, `""`, 1),
		strings.Replace(good, `"stdio"`, `"sse"`, 1),
		strings.Replace(good, `"stdio"`, `"http"`, 1),
		strings.Replace(good, `"stdio"`, `"websocket"`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":["*"]`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":["read_file"]`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":true`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":null`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":{}`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":[],"autoApprove":[]`, 1),
		"[null]", "[{},{}]"}
	for _, body := range bad {
		if err := verifySessionInventory([]byte(body), "/project/mcp.json", []string{"owned"}); err == nil {
			t.Fatal("invalid inventory accepted", body)
		}
	}
}

type sessionObservationRunner struct {
	command legacyports.Command
	result  legacyports.CommandResult
	err     error
	bounded bool
	cancel  context.CancelFunc
	onRun   func()
}

func (runner *sessionObservationRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.command = command
	_, runner.bounded = ctx.Deadline()
	if runner.onRun != nil {
		runner.onRun()
	}
	if runner.cancel != nil {
		runner.cancel()
	}
	return runner.result, runner.err
}

func TestSessionNativeInventoryExitAndCapability(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "mcp.json")
	owned := []byte(fmt.Sprintf(`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":%q,"cwd":%q},"autoApprove":[]}}}`, filepath.Join(project, "owned-probe.exe"), project))
	if err := os.WriteFile(path, owned, 0o600); err != nil {
		t.Fatal(err)
	}
	plan := clients.SessionPlan{Executable: "/native/cline", Directory: project, Servers: []string{"owned"},
		ConfigDigest: fmt.Sprintf("%x", sha256.Sum256(owned)),
		Environment: map[string]string{"CLINE_MCP_SETTINGS_PATH": path, "CLINE_NO_AUTO_UPDATE": "1"}}
	good := []byte(fmt.Sprintf(`[{"name":"owned","path":%q,"transportType":"stdio","disabled":false}]`, path))
	config := nativeconfig.New()
	// No real Cline executable is installed or launched in this regression.
	// Revoked consent must prevent even the runner entry point from being called.
	preCanceled, cancelBefore := context.WithCancel(context.Background())
	cancelBefore()
	preRunner := &sessionObservationRunner{result: legacyports.CommandResult{Stdout: good, CleanupComplete: true}}
	if err := New().VerifySession(preCanceled, clients.Env{Runner: preRunner, NativeConfig: config}, plan); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled observation returned %v, want context.Canceled", err)
	}
	if preRunner.command.Argv != nil {
		t.Fatalf("pre-canceled observation reached native runner: %+v", preRunner.command)
	}
	for _, failure := range []struct {
		result legacyports.CommandResult
		err    error
	}{{legacyports.CommandResult{ExitCode: 1, Stdout: good, CleanupComplete: true}, nil}, {legacyports.CommandResult{Stdout: good, CleanupComplete: true}, errors.New("native failure")},
		{legacyports.CommandResult{Stdout: []byte("Usage: cline"), CleanupComplete: true}, nil}, {legacyports.CommandResult{Stdout: make([]byte, (1<<20)+1), CleanupComplete: true}, nil},
		{legacyports.CommandResult{Stdout: good}, nil}, {legacyports.CommandResult{Stdout: good, CleanupComplete: true, NoChildStarted: true}, nil}} {
		runner := &sessionObservationRunner{result: failure.result, err: failure.err}
		if err := New().VerifySession(context.Background(), clients.Env{Runner: runner, NativeConfig: config}, plan); err == nil {
			t.Fatal("native failure accepted")
		}
	}
	runner := &sessionObservationRunner{result: legacyports.CommandResult{Stdout: good, CleanupComplete: true}}
	if err := New().VerifySession(context.Background(), clients.Env{Runner: runner, NativeConfig: config}, plan); err != nil {
		t.Fatal(err)
	}
	if !runner.bounded || runner.command.StdoutLimitBytes != 1<<20 || runner.command.Dir != project ||
		!reflect.DeepEqual(runner.command.Argv, []string{"/native/cline", "--auto-approve", "false", "--cwd", project, "config", "mcp", "--json"}) {
		t.Fatalf("native observation changed scope or command: %+v", runner.command)
	}
	canceled, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.cancel = cancel
	if err := New().VerifySession(canceled, clients.Env{Runner: runner, NativeConfig: config}, plan); !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancellation admitted native inventory: %v", err)
	}
}

// This read-only file seam reproduces consent revocation while the selected
// MCP source is being read. Native runners must never see that canceled call.
type cancelNativeSourceOnRead struct {
	nativeconfig.FileIO
	body   []byte
	cancel context.CancelFunc
}

func (file cancelNativeSourceOnRead) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	file.cancel()
	return bytes.Clone(file.body), 0o600, true, nil
}

func TestSessionNativeInventoryCancellationDuringSourceRead(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "mcp.json")
	// The physical source must exist even when a test-only file IO cancels
	// during its later read. Synthetic in-memory bytes are not a file lease.
	owned := []byte(fmt.Sprintf(`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":%q,"cwd":%q},"autoApprove":[]}}}`,
		filepath.Join(project, "owned-probe.exe"), project))
	if err := os.WriteFile(path, owned, 0o600); err != nil {
		t.Fatal(err)
	}
	plan := clients.SessionPlan{Executable: "/native/cline", Directory: project, Servers: []string{"owned"},
		ConfigDigest: fmt.Sprintf("%x", sha256.Sum256(owned)),
		Environment: map[string]string{"CLINE_MCP_SETTINGS_PATH": path, "CLINE_NO_AUTO_UPDATE": "1"}}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runner := &sessionObservationRunner{result: legacyports.CommandResult{CleanupComplete: true}}
	kernel := nativeconfig.NewWithFileIO(cancelNativeSourceOnRead{body: owned, cancel: cancel})
	err := New().VerifySession(ctx, clients.Env{Runner: runner, NativeConfig: kernel}, plan)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("source-read cancellation returned %v, want context.Canceled", err)
	}
	if runner.command.Argv != nil {
		t.Fatalf("revoked consent reached native runner: %+v", runner.command)
	}
}

func TestSessionNativeInventoryRefusesSelectedSourceChanges(t *testing.T) {
	project := t.TempDir()
	path := filepath.Join(project, "mcp.json")
	baseline := []byte(fmt.Sprintf(`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":%q,"cwd":%q},"autoApprove":[]}}}`, filepath.Join(project, "owned-probe.exe"), project))
	foreign := []byte(fmt.Sprintf(`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":%q,"cwd":%q},"autoApprove":[]}}}`, filepath.Join(project, "foreign-probe.exe"), project))
	plan := clients.SessionPlan{Executable: "/native/cline", Directory: project, Servers: []string{"owned"},
		ConfigDigest: fmt.Sprintf("%x", sha256.Sum256(baseline)),
		Environment: map[string]string{"CLINE_MCP_SETTINGS_PATH": path, "CLINE_NO_AUTO_UPDATE": "1"}}
	rows := []byte(fmt.Sprintf(`[{"name":"owned","path":%q,"transportType":"stdio","disabled":false}]`, path))
	kernel := nativeconfig.New()
	for _, mutation := range []string{"stale-before", "changed-during", "unchanged"} {
		t.Run(mutation, func(t *testing.T) {
			if err := os.WriteFile(path, baseline, 0o600); err != nil {
				t.Fatal(err)
			}
			runner := &sessionObservationRunner{result: legacyports.CommandResult{Stdout: rows, CleanupComplete: true}}
			if mutation == "stale-before" {
				if err := os.WriteFile(path, foreign, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if mutation == "changed-during" {
				runner.onRun = func() {
					if err := os.WriteFile(path, foreign, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			err := New().VerifySession(t.Context(), clients.Env{Runner: runner, NativeConfig: kernel}, plan)
			if mutation == "unchanged" {
				if err != nil {
					t.Fatal("unchanged owned config refused", err)
				}
			} else {
				if err == nil {
					t.Fatal("changed Project MCP command accepted from unchanged native inventory")
				}
				if mutation == "stale-before" && runner.command.Argv != nil {
					t.Fatal("stale config reached native CLI inspection")
				}
			}
			want := foreign
			if mutation == "unchanged" {
				want = baseline
			}
			body, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(body, want) {
				t.Fatal("refused inspector restored or changed concurrent bytes", err)
			}
		})
	}
}

// A byte-for-byte replacement with unchanged permissions is still a foreign
// file incarnation. This test never authorizes or modifies that replacement.
func TestSessionNativeInventoryRefusesSameBytesFileReplacement(t *testing.T) {
	req, path := ownedSessionFixture(t)
	kernel := nativeconfig.New()
	plan, err := New().PlanSession(t.Context(), clients.Env{NativeConfig: kernel}, req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := []byte(fmt.Sprintf(`[{"name":"owned","path":%q,"transportType":"stdio","disabled":false}]`, path))
	var replacementErr error
	runner := &sessionObservationRunner{result: legacyports.CommandResult{Stdout: rows, CleanupComplete: true}}
	runner.onRun = func() {
		replacement := path + ".foreign"
		if err := os.WriteFile(replacement, body, original.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
		replacementErr = os.Rename(replacement, path)
	}
	verificationErr := New().VerifySession(t.Context(), clients.Env{Runner: runner, NativeConfig: kernel}, plan)
	if replacementErr != nil {
		// Some Windows filesystems deny replacing an open file by design. A
		// refused rename supplies no replacement-path regression evidence.
		t.Skipf("filesystem refused replacement of the held native file: %v", replacementErr)
	}
	replaced, statErr := os.Lstat(path)
	if statErr != nil || os.SameFile(original, replaced) || original.Mode() != replaced.Mode() ||
		!bytes.Equal(body, mustSessionBytes(t, path)) {
		t.Fatal("same-byte replacement was not independently established", statErr)
	}
	if !errors.Is(verificationErr, nativeconfig.ErrConcurrentChange) {
		t.Fatalf("same-byte foreign file was accepted by native inventory: %v", verificationErr)
	}
}

func mustSessionBytes(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestSessionNativeInventoryCancellationDuringFinalSourceRead(t *testing.T) {
	req, path := ownedSessionFixture(t)
	kernel := nativeconfig.New()
	plan, err := New().PlanSession(t.Context(), clients.Env{NativeConfig: kernel}, req)
	if err != nil {
		t.Fatal(err)
	}
	original, err := kernel.ReadExactFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	files := &sessionReadIO{beforeRead: func(read int) {
		if read == 2 {
			cancel()
		}
	}}
	body, err := json.Marshal([]map[string]any{{"name": "owned", "path": path, "transportType": "stdio", "disabled": false}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	runner := &sessionObservationRunner{
		result: legacyports.CommandResult{Stdout: body, CleanupComplete: true},
		onRun:  func() { calls++ },
	}
	err = New().VerifySession(ctx, clients.Env{Runner: runner, NativeConfig: nativeconfig.NewWithFileIO(files)}, plan)
	if !errors.Is(err, context.Canceled) || calls != 1 || files.reads != 2 {
		t.Fatalf("final source-read cancellation admitted observation: calls=%d, reads=%d, err=%v", calls, files.reads, err)
	}
	after, err := kernel.ReadExactFile(path)
	if err != nil || after.Exists != original.Exists || after.Mode != original.Mode || !bytes.Equal(after.Body, original.Body) {
		t.Fatal("final source-read cancellation changed original source bytes or mode", err)
	}
}
