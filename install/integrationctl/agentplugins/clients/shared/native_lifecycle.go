package shared

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// NativeConfigActivation is the shared lifecycle of clients that install
// through the native config kernel. They differ only in messages and in which
// verify/activate pair they call; the committed-cleanup degradation path is
// the same for all of them.
type NativeConfigActivation struct {
	Automatic         bool
	UnavailableAction string
	RepairAction      string
	RetryAction       string
	CompletedAction   string
	Verify            func() error
	Activate          func(ctx context.Context, request domain.ActivationRequest) error
}

// CompleteNativeConfigActivation runs one native-config client's activate or
// verify-only path. Callers pass Activate as a closure over Env.NativeConfig so
// this helper never names the kernel type.
func CompleteNativeConfigActivation(ctx context.Context, request domain.ActivationRequest, client NativeConfigActivation) (domain.ActivationOutcome, error) {
	outcome := StartedActivation(request)
	if !client.Automatic {
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = append(outcome.UserActions, client.UnavailableAction)
		return outcome, nil
	}
	if request.VerifyOnly {
		if err := client.Verify(); err != nil {
			return FailedActivation(outcome, client.RepairAction, err)
		}
	} else if err := client.Activate(ctx, request); err != nil {
		if !CommittedNativeCleanup(&outcome, err) {
			return FailedActivation(outcome, client.RetryAction, err)
		}
	}
	outcome.Activation = domain.ActivationActive
	outcome.Verification = domain.VerificationInstalled
	outcome.UserActions = append(outcome.UserActions, client.CompletedAction)
	return outcome, nil
}

// CommittedNativeCleanup records that the native config write committed but
// lock cleanup degraded. The install is still done; the operator retries only
// if a later lock reports busy.
func CommittedNativeCleanup(outcome *domain.ActivationOutcome, err error) bool {
	if !nativeconfig.IsCommittedCleanup(err) {
		return false
	}
	outcome.UserActions = append(outcome.UserActions, "native config was committed, but lock cleanup degraded; retry repair if the next operation reports a busy lock")
	return true
}

// CommittedNativeDeactivationCleanup is the matching degrade path for remove.
func CommittedNativeDeactivationCleanup(outcome *domain.DeactivationOutcome, err error) bool {
	if !nativeconfig.IsCommittedCleanup(err) {
		return false
	}
	outcome.UserActions = append(outcome.UserActions, "native config removal was committed, but lock cleanup degraded; retry removal if the next operation reports a busy lock")
	return true
}

// HasNativeConfigRoot reports that a native-config client has a writable root
// and only native components, which is what automatic activation requires.
func HasNativeConfigRoot(request domain.ActivationRequest) bool {
	return strings.TrimSpace(request.Client.ConfigRoot) != "" && OnlyNativeComponents(request.Plan.Components)
}
