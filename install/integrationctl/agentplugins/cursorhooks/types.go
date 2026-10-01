// Package cursorhooks plans one owned Cursor user-scope stop observer. All
// functions are pure. The host supplies trusted executable/profile facts and
// performs no-follow I/O, locking, CAS, consent and native qualification.
package cursorhooks

import "errors"

const (
	MaxDocumentBytes = 4 << 20
	MaxDepth         = 64    // Root container counts as one.
	MaxNodes         = 65536 // Containers, object keys and scalar values.
	MaxTokenBytes    = 4096
	MaxArgvBytes     = 16384
	MaxArgs          = 32
)

type Operation string

const (
	Install Operation = "install"
	Update  Operation = "update"
	Remove  Operation = "remove"
	Repair  Operation = "repair"
)

// ShellContract selects a caller-qualified command grammar, not a detected
// shell or a statement of stop/model readiness. Only local Linux user hooks
// have supplied native command evidence. No Windows shell is invented here.
type ShellContract string

const (
	LinuxUserShell32212 ShellContract = "cursor-linux-user-3.22.12-single-quote"
	WindowsUnqualified  ShellContract = "cursor-windows-unqualified"
)

var (
	ErrConflict        = errors.New("cursor hooks conflict")
	ErrUnsupported     = errors.New("cursor hooks unsupported")
	ErrAbsenceUnproven = errors.New("cursor hooks absence_unproven")
)

// HookSpec fixes the entire invocation:
// <Executable> cursor-event stop --binding <Selector>.
// Both paths are explicit bounded absolute Linux paths. No caller-supplied
// native event, timeout, env, model, matcher or additional argv is accepted.
type HookSpec struct {
	Executable string `json:"executable"`
	Selector   string `json:"selector"`
}

type Request struct {
	Document  []byte // Zero length means absent; whitespace-only is malformed.
	Operation Operation
	Specs     []HookSpec // Exactly one, except Remove; optional on Repair.
	Previous  *Receipt
	Shell     ShellContract
	// ExecutableVerified attests the host's current no-follow verification of
	// an owned fixed regular executable and confined selector parents. Pure
	// planning cannot stat files. Required for Install, Update and Repair.
	ExecutableVerified bool
}

// Receipt contains no foreign config bytes. The host binds it to its physical
// profile, path, binding and generation in an existing trusted ledger.
// RemainderDigest proves absence for Repair only; ordinary ownership checks
// deliberately allow legitimate foreign edits.
type Receipt struct {
	Version         int           `json:"version"`
	Event           string        `json:"event"`
	Spec            HookSpec      `json:"spec"`
	Shell           ShellContract `json:"shell"`
	EntryDigest     string        `json:"entry_digest"`
	RemainderDigest string        `json:"remainder_digest"`
}

type Result struct {
	Desired  []byte
	Receipt  *Receipt // Nil after Remove, or on refusal.
	NoOp     bool
	Conflict bool
}
