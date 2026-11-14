package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) applyAdd(ctx context.Context, operationID string, manifest domain.IntegrationManifest, resolved ports.ResolvedSource, policy domain.InstallPolicy, planned []plannedTargetInstall) (domain.Report, error) {
	unlock, err := s.LockManager.Acquire(ctx, "state")
	if err != nil {
		return domain.Report{}, domain.NewError(domain.ErrLockAcquire, "acquire integrationctl state lock", err)
	}
	defer func() { _ = unlock() }()

	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return domain.Report{}, err
	}
	if _, exists := findInstallation(state.Installations, manifest.IntegrationID); exists {
		return domain.Report{}, domain.NewError(domain.ErrStateConflict, "integration already exists in state: "+manifest.IntegrationID, nil)
	}

	startedAt := s.now().UTC().Format(time.RFC3339)
	workspaceRoot := s.workspaceRootForPolicy(policy)
	if err := s.Journal.Start(ctx, domain.OperationRecord{
		OperationID:   operationID,
		Type:          "add",
		IntegrationID: manifest.IntegrationID,
		Status:        "in_progress",
		StartedAt:     startedAt,
	}); err != nil {
		return domain.Report{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.Journal.Finish(ctx, operationID, "failed")
		}
	}()

	runtime := addApplyRuntime{
		operationID:   operationID,
		startedAt:     startedAt,
		workspaceRoot: workspaceRoot,
		state:         state,
		applied:       make([]appliedTargetInstall, 0, len(planned)),
		reportTargets: make([]domain.TargetReport, 0, len(planned)),
	}
	for _, target := range planned {
		persisted, err := s.applyAddedTarget(ctx, manifest, resolved, policy, target, &runtime)
		if err != nil {
			committed = persisted
			return domain.Report{}, err
		}
	}

	runtime.state.Installations = upsertInstallation(runtime.state.Installations, installationRecordFromApplied(manifest, policy, workspaceRoot, startedAt, runtime.applied))
	if err := s.StateStore.Save(ctx, runtime.state); err != nil {
		return domain.Report{}, err
	}
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: "state", Action: "persist_state", Status: "ok"}); err != nil {
		return domain.Report{}, err
	}
	if err := s.Journal.Finish(ctx, operationID, "committed"); err != nil {
		return domain.Report{}, err
	}
	committed = true
	sort.Slice(runtime.reportTargets, func(i, j int) bool { return runtime.reportTargets[i].TargetID < runtime.reportTargets[j].TargetID })
	return domain.Report{
		OperationID:          operationID,
		Summary:              fmt.Sprintf("Installed integration %q at version %s.", manifest.IntegrationID, manifest.Version),
		RequestedTargetCount: len(planned),
		Targets:              runtime.reportTargets,
	}, nil
}
