package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Repair is the one lifecycle operation where an owned native object may be
// absent: recreating that exact object is its purpose. A foreign or
// indeterminate object remains blocking, and an existing managed object still
// has to match its recorded ownership digest.
func (service Service) observeGroupNativeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding, repair bool) error {
	if repair && service.ClientPreparation != nil {
		if err := service.ClientPreparation.RevalidateClient(ctx, client, plan); err != nil {
			return err
		}
	}
	if !repair || service.NativeObserver == nil {
		return service.observeNativeIdentity(ctx, client, plan, managed)
	}
	observation, err := service.NativeObserver.ObserveNativeIdentity(ctx, client, cloneLocalObservationPlan(plan), cloneLocalObservationBinding(managed))
	if err != nil {
		return fmt.Errorf("observe native identity for %s: %w", client.ClientID, err)
	}
	return validateGroupNativeIdentityObservation(observation, managed)
}

func (service Service) observeGroupPreparedIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding, repair bool) error {
	if !repair {
		return service.observePreparedIdentity(ctx, client, plan, managed)
	}
	observation, err := service.preparedIdentityObservation(ctx, client, plan, managed)
	if err != nil {
		return fmt.Errorf("observe prepared identity for %s: %w", client.ClientID, err)
	}
	return validateGroupNativeIdentityObservation(observation, managed)
}

func validateGroupNativeIdentityObservation(observation domain.NativeIdentityObservation, managed *domain.ClientBinding) error {
	if observation.State == domain.NativeIdentityAbsent && managed != nil {
		return nil
	}
	switch observation.State {
	case domain.NativeIdentityManaged:
		if managed == nil {
			return fmt.Errorf("native identity already exists without matching agentplugins ownership; automatic adoption is disabled")
		}
		expected := managedDigest(*managed)
		if expected == "" || observation.Digest == "" || expected != observation.Digest {
			return fmt.Errorf("native identity ownership digest is stale or does not match")
		}
		return nil
	case domain.NativeIdentityUnmanaged:
		return fmt.Errorf("native identity is unmanaged; automatic adoption is disabled")
	case domain.NativeIdentityIndeterminate:
		return fmt.Errorf("native identity ownership is indeterminate; refusing repair")
	case domain.NativeIdentityAbsent:
		return fmt.Errorf("native identity is absent without a recorded managed binding")
	default:
		return fmt.Errorf("native identity observer returned an unknown state")
	}
}

// observeGroupRecoveryEligibility determines whether a confirmed repair target
// qualifies for the absent-managed-object recovery path: an intact prior
// managed ownership receipt with a nonempty digest, a native observer capable
// of the later post-restoration verification, and the active path positively
// confirmed absent through filesystem-only observation. It deliberately never
// runs native client discovery, so a failed native registry command can never
// be mistaken for proof of absence. At its first call site (the per-target
// preflight loop) a false result simply falls through to the ordinary,
// CLI-inclusive identity check; at its second call site (the pre-commit
// recheck of an already-committed recovering target) a false result is a hard
// refusal instead, since the target's own recovery path was already chosen.
//
// This is deliberately restricted to clients that declare
// SupportsPreparedRecovery: Codex is the only one with evidence (run05) that
// its native registry command fails outright while the target it would report
// on is absent. Other clients' registry commands have not been shown to share
// that failure mode, so they keep going through the ordinary, immediate
// CLI-inclusive check.
func (service Service) observeGroupRecoveryEligibility(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) bool {
	if !plan.SelectedDelivery.EffectiveTraitsForScope(client.ClientID, plan.Scope).SupportsPreparedRecovery || managed == nil || managedDigest(*managed) == "" || service.NativeObserver == nil {
		return false
	}
	observation, err := service.preparedIdentityObservation(ctx, client, plan, managed)
	if err != nil {
		return false
	}
	return observation.State == domain.NativeIdentityAbsent
}

// groupRecoveryPostApplyVerify builds the transaction.DirectoryGroup's
// PostApplyVerify hook for a group containing at least one recovering target,
// or returns nil when the group has none. It runs full, native-registry-
// inclusive identity discovery exactly once per recovering target, only after
// every directory in the group has already been restored, and requires the
// client to now confirm exact managed ownership reconciled against the same
// recorded digest. A returned error rolls the whole group back before any
// state commit or external activation.
//
// Known residual limitations, not fixed by this contract:
//
//   - Absent swaps use exclusive publication and persisted root identity/tree
//     fingerprints for rollback and recovery. Changed or replaced provisional
//     objects retain their journal and fail closed. A noncooperating writer
//     changing the quarantine after its final fingerprint check and before
//     removal remains an unresolved filesystem concurrency limitation.
//   - Codex's own registry command is global across every installation's
//     marketplace entries in its config.toml, not scoped to one installation:
//     one installation has at most one Codex client binding, so a single
//     RepairGroup call can never itself carry two recovering targets, but a
//     wiped managed root can still leave two separate installations each with
//     their own absent Codex-owned directory. Repairing installation A
//     restores its directory, but this hook's `codex plugin list --json` call
//     still fails because installation B's marketplace entry is missing, so
//     the repair of A rolls back too; repairing B is symmetric. Neither gets
//     fixed until an operator restores or de-registers the other missing
//     source. This is reachable, not unreached -- but it fails closed (a
//     rollback, never silent adoption) and is no worse than before this fix
//     (a preflight refusal instead of a rollback), so it is left as a
//     documented limitation rather than solved here.
func (service Service) groupRecoveryPostApplyVerify(planned []plannedGroupTarget) func(context.Context) error {
	recovering := make([]plannedGroupTarget, 0, len(planned))
	for _, target := range planned {
		if target.recovering {
			recovering = append(recovering, target)
		}
	}
	if len(recovering) == 0 {
		return nil
	}
	return func(ctx context.Context) error {
		for _, target := range recovering {
			// Eligibility already required a non-nil NativeObserver; recovering
			// can only be true when that held at preflight time.
			observation, err := service.NativeObserver.ObserveNativeIdentity(ctx, target.input.Client, cloneLocalObservationPlan(target.plan), cloneLocalObservationBinding(target.managed))
			if err != nil {
				return fmt.Errorf("verify restored native identity for %s: %w", target.input.Client.ClientID, err)
			}
			if err := validateGroupRecoveryVerification(observation, target.managed); err != nil {
				return fmt.Errorf("restored native identity for %s: %w", target.input.Client.ClientID, err)
			}
		}
		return nil
	}
}

// validateGroupRecoveryVerification requires the same managed-identity proof
// the ordinary repair gate (validateGroupNativeIdentityObservation) already
// accepts as sufficient: a Managed finding whose digest matches the recorded
// receipt. Unlike the ordinary gate, an absent finding is no longer
// acceptable here -- the entire purpose of this check is confirming the
// reconstructed directory's digest, not merely its absence.
//
// Known residual gap, shared with every other Managed check in this
// codebase (not unique to recovery): a Managed finding with a matching digest
// can, for a real provider, be reached through the filesystem/digest fallback
// in observeIdentity even when the native registry itself reported the
// package as not found (registryClear) rather than confirming it -- see
// NativeIdentityObservation.NativeDiscoveryReconciled. Requiring that field
// here was tried and reverted: it is not part of the documented
// NativeIdentityObserver contract, so any other implementation of that
// interface (including this package's own test fakes and this repo's shared
// CLI test fixture) can validly omit it, and requiring it broke unrelated
// tests without a corresponding real-world exploit demonstrated. This check
// therefore proves exact digest reconstruction, not that the native registry
// itself already reports the package as installed; it does not by itself
// prove the package is activated or otherwise in active use.
func validateGroupRecoveryVerification(observation domain.NativeIdentityObservation, managed *domain.ClientBinding) error {
	if managed == nil {
		return fmt.Errorf("recovery verification requires a recorded managed binding")
	}
	if observation.State != domain.NativeIdentityManaged {
		return fmt.Errorf("state is %q, want managed", observation.State)
	}
	expected := managedDigest(*managed)
	if expected == "" || observation.Digest == "" || expected != observation.Digest {
		return fmt.Errorf("restored digest does not match the recorded receipt")
	}
	return nil
}

func (session *groupSession) freezeGroupProfiles() error {
	selected := make([]domain.DetectedClient, len(session.input.Targets))
	for i, t := range session.input.Targets {
		selected[i] = t.Client
	}
	service, frozen, err := session.service.freezeProfiles(session.ctx, session.installationID, selected, true)
	if err != nil {
		return err
	}
	session.service = service
	session.input.Targets = append([]AddInput(nil), session.input.Targets...)
	for i := range session.input.Targets {
		session.input.Targets[i].Client = frozen[i]
		session.input.Targets[i].InstallationID = session.installationID
	}
	return nil
}
