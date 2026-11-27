package installer

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// WithInstallationPolicy returns an independent engine view for one policy
// session. Prepare and Apply must use the same returned view. Existing handles,
// namespace guards, and the original engine retain their authority unchanged.
func (e *Engine) WithInstallationPolicy(gate ports.InstallationPolicy) *Engine {
	if e == nil {
		return nil
	}
	copy := *e
	copy.cfg.InstallationPolicy = gate
	return &copy
}

type reviewedInstallationPolicy struct {
	gate        ports.InstallationPolicy
	fingerprint string
}

// Keep gate origin through confirmation and core lifecycle wrappers without
// coupling the SDK to a concrete policy-file adapter or hiding its cause.
type installationPolicyCheckError struct{ cause error }

func (err *installationPolicyCheckError) Error() string {
	return err.cause.Error()
}

func (err *installationPolicyCheckError) Unwrap() error {
	return err.cause
}

func (reviewed reviewedInstallationPolicy) Check(ctx context.Context, request domain.InstallPolicyRequest) (string, error) {
	fingerprint, err := reviewed.gate.Check(ctx, request)
	if err != nil {
		return "", &installationPolicyCheckError{cause: err}
	}
	if fingerprint != reviewed.fingerprint {
		return "", domain.ErrInstallPolicyChanged
	}
	return fingerprint, nil
}

func (e *Engine) reviewInstallationPolicy(ctx context.Context, prepared *PreparedOperation) error {
	if e.cfg.InstallationPolicy == nil || prepared.req.Operation == OpRemove {
		return nil
	}
	for _, request := range preparedInstallationPolicyRequests(prepared) {
		fingerprint, err := e.cfg.InstallationPolicy.Check(ctx, request)
		if err != nil {
			return err
		}
		if prepared.installationPolicyReviewed && fingerprint != prepared.installationPolicyFingerprint {
			return domain.ErrInstallPolicyChanged
		}
		prepared.installationPolicyFingerprint = fingerprint
		prepared.installationPolicyReviewed = true
	}
	return nil
}

func (e *Engine) recheckInstallationPolicy(ctx context.Context, prepared *PreparedOperation) error {
	if e.cfg.InstallationPolicy == nil || prepared.req.Operation == OpRemove {
		return nil
	}
	if !prepared.installationPolicyReviewed {
		return domain.ErrInstallPolicyChanged
	}
	reviewed := reviewedInstallationPolicy{gate: e.cfg.InstallationPolicy, fingerprint: prepared.installationPolicyFingerprint}
	for _, request := range preparedInstallationPolicyRequests(prepared) {
		if _, err := reviewed.Check(ctx, request); err != nil {
			return err
		}
	}
	return nil
}

func preparedInstallationPolicyRequests(prepared *PreparedOperation) []domain.InstallPolicyRequest {
	req := prepared.req
	if len(req.Targets) == 0 {
		return []domain.InstallPolicyRequest{{
			Source: prepared.envelope.Source, OriginMode: prepared.snapshot.OriginMode,
			Envelope: &prepared.envelope, Scope: req.Scope, Targets: []domain.ClientID{prepared.client.ClientID},
			GitPinned: req.GitPinned, ArchivePinned: req.ArchivePinned,
		}}
	}
	requests := make([]domain.InstallPolicyRequest, len(prepared.envelopes))
	// Group sealing supports direct sources only. Acquired Directory provenance
	// is a single-target contract and is rejected before group preparation.
	for i := range prepared.envelopes {
		gitPinned, archivePinned := targetPolicyPins(req, req.Targets[i])
		requests[i] = domain.InstallPolicyRequest{
			Source: prepared.envelopes[i].Source, Envelope: &prepared.envelopes[i],
			OriginMode: domain.OriginModeDirect,
			Scope:      req.Scope, Targets: []domain.ClientID{domain.ClientID(req.Targets[i].ClientID)},
			GitPinned: gitPinned, ArchivePinned: archivePinned,
		}
	}
	return requests
}

func targetPolicyPins(req Request, target ClientTarget) (*bool, *bool) {
	gitPinned, archivePinned := req.GitPinned, req.ArchivePinned
	if target.GitPinned != nil {
		gitPinned = target.GitPinned
	}
	if target.ArchivePinned != nil {
		archivePinned = target.ArchivePinned
	}
	return gitPinned, archivePinned
}

func clonePolicyPin(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func installationPolicyFailure(op Operation, err error) (Result, error) {
	reason, known := installationPolicyConflictReason(err)
	if !known {
		reason = "policy_denied"
	}
	return Result{Operation: op, Outcome: OutcomeConflict, Reason: reason}, err
}

func installationPolicyConflictReason(err error) (string, bool) {
	if errors.Is(err, domain.ErrInstallPolicyChanged) {
		return "policy_changed", true
	}
	var denied *domain.InstallPolicyError
	var gateFailure *installationPolicyCheckError
	if errors.As(err, &denied) || errors.As(err, &gateFailure) {
		return "policy_denied", true
	}
	return "", false
}
