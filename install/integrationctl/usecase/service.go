package usecase

import (
	"context"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type Service struct {
	SourceResolver       ports.SourceResolver
	ManifestLoader       ports.ManifestLoader
	StateStore           ports.StateStore
	WorkspaceLock        ports.WorkspaceLockStore
	LockManager          ports.LockManager
	Journal              ports.OperationJournal
	Evidence             ports.EvidenceRegistry
	Adapters             map[domain.TargetID]ports.TargetAdapter
	CurrentWorkspaceRoot string
	Now                  func() time.Time
}

type AddInput struct {
	Source          string
	Targets         []string
	Scope           string
	AutoUpdate      *bool
	AdoptNewTargets string
	AllowPrerelease *bool
	DryRun          bool
}

type NamedDryRunInput struct {
	Name   string
	Target string
	DryRun bool
}

func (s Service) List(ctx context.Context) (domain.Report, error) {
	return s.list(ctx)
}

func (s Service) Doctor(ctx context.Context) (domain.Report, error) {
	return s.doctor(ctx)
}

func (s Service) Add(ctx context.Context, in AddInput) (domain.Report, error) {
	return s.add(ctx, in)
}

func (s Service) Update(ctx context.Context, in NamedDryRunInput) (domain.Report, error) {
	return s.executeExisting(ctx, in, "update_version")
}

func (s Service) Remove(ctx context.Context, in NamedDryRunInput) (domain.Report, error) {
	return s.executeExisting(ctx, in, "remove_orphaned_target")
}

func (s Service) Repair(ctx context.Context, in NamedDryRunInput) (domain.Report, error) {
	return s.executeExisting(ctx, in, "repair_drift")
}

func (s Service) Enable(ctx context.Context, in NamedDryRunInput) (domain.Report, error) {
	return s.executeExisting(ctx, in, "enable_target")
}

func (s Service) Disable(ctx context.Context, in NamedDryRunInput) (domain.Report, error) {
	return s.executeExisting(ctx, in, "disable_target")
}

func (s Service) UpdateAll(ctx context.Context, dryRun bool) (domain.Report, error) {
	return s.updateAll(ctx, dryRun)
}

func (s Service) Sync(ctx context.Context, dryRun bool) (domain.Report, error) {
	return s.sync(ctx, dryRun)
}
