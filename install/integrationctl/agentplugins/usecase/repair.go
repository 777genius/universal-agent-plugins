package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Repair replaces a missing or modified managed directory from a freshly
// resolved package. It never uses a persisted target until that target has
// been matched to the target resolver's current safe result.
func (service Service) Repair(ctx context.Context, input AddInput) (AddResult, error) {
	session := &repairSession{service: service, ctx: ctx, input: input}
	if err := session.validateRepairInput(); err != nil {
		return AddResult{}, err
	}
	release, err := service.beginMutation(ctx, session.input.DryRun, session.input.Confirmed)
	if err != nil {
		return AddResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	if err := session.loadRepairTarget(); err != nil {
		return session.result, err
	}
	if err := session.verifyRepairPreconditions(); err != nil {
		return session.result, err
	}
	if session.verifyErr == nil {
		return session.repairNative()
	}
	return session.repairPackage()
}

// RefreshProjection restages an intact, exactly bound package with the caller's
// current host projection inputs. It is intended for host configuration changes
// that do not change the resolved package revision. Unlike Repair, it can
// commit a new projected digest even when the installed package is intact.
// A repeated refresh with identical projection output is a no-op.
func (service Service) RefreshProjection(ctx context.Context, input AddInput) (AddResult, error) {
	session := &repairSession{service: service, ctx: ctx, input: input}
	if err := session.validateRepairInput(); err != nil {
		return AddResult{}, err
	}
	release, err := service.beginMutation(ctx, session.input.DryRun, session.input.Confirmed)
	if err != nil {
		return AddResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	if err := session.loadRepairTarget(); err != nil {
		return session.result, err
	}
	if err := session.verifyRepairPreconditions(); err != nil {
		return session.result, err
	}
	return session.refreshIntactProjection()
}

func (service Service) verifyRepairPrecondition(ctx context.Context, activePath, managedDigest string, reviewedKind ports.VerificationKind, reviewedDigest string) error {
	err := service.Stager.Verify(ctx, activePath, managedDigest)
	var observed *ports.VerificationError
	if !errors.As(err, &observed) {
		if err == nil {
			return fmt.Errorf("managed native object changed after repair preflight; rerun repair")
		}
		return fmt.Errorf("revalidate managed native object before repair commit: %w", err)
	}
	if observed.Kind != reviewedKind {
		return fmt.Errorf("managed native object changed after repair preflight; rerun repair")
	}
	if reviewedKind == ports.VerificationDigestMismatch && (reviewedDigest == "" || observed.ActualDigest != reviewedDigest) {
		return fmt.Errorf("managed native object digest changed after repair preflight; rerun repair")
	}
	return nil
}

func sameLifecycleOutcome(left, right domain.ActivationOutcome) bool {
	return left.Activation == right.Activation && left.Authentication == right.Authentication &&
		left.Policy == right.Policy && left.Verification == right.Verification
}
