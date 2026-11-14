// Independent public SDK consumer. Synthetic stderr records prove dispatch;
// this executable neither launches Cursor nor delivers notifications.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	sdk "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/claude"
	"github.com/777genius/plugin-kit-ai/sdk/codex"
	"github.com/777genius/plugin-kit-ai/sdk/cursor"
	"github.com/777genius/plugin-kit-ai/sdk/gemini"
)

func main() {
	mode := os.Args[2]
	if mode == "owned-broken" || mode == "low-fd-broken" {
		os.Exit(ownedBroken(mode == "low-fd-broken"))
	}
	// Prime Go's shared poller before the descriptor baseline. Its epoll/eventfd
	// handles are runtime resources, not observer leaks; owned pipe handles must
	// still disappear (including both originals) after the runner returns.
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	_ = r.Close()
	_ = w.Close()
	baseline := runtime.NumGoroutine()
	fdBaseline := fileCount()
	cfg := sdk.Config{}
	if mode == "owned" {
		cfg.IO = sdk.NewCursorObserverPipeIO(os.Stdin, os.Stdout)
	}
	app := sdk.New(cfg)
	var callbackActive atomic.Int32
	var observerReturned, lateCallback atomic.Bool
	callbackEntered := func() {
		callbackActive.Add(1)
		if observerReturned.Load() {
			lateCallback.Store(true)
		}
	}
	record := func(kind string, event any) {
		_ = json.NewEncoder(os.Stderr).Encode(struct {
			Kind  string `json:"kind"`
			Event any    `json:"event"`
		}{kind, event})
	}
	app.Cursor().OnStopContext(func(ctx context.Context, e *cursor.StopEvent) (*cursor.StopResponse, error) {
		callbackEntered()
		defer callbackActive.Add(-1)
		record("cursor", e)
		switch mode {
		case "panic":
			panic("private callback content")
		case "error":
			return nil, errors.New("private callback error")
		case "cooperative":
			<-ctx.Done()
			return nil, ctx.Err()
		case "nil":
			return nil, nil
		}
		return &cursor.StopResponse{}, nil
	})
	if mode == "simple" {
		app.Cursor().OnStop(func(e *cursor.StopEvent) *cursor.StopResponse {
			callbackEntered()
			defer callbackActive.Add(-1)
			record("cursor", e)
			return nil
		})
	}
	app.Claude().OnStop(func(e *claude.StopEvent) *claude.Response { record("claude-stop", e); return claude.Allow() })
	app.Claude().OnNotification(func(e *claude.NotificationEvent) *claude.NotificationResponse {
		record("claude-notification", e)
		return nil
	})
	app.Claude().OnPreToolUse(func(e *claude.PreToolUseEvent) *claude.PreToolResponse {
		record("claude-permission", e)
		return claude.PreToolBlockExit2("fixture denial")
	})
	app.Codex().OnStop(func(e *codex.StopEvent) *codex.Response { record("codex", e); return nil })
	app.Gemini().OnAfterAgent(func(e *gemini.AfterAgentEvent) *gemini.AfterAgentResponse {
		record("gemini-agent", e)
		return gemini.AfterAgentContinue()
	})
	app.Gemini().OnNotification(func(e *gemini.NotificationEvent) *gemini.NotificationResponse {
		record("gemini-notification", e)
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	if mode == "canceled" || mode == "cancelled" {
		cancel()
	}
	if mode == "cancel-reading" {
		time.AfterFunc(25*time.Millisecond, cancel)
	}
	if mode == "ordinary" {
		os.Exit(app.RunContext(ctx))
	}
	code := app.RunCursorObserver(ctx)
	observerReturned.Store(true)
	// Owned descriptors and the registered callback must be finished immediately;
	// waiting for incidental context timer tails must not hide either violation.
	joined := callbackActive.Load() == 0
	closed := runtime.GOOS != "linux" || fileCount() <= fdBaseline-2
	if mode == "leak-probe" {
		// Self-check the observation deadline in this disposable process only.
		go func() { select {} }()
	}
	quiet, stacks := observerQuiescent(baseline)
	_ = json.NewEncoder(os.Stderr).Encode(struct {
		Clean  bool   `json:"clean"`
		Code   int    `json:"code"`
		Stacks string `json:"stacks,omitempty"`
	}{closed && joined && !lateCallback.Load() && quiet, code, stacks})
	os.Exit(code)
}

// RunCursorObserver joins owned work, but context's timer/AfterFunc goroutine
// can still be exiting after its final operation. Observe global quiescence
// within a failure deadline; persistent goroutines still fail with stack evidence.
func observerQuiescent(baseline int) (bool, string) {
	deadline := time.NewTimer(100 * time.Millisecond)
	defer deadline.Stop()
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for {
		if runtime.NumGoroutine() <= baseline {
			return true, ""
		}
		select {
		case <-deadline.C:
			buf := make([]byte, 64<<10)
			return false, string(buf[:runtime.Stack(buf, true)])
		case <-poll.C:
		}
	}
}

func ownedBroken(closeStdio bool) int {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	in, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	reader, out, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	if _, err := writer.Write(input); err != nil {
		panic(err)
	}
	_ = writer.Close()
	_ = reader.Close()
	if closeStdio {
		_ = os.Stdin.Close()
		_ = os.Stdout.Close()
	}
	baseline, fdBaseline := runtime.NumGoroutine(), fileCount()
	app := sdk.New(sdk.Config{Args: []string{"TEST", "CursorStop"}, IO: sdk.NewCursorObserverPipeIO(in, out)})
	app.Cursor().OnStop(func(e *cursor.StopEvent) *cursor.StopResponse {
		_ = json.NewEncoder(os.Stderr).Encode(struct {
			Kind  string            `json:"kind"`
			Event *cursor.StopEvent `json:"event"`
		}{"cursor", e})
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	code := app.RunCursorObserver(ctx)
	_ = json.NewEncoder(os.Stderr).Encode(struct {
		Clean bool `json:"clean"`
		Code  int  `json:"code"`
	}{runtime.NumGoroutine() <= baseline && (runtime.GOOS != "linux" || fileCount() == fdBaseline-2), code})
	return code
}

func fileCount() int {
	if runtime.GOOS != "linux" {
		return 0
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		panic(err)
	}
	return len(entries)
}
