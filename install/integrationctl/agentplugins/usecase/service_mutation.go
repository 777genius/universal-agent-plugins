package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func (service Service) Add(ctx context.Context, input AddInput) (AddResult, error) {
	return service.apply(ctx, input, false)
}

func (service Service) apply(ctx context.Context, input AddInput, replace bool) (AddResult, error) {
	session := &applySession{service: service, ctx: ctx, input: input, replace: replace}
	if err := session.validateApplyInput(); err != nil {
		return AddResult{}, err
	}
	prepared, err := session.service.prepareHostInputs(ctx, []AddInput{session.input}, session.input.DryRun)
	if err != nil {
		return AddResult{}, err
	}
	session.input = prepared[0]
	if err := session.resolveInstallation(); err != nil {
		return AddResult{}, err
	}
	service, frozen, err := session.service.freezeProfiles(ctx, session.installationID, []domain.DetectedClient{session.input.Client}, true)
	if err != nil {
		return AddResult{}, err
	}
	session.service, session.input.Client = service, frozen[0]
	session.input.InstallationID = session.installationID
	release, err := service.beginEnvironmentMutation(ctx, session.input.DryRun, session.input.Confirmed, session.input.EnvironmentAuthority)
	if err != nil {
		return AddResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	if err := session.resolveInstallation(); err != nil {
		return AddResult{}, err
	}
	if err := session.validateApplyTransitions(); err != nil {
		return AddResult{}, err
	}
	if err := session.planAndPreflight(); err != nil {
		return session.result, err
	}
	if err := session.resolveBinding(); err != nil {
		return session.result, err
	}
	if session.input.DryRun {
		return session.dryRunPath()
	}
	if err := session.service.revalidateHost(ctx, session.input.Client); err != nil {
		return session.result, err
	}
	done, result, err := session.noChangeOrResume()
	if done {
		return result, err
	}
	return session.stageAndCommit()
}

func (service Service) stagePackage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints, dataPath string) (domain.StagedDelivery, error) {
	if strings.TrimSpace(dataPath) == "" {
		return service.Stager.Stage(ctx, envelope, plan, operationID, hints)
	}
	aware, ok := service.Stager.(ports.PluginDataAwareStager)
	if !ok {
		return domain.StagedDelivery{}, fmt.Errorf("package stager cannot bind the owned PLUGIN_DATA locator")
	}
	return aware.StageWithPluginData(ctx, envelope, plan, operationID, hints, dataPath)
}

func (service Service) beginMutation(ctx context.Context, dryRun, confirmed bool) (ports.UnlockFunc, error) {
	if err := service.checkProfiles(ctx); err != nil {
		return nil, err
	}
	if err := service.guardEnvironmentRecovery(); err != nil {
		return nil, err
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	kernel.PhysicalAuthority = service.authorityPort()
	if err := kernel.PrevalidateRecovery(ctx); err != nil {
		return nil, err
	}
	if dryRun || !confirmed {
		return nil, nil
	}
	if service.Lock == nil {
		return nil, fmt.Errorf("agentplugins mutation lock is required")
	}
	release, err := service.Lock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	if guard, ok := service.StateStore.(interface{ RequireMutationReady() error }); ok {
		if err := guard.RequireMutationReady(); err != nil {
			_ = release()
			return nil, err
		}
	}
	if err := service.validateRecordedDeliveries(); err != nil {
		_ = release()
		return nil, err
	}
	if err := service.checkProfiles(ctx); err != nil {
		_ = release()
		return nil, err
	}
	kernel = service.Kernel
	kernel.StateStore = service.StateStore
	kernel.PhysicalAuthority = service.authorityPort()
	if err := service.guardEnvironmentRecovery(); err != nil {
		_ = release()
		return nil, err
	}
	if err := kernel.Recover(ctx); err != nil {
		_ = release()
		return nil, fmt.Errorf("recover interrupted mutation: %w", err)
	}
	return release, nil
}

func firstEnvironmentSourceKey(authority *domain.EnvironmentAuthority, fallback string) string {
	if authority != nil && authority.SourceKey != "" {
		return authority.SourceKey
	}
	return fallback
}

func (service Service) beginEnvironmentMutation(ctx context.Context, dryRun, confirmed bool, authority *domain.EnvironmentAuthority) (ports.UnlockFunc, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return nil, err
	}
	if err := transaction.ValidateEnvironmentRecoveryScope(state, authority); err != nil {
		return nil, err
	}
	service.environmentAuthority = authority
	return service.beginMutation(ctx, dryRun, confirmed)
}

func (service Service) now() time.Time {
	if service.Now != nil {
		return service.Now().UTC()
	}
	return time.Now().UTC()
}
