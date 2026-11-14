package usecase

import (
	"context"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type plannedTargetInstall struct {
	TargetID domain.TargetID
	Delivery domain.Delivery
	Adapter  ports.TargetAdapter
	Inspect  ports.InspectResult
	Plan     ports.AdapterPlan
}

type appliedTargetInstall struct {
	Planned plannedTargetInstall
	Result  ports.ApplyResult
	Verify  ports.InspectResult
}

func (s Service) add(ctx context.Context, in AddInput) (domain.Report, error) {
	resolved, err := s.SourceResolver.Resolve(ctx, domain.IntegrationRef{Raw: in.Source})
	if err != nil {
		return domain.Report{}, err
	}
	defer cleanupResolvedSource(resolved)
	manifest, err := s.ManifestLoader.Load(ctx, resolved)
	if err != nil {
		return domain.Report{}, err
	}
	if err := s.ensureAddNotManagedYet(ctx, manifest.IntegrationID); err != nil {
		return domain.Report{}, err
	}
	selectedTargets, err := resolveRequestedTargets(manifest, in.Targets)
	if err != nil {
		return domain.Report{}, err
	}
	policy := domain.InstallPolicy{
		Scope:           defaultString(in.Scope, "user"),
		AutoUpdate:      defaultBool(in.AutoUpdate, true),
		AdoptNewTargets: defaultString(in.AdoptNewTargets, "manual"),
		AllowPrerelease: defaultBool(in.AllowPrerelease, false),
	}
	opPrefix := "add"
	summary := fmt.Sprintf("Install plan for integration %q at version %s.", manifest.IntegrationID, manifest.Version)
	if in.DryRun {
		opPrefix = "plan_add"
		summary = fmt.Sprintf("Dry-run plan for integration %q at version %s.", manifest.IntegrationID, manifest.Version)
	}
	report := domain.Report{
		OperationID:          operationID(opPrefix, manifest.IntegrationID, s.now()),
		Summary:              summary,
		RequestedTargetCount: len(selectedTargets),
	}
	planned := make([]plannedTargetInstall, 0, len(selectedTargets))
	for _, target := range selectedTargets {
		item, err := s.planTargetInstall(ctx, manifest, policy, target)
		if err != nil {
			return domain.Report{}, err
		}
		planned = append(planned, item)
		report.Targets = append(report.Targets, toTargetReport(manifest.IntegrationID, item.Delivery, item.Inspect, item.Plan))
	}
	sort.Slice(report.Targets, func(i, j int) bool { return report.Targets[i].TargetID < report.Targets[j].TargetID })
	if in.DryRun {
		return report, nil
	}
	ready, blocked := splitBlockedTargetInstalls(planned)
	if len(ready) == 0 {
		return domain.Report{}, firstBlockedTargetInstallError(blocked)
	}
	report, err = s.applyAdd(ctx, report.OperationID, manifest, resolved, policy, ready)
	if err != nil {
		return domain.Report{}, err
	}
	if len(blocked) > 0 {
		report.Summary = partialAddSummary(manifest, len(ready))
		report.RequestedTargetCount = len(selectedTargets)
		report.SkippedTargets = append(report.SkippedTargets, blockedTargetIDs(blocked)...)
		report.Warnings = append(report.Warnings, skippedBlockedTargetWarnings(manifest.IntegrationID, blocked)...)
	}
	return report, nil
}

func (s Service) ensureAddNotManagedYet(ctx context.Context, integrationID string) error {
	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return err
	}
	if _, exists := findInstallation(state.Installations, integrationID); exists {
		return domain.NewError(domain.ErrStateConflict, "integration already exists in state: "+integrationID, nil)
	}
	return nil
}

func splitBlockedTargetInstalls(planned []plannedTargetInstall) ([]plannedTargetInstall, []plannedTargetInstall) {
	ready := make([]plannedTargetInstall, 0, len(planned))
	blocked := make([]plannedTargetInstall, 0, len(planned))
	for _, target := range planned {
		if target.Plan.Blocking {
			blocked = append(blocked, target)
			continue
		}
		ready = append(ready, target)
	}
	return ready, blocked
}

func firstBlockedTargetInstallError(blocked []plannedTargetInstall) error {
	if len(blocked) == 0 {
		return domain.NewError(domain.ErrMutationApply, "no installable targets were planned", nil)
	}
	return domain.NewError(domain.ErrMutationApply, "planned mutation is blocked for target "+string(blocked[0].TargetID)+"; rerun with --dry-run to inspect manual steps", nil)
}

func partialAddSummary(manifest domain.IntegrationManifest, appliedTargets int) string {
	targetWord := "targets"
	if appliedTargets == 1 {
		targetWord = "target"
	}
	return fmt.Sprintf("Installed integration %q at version %s on %d %s.", manifest.IntegrationID, manifest.Version, appliedTargets, targetWord)
}

func blockedTargetIDs(blocked []plannedTargetInstall) []string {
	out := make([]string, 0, len(blocked))
	for _, target := range blocked {
		out = append(out, string(target.TargetID))
	}
	return out
}

func skippedBlockedTargetWarnings(integrationID string, blocked []plannedTargetInstall) []string {
	out := make([]string, 0, len(blocked))
	for _, target := range blocked {
		step := ""
		if len(target.Plan.ManualSteps) > 0 {
			step = target.Plan.ManualSteps[0]
		}
		if step == "" {
			step = fmt.Sprintf("run `plugin-kit-ai add %s --target %s --dry-run` to inspect the manual steps and retry later", integrationID, target.TargetID)
		}
		out = append(out, fmt.Sprintf("Skipped %q - %s.", target.TargetID, step))
	}
	return out
}
