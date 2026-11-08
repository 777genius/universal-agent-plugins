package commands

import (
	"context"
	"errors"

	authorbootstrap "github.com/777genius/plugin-kit-ai/cli/internal/authoring/bootstrap"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/mcpruntime"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/readiness"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
)

// runCommand owns the authoring operation; Cobra has already decoded its input.
func (a App) runCommand(ctx context.Context, name string, req request) (report.Report, error) {
	if name == "capabilities" {
		r := report.New(name, a.Revision)
		names := a.commandNames()
		if a.PublicContract {
			names = req.inventory
		}
		c, err := readiness.Engine(names)
		r.Capabilities = c
		if err != nil {
			r.AddError("capabilities_unavailable", "Check the embedded engine metadata.")
		}
		return r, err
	}
	if name == "init" {
		return a.init(ctx, req)
	}
	if name == "dev" && !req.once {
		return a.dev(ctx, req)
	}
	p, e := a.Projects.Read(ctx, req.root)
	r := report.Build(name, a.Revision, p, req.release)
	if req.disclose {
		r.Root = req.root
	}
	if e != nil {
		code, action := failure(e, "read")
		r.AddError(code, action)
		return r, e
	}
	if name == "bootstrap" {
		return a.runBootstrap(ctx, req, p, r)
	}
	if len(req.targets) > 0 {
		compatibility, err := readiness.Compatibility(a.ClientRegistry, p, req.targets)
		if err != nil {
			r.AddError("compatibility_unavailable", "Select explicit supported clients.")
			return r, err
		}
		r.AddCompatibility(compatibility)
	}
	if name == "doctor" {
		r.AddDoctor(readiness.Doctor(p))
	}
	if req.runtime == "mcp" {
		if err := a.runMCP(ctx, req, p, &r); err != nil {
			return r, err
		}
	}
	if !r.Successful() {
		return r, errors.New("authoring checks incomplete or failed")
	}
	return r, nil
}

// runBootstrap preserves the read-only planning and mutating apply paths,
// reporting commit and cleanup status before returning any failure.
func (a App) runBootstrap(ctx context.Context, req request, p project.Result, r report.Report) (report.Report, error) {
	if req.dryRun {
		r.Mode = "read"
	} else {
		r.Mode = "local_mutation"
	}
	if !r.Successful() {
		planErr := &authorbootstrap.Error{Code: "bootstrap_template_unrecognized"}
		code, action := bootstrapFailure(planErr)
		r.AddError(code, action)
		return r, planErr
	}
	plan, planErr := (authorbootstrap.Service{Runner: a.BootstrapRunner}).Plan(req.root, p)
	r.Bootstrap = &report.BootstrapDetail{Runtime: plan.Runtime, Manager: plan.Manager, Argv: append([]string{}, plan.Command...), Planned: planErr == nil}
	if planErr != nil {
		code, action := bootstrapFailure(planErr)
		r.AddError(code, action)
		return r, planErr
	}
	if req.dryRun {
		closeErr := plan.Close()
		if closeErr != nil {
			code, action := bootstrapFailure(closeErr)
			r.AddError(code, action)
		}
		return r, closeErr
	}
	committed, applyErr := (authorbootstrap.Service{Runner: a.BootstrapRunner}).Apply(ctx, req.root, plan)
	r.Committed = committed
	if committed {
		r.Paths = append(r.Paths, "node_modules")
	}
	if applyErr != nil {
		code, action := bootstrapFailure(applyErr)
		if errors.Is(applyErr, context.Canceled) || errors.Is(applyErr, context.DeadlineExceeded) {
			code, action = failure(applyErr, "bootstrap")
		}
		r.AddError(code, action)
		return r, applyErr
	}
	return r, nil
}

// runMCP records the selected runtime stages and cleanup evidence even if
// execution fails, then returns the original runtime error.
func (a App) runMCP(ctx context.Context, req request, p project.Result, r *report.Report) error {
	evidence, runtimeErr := mcpruntime.Run(ctx, mcpruntime.Options{SourceRoot: req.root, Scratch: a.Projects.Scratch, Project: p, Server: req.server, Tool: req.tool, Fixture: req.fixture, AllowNetwork: req.allowNetwork, Deadline: req.deadline, Projects: a.Projects})
	r.SetRuntime(evidence.Transport, evidence.Initialize, evidence.ListTools, evidence.ToolCall, evidence.ToolCount, evidence.Cleanup, runtimeErrorCode(runtimeErr))
	return runtimeErr
}
