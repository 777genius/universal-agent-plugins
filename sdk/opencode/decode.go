// Package opencode decodes the neutral OpenCode observer wire. It does not
// implement OpenCode's native event types or any notification policy.
package opencode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	Version  = 1
	MaxBytes = 4096
)

type Kind string

const (
	TurnIdleVerified Kind = "turn_idle_verified"
	QuestionAsked     Kind = "question_asked"
	PermissionAsked   Kind = "permission_asked"
	TerminalError     Kind = "terminal_error"
	Unknown           Kind = "unknown"
)

type ObservedEvent struct {
	Version    int    `json:"version"`
	Kind       Kind   `json:"kind"`
	SessionID  string `json:"sessionID,omitempty"`
	TurnID     string `json:"turnID,omitempty"`
	MessageID  string `json:"messageID,omitempty"`
	RequestID  string `json:"requestID,omitempty"`
	NativeType string `json:"nativeType,omitempty"`
	RootSession *bool  `json:"rootSession,omitempty"`
}

func validID(s string) bool { return len(s) > 0 && len(s) <= 256 }

// Decode accepts additive fields and unfamiliar event kinds within wire v1.
// Consumers must ignore Unknown; a changed required shape needs a new version.
func Decode(data []byte) (ObservedEvent, error) {
	var e ObservedEvent
	if len(data) == 0 || len(data) > MaxBytes {
		return e, errors.New("opencode: invalid wire length")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if err := d.Decode(&e); err != nil {
		return e, fmt.Errorf("opencode: decode: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return e, errors.New("opencode: trailing data")
	}
	if e.Version != Version {
		return e, errors.New("opencode: unsupported version")
	}
	if e.Kind == "" {
		return e, errors.New("opencode: missing kind")
	}
	for _, v := range []string{e.SessionID, e.TurnID, e.MessageID, e.RequestID} {
		if len(v) > 256 {
			return e, errors.New("opencode: oversized ID")
		}
	}
	switch e.Kind {
	case TurnIdleVerified:
		if !validID(e.SessionID) || !validID(e.TurnID) || !validID(e.MessageID) || e.RequestID != "" || e.RootSession == nil {
			return e, errors.New("opencode: invalid completion")
		}
	case QuestionAsked, PermissionAsked:
		if !validID(e.SessionID) || !validID(e.TurnID) || !validID(e.RequestID) || e.MessageID != "" || e.RootSession == nil {
			return e, errors.New("opencode: invalid request")
		}
	case TerminalError:
		if !validID(e.SessionID) || !validID(e.TurnID) || e.RequestID != "" || e.MessageID != "" || e.RootSession == nil {
			return e, errors.New("opencode: invalid error")
		}
	case Unknown:
		if e.NativeType == "" || len(e.NativeType) > 80 {
			return e, errors.New("opencode: invalid native type")
		}
	default:
		e.Kind = Unknown
	}
	return e, nil
}
