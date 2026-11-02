package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func validateNativeBinding(client domain.ClientBinding, detected domain.DetectedClient) error {
	return validateNativeBindingAt(client, detected, detected.ConfigRoot)
}

func validateNativeBindingAt(client domain.ClientBinding, detected domain.DetectedClient, nativeRoot string) error {
	if err := client.ValidateLocalEntryObservation(); err != nil {
		return err
	}
	if client.NativeActivationAttempt != "" {
		return fmt.Errorf("native activation attempt %s is unresolved; review client state before another mutation", client.NativeActivationAttempt)
	}
	if err := client.SelectedDelivery.Validate(); err != nil {
		return err
	}
	if !client.SelectedDelivery.EffectiveTraitsForScope(detected.ClientID, domain.InstallScope(client.Scope)).BindsNativeProfileRoot {
		return nil
	}
	if client.NativeProfileRoot == "" {
		return fmt.Errorf("legacy native binding has no proven native profile root; reviewed rebind is required")
	}
	if nativeRoot == "" || !filepath.IsAbs(nativeRoot) || filepath.Clean(nativeRoot) != client.NativeProfileRoot {
		return fmt.Errorf("selected native profile differs from the binding's native profile root")
	}
	return nil
}

type profileObserver struct {
	NativeIdentityObserver
	check    func() error
	fallback func(context.Context, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error)
}

func (o profileObserver) ObserveNativeIdentity(ctx context.Context, c domain.DetectedClient, p domain.DeliveryPlan, b *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if err := o.check(); err != nil {
		return domain.NativeIdentityObservation{}, err
	}
	c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
	result, err := o.NativeIdentityObserver.ObserveNativeIdentity(ctx, c, p, b)
	if guardErr := o.check(); guardErr != nil {
		return domain.NativeIdentityObservation{}, guardErr
	}
	return result, err
}
func (o profileObserver) ObservePreparedIdentity(ctx context.Context, c domain.DetectedClient, p domain.DeliveryPlan, b *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if err := o.check(); err != nil {
		return domain.NativeIdentityObservation{}, err
	}
	c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
	var result domain.NativeIdentityObservation
	var err error
	if prepared, ok := o.NativeIdentityObserver.(PreparedIdentityObserver); ok {
		result, err = prepared.ObservePreparedIdentity(ctx, c, p, b)
	} else {
		result, err = o.fallback(ctx, p, b)
	}
	if guardErr := o.check(); guardErr != nil {
		return domain.NativeIdentityObservation{}, guardErr
	}
	return result, err
}

type profileStateStore struct {
	transaction.StateStore
	check func() error
}

func (s profileStateStore) Save(state domain.StateFileV2) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.StateStore.Save(state)
}
func (s profileStateStore) RequireMutationReady() error {
	if guard, ok := s.StateStore.(interface{ RequireMutationReady() error }); ok {
		return guard.RequireMutationReady()
	}
	return nil
}

type profilePlanner struct {
	ports.DeliveryPlanner
	profiles []domain.DetectedClient
	check    func() error
}

func (p profilePlanner) Plan(ctx context.Context, r domain.PlanRequest) (domain.DeliveryPlan, error) {
	if err := p.check(); err != nil {
		return domain.DeliveryPlan{}, err
	}
	for _, c := range p.profiles {
		if c.ClientID == r.Client.ClientID {
			r.Client.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
			r.Client.ProfileNamespace = c.ProfileNamespace
		}
	}
	authority, namespace := domain.CloneProfileAuthority(r.Client.ProfileAuthority), r.Client.ProfileNamespace
	detected := make(map[domain.ClientID]domain.DetectedClient, len(r.Detected))
	for id, c := range r.Detected {
		c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
		detected[id] = c
	}
	r.Detected = detected
	plan, err := p.DeliveryPlanner.Plan(ctx, r)
	if guardErr := p.check(); guardErr != nil {
		return domain.DeliveryPlan{}, guardErr
	}
	return plan.WithProfileAuthority(authority, namespace), err
}

type profileTargets struct {
	ports.DeliveryTargetResolver
	check func() error
}

func (p profileTargets) ResolveTarget(ctx context.Context, c domain.DetectedClient, scope domain.InstallScope, artifact string) (domain.DeliveryTarget, error) {
	if err := p.check(); err != nil {
		return domain.DeliveryTarget{}, err
	}
	authority, namespace := domain.CloneProfileAuthority(c.ProfileAuthority), c.ProfileNamespace
	c.ProfileAuthority = domain.CloneProfileAuthority(authority)
	target, err := p.DeliveryTargetResolver.ResolveTarget(ctx, c, scope, artifact)
	if guardErr := p.check(); guardErr != nil {
		return domain.DeliveryTarget{}, guardErr
	}
	return target.WithProfileAuthority(authority, namespace), err
}
