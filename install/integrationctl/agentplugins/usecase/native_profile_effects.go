package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

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
func (s profileStager) VerifyCommandSkills(ctx context.Context, root string, e domain.PackageEnvelope, p domain.DeliveryPlan) error {
	if err := s.check(); err != nil {
		return err
	}
	inner, ok := s.PackageStager.(ports.CommandSkillVerifier)
	if !ok {
		return fmt.Errorf("command skill verifier is missing")
	}
	return inner.VerifyCommandSkills(ctx, root, e, p)
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

func (s profileStager) ManagedMCPNamesForScope(ctx context.Context, id domain.ClientID, scope domain.InstallScope, path, digest string) ([]string, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if scope != domain.ScopeUser && scope != domain.ScopeProject {
		return nil, fmt.Errorf("managed MCP selection requires an explicit user or project scope")
	}
	return readManagedSelection(ctx, s.PackageStager, id, scope, path, digest)
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
