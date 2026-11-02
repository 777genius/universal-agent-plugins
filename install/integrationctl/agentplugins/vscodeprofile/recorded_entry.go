package vscodeprofile

import (
	"bytes"
	"errors"
	"fmt"
)

// ErrRecordedEntryAbsent distinguishes a parsed missing selector from malformed
// bytes, nonboolean values and forbidden transitions. It supplies no ownership.
var ErrRecordedEntryAbsent = errors.New("recorded entry absent")

// VerifyRecordedEntry returns a receipt from the actual native snapshot. Its
// caller must independently prove ownership before supplying the recorded bool.
// This parser-only operation neither adopts entries nor establishes filesystem
// identity. A native true-to-false transition is accepted; absence never is.
func VerifyRecordedEntry(settings []byte, id Identity, recordedEnabled bool) (Result, error) {
	digest := SnapshotDigest(settings)
	original := Result{Settings: bytes.Clone(settings), BeforeDigest: digest, AfterDigest: digest}
	if err := validateIdentity(id); err != nil {
		return original, conflict(err)
	}
	doc, err := parseSettings(settings, id.PluginRoot)
	if err != nil {
		return original, conflict(err)
	}
	enabled, present, err := doc.current(id.PluginRoot)
	if err != nil {
		return original, conflict(err)
	}
	if !present {
		return original, fmt.Errorf("%w: %w", ErrConflict, ErrRecordedEntryAbsent)
	}
	if !recordedEnabled && enabled {
		return original, conflict(fmt.Errorf("recorded entry absent or changed"))
	}
	original.Receipt = newReceipt(id, enabled)
	original.Disabled = !enabled
	return original, nil
}
