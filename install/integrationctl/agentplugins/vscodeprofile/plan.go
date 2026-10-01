package vscodeprofile

import (
	"bytes"
	"fmt"
)

// Plan returns caller-owned bytes. Every refusal returns original bytes, no
// authoritative receipt and ErrConflict; it never partially publishes a patch.
func Plan(req Request) (Result, error) {
	digest := SnapshotDigest(req.Settings)
	original := Result{Settings: bytes.Clone(req.Settings), BeforeDigest: digest, AfterDigest: digest}
	if err := validateRequest(req); err != nil {
		return original, conflict(err)
	}
	doc, err := parseSettings(req.Settings, req.Identity.PluginRoot)
	if err != nil {
		return original, conflict(err)
	}
	enabled, present, err := doc.current(req.Identity.PluginRoot)
	if err != nil {
		return original, conflict(err)
	}
	result, err := reconcile(req, doc, enabled, present)
	if err != nil {
		return original, conflict(err)
	}
	if !result.Changed {
		result.Settings = bytes.Clone(req.Settings)
	}
	// Revalidate both input and output budgets. Growing a legal document beyond
	// any ceiling is a refusal, not a partially successful registration.
	if _, err := parseSettings(result.Settings, req.Identity.PluginRoot); err != nil {
		return original, conflict(err)
	}
	result.BeforeDigest = digest
	result.AfterDigest = SnapshotDigest(result.Settings)
	return result, nil
}

func validateRequest(req Request) error {
	if err := validateIdentity(req.Identity); err != nil {
		return err
	}
	switch req.Action {
	case Install:
		if req.Previous != nil {
			if err := validatePrevious(req); err != nil {
				return err
			}
		}
	case Update, Remove, Repair, Enable:
		if err := validatePrevious(req); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported operation")
	}
	if req.Action != Enable && req.EnableSnapshotDigest != "" {
		return fmt.Errorf("enable decision requires separate operation")
	}
	return nil
}

func reconcile(req Request, doc *document, enabled, present bool) (Result, error) {
	root := req.Identity.PluginRoot
	if req.Previous == nil {
		if present {
			return Result{}, fmt.Errorf("unowned location is foreign")
		}
		doc.set(root, true)
		return Result{Settings: doc.ast.Pack(), Changed: true, Receipt: newReceipt(req.Identity, true)}, nil
	}
	if !present {
		return reconcileAbsent(req, doc)
	}
	if !req.Previous.Enabled && enabled {
		return Result{}, fmt.Errorf("disabled owned location was altered")
	}
	if req.Action == Remove {
		doc.remove(root)
		return Result{Settings: doc.ast.Pack(), Changed: true}, nil
	}
	if req.Action == Enable {
		if enabled || req.EnableSnapshotDigest != SnapshotDigest(req.Settings) {
			return Result{}, fmt.Errorf("enable requires the exact disabled snapshot")
		}
		doc.set(root, true)
		return Result{Settings: doc.ast.Pack(), Changed: true, Receipt: newReceipt(req.Identity, true)}, nil
	}
	// Native true -> false is accepted without writing. Persist the false receipt
	// even when no document bytes change so absence repair remains disabled.
	return Result{Receipt: newReceipt(req.Identity, enabled), Disabled: !enabled}, nil
}

func reconcileAbsent(req Request, doc *document) (Result, error) {
	switch req.Action {
	case Remove:
		return Result{}, nil
	case Repair:
		enabled := req.Previous.Enabled
		doc.set(req.Identity.PluginRoot, enabled)
		return Result{Settings: doc.ast.Pack(), Changed: true, Receipt: newReceipt(req.Identity, enabled), Disabled: !enabled}, nil
	default:
		return Result{}, fmt.Errorf("owned location absent; explicit repair required")
	}
}

func conflict(err error) error { return fmt.Errorf("%w: %v", ErrConflict, err) }

// VerifyOwned checks the exact current identity and selector. Native false is
// an accepted disabled transition from true, never evidence to re-enable.
func VerifyOwned(settings []byte, id Identity, receipt *Receipt) (Verification, error) {
	req := Request{Settings: settings, Identity: id, Previous: receipt, Action: Install}
	if err := validateIdentity(id); err != nil {
		return Verification{}, conflict(err)
	}
	if err := validatePrevious(req); err != nil {
		return Verification{}, conflict(err)
	}
	doc, err := parseSettings(settings, id.PluginRoot)
	if err != nil {
		return Verification{}, conflict(err)
	}
	enabled, present, err := doc.current(id.PluginRoot)
	if err != nil {
		return Verification{}, conflict(err)
	}
	if !present || (!receipt.Enabled && enabled) {
		return Verification{}, conflict(fmt.Errorf("owned selector absent or changed"))
	}
	return Verification{Disabled: !enabled}, nil
}
