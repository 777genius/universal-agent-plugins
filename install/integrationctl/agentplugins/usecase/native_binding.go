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
	if err := client.ValidateLocalEntryObservation(); err != nil {
		return err
	}
	if client.NativeActivationAttempt != "" {
		return fmt.Errorf("native activation attempt %s is unresolved; review client state before another mutation", client.NativeActivationAttempt)
	}
	if err := client.SelectedDelivery.Validate(); err != nil {
		return err
	}
	if !client.SelectedDelivery.EffectiveTraits(detected.ClientID).BindsNativeProfileRoot {
		return nil
	}
	if client.NativeProfileRoot == "" {
		return fmt.Errorf("legacy native binding has no proven native profile root; reviewed rebind is required")
	}
	if detected.ConfigRoot == "" || !filepath.IsAbs(detected.ConfigRoot) || filepath.Clean(detected.ConfigRoot) != client.NativeProfileRoot {
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

type profileStager struct {
	ports.PackageStager
	check func() error
}

func (s profileStager) Stage(ctx context.Context, e domain.PackageEnvelope, p domain.DeliveryPlan, id string, h domain.CompatibilityHints) (domain.StagedDelivery, error) {
	if err := s.check(); err != nil {
		return domain.StagedDelivery{}, err
	}
	return s.PackageStager.Stage(ctx, e, p, id, h)
}
func (s profileStager) Discard(ctx context.Context, d domain.StagedDelivery) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.PackageStager.Discard(ctx, d)
}
func (s profileStager) Verify(ctx context.Context, path, digest string) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.PackageStager.Verify(ctx, path, digest)
}
func (s profileStager) StageWithPluginData(ctx context.Context, e domain.PackageEnvelope, p domain.DeliveryPlan, id string, h domain.CompatibilityHints, data string) (domain.StagedDelivery, error) {
	if err := s.check(); err != nil {
		return domain.StagedDelivery{}, err
	}
	inner, ok := s.PackageStager.(ports.PluginDataAwareStager)
	if !ok {
		return domain.StagedDelivery{}, fmt.Errorf("plugin data stager is missing")
	}
	return inner.StageWithPluginData(ctx, e, p, id, h, data)
}
func (s profileStager) ProjectActiveNative(ctx context.Context, e domain.PackageEnvelope, p domain.DeliveryPlan, path, data string) ([]domain.NativeObjectOwnership, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	inner, ok := s.PackageStager.(ports.ActiveNativeProjector)
	if !ok {
		return nil, fmt.Errorf("native projector is missing")
	}
	return inner.ProjectActiveNative(ctx, e, p, path, data)
}
func (s profileStager) PreflightManagedStdio(command string) error {
	if err := s.check(); err != nil {
		return err
	}
	if inner, ok := s.PackageStager.(ports.ManagedStdioPreflighter); ok {
		return inner.PreflightManagedStdio(command)
	}
	return nil
}
func (s profileStager) ManagedMCPNames(ctx context.Context, id domain.ClientID, path, digest string) ([]string, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if inner, ok := s.PackageStager.(managedSelectionReader); ok {
		return inner.ManagedMCPNames(ctx, id, path, digest)
	}
	return nil, nil
}

type profileData struct {
	PluginDataManager
	check func() error
}

func (d profileData) EnsureData(ctx context.Context, id, backend, scope string) (domain.DataReceipt, bool, error) {
	if err := d.check(); err != nil {
		return domain.DataReceipt{}, false, err
	}
	return d.PluginDataManager.EnsureData(ctx, id, backend, scope)
}
func (d profileData) PrepareRuntime(ctx context.Context, e domain.PackageEnvelope, p domain.DeliveryPlan, path string) error {
	if err := d.check(); err != nil {
		return err
	}
	return d.PluginDataManager.PrepareRuntime(ctx, e, p, path)
}
func (d profileData) PurgeData(ctx context.Context, r domain.DataReceipt) error {
	if err := d.check(); err != nil {
		return err
	}
	return d.PluginDataManager.PurgeData(ctx, r)
}
func (d profileData) PreflightDataPath(path string) (string, bool, error) {
	if err := d.check(); err != nil {
		return "", false, err
	}
	if inner, ok := d.PluginDataManager.(ports.DataPathPreflighter); ok {
		return inner.PreflightDataPath(path)
	}
	return "", false, fmt.Errorf("plugin data preflight is missing")
}

type profileActivator struct {
	ports.ClientActivator
	check func() error
}

func (a profileActivator) Activate(ctx context.Context, r domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := a.check(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	r.Client.ProfileAuthority = domain.CloneProfileAuthority(r.Client.ProfileAuthority)
	return a.ClientActivator.Activate(ctx, r)
}
func (a profileActivator) Deactivate(ctx context.Context, r domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if err := a.check(); err != nil {
		return domain.DeactivationOutcome{}, err
	}
	r.Client.ProfileAuthority = domain.CloneProfileAuthority(r.Client.ProfileAuthority)
	return a.ClientActivator.Deactivate(ctx, r)
}
func (a profileActivator) PreflightActivation(r domain.ActivationRequest) error {
	if err := a.check(); err != nil {
		return err
	}
	if inner, ok := a.ClientActivator.(ports.ActivationPreflighter); ok {
		r.Client.ProfileAuthority = domain.CloneProfileAuthority(r.Client.ProfileAuthority)
		if err := inner.PreflightActivation(r); err != nil {
			return err
		}
		return a.check()
	}
	return nil
}
func (a profileActivator) VerifierAvailable(c domain.DetectedClient, p domain.DeliveryPlan, exe string) bool {
	inner, ok := a.ClientActivator.(ports.ActivationVerifierClassifier)
	if !ok || a.check() != nil {
		return false
	}
	c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
	available := inner.VerifierAvailable(c, p, exe)
	return a.check() == nil && available
}

func (service Service) guardFrozenProfiles(ctx context.Context, frozen, shared []domain.DetectedClient) Service {
	guarded := append([]domain.DetectedClient(nil), frozen...)
	for i := range guarded {
		guarded[i].ProfileAuthority = domain.CloneProfileAuthority(guarded[i].ProfileAuthority)
	}
	service.PhysicalProfiles = guarded
	service.PhysicalAuthority = service.authorityPort()
	base := service
	service.profileCheck = func() error {
		for _, c := range append(append([]domain.DetectedClient(nil), guarded...), shared...) {
			if err := base.checkProfile(ctx, c); err != nil {
				return err
			}
		}
		kernel := base.Kernel
		kernel.StateStore = base.StateStore
		kernel.PhysicalAuthority = base.authorityPort()
		return kernel.PrevalidateRecovery(ctx)
	}
	service.Kernel.PhysicalAuthority = service.PhysicalAuthority
	service.StateStore = profileStateStore{StateStore: service.StateStore, check: service.profileCheck}
	service.Targets = profileTargets{DeliveryTargetResolver: service.Targets, check: service.profileCheck}
	service.Planner = profilePlanner{DeliveryPlanner: service.Planner, profiles: guarded, check: service.profileCheck}
	service.Stager = profileStager{PackageStager: service.Stager, check: service.profileCheck}
	if service.PluginData != nil {
		service.PluginData = profileData{PluginDataManager: service.PluginData, check: service.profileCheck}
	}
	service.Activator = profileActivator{ClientActivator: service.Activator, check: service.profileCheck}
	if service.NativeObserver != nil {
		service.NativeObserver = profileObserver{NativeIdentityObserver: service.NativeObserver, check: service.profileCheck, fallback: base.filesystemIdentityObservation}
	}
	return service
}
