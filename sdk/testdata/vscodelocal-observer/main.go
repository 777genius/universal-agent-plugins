// This independent TEST executable imports only the public SDK module. Its
// captured IO exercises RunContext, not a copied decoder or process supervisor.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/vscodelocal"
)

type emptyEnv struct{}

func (emptyEnv) LookupEnv(string) (string, bool) { return "", false }

type capturedIO struct {
	body   []byte
	stdout string
	stderr string
	mode   string
}

func (c *capturedIO) ReadStdin(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.mode == "read-error" {
		return nil, errors.New("TEST read failed")
	}
	return c.body, nil
}

func (c *capturedIO) WriteStdout(b []byte) error {
	if c.mode == "write-error" {
		return errors.New("TEST write failed")
	}
	c.stdout += string(b)
	return nil
}

func (c *capturedIO) WriteStderr(s string) error {
	c.stderr += s
	return nil
}

type observation struct {
	Callback       string `json:"callback"`
	Calls          int    `json:"calls"`
	NativeName     string `json:"native_name"`
	Timestamp      string `json:"timestamp"`
	CWD            string `json:"cwd"`
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Known          bool   `json:"known"`
	Active         bool   `json:"active"`
	Eligible       bool   `json:"eligible"`
	AgentID        string `json:"agent_id"`
	AgentType      string `json:"agent_type"`
	Code           int    `json:"code"`
	Stdout         string `json:"stdout"`
	Stderr         string `json:"stderr"`
}

func (o *observation) observe(kind string, common vscodelocal.CommonEvent, active *bool) {
	o.Callback, o.Calls = kind, o.Calls+1
	o.NativeName, o.Timestamp = common.HookEventName, common.Timestamp
	o.CWD, o.SessionID, o.TranscriptPath = common.CWD, common.SessionID, common.TranscriptPath
	o.Known = active != nil
	if active != nil {
		o.Active = *active
	}
	// TEST-only informational admission: the SDK must not coerce absent/true/
	// mismatched/subagent input into an eligible Stop. No delivery happens.
	o.Eligible = kind == "Stop" && common.HookEventName == "Stop" && o.Known && !o.Active
}

func register(app *pluginkitai.App, c *capturedIO, o *observation) {
	app.VSCodeLocal().OnStop(func(e *vscodelocal.StopEvent) *vscodelocal.StopResponse {
		o.observe("Stop", e.CommonInput, e.StopHookActive)
		if c.mode == "panic" {
			panic("TEST callback panic")
		}
		if c.mode == "nil" {
			return nil
		}
		return &vscodelocal.StopResponse{}
	})
	app.VSCodeLocal().OnSubagentStop(func(e *vscodelocal.SubagentStopEvent) *vscodelocal.SubagentStopResponse {
		o.observe("SubagentStop", e.CommonInput, e.StopHookActive)
		o.AgentID, o.AgentType = e.AgentID, e.AgentType
		if c.mode == "nil" {
			return nil
		}
		return &vscodelocal.SubagentStopResponse{}
	})
}

func main() {
	if len(os.Args) != 3 {
		os.Exit(64)
	}
	// A finite TEST outer bound intentionally exceeds the SDK's bound, so the
	// actual SDK decoder must reject oversized injected IO itself.
	body, err := io.ReadAll(io.LimitReader(os.Stdin, 2<<20))
	if err != nil {
		os.Exit(65)
	}
	c := &capturedIO{body: body, mode: os.Args[2]}
	app := pluginkitai.New(pluginkitai.Config{
		Args: []string{"TEST-observer", os.Args[1]}, IO: c, Env: emptyEnv{},
	})
	var o observation
	register(app, c, &o)
	if c.mode == "handler-error" {
		app.Use(func(_ pluginkitai.Next) pluginkitai.Next {
			return func(_ pluginkitai.InvocationContext) pluginkitai.Handled {
				return pluginkitai.Handled{Err: errors.New("TEST middleware failed")}
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	if c.mode == "cancelled" {
		cancel()
	}
	if c.mode == "cursor-runner" {
		o.Code = app.RunCursorObserver(ctx)
	} else {
		o.Code = app.RunContext(ctx)
	}
	cancel()
	o.Stdout, o.Stderr = c.stdout, c.stderr
	if err := json.NewEncoder(os.Stdout).Encode(o); err != nil {
		os.Exit(66)
	}
}
