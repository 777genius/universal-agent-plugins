package providers

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type packageVerifier interface {
	Verify(context.Context, string, string) error
}

// NativeIdentityObserver combines package ownership with the client's native
// registry. A manager-owned path is not proof that the logical identity is
// free: another prepared marketplace, local plugin, installed CLI entry, or
// native config entry can already claim the same manifest identity.
// Native registry discovery executes trusted, short-lived list commands. A
// tree-aware runner is used when available so Linux requires atomic cgroup
// containment and Windows uses a Job Object. Plain runners remain supported as
// injected test/provider implementations, but OS execution never claims cleanup
// based on descendant sampling.
type NativeIdentityObserver struct {
	Stager           packageVerifier
	Runner           ports.CommandRunner
	DiscoveryTimeout time.Duration
	NativeConfig     *nativeconfig.Kernel
	// Registry supplies the client adapters that inspect native identity. It is
	// injected by the composition root and never defaulted to "every client".
	Registry *clients.Registry
}

const defaultNativeDiscoveryTimeout = 15 * time.Second

// The finding vocabulary belongs to the client contract: these names stay here
// as aliases so in-package tests keep reading the same tokens.
type registryFinding = clients.RegistryFinding

const (
	registryClear         = clients.RegistryClear
	registryExpected      = clients.RegistryExpected
	registryCollision     = clients.RegistryCollision
	registryIndeterminate = clients.RegistryIndeterminate
)

func (observer NativeIdentityObserver) requireRegistry() error {
	if observer.Registry == nil {
		return clients.ErrRegistryRequired
	}
	return nil
}

func (observer NativeIdentityObserver) env() clients.Env {
	return clients.Env{Runner: observer.Runner, NativeConfig: observer.nativeConfigKernel()}
}

func (observer NativeIdentityObserver) nativeConfigKernel() nativeconfig.Kernel {
	if observer.NativeConfig != nil {
		return *observer.NativeConfig
	}
	return nativeconfig.Kernel{}
}

func (observer NativeIdentityObserver) ObserveNativeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	return observer.observeIdentity(ctx, client, plan, managed, true)
}

// ObservePreparedIdentity inspects only filesystem-backed package identities.
// It deliberately excludes native CLI discovery so dry-run cannot launch the
// detected client executable.
func (observer NativeIdentityObserver) ObservePreparedIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	return observer.observeIdentity(ctx, client, plan, managed, false)
}

func (observer NativeIdentityObserver) observeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding, includeNativeRegistry bool) (domain.NativeIdentityObservation, error) {
	if err := ctx.Err(); err != nil {
		return domain.NativeIdentityObservation{}, err
	}
	name := strings.TrimSpace(plan.DeclaredName)
	if name == "" {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, nil
	}
	if err := observer.requireRegistry(); err != nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, err
	}
	prepared, preparedErr := observer.inspectPreparedRegistry(client.ClientID, plan, name, managed != nil)
	if preparedErr != nil {
		prepared = registryIndeterminate
	}
	native, attempted, err := observer.discoverNativeFinding(ctx, client, plan, managed, includeNativeRegistry)
	if err != nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate, NativeDiscoveryState: domain.NativeIdentityIndeterminate,
			NativeDiscoveryAttempted: attempted}, err
	}
	return observer.finishIdentity(ctx, plan, managed, prepared, preparedErr, native, attempted)
}

func (observer NativeIdentityObserver) discoverNativeFinding(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding, includeNativeRegistry bool) (registryFinding, bool, error) {
	if !includeNativeRegistry {
		return registryClear, false, nil
	}
	inspector, hasInspector := clients.As[clients.RegistryInspector](observer.Registry, client.ClientID)
	attempted := observer.Runner != nil && strings.TrimSpace(plan.NativeRegistryExecutable) != "" &&
		hasInspector && inspector.UsesNativeRegistryExecutable()
	nativeCtx := ctx
	cancelNative := func() {}
	if attempted {
		timeout := observer.DiscoveryTimeout
		if timeout <= 0 {
			timeout = defaultNativeDiscoveryTimeout
		}
		nativeCtx, cancelNative = context.WithTimeout(ctx, timeout)
	}
	native, err := observer.inspectNativeRegistry(nativeCtx, client, plan, managed)
	cancelNative()
	return native, attempted, err
}

func (observer NativeIdentityObserver) finishIdentity(ctx context.Context, plan domain.DeliveryPlan, managed *domain.ClientBinding, prepared registryFinding, preparedErr error, native registryFinding, attempted bool) (domain.NativeIdentityObservation, error) {
	observed := func(state domain.NativeIdentityState) domain.NativeIdentityObservation {
		return identityObservation(state, native, managed != nil, attempted)
	}
	if preparedErr != nil {
		return observed(domain.NativeIdentityIndeterminate), preparedErr
	}
	if state, done := combinedIdentityState(prepared, native, managed != nil); done {
		return observed(state), nil
	}
	return observer.verifyOwnedPackage(ctx, plan, managed, observed)
}

func identityObservation(state domain.NativeIdentityState, native registryFinding, owned, attempted bool) domain.NativeIdentityObservation {
	discoveryState := domain.NativeIdentityAbsent
	discoveryReconciled := false
	switch native {
	case registryExpected:
		discoveryState = domain.NativeIdentityManaged
		discoveryReconciled = owned
	case registryCollision:
		discoveryState = domain.NativeIdentityUnmanaged
	case registryIndeterminate:
		discoveryState = domain.NativeIdentityIndeterminate
	}
	return domain.NativeIdentityObservation{State: state, NativeDiscoveryState: discoveryState,
		NativeDiscoveryReconciled: discoveryReconciled, NativeDiscoveryAttempted: attempted}
}

func combinedIdentityState(prepared, native registryFinding, owned bool) (domain.NativeIdentityState, bool) {
	if prepared == registryCollision || native == registryCollision {
		return domain.NativeIdentityUnmanaged, true
	}
	if prepared == registryIndeterminate || native == registryIndeterminate {
		return domain.NativeIdentityIndeterminate, true
	}
	if !owned && (prepared == registryExpected || native == registryExpected) {
		return domain.NativeIdentityUnmanaged, true
	}
	return domain.NativeIdentityAbsent, false
}

func (observer NativeIdentityObserver) verifyOwnedPackage(ctx context.Context, plan domain.DeliveryPlan, managed *domain.ClientBinding, observed func(domain.NativeIdentityState) domain.NativeIdentityObservation) (domain.NativeIdentityObservation, error) {
	_, statErr := os.Lstat(plan.ActivePath)
	if os.IsNotExist(statErr) {
		return observed(domain.NativeIdentityAbsent), nil
	}
	if statErr != nil {
		return observed(domain.NativeIdentityIndeterminate), statErr
	}
	if managed == nil {
		return observed(domain.NativeIdentityUnmanaged), nil
	}
	expected := shared.ManagedPackageDigest(*managed)
	if expected == "" || observer.Stager == nil {
		return observed(domain.NativeIdentityIndeterminate), nil
	}
	if err := observer.Stager.Verify(ctx, plan.ActivePath, expected); err != nil {
		var verification *ports.VerificationError
		if errors.As(err, &verification) && verification.Kind == ports.VerificationDigestMismatch {
			result := observed(domain.NativeIdentityIndeterminate)
			result.Digest = verification.ActualDigest
			return result, nil
		}
		return observed(domain.NativeIdentityIndeterminate), err
	}
	result := observed(domain.NativeIdentityManaged)
	result.Digest = expected
	result.ReceiptReconciled = true
	return result, nil
}

func (observer NativeIdentityObserver) inspectPreparedRegistry(id domain.ClientID, plan domain.DeliveryPlan, name string, owned bool) (registryFinding, error) {
	if inspector, ok := clients.As[clients.PreparedRegistryInspector](observer.Registry, id); ok {
		return inspector.InspectPreparedRegistry(plan, name, owned)
	}
	return shared.InspectUnqualifiedPluginRoot(plan.TargetRoot, name, plan.ActivePath, owned)
}

func (observer NativeIdentityObserver) inspectNativeRegistry(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	inspector, ok := clients.As[clients.RegistryInspector](observer.Registry, client.ClientID)
	if !ok {
		return registryIndeterminate, nil
	}
	return inspector.InspectNativeRegistry(ctx, observer.env(), client, plan, managed)
}
