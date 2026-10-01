package cursorhooks

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tailscale/hujson"
)

// Plan returns an independent desired document on success. No-op/refusal bytes
// equal the input exactly; oversized refused input is borrowed to avoid copying
// beyond the budget. Every ownership/collision check precedes private AST edits.
func Plan(req Request) (Result, error) {
	unchanged := req.Document
	if len(unchanged) <= MaxDocumentBytes {
		unchanged = bytes.Clone(unchanged)
	}
	fail := func(err error) (Result, error) {
		return Result{Desired: unchanged, Conflict: true}, fmt.Errorf("%w: %w", ErrConflict, err)
	}
	if req.Operation != Install && req.Operation != Update && req.Operation != Remove && req.Operation != Repair {
		return fail(fmt.Errorf("unsupported operation"))
	}
	d, err := parseDocument(req.Document)
	if err != nil {
		return fail(err)
	}
	old, loc, err := previous(req, d)
	if err != nil {
		return fail(err)
	}
	entry, spec, err := requested(req, old)
	if err != nil {
		return fail(err)
	}
	if err := collision(d, entry, loc, req.Operation); err != nil {
		return fail(err)
	}
	if req.Operation != Remove && loc != nil && valueDigest("stop-entry", entry) == req.Previous.EntryDigest {
		return Result{Desired: unchanged, Receipt: receiptFor(req.Shell, spec, entry, d, loc), NoOp: true}, nil
	}
	if req.Operation == Install && req.Previous != nil {
		return fail(fmt.Errorf("changed specification requires Update"))
	}
	apply(d, req.Operation, loc, entry)
	body, err := d.render()
	if err != nil {
		return fail(err)
	}
	result := Result{Desired: body}
	if req.Operation != Remove {
		newLoc := matches(d, commandIdentity(entry))[0]
		result.Receipt = receiptFor(req.Shell, spec, entry, d, &newLoc)
	}
	return result, nil
}

func previous(req Request, d *document) (hujson.Value, *location, error) {
	if req.Previous == nil && req.Operation == Install {
		return hujson.Value{}, nil, nil
	}
	entry, err := validateReceipt(req.Previous)
	if err != nil {
		return entry, nil, err
	}
	loc, err := ownedLocation(d, req.Previous, entry)
	if req.Operation != Repair {
		return entry, loc, err
	}
	if err != nil && !errors.Is(err, ErrAbsenceUnproven) {
		return entry, nil, err
	}
	if remainderDigest(d, loc) != req.Previous.RemainderDigest {
		return entry, nil, ErrAbsenceUnproven
	}
	return entry, loc, nil
}

func requested(req Request, old hujson.Value) (hujson.Value, HookSpec, error) {
	if req.Operation == Remove {
		return hujson.Value{}, HookSpec{}, nil
	}
	if !req.ExecutableVerified {
		return hujson.Value{}, HookSpec{}, fmt.Errorf("current owned executable/selector verification required")
	}
	specs := req.Specs
	if req.Operation == Repair && len(specs) == 0 {
		specs = []HookSpec{req.Previous.Spec}
	}
	if len(specs) != 1 {
		return hujson.Value{}, HookSpec{}, fmt.Errorf("exactly one fixed stop specification required")
	}
	entry, err := desiredEntry(req.Shell, specs[0])
	if err == nil && req.Operation == Repair && valueDigest("stop-entry", entry) != valueDigest("stop-entry", old) {
		err = fmt.Errorf("Repair requires fixed prior specification")
	}
	return entry, specs[0], err
}

func collision(d *document, entry hujson.Value, old *location, op Operation) error {
	if op != Remove {
		for _, loc := range matches(d, commandIdentity(entry)) {
			if old == nil || loc.array != old.array || loc.index != old.index {
				return fmt.Errorf("unowned desired command collision")
			}
		}
	}
	return nil
}

func apply(d *document, op Operation, loc *location, entry hujson.Value) {
	if loc == nil {
		d.appendStop(entry)
	} else if op == Remove {
		loc.array.Elements = append(loc.array.Elements[:loc.index], loc.array.Elements[loc.index+1:]...)
	} else {
		original := loc.array.Elements[loc.index]
		entry.BeforeExtra, entry.AfterExtra = original.BeforeExtra, original.AfterExtra
		loc.array.Elements[loc.index] = entry
	}
}

func receiptFor(shell ShellContract, spec HookSpec, entry hujson.Value, d *document, loc *location) *Receipt {
	return &Receipt{
		Version: 1, Event: "stop", Shell: shell, Spec: spec,
		EntryDigest: valueDigest("stop-entry", entry), RemainderDigest: remainderDigest(d, loc),
	}
}
