package integrationctl

import "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"

type AddParams struct {
	Source          string
	Targets         []string
	Scope           string
	AutoUpdate      *bool
	AdoptNewTargets string
	AllowPrerelease *bool
	DryRun          bool
}

type UpdateParams struct {
	Name   string
	All    bool
	DryRun bool
}

type RemoveParams struct {
	Name   string
	DryRun bool
}

type RepairParams struct {
	Name   string
	Target string
	DryRun bool
}

type ToggleParams struct {
	Name   string
	Target string
	DryRun bool
}

type SyncParams struct {
	DryRun bool
}

type Result struct {
	OperationID string
	Summary     string
	Report      domain.Report
}

type Report = domain.Report

func ExitCodeFromErr(err error) int {
	return domain.ExitCodeFromErr(err)
}
