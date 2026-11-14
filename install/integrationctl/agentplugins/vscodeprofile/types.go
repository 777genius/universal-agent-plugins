// Package vscodeprofile plans one explicitly owned chat.pluginLocations boolean
// in a supplied VS Code profile JSONC snapshot. It performs no I/O and establishes
// registration bytes only, never native availability, trust or activation.
package vscodeprofile

import "errors"

const (
	MaxSettingsBytes = 4 << 20
	MaxSettingsDepth = 64
	// MaxSettingsNodes counts containers, object keys and scalar values.
	MaxSettingsNodes = 65536
	MaxIdentityBytes = 4096
	selector         = "chat.pluginLocations"
)

type Action string

const (
	Install Action = "install"
	Update  Action = "update"
	Remove  Action = "remove"
	Repair  Action = "repair"
	// Enable is separate from repeat/update/repair, and requires the exact
	// current disabled snapshot digest. Consent is the caller's responsibility.
	Enable Action = "enable"
)

var ErrConflict = errors.New("VS Code profile ownership conflict")

// Identity freezes the selected physical settings path/profile and the exact
// projected plugin root/package. Digests cover the complete package/projection
// bytes as established by the caller. No path discovery or physical resolution
// occurs here; supported absolute paths are POSIX or drive-rooted Windows.
type Identity struct {
	SettingsPath     string `json:"settings_path"`
	ProfileID        string `json:"profile_id"`
	PluginRoot       string `json:"plugin_root"`
	PackageID        string `json:"package_id"`
	PackageDigest    string `json:"package_digest"`
	ProjectionDigest string `json:"projection_digest"`
}

// Receipt binds the exact selector/value and complete identity. It contains no
// foreign settings. Its deterministic digest detects drift, not forgery: the
// caller must supply a trusted persisted receipt, never synthesize adoption.
type Receipt struct {
	Version  string   `json:"version"`
	Selector string   `json:"selector"`
	Identity Identity `json:"identity"`
	Enabled  bool     `json:"enabled"`
	Digest   string   `json:"digest"`
}

type Request struct {
	Settings []byte
	Identity Identity
	Action   Action
	Previous *Receipt
	// EnableSnapshotDigest is accepted only by Enable. Use SnapshotDigest on
	// the reviewed disabled preimage; changing even foreign bytes invalidates it.
	EnableSnapshotDigest string
}

type Result struct {
	Settings     []byte
	Receipt      *Receipt
	Changed      bool
	Disabled     bool
	BeforeDigest string
	AfterDigest  string
}

// Verification describes registered bytes, including the accepted native
// true-to-false transition. Absent selectors are not verified owned; Repair is
// the only operation allowed to restore them using a trusted recorded receipt.
type Verification struct{ Disabled bool }
