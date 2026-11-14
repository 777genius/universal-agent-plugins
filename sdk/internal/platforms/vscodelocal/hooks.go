// Package vscodelocal implements the VS Code Local hook observer wire subset.
package vscodelocal

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

// CommonInput follows the Local wire, independently of other Copilot harnesses.
// Timestamp is retained as a string; semantic admission belongs to the consumer.
type CommonInput struct {
	Timestamp      string `json:"timestamp"`
	HookEventName  string `json:"hook_event_name"`
	CWD            string `json:"cwd,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	TranscriptPath string `json:"transcript_path,omitempty"`
}

type StopInput struct {
	CommonInput
	StopHookActive *bool `json:"stop_hook_active"`
}

type SubagentStopInput struct {
	CommonInput
	StopHookActive *bool  `json:"stop_hook_active"`
	AgentID        string `json:"agent_id"`
	AgentType      string `json:"agent_type"`
}

// ObserverOutcome deliberately has no native decision or context fields.
type ObserverOutcome struct{}

func DecodeStop(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[StopInput](env.Stdin, "VS Code Local Stop input")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func DecodeSubagentStop(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[SubagentStopInput](env.Stdin, "VS Code Local SubagentStop input")
	if err != nil {
		return nil, "", err
	}
	return dto, dto.HookEventName, nil
}

func EncodeObserver(v any) runtime.Result {
	if _, ok := v.(ObserverOutcome); !ok {
		return runtime.Result{ExitCode: 1, Stderr: "internal hook type mismatch: VS Code Local observer outcome\n"}
	}
	return runtime.Result{ExitCode: 0, Stdout: []byte("{}")}
}
