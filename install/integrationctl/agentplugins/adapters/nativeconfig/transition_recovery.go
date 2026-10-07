package nativeconfig

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// Storage constraints, checked before a provider can persist authority and on
// every reopen. These do not claim to bound ordinary native config reads.
const MaxTransitionConfigBytes = 2 << 20
const MaxTransitionEntries = 256

type TransitionObservation string

const (
	TransitionSource TransitionObservation = "source"
	TransitionTarget TransitionObservation = "target"
)

type TransitionStateDecision string

const (
	TransitionStateOld     TransitionStateDecision = "old"
	TransitionStateDesired TransitionStateDecision = "desired"
	TransitionStateUnknown TransitionStateDecision = "unknown"
)

type TransitionRecoveryRequest struct {
	Paths    Paths
	Prepared PreparedTransition
	// Callbacks run under the same candidate lease as classification and restore.
	// An error alone grants no restore authority. Desired or unknown never restores.
	Decide   func(TransitionObservation) (TransitionStateDecision, error)
	Complete func(TransitionObservation) error
}

// ValidatePreparedTransition checks stored bytes, mode, paths and every receipt
// against both strict native documents, rather than trusting a phase or hash alone.
func ValidatePreparedTransition(paths Paths, p PreparedTransition) error {
	if err := validatePreparedTransitionFacts(paths, p); err != nil {
		return err
	}
	if err := validatePreparedTransitionIdentities(paths, p); err != nil {
		return err
	}
	for _, side := range []preparedTransitionSide{
		{p.Original.Body, p.SourceCodec, true}, {p.TargetBytes, p.TargetCodec, false},
	} {
		if err := validatePreparedTransitionSide(paths, p, side); err != nil {
			return err
		}
	}
	return nil
}

func validatePreparedTransitionFacts(paths Paths, p PreparedTransition) error {
	if !p.Original.Exists || p.Original.Mode != p.Original.Mode.Perm() || len(p.Original.Body) == 0 || len(p.Original.Body) > MaxTransitionConfigBytes || len(p.TargetBytes) == 0 || len(p.TargetBytes) > MaxTransitionConfigBytes || len(p.Entries) == 0 || len(p.Entries) > MaxTransitionEntries {
		return fmt.Errorf("invalid bounded transition preimage")
	}
	if p.Path != paths.JSON && p.Path != paths.JSONC {
		return fmt.Errorf("transition path is not a candidate")
	}
	sum := sha256.Sum256(p.TargetBytes)
	if p.TargetHash != fmt.Sprintf("sha256:%x", sum) {
		return fmt.Errorf("transition target hash mismatch")
	}
	// validateTransition also requires a desired server; validate the closed
	// identity/path portion here and prove desired receipt directly from bytes.
	if (p.SourceCodec != CodecOpenCode && p.SourceCodec != CodecOpenCodeV2) || (p.TargetCodec != CodecOpenCode && p.TargetCodec != CodecOpenCodeV2) || p.SourceCodec == p.TargetCodec {
		return fmt.Errorf("invalid transition codecs")
	}
	return nil
}

func validatePreparedTransitionIdentities(paths Paths, p PreparedTransition) error {
	for i, e := range p.Entries {
		if err := validateRequest(Request{Paths: paths, Codec: p.SourceCodec, Action: ActionRemove, Name: e.Name, Owned: &e.SourceReceipt}); err != nil {
			return err
		}
		if e.LogicalID != "opencode-mcp:"+e.Name || i > 0 && p.Entries[i-1].Name >= e.Name {
			return fmt.Errorf("invalid transition identities")
		}
	}
	return nil
}

type preparedTransitionSide struct {
	body   []byte
	codec  Codec
	source bool
}

func validatePreparedTransitionSide(paths Paths, p PreparedTransition, side preparedTransitionSide) error {
	doc, err := parseDocument(side.body, p.Path == paths.JSONC)
	if err != nil {
		return err
	}
	entries, err := codecCollection(doc, side.codec, false)
	if err != nil || entries == nil {
		return fmt.Errorf("missing transition collection: %w", err)
	}
	for _, e := range p.Entries {
		receipt := e.TargetReceipt
		if side.source {
			receipt = e.SourceReceipt
		}
		member, _ := objectMember(entries, e.Name)
		if err := verifyReceipt(&receipt, p.Path, side.codec, e.Name, member); err != nil {
			return err
		}
		if side.source && e.PreviouslyOwnedTarget != nil {
			if err := validatePreparedPreviouslyOwnedTarget(doc, p, e); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePreparedPreviouslyOwnedTarget(doc *document, p PreparedTransition, e PreparedTransitionEntry) error {
	dest, err := codecCollection(doc, p.TargetCodec, false)
	if err != nil || dest == nil {
		return ErrNotOwned
	}
	member, _ := objectMember(dest, e.Name)
	return verifyReceipt(e.PreviouslyOwnedTarget, p.Path, p.TargetCodec, e.Name, member)
}

// ReconcileDialectTransition holds exclusion across classification, authoritative
// state publication, and the immediate target->exact-preimage conditional write.
// Like Apply, portable CAS has a syscall-sized race against noncooperating hosts.
func (kernel Kernel) ReconcileDialectTransition(req TransitionRecoveryRequest) (observation TransitionObservation, resultErr error) {
	if err := kernel.RequireFileIO(); err != nil {
		return "", err
	}
	if err := ValidatePreparedTransition(req.Paths, req.Prepared); err != nil {
		return "", err
	}
	if req.Decide == nil || req.Complete == nil {
		return "", fmt.Errorf("transition recovery callbacks required")
	}
	release, err := kernel.acquireDialectTransitionLease(req.Paths, req.Prepared.SourceCodec)
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if err := release(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("unlock native recovery: %w", err))
		}
		if committed && resultErr != nil {
			resultErr = &CommittedCleanupError{Err: resultErr}
		}
	}()
	observation, resultErr = kernel.reconcileDialectTransitionLocked(req, &committed)
	return observation, resultErr
}

func (kernel Kernel) reconcileDialectTransitionLocked(req TransitionRecoveryRequest, committed *bool) (TransitionObservation, error) {
	file, err := kernel.resolve(req.Paths)
	if err != nil {
		return "", err
	}
	observation, err := classifyTransitionRecovery(file, req.Prepared)
	if err != nil {
		return "", err
	}
	decision, decisionErr := req.Decide(observation)
	if decision == TransitionStateUnknown || decision != TransitionStateOld && decision != TransitionStateDesired {
		return observation, errors.Join(decisionErr, fmt.Errorf("transition state is unresolved"))
	}
	if observation == TransitionSource && decision != TransitionStateOld {
		return observation, ErrConcurrentChange
	}
	if decision == TransitionStateDesired {
		if decisionErr != nil {
			return observation, decisionErr
		}
		*committed = true
		return observation, req.Complete(observation)
	}
	if observation == TransitionTarget {
		if err := kernel.restoreTransitionPreimage(file, req.Prepared, decisionErr); err != nil {
			return observation, err
		}
		observation = TransitionSource
	}
	return observation, errors.Join(decisionErr, req.Complete(observation))
}

func classifyTransitionRecovery(file resolvedFile, p PreparedTransition) (TransitionObservation, error) {
	if file.path != p.Path || !file.exists || file.mode.Perm() != p.Original.Mode.Perm() {
		return "", ErrConcurrentChange
	}
	switch {
	case bytes.Equal(file.body, p.Original.Body):
		return TransitionSource, nil
	case bytes.Equal(file.body, p.TargetBytes):
		return TransitionTarget, nil
	default:
		return "", ErrConcurrentChange
	}
}

func (kernel Kernel) restoreTransitionPreimage(file resolvedFile, p PreparedTransition, decisionErr error) error {
	if err := kernel.verifyAlternateAbsent(file); err != nil {
		return err
	}
	if err := kernel.compareAndSwap(p.Path, p.TargetBytes, true, p.Original.Body, p.Original.Mode); err != nil {
		return errors.Join(decisionErr, err)
	}
	restored, mode, exists, err := kernel.files.ReadNoFollow(p.Path)
	if err != nil || !exists || mode.Perm() != p.Original.Mode.Perm() || !bytes.Equal(restored, p.Original.Body) {
		return errors.Join(err, ErrConcurrentChange)
	}
	return kernel.verifyAlternateAbsent(file)
}

// ReadTransitionFileNoFollow reuses the native no-follow opener for private
// bounded journal/projection reads. Callers also enforce contained ancestors.
func ReadTransitionFileNoFollow(path string, limit int64) (body []byte, resultErr error) {
	file, err := openNoFollow(path)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("invalid bounded transition file")
	}
	body, err = io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("transition file exceeds size limit")
	}
	return body, nil
}
