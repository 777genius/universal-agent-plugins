package integrationctl

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/usecase"
)

func Add(ctx context.Context, p AddParams) (Result, error) {
	svc, err := newService()
	if err != nil {
		return Result{}, err
	}
	report, err := svc.Add(ctx, usecase.AddInput{
		Source:          p.Source,
		Targets:         p.Targets,
		Scope:           p.Scope,
		AutoUpdate:      p.AutoUpdate,
		AdoptNewTargets: p.AdoptNewTargets,
		AllowPrerelease: p.AllowPrerelease,
		DryRun:          p.DryRun,
	})
	if err != nil {
		return Result{}, err
	}
	return wrapResult(report), nil
}

func Update(ctx context.Context, p UpdateParams) (Result, error) {
	svc, err := newService()
	if err != nil {
		return Result{}, err
	}
	var (
		report domain.Report
	)
	if p.All {
		report, err = svc.UpdateAll(ctx, p.DryRun)
	} else {
		report, err = svc.Update(ctx, usecase.NamedDryRunInput{Name: p.Name, DryRun: p.DryRun})
	}
	if err != nil {
		return Result{}, err
	}
	return wrapResult(report), nil
}

func Remove(ctx context.Context, p RemoveParams) (Result, error) {
	svc, err := newService()
	if err != nil {
		return Result{}, err
	}
	report, err := svc.Remove(ctx, usecase.NamedDryRunInput{Name: p.Name, DryRun: p.DryRun})
	if err != nil {
		return Result{}, err
	}
	return wrapResult(report), nil
}

func Repair(ctx context.Context, p RepairParams) (Result, error) {
	svc, err := newService()
	if err != nil {
		return Result{}, err
	}
	report, err := svc.Repair(ctx, usecase.NamedDryRunInput{Name: p.Name, Target: p.Target, DryRun: p.DryRun})
	if err != nil {
		return Result{}, err
	}
	return wrapResult(report), nil
}

func Enable(ctx context.Context, p ToggleParams) (Result, error) {
	svc, err := newService()
	if err != nil {
		return Result{}, err
	}
	report, err := svc.Enable(ctx, usecase.NamedDryRunInput{Name: p.Name, Target: p.Target, DryRun: p.DryRun})
	if err != nil {
		return Result{}, err
	}
	return wrapResult(report), nil
}

func Disable(ctx context.Context, p ToggleParams) (Result, error) {
	svc, err := newService()
	if err != nil {
		return Result{}, err
	}
	report, err := svc.Disable(ctx, usecase.NamedDryRunInput{Name: p.Name, Target: p.Target, DryRun: p.DryRun})
	if err != nil {
		return Result{}, err
	}
	return wrapResult(report), nil
}

func Sync(ctx context.Context, p SyncParams) (Result, error) {
	svc, err := newService()
	if err != nil {
		return Result{}, err
	}
	report, err := svc.Sync(ctx, p.DryRun)
	if err != nil {
		return Result{}, err
	}
	return wrapResult(report), nil
}

func List(ctx context.Context) (domain.Report, error) {
	svc, err := newService()
	if err != nil {
		return domain.Report{}, err
	}
	return svc.List(ctx)
}

func Doctor(ctx context.Context) (domain.Report, error) {
	svc, err := newService()
	if err != nil {
		return domain.Report{}, err
	}
	return svc.Doctor(ctx)
}

func wrapResult(report domain.Report) Result {
	return Result{
		OperationID: report.OperationID,
		Summary:     report.Summary,
		Report:      report,
	}
}
