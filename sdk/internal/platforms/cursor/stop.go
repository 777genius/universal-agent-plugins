package cursor

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// StopStatus retains unknown native strings without coercing them to success.
type StopStatus string

const (
	StopStatusCompleted StopStatus = "completed"
	StopStatusAborted   StopStatus = "aborted"
	StopStatusError     StopStatus = "error"
)

type ModelParam struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type StopInput struct {
	ConversationID string       `json:"conversation_id"`
	GenerationID   string       `json:"generation_id"`
	HookEventName  string       `json:"hook_event_name"`
	CursorVersion  string       `json:"cursor_version"`
	WorkspaceRoots []string     `json:"workspace_roots"`
	Model          string       `json:"model"`
	ModelID        string       `json:"model_id,omitempty"`
	ModelParams    []ModelParam `json:"model_params,omitempty"`
	UserEmail      *string      `json:"user_email"`
	TranscriptPath *string      `json:"transcript_path"`
	Status         StopStatus   `json:"status"`
	LoopCount      *int         `json:"loop_count"`
}

type StopOutcome struct{}

func DecodeStop(env runtime.Envelope) (any, string, error) {
	dto, err := runtime.DecodeJSONPayload[StopInput](env.Stdin, "Cursor stop input")
	if err != nil {
		return nil, "", err
	}
	if err := validateStop(dto); err != nil {
		return nil, "", err
	}
	return dto, "stop", nil
}

func validateStop(dto *StopInput) error {
	if dto.HookEventName != "stop" {
		return fmt.Errorf("Cursor observer requires native stop event")
	}
	if !validID(dto.ConversationID) || !validID(dto.GenerationID) {
		return fmt.Errorf("Cursor stop IDs must be nonempty, at most 256 UTF-8 bytes and contain no controls")
	}
	if dto.LoopCount != nil && (*dto.LoopCount < 0 || int64(*dto.LoopCount) > 2147483647) {
		return fmt.Errorf("Cursor stop loop_count outside observer admission range")
	}
	return nil
}

func validID(id string) bool {
	if len(id) == 0 || len(id) > 256 || !utf8.ValidString(id) {
		return false
	}
	for _, r := range id {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func EncodeStop(v any) runtime.Result {
	if _, ok := v.(StopOutcome); !ok {
		return runtime.Result{ExitCode: 1, Stderr: "internal hook type mismatch: Cursor stop outcome\n"}
	}
	return runtime.Result{ExitCode: 0, Stdout: []byte("{}")}
}
