package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/mcpruntime"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
)

func TestDevSessionLockConflictsDeterministicallyAndReleases(t *testing.T) {
	cache, root := t.TempDir(), t.TempDir()
	for _, name := range []string{"HOME", "LOCALAPPDATA", "XDG_CACHE_HOME"} {
		t.Setenv(name, cache)
	}
	t.Setenv("TMPDIR", t.TempDir())
	release, err := acquireDevSession(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	// A second process can have a different operation scratch/TMPDIR. The
	// canonical per-user lock must still serialize the same project.
	t.Setenv("TMPDIR", t.TempDir())
	if _, err := acquireDevSession(context.Background(), filepath.Join(root, ".")); runtimeErrorCode(err) != "runtime_dev_session_conflict" {
		t.Fatalf("second session diagnostic = %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("release session: %v", err)
	}
	releaseAgain, err := acquireDevSession(context.Background(), root)
	if err != nil {
		t.Fatalf("released session remained locked: %v", err)
	}
	if err := releaseAgain(); err != nil {
		t.Fatalf("release reacquired session: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(cache, "agentplugins", "author-dev-locks"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one stable private lock file: %v %v", entries, err)
	}
}

func TestDevSessionLockPathFailsClosedWithoutStableCache(t *testing.T) {
	if _, err := devSessionLockPath("relative-cache", t.TempDir()); runtimeErrorCode(err) != "runtime_dev_session_lock_unavailable" {
		t.Fatalf("relative per-user cache accepted: %v", err)
	}
}

func TestDevPendingDebouncesAndBoundsContinuousChanges(t *testing.T) {
	start := time.Unix(100, 0)
	first := project.Result{}
	first.Input.Identity.TreeDigest = "first"
	second := project.Result{}
	second.Input.Identity.TreeDigest = "second"
	var pending devPending
	pending.observe(first, start)
	if pending.ready(start.Add(devDebounce - time.Millisecond)) {
		t.Fatal("change became ready before the quiet debounce")
	}
	pending.observe(second, start.Add(devDebounce-time.Millisecond))
	if pending.ready(start.Add(2*devDebounce - 2*time.Millisecond)) {
		t.Fatal("second change was not coalesced")
	}
	if !pending.ready(start.Add(2 * devDebounce)) {
		t.Fatal("quiet coalesced change never became ready")
	}
	if got := pending.take().Input.Identity.TreeDigest; got != "second" || pending.set {
		t.Fatalf("coalesced digest=%q pending=%+v", got, pending)
	}
	for elapsed := time.Duration(0); elapsed < devMaxCoalesce; elapsed += devDebounce / 2 {
		pending.observe(second, start.Add(elapsed))
	}
	if !pending.ready(start.Add(devMaxCoalesce)) {
		t.Fatal("continuous edits exceeded the maximum coalescing window")
	}
}

func TestContinuousDevReportsFailuresAndKeepsWatching(t *testing.T) {
	cache := t.TempDir()
	for _, name := range []string{"HOME", "LOCALAPPDATA", "XDG_CACHE_HOME"} {
		t.Setenv(name, cache)
	}
	var runtimeFails atomic.Bool
	runtimeFails.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if runtimeFails.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if req.Header.Get("Mcp-Protocol-Version") == "" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"fixture","version":"1"}}}`)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	root := t.TempDir()
	plugin := filepath.Join(root, "plugin.json")
	writePlugin := func(marker string) {
		t.Helper()
		body := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"dev-fixture","extensions":{"dev.test":{"marker":"` + marker + `"}}}`
		if err := os.WriteFile(plugin, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writePlugin("initial")
	mcp := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"selected":{"type":"streamable-http","url":"` + server.URL + `"}}}`
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(mcp), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var codes []string
	cycles := 0
	var human bytes.Buffer
	app := App{Projects: project.Service{Scratch: t.TempDir()}, Revision: "dev-test"}
	_, err := app.dev(ctx, request{root: root, server: "selected", allowNetwork: true, deadline: time.Second,
		cycleOutput: func(r report.Report, cycleErr error) error {
			if err := writePrivateHuman(&human, r); err != nil {
				return err
			}
			cycles++
			codes = append(codes, runtimeErrorCode(cycleErr))
			switch cycles {
			case 1:
				runtimeFails.Store(false)
				writePlugin("runtime-recovered")
			case 2:
				if err := os.WriteFile(plugin, []byte(`{"broken":`), 0600); err != nil {
					return err
				}
			case 3:
				writePlugin("read-recovered")
			case 4:
				cancel()
			}
			return nil
		}})
	if !errors.Is(err, context.Canceled) || cycles != 4 {
		t.Fatalf("continuous dev stopped early: cycles=%d codes=%v err=%v", cycles, codes, err)
	}
	if codes[0] != "runtime_http_status_failed" || codes[1] != "" || codes[2] == "" || codes[3] != "" {
		t.Fatalf("unexpected cycle diagnostics: %v", codes)
	}
	if strings.Count(human.String(), "dev: readiness") != 4 || strings.Contains(human.String(), root) || strings.Contains(human.String(), cache) {
		t.Fatalf("continuous human cycle output was missing or unsafe: %s", human.String())
	}
}

func TestContinuousDevOutputFailureCancelsAndJoinsActiveCycle(t *testing.T) {
	cache := t.TempDir()
	for _, name := range []string{"HOME", "LOCALAPPDATA", "XDG_CACHE_HOME"} {
		t.Setenv(name, cache)
	}
	cycleStarted := make(chan struct{})
	cycleReturned := make(chan struct{})

	root := t.TempDir()
	plugin := filepath.Join(root, "plugin.json")
	if err := os.WriteFile(plugin, []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"dev-output-fixture"}`), 0600); err != nil {
		t.Fatal(err)
	}
	mcp := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"selected":{"type":"streamable-http","url":"https://example.test/mcp"}}}`
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(mcp), 0600); err != nil {
		t.Fatal(err)
	}
	writeErr := make(chan error, 1)
	go func() {
		<-cycleStarted
		writeErr <- os.WriteFile(plugin, []byte(`{"broken":`), 0600)
	}()

	scratch := t.TempDir()
	outputFailure := errors.New("synthetic cycle output failure")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	app := App{Projects: project.Service{Scratch: scratch}, Revision: "dev-output-test"}
	_, err := app.dev(ctx, request{
		root: root, server: "selected", allowNetwork: true, deadline: 5 * time.Second,
		cycleOutput: func(report.Report, error) error { return outputFailure },
		runMCP: func(ctx context.Context, _ mcpruntime.Options) (mcpruntime.Evidence, error) {
			close(cycleStarted)
			<-ctx.Done()
			close(cycleReturned)
			return mcpruntime.Evidence{}, ctx.Err()
		},
	})
	if writeFailure := <-writeErr; writeFailure != nil {
		t.Fatal(writeFailure)
	}
	if !errors.Is(err, outputFailure) {
		t.Fatalf("output failure was not preserved: %v", err)
	}
	select {
	case <-cycleReturned:
	default:
		t.Fatal("runtime cycle continued after dev returned")
	}
	if entries, readErr := os.ReadDir(scratch); readErr != nil || len(entries) != 0 {
		t.Fatalf("active runtime cycle was not joined and cleaned: %v %v", entries, readErr)
	}
	release, lockErr := acquireDevSession(context.Background(), root)
	if lockErr != nil {
		t.Fatalf("project lock remained held after joined cycle: %v", lockErr)
	}
	if releaseErr := release(); releaseErr != nil {
		t.Fatalf("release reacquired project lock: %v", releaseErr)
	}
}

func TestContinuousDevMalformedEditJoinsLongRunningCycleBeforeReport(t *testing.T) {
	cache := t.TempDir()
	for _, name := range []string{"HOME", "LOCALAPPDATA", "XDG_CACHE_HOME"} {
		t.Setenv(name, cache)
	}
	root := t.TempDir()
	plugin := filepath.Join(root, "plugin.json")
	if err := os.WriteFile(plugin, []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"dev-invalidation-fixture"}`), 0600); err != nil {
		t.Fatal(err)
	}
	mcp := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"selected":{"type":"streamable-http","url":"https://example.test/mcp"}}}`
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(mcp), 0600); err != nil {
		t.Fatal(err)
	}

	cycleStarted := make(chan struct{})
	cycleJoined := make(chan struct{})
	writeDone := make(chan error, 1)
	go func() {
		<-cycleStarted
		if err := os.WriteFile(plugin, nil, 0600); err != nil {
			writeDone <- err
			return
		}
		// Model an editor's non-atomic truncate/write sequence long enough for
		// the watcher to observe the transient empty file.
		time.Sleep(2 * devPollInterval)
		writeDone <- os.WriteFile(plugin, []byte(`{"broken":`), 0600)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var activeChildren atomic.Int32
	var reports atomic.Int32
	app := App{Projects: project.Service{Scratch: t.TempDir()}, Revision: "dev-invalidation-test"}
	_, err := app.dev(ctx, request{
		root: root, server: "selected", allowNetwork: true, deadline: 5 * time.Second,
		cycleOutput: func(r report.Report, cycleErr error) error {
			if reports.Add(1) != 1 {
				return errors.New("late stale runtime report")
			}
			select {
			case <-cycleJoined:
			default:
				return errors.New("invalid-cycle report preceded runtime join")
			}
			if activeChildren.Load() != 0 {
				return errors.New("runtime child remained active at invalid-cycle report")
			}
			if cycleErr == nil || r.Error == nil {
				return errors.New("malformed edit did not produce an invalid-cycle report")
			}
			cancel()
			return nil
		},
		runMCP: func(ctx context.Context, _ mcpruntime.Options) (mcpruntime.Evidence, error) {
			activeChildren.Add(1)
			close(cycleStarted)
			<-ctx.Done()
			activeChildren.Add(-1)
			close(cycleJoined)
			return mcpruntime.Evidence{Transport: "streamable_http"}, ctx.Err()
		},
	})
	if writeErr := <-writeDone; writeErr != nil {
		t.Fatal(writeErr)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("continuous dev result=%v", err)
	}
	time.Sleep(2 * devPollInterval)
	if got := reports.Load(); got != 1 {
		t.Fatalf("reports=%d, want only the invalid cycle", got)
	}
	if got := activeChildren.Load(); got != 0 {
		t.Fatalf("active runtime children=%d after dev return", got)
	}
}

func TestContinuousDevRestartsAfterIdenticalContentRecovery(t *testing.T) {
	cache := t.TempDir()
	for _, name := range []string{"HOME", "LOCALAPPDATA", "XDG_CACHE_HOME"} {
		t.Setenv(name, cache)
	}
	root := t.TempDir()
	plugin := filepath.Join(root, "plugin.json")
	original := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"dev-identical-recovery-fixture"}`)
	if err := os.WriteFile(plugin, original, 0600); err != nil {
		t.Fatal(err)
	}
	mcp := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"selected":{"type":"streamable-http","url":"https://example.test/mcp"}}}`
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(mcp), 0600); err != nil {
		t.Fatal(err)
	}

	firstStarted := make(chan struct{})
	firstJoined := make(chan struct{})
	restarted := make(chan struct{})
	restartJoined := make(chan struct{})
	writeDone := make(chan error, 1)
	go func() {
		<-firstStarted
		writeDone <- os.WriteFile(plugin, []byte(`{"broken":`), 0600)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		<-restarted
		cancel()
	}()
	var activeChildren atomic.Int32
	var reports atomic.Int32
	var starts atomic.Int32
	scratch := t.TempDir()
	app := App{Projects: project.Service{Scratch: scratch}, Revision: "dev-identical-recovery-test"}
	_, err := app.dev(ctx, request{
		root: root, server: "selected", allowNetwork: true, deadline: 5 * time.Second,
		cycleOutput: func(r report.Report, cycleErr error) error {
			if reports.Add(1) != 1 {
				return errors.New("late stale runtime report")
			}
			select {
			case <-firstJoined:
			default:
				return errors.New("invalid-cycle report preceded runtime join")
			}
			if activeChildren.Load() != 0 {
				return errors.New("runtime child remained active at invalid-cycle report")
			}
			if cycleErr == nil || r.Error == nil {
				return errors.New("malformed edit did not produce an invalid-cycle report")
			}
			return os.WriteFile(plugin, original, 0600)
		},
		runMCP: func(ctx context.Context, _ mcpruntime.Options) (mcpruntime.Evidence, error) {
			start := starts.Add(1)
			if activeChildren.Add(1) != 1 {
				return mcpruntime.Evidence{}, errors.New("overlapping runtime children")
			}
			switch start {
			case 1:
				close(firstStarted)
			case 2:
				close(restarted)
			default:
				return mcpruntime.Evidence{}, errors.New("unexpected extra runtime start")
			}
			<-ctx.Done()
			activeChildren.Add(-1)
			if start == 1 {
				close(firstJoined)
			} else {
				close(restartJoined)
			}
			return mcpruntime.Evidence{Transport: "streamable_http"}, ctx.Err()
		},
	})
	if writeErr := <-writeDone; writeErr != nil {
		t.Fatal(writeErr)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("continuous dev result=%v", err)
	}
	if got := starts.Load(); got != 2 {
		t.Fatalf("runtime starts=%d, want initial and identical-content recovery", got)
	}
	select {
	case <-restartJoined:
	default:
		t.Fatal("restarted runtime was not canceled and joined")
	}
	if got := reports.Load(); got != 1 {
		t.Fatalf("reports=%d, want only the invalid cycle", got)
	}
	if got := activeChildren.Load(); got != 0 {
		t.Fatalf("active runtime children=%d after dev return", got)
	}
	got, readErr := os.ReadFile(plugin)
	if readErr != nil || !bytes.Equal(got, original) {
		t.Fatalf("plugin was not restored byte-identically: %q %v", got, readErr)
	}
	if entries, readErr := os.ReadDir(scratch); readErr != nil || len(entries) != 0 {
		t.Fatalf("runtime scratch was not clean after cancellation: %v %v", entries, readErr)
	}
}
