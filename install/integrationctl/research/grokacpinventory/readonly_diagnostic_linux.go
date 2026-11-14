//go:build linux

package main

import (
	"errors"
	"os"
	"slices"
	"strings"
)

// These are source-owned guard names, never exception text from a native child.
const readonlyDiagnosticCodes = `readonly-helper-unclassified readonly-primitive-refused
readonly-primitive-missing readonly-primitive-permission
ordinary-bwrap-refused bwrap-capability-refused readonly-identity-refused
readonly-directory-refused readonly-directory-join-refused overflow-id-refused
readonly-input-authority readonly-input-directory-read readonly-input-bound
readonly-input-directory-open readonly-input-directory-final readonly-input-directory-changed
readonly-root-owner readonly-overflow-ambiguity readonly-input-proof readonly-file-proof
readonly-file-capability-refused readonly-runtime-root readonly-device readonly-parent-namespace
readonly-result-refused readonly-proof-type-refused readonly-stage-refused readonly-completion-refused
readonly-original-authority-missing readonly-original-authority-changed original-deadline-refused
file-stat-refused absolute-path-refused held-file-refused held-file-changed file-authority-refused
file-read-refused held-content-changed system-ancestor-refused system-executable-refused sealed-copy-refused
mount-authority-refused sealed-directory-refused sealed-directory-changed sealed-mount-changed
sealed-deletion-unproved json-structure-refused json-key-refused json-delimiter-refused json-trailing-refused
json-fields-refused json-field-missing result-isolation-refused result-bound-refused result-identity-refused
result-reason-refused collector-frame-refused strict-cleanup-unproved strict-exchange-missing
input-root-mode-refused sealed-cleanup-refused receipt-output-refused system-selection-refused
ordinary-bwrap-selection-refused`

type readonlyRefusal struct {
	cause error
	code  string
}

func (e *readonlyRefusal) Error() string { return e.code }
func (e *readonlyRefusal) Unwrap() error { return e.cause }

func readonlyPrimitiveRefusal(cause error) error {
	code := "readonly-primitive-refused"
	switch {
	case errors.Is(cause, os.ErrNotExist):
		code = "readonly-primitive-missing"
	case errors.Is(cause, os.ErrPermission):
		code = "readonly-primitive-permission"
	}
	return errors.Join(cause, errors.New(code))
}

func readonlyDiagnostic(mode string, cause error) error {
	if mode != readonlyMode || cause == nil {
		return cause
	}
	code, pending := "readonly-helper-unclassified", []error{cause}
	allowed := strings.Fields(readonlyDiagnosticCodes)
	for count := 0; count < 64 && len(pending) > 0; count++ {
		current := pending[0]
		pending = pending[1:]
		if current == nil {
			continue
		}
		if slices.Contains(allowed, current.Error()) {
			code = current.Error()
			break
		}
		var joined interface{ Unwrap() []error }
		if errors.As(current, &joined) {
			pending = append(joined.Unwrap(), pending...)
		} else if wrapped := errors.Unwrap(current); wrapped != nil {
			pending = append([]error{wrapped}, pending...)
		}
	}
	return &readonlyRefusal{cause: cause, code: code}
}

func readonlyCaptureRefusal(mode string, cause error) error {
	cleanupErr := errors.New("strict-cleanup-unproved")
	var refusal *readonlyRefusal
	if mode == readonlyMode && errors.As(cause, &refusal) && refusal.code != "readonly-helper-unclassified" {
		// Only the collector exchange supplies this closed category. The
		// supervisor's native stderr and cleanup causes remain unpublished.
		return errors.Join(errors.New(refusal.code), cleanupErr)
	}
	return cleanupErr
}

func controllerRefusal(err error) string {
	var selected *readonlyRefusal
	if errors.As(err, &selected) {
		return "grok-acp-controller-refused:" + selected.code
	}
	return "grok-acp-controller-refused"
}
