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
	prepared, err := session.service.prepareHostInputs(ctx, []AddInput{session.input}, session.input.DryRun)
	if err != nil {
		return AddResult{}, err
	}
	session.input = prepared[0]
	release, err := service.beginEnvironmentMutation(ctx, session.input.DryRun, session.input.Confirmed, session.input.EnvironmentAuthority)
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
	if !session.input.DryRun {
		if err := session.service.revalidateHost(ctx, session.input.Client); err != nil {
			return session.result, err
		}
	}
	if session.verifyErr == nil {
		if rendersCommands(session.input.Envelope, session.input.Client.ClientID) {
			return session.repairRenderedProjection()
		}
		return session.repairNative()
	}
	return session.repairPackage()
}

// repairRenderedProjection guards an intact package whose tree also depends
// on installer rendering. Repair restores recorded bytes only, so a confirmed
// repair stops with ErrProjectionRefreshRequired when the installed command
// skills differ from what the current rules render. Other host projection
// changes do not stop it, as they do not for a package without commands.
// Without confirmation nothing is checked, and the native check is not
// attempted either, since an outdated projection would fail it for the wrong
// reason.
func (session *repairSession) repairRenderedProjection() (AddResult, error) {
	if session.input.DryRun {
		return session.result, nil
	}
	if !session.input.Confirmed {
		session.result.RequiresConfirmation = true
		return session.result, nil
	}
	current, verified, err := session.service.commandSkillsCurrent(session.ctx, session.target.ActivePath, session.input.Envelope, session.plan)
	if err != nil {
		return session.result, err
	}
	if !verified {
		delivery, err := session.stageRepairDelivery(true)
		if err != nil {
			return session.result, err
		}
		session.service.discardSettledDelivery(session.input.OperationID, delivery)
		current = delivery.ArtifactDigest == session.expectedDigest
	}
	if !current {
		return session.result, ErrProjectionRefreshRequired
	}
	return session.repairNative()
}

// RefreshProjection restages an intact, exactly bound package with the caller's
// current host projection inputs. It is intended for host configuration changes
// that do not change the resolved package revision. Unlike Repair, it can
// commit a new projected digest even when the installed package is intact.
// A repeated refresh with identical projection output is a no-op.
func (service Service) RefreshProjection(ctx context.Context, input AddInput) (AddResult, error) {
	input.refreshSelectedDelivery = true
	session := &repairSession{service: service, ctx: ctx, input: input}
	if err := session.validateRepairInput(); err != nil {
		return AddResult{}, err
	}
	prepared, err := session.service.prepareHostInputs(ctx, []AddInput{session.input}, session.input.DryRun)
	if err != nil {
		return AddResult{}, err
	}
	session.input = prepared[0]
	release, err := service.beginEnvironmentMutation(ctx, session.input.DryRun, session.input.Confirmed, session.input.EnvironmentAuthority)
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
	if !session.input.DryRun {
		if err := session.service.revalidateHost(ctx, session.input.Client); err != nil {
			return session.result, err
		}
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
		left.Policy == right.Policy && left.Verification == right.Verification && left.LocalEntryObservation.Equal(right.LocalEntryObservation)
}
