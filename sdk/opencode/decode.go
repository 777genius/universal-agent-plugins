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
	QuestionAsked    Kind = "question_asked"
	PermissionAsked  Kind = "permission_asked"
	TerminalError    Kind = "terminal_error"
	Unknown          Kind = "unknown"
)

// Provenance is additive private candidate evidence. NativeTime is epoch
// milliseconds; a V1 lower bound is not a request birth timestamp. Consumers
// still need qualified clock mapping, runtime eligibility and durable admission.
type Provenance struct {
	Generation      string `json:"generation"`
	ObservationID   string `json:"observationID"`
	NativeEventID   string `json:"nativeEventID,omitempty"`
	NativeMessageID string `json:"nativeMessageID,omitempty"`
	NativeTime      int64  `json:"nativeTime"`
	TimeBasis       string `json:"timeBasis"`
}

type ObservedEvent struct {
	Version     int         `json:"version"`
	Kind        Kind        `json:"kind"`
	SessionID   string      `json:"sessionID,omitempty"`
	TurnID      string      `json:"turnID,omitempty"`
	MessageID   string      `json:"messageID,omitempty"`
	RequestID   string      `json:"requestID,omitempty"`
	NativeType  string      `json:"nativeType,omitempty"`
	RootSession *bool       `json:"rootSession,omitempty"`
	Provenance  *Provenance `json:"provenance,omitempty"`
}

func validID(s string) bool {
	if len(s) == 0 || len(s) > 256 {
		return false
	}
	for i := range s {
		if s[i] < 32 {
			return false
		}
	}
	return true
}

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
	if p := e.Provenance; p != nil {
		if p.NativeTime <= 0 || p.NativeTime > 9007199254740991 || len(p.ObservationID) == 0 || len(p.ObservationID) > 2048 ||
			(p.NativeEventID != "" && !validID(p.NativeEventID)) ||
			(p.NativeMessageID != "" && !validID(p.NativeMessageID)) {
			return e, errors.New("opencode: invalid provenance")
		}
		if p.Generation == "v2" {
			if p.TimeBasis != "envelope_created" || !validID(p.NativeEventID) {
				return e, errors.New("opencode: invalid v2 provenance")
			}
		} else if p.Generation == "v1" {
			if e.Kind == TerminalError && !validID(p.NativeMessageID) {
				return e, errors.New("opencode: missing native terminal message")
			}
			if p.TimeBasis != "assistant_created_lower_bound" && p.TimeBasis != "assistant_completed" {
				return e, errors.New("opencode: invalid v1 provenance")
			}
			if (e.Kind == TurnIdleVerified && p.TimeBasis != "assistant_completed") ||
				((e.Kind == QuestionAsked || e.Kind == PermissionAsked || e.Kind == TerminalError) && p.TimeBasis != "assistant_created_lower_bound") {
				return e, errors.New("opencode: native time basis contradicts fact")
			}
		} else {
			return e, errors.New("opencode: invalid native generation")
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
