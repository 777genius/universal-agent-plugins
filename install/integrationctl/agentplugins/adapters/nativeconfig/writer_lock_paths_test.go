package nativeconfig

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestWriterLockPathsSerializeMCPWrite(t *testing.T) {
	cases := []struct {
		codec Codec
		jsonc bool
		hold  int
	}{
		{CodecMCPServers, false, 0}, {CodecGemini, false, 0},
		{CodecOpenCode, false, 0}, {CodecWindsurf, false, 0}, {CodecCline, false, 0},
		{CodecGemini, true, 0}, {CodecGemini, true, 1},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/jsonc=%t/hold=%d", tc.codec, tc.jsonc, tc.hold), func(t *testing.T) {
			root := t.TempDir()
			paths := Paths{JSON: filepath.Join(root, "z-TEST.json")}
			selected := paths.JSON
			if tc.jsonc {
				paths.JSONC = filepath.Join(root, "a-TEST.jsonc")
				selected = paths.JSONC
			}
			locks, err := WriterLockPaths(paths, tc.codec)
			if err != nil {
				t.Fatal(err)
			}
			suffix := ".agentplugins.lock"
			if tc.codec == CodecCline {
				suffix = ".lock"
			}
			want := []string{paths.JSON + suffix}
			if tc.jsonc {
				want = append(want, paths.JSONC+suffix)
			}
			slices.Sort(want)
			if !slices.Equal(locks, want) {
				t.Fatalf("lock identities: got %v, want %v", locks, want)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("path query mutated filesystem: %v, %v", entries, err)
			}
			release := holdExportedWriterLock(t, locks[tc.hold], tc.codec)
			child := startWriterLockChild(t, "write", paths, tc.codec, "")
			assertWriterLockChildBlocked(t, child)
			if _, err := os.Lstat(selected); !os.IsNotExist(err) {
				t.Fatalf("writer changed settings before release: %v", err)
			}
			key := codecCollectionKey(tc.codec)
			// Publish a foreign sibling only after the writer is waiting. It must
			// read live settings after acquiring, including JSONC selection drift.
			mustWrite(t, selected, fmt.Sprintf(`{"hooks":{"retained":true},%q:{"foreign-tool":{"opaque":true}}}`, key))
			release()
			child.wait(t)
			assertWriterLockForeignEntries(t, selected, key)
			present, _, err := New().Inspect(paths, tc.codec, "TEST-owned", nil)
			if err != nil || !present {
				t.Fatalf("MCP write did not select live candidate: present=%t err=%v", present, err)
			}
		})
	}
}

func assertWriterLockForeignEntries(t *testing.T, path, key string) {
	t.Helper()
	var doc map[string]json.RawMessage
	var hooks map[string]bool
	var servers map[string]struct{ Opaque bool }
	if err := json.Unmarshal([]byte(mustRead(t, path)), &doc); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(doc["hooks"], &hooks); err != nil || !hooks["retained"] {
		t.Fatalf("foreign hook lost: %v %v", hooks, err)
	}
	if err := json.Unmarshal(doc[key], &servers); err != nil || !servers["foreign-tool"].Opaque {
		t.Fatalf("foreign MCP entry lost: %v %v", servers, err)
	}
}

func TestWriterLockPathsWaitForActualKernelHolder(t *testing.T) {
	paths := Paths{JSON: filepath.Join(t.TempDir(), "TEST-settings.json")}
	locks, err := WriterLockPaths(paths, CodecGemini)
	if err != nil {
		t.Fatal(err)
	}
	file, err := New().BeginExactFile(paths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	child := startWriterLockChild(t, "lock", paths, CodecGemini, locks[0])
	assertWriterLockChildBlocked(t, child)
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	child.wait(t)
}

func TestWriterLockPathsRejectInvalidCandidatesLikeApply(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "TEST-settings.json")
	cases := []struct {
		paths Paths
		codec Codec
	}{
		{Paths{}, CodecGemini}, {Paths{JSON: "relative.json"}, CodecGemini},
		{Paths{JSON: root + string(os.PathSeparator) + "./TEST-settings.json"}, CodecGemini},
		{Paths{JSON: path, JSONC: "relative.jsonc"}, CodecGemini},
		{Paths{JSON: path, JSONC: path}, CodecGemini}, {Paths{JSON: path}, Codec("unknown")},
		{Paths{JSON: path + "\x00"}, CodecGemini}, {Paths{JSON: path, JSONC: path + "\x00"}, CodecGemini},
	}
	for _, tc := range cases {
		locks, queryErr := WriterLockPaths(tc.paths, tc.codec)
		_, applyErr := New().Apply(Request{Paths: tc.paths, Codec: tc.codec, Action: ActionAdd, Name: "TEST-owned"})
		if queryErr == nil || applyErr == nil || queryErr.Error() != applyErr.Error() || locks != nil {
			t.Fatalf("inconsistent validation: locks=%v query=%v apply=%v", locks, queryErr, applyErr)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid request mutated filesystem: %v, %v", entries, err)
	}
}

func holdExportedWriterLock(t *testing.T, path string, codec Codec) func() {
	t.Helper()
	var release func() error
	if codec == CodecCline {
		// Independent Cline-compatible populated-directory holder, without the
		// writer's process mutex, naming helper, heartbeat or stale recovery.
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(path, "owner.TEST-holder"), "TEST-holder")
		release = func() error { return os.RemoveAll(path) }
	} else {
		var err error
		release, err = lockNativeConfig(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	held := true
	unlock := func() {
		if held {
			held = false
			if err := release(); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(unlock)
	return unlock
}

type writerLockChild struct {
	finished chan struct{}
	err      error
	stderr   bytes.Buffer
}

func startWriterLockChild(t *testing.T, mode string, paths Paths, codec Codec, lock string) *writerLockChild {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestWriterLockPathsChildProcess$", "--", mode, paths.JSON, paths.JSONC, string(codec), lock)
	root := filepath.Dir(paths.JSON)
	cmd.Dir = root
	cmd.Env = []string{"UAP_TEST_WRITER_LOCK_CHILD=1", "HOME=" + root, "USERPROFILE=" + root, "TMPDIR=" + root, "TMP=" + root, "TEMP=" + root}
	child := &writerLockChild{finished: make(chan struct{})}
	cmd.Stderr = &child.stderr
	stdout, err := cmd.StdoutPipe()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "started" {
		cancel()
		_ = cmd.Wait()
		t.Fatalf("child did not start fixture: %s %s", scanner.Text(), child.stderr.String())
	}
	go func() { _, _ = io.Copy(io.Discard, stdout) }()
	go func() { child.err = cmd.Wait(); close(child.finished) }()
	t.Cleanup(func() { cancel(); <-child.finished })
	return child
}

func assertWriterLockChildBlocked(t *testing.T, child *writerLockChild) {
	t.Helper()
	select {
	case <-child.finished:
		t.Fatalf("independent process bypassed held lock: %v %s", child.err, child.stderr.String())
	case <-time.After(250 * time.Millisecond):
	}
}

func (child *writerLockChild) wait(t *testing.T) {
	t.Helper()
	<-child.finished // The child has its own five-second kill/reap deadline.
	if child.err != nil {
		t.Fatalf("child failed after release: %v %s", child.err, child.stderr.String())
	}
}

func TestWriterLockPathsChildProcess(t *testing.T) {
	if os.Getenv("UAP_TEST_WRITER_LOCK_CHILD") != "1" {
		return
	}
	args := os.Args[len(os.Args)-5:]
	fmt.Println("started")
	var err error
	if args[0] == "write" {
		_, err = New().Apply(Request{Paths: Paths{JSON: args[1], JSONC: args[2]}, Codec: Codec(args[3]), Action: ActionAdd, Name: "TEST-owned", Server: Server{Type: "stdio", Command: "TEST-unexecuted-command"}})
	} else {
		var release func() error
		release, err = lockNativeConfig(args[4])
		if err == nil {
			err = release()
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}
