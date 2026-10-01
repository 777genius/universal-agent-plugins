// Independent public SDK consumer. Synthetic stderr records prove dispatch;
// this executable neither launches Cursor nor delivers notifications.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"time"

	sdk "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/claude"
	"github.com/777genius/plugin-kit-ai/sdk/codex"
	"github.com/777genius/plugin-kit-ai/sdk/cursor"
	"github.com/777genius/plugin-kit-ai/sdk/gemini"
)

func main() {
	mode := os.Args[2]
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
	record := func(kind string, event any) {
		_ = json.NewEncoder(os.Stderr).Encode(struct {
			Kind  string `json:"kind"`
			Event any    `json:"event"`
		}{kind, event})
	}
	app.Cursor().OnStopContext(func(ctx context.Context, e *cursor.StopEvent) (*cursor.StopResponse, error) {
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
	if mode == "cancelled" {
		cancel()
	}
	if mode == "cancel-reading" {
		time.AfterFunc(25*time.Millisecond, cancel)
	}
	if mode == "ordinary" {
		os.Exit(app.RunContext(ctx))
	}
	code := app.RunCursorObserver(ctx)
	// Verify return/cleanup in the process, rather than relying only on os.Exit
	// to remove indefinitely abandoned IO goroutines.
	_ = json.NewEncoder(os.Stderr).Encode(struct {
		Clean bool `json:"clean"`
		Code  int  `json:"code"`
	}{runtime.NumGoroutine() <= baseline && (runtime.GOOS != "linux" || fileCount() <= fdBaseline-2), code})
	os.Exit(code)
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
