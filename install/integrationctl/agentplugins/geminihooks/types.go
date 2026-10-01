// Package geminihooks plans owned singleton command hooks in Gemini CLI USER
// settings. It is pure: hosts provide bytes and a qualified shell, retain the
// receipt in their own ledger, and perform locking, CAS and effective-policy
// checks. It never changes hooksConfig or trust/security settings.
package geminihooks

import "errors"

type Shell string

const (
	Bash       Shell = "bash"
	PowerShell Shell = "powershell"
)

type Operation string

const (
	Install Operation = "install"
	Update  Operation = "update"
	Remove  Operation = "remove"
	// MaxSettingsBytes bounds parsing and copying of caller-provided settings.
	MaxSettingsBytes = 4 << 20
)

// ErrConflict identifies unsupported input, ambiguity or lost ownership.
var ErrConflict = errors.New("Gemini hooks conflict")

// HookSpec describes one named command in one singleton native event group.
// Argv includes the executable. Timeout is milliseconds; zero omits the field.
type HookSpec struct {
	Event, Name string
	Argv        []string
	Matcher     string
	Timeout     int
}

type Request struct {
	Settings  []byte // Zero length means absent; whitespace-only is malformed.
	Shell     Shell
	Operation Operation
	Hooks     []HookSpec // Complete desired owned set; ignored for Remove.
	Previous  *Receipt
}

// OwnedGroup stores selectors and a digest, never source settings or argv.
type OwnedGroup struct {
	Event  string `json:"event"`
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// Receipt is scoped by the host to its explicit settings path. It is ownership
// evidence, not an authentication credential or proof of effective activation.
type Receipt struct {
	Version int          `json:"version"`
	Groups  []OwnedGroup `json:"groups"`
}

type Result struct {
	Desired        []byte
	Receipt        *Receipt
	NoOp, Conflict bool
}
