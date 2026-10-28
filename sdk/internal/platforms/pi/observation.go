// Package pi implements the bounded, read-only Pi observer process carrier.
package pi

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type LoadedRevision struct {
	ProjectionDigest string `json:"projection_digest"`
	CanonicalDigest  string `json:"canonical_digest"`
	BootstrapVersion int    `json:"bootstrap_version"`
	BindingID        string `json:"binding_id"`
	RuntimeRoot      string `json:"runtime_root"`
	PrimarySHA256    string `json:"primary_sha256"`
}
type Settled struct {
	Aborted         bool   `json:"aborted"`
	BoundaryOutcome string `json:"boundary_outcome,omitempty"`
}
type Prompt struct {
	Kind string `json:"kind"`
}
type ObservationInput struct {
	SchemaVersion  int            `json:"schema_version"`
	Event          string         `json:"event"`
	ObserverID     string         `json:"observer_id"`
	ObservationID  string         `json:"observation_id"`
	SessionID      string         `json:"session_id"`
	CWD            string         `json:"cwd"`
	Mode           string         `json:"mode"`
	PiPID          int            `json:"pi_pid"`
	LoadedRevision LoadedRevision `json:"loaded_revision"`
	Settled        *Settled       `json:"settled,omitempty"`
	Prompt         *Prompt        `json:"prompt,omitempty"`
}
type ObservationOutcome struct{}

func DecodeObservation(envelope runtime.Envelope) (any, string, error) {
	value, err := Decode(envelope.Stdin)
	return value, "observation", err
}

// Decode validates one strict, bounded observer frame. Validation does not
// authenticate its producer or authorize notification delivery.
func Decode(body []byte) (*ObservationInput, error) {
	invalid := func() (*ObservationInput, error) { return nil, fmt.Errorf("invalid Pi observation frame") }
	if len(body) == 0 || len(body) > 65536 || !utf8.Valid(body) {
		return invalid()
	}
	check := json.NewDecoder(bytes.NewReader(body))
	if err := uniqueJSON(check, 0); err != nil {
		return invalid()
	}
	if _, err := check.Token(); err != io.EOF {
		return invalid()
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var value ObservationInput
	if err := decoder.Decode(&value); err != nil {
		return invalid()
	}
	if value.SchemaVersion != 1 || value.PiPID <= 0 || int64(value.PiPID) > 2147483647 || !bounded(value.ObserverID, 128) || !bounded(value.ObservationID, 128) || !bounded(value.SessionID, 256) || !absolute(value.CWD) {
		return invalid()
	}
	switch value.Mode {
	case "tui", "rpc", "json", "print":
	default:
		return invalid()
	}
	revision := value.LoadedRevision
	if revision.BootstrapVersion != 1 || !digest(revision.ProjectionDigest) || !digest(revision.CanonicalDigest) || !bounded(revision.BindingID, 256) || !absolute(revision.RuntimeRoot) || !hash(revision.PrimarySHA256) {
		return invalid()
	}
	switch value.Event {
	case "agent_settled":
		if value.Settled == nil || value.Prompt != nil {
			return invalid()
		}
		// Read only the union member; other top-level values are not all objects.
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(body, &fields)
		var settled map[string]json.RawMessage
		_ = json.Unmarshal(fields["settled"], &settled)
		if raw, exists := settled["aborted"]; !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return invalid()
		}
		switch value.Settled.BoundaryOutcome {
		case "", "completed", "aborted", "error":
		default:
			return invalid()
		}
	case "ui_prompt_start":
		if value.Prompt == nil || value.Settled != nil {
			return invalid()
		}
		switch value.Prompt.Kind {
		case "select", "confirm", "input", "editor", "custom":
		default:
			return invalid()
		}
	default:
		return invalid()
	}
	return &value, nil
}

func bounded(value string, limit int) bool {
	return value != "" && len(value) <= limit && utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
}
func absolute(value string) bool {
	return bounded(value, 4096) && filepath.IsAbs(value) && filepath.Clean(value) == value
}
func hash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}
func digest(value string) bool {
	return strings.HasPrefix(value, "sha256:") && hash(strings.TrimPrefix(value, "sha256:"))
}

func uniqueJSON(decoder *json.Decoder, depth int) error {
	if depth > 16 {
		return fmt.Errorf("JSON depth")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate JSON field")
			}
			seen[name] = true
			if err := uniqueJSON(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := uniqueJSON(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}

func EncodeObservation(value any) runtime.Result {
	if _, ok := value.(ObservationOutcome); !ok {
		return runtime.Result{ExitCode: 1, Stderr: "internal Pi observer response mismatch\n"}
	}
	return runtime.Result{Stdout: []byte("{}")}
}
