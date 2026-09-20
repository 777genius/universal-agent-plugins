package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/spf13/cobra"
)

func TestRunIntegrationResultActionPrintsStartLineBeforeReport(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := runIntegrationResultAction(cmd, `Installing integration "notion" across managed targets...`, integrationFailureContext{
		Action: "add",
		Name:   "notion",
	}, nil, func(ctx context.Context) (integrationctl.Result, error) {
		return integrationctl.Result{
			Report: domain.Report{
				OperationID: "add_notion_123",
				Summary:     `Installed integration "notion" at version 0.1.1.`,
				Targets: []domain.TargetReport{
					{TargetID: "cursor", DeliveryKind: "cursor-mcp", State: "installed"},
				},
			},
		}, nil
	})
	if err != nil {
		t.Fatalf("runIntegrationResultAction error = %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		`⏳ Installing integration "notion" across managed targets...`,
		`✅ Installed integration "notion" at version 0.1.1.`,
		"📊 Progress: [████████] 1/1 target(s) changed successfully",
		"🚀 Ready now: cursor",
		"✅ cursor - installed Cursor MCP setup",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("run output missing %q:\n%s", want, output)
		}
	}
}

func TestIntegrationStartLineHelpersRespectDryRun(t *testing.T) {
	t.Parallel()

	if got := integrationStartLineForAdd(integrationctl.AddParams{Source: "notion", DryRun: true}); got != "" {
		t.Fatalf("dry-run add start line = %q", got)
	}
	if got := integrationStartLineForAdd(integrationctl.AddParams{Source: "notion"}); got != `Installing integration "notion" across managed targets...` {
		t.Fatalf("add start line = %q", got)
	}
	if got := integrationStartLineForUpdate(integrationctl.UpdateParams{All: true}); got != "Updating all managed integrations..." {
		t.Fatalf("update all start line = %q", got)
	}
	if got := integrationStartLineForRepair(integrationctl.RepairParams{Name: "notion", Target: "codex"}); got != `Repairing managed integration "notion" for target "codex"...` {
		t.Fatalf("repair target start line = %q", got)
	}
	if got := integrationStartLineForToggle("Enabling", "notion", "cursor", false); got != `Enabling managed integration "notion" for target "cursor"...` {
		t.Fatalf("toggle target start line = %q", got)
	}
}

func TestRunIntegrationResultActionFormatsExistingIntegrationConflict(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := runIntegrationResultAction(cmd, `Installing integration "notion" across managed targets...`, integrationFailureContext{
		Action: "add",
		Name:   "notion",
	}, nil, func(ctx context.Context) (integrationctl.Result, error) {
		return integrationctl.Result{}, domain.NewError(domain.ErrStateConflict, "integration already exists in state: notion", nil)
	})
	if err == nil {
		t.Fatal("expected error")
	}

	output := stdout.String() + stderr.String()
	for _, want := range []string{
		`⏳ Installing integration "notion" across managed targets...`,
		`❌ Integration "notion" is already managed.`,
		"💡 Try `plugin-kit-ai update notion` to refresh it, or `plugin-kit-ai integrations list` to inspect current state.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("formatted conflict output missing %q:\n%s", want, output)
		}
	}
}

func TestRunIntegrationResultActionFormatsUnsupportedTarget(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := runIntegrationResultAction(cmd, `Installing integration "slack" across managed targets...`, integrationFailureContext{
		Action: "add",
		Name:   "slack",
	}, nil, func(ctx context.Context) (integrationctl.Result, error) {
		return integrationctl.Result{}, domain.NewError(domain.ErrUnsupportedTarget, "manifest does not expose target codex", nil)
	})
	if err == nil {
		t.Fatal("expected error")
	}

	output := stdout.String() + stderr.String()
	for _, want := range []string{
		`⏳ Installing integration "slack" across managed targets...`,
		`❌ "slack" does not support target "codex".`,
		"💡 Run `plugin-kit-ai add slack --dry-run` without `--target` to inspect the targets it supports.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("formatted unsupported-target output missing %q:\n%s", want, output)
		}
	}
}

func TestRunIntegrationResultActionFormatsMissingManagedIntegration(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := runIntegrationResultAction(cmd, `Removing managed integration "vercel"...`, integrationFailureContext{
		Action: "remove",
		Name:   "vercel",
	}, nil, func(ctx context.Context) (integrationctl.Result, error) {
		return integrationctl.Result{}, domain.NewError(domain.ErrStateConflict, "integration not found in state: vercel", nil)
	})
	if err == nil {
		t.Fatal("expected error")
	}

	output := stdout.String() + stderr.String()
	for _, want := range []string{
		`⏳ Removing managed integration "vercel"...`,
		`❌ Integration "vercel" is not managed yet.`,
		"💡 Run `plugin-kit-ai integrations list` to inspect managed integrations before updating, repairing, or removing one.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("formatted missing-integration output missing %q:\n%s", want, output)
		}
	}
}

func TestRunIntegrationResultActionFormatsPartialProgressFailure(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := runIntegrationResultAction(cmd, `Updating managed integration "notion"...`, integrationFailureContext{
		Action: "update",
		Name:   "notion",
	}, nil, func(ctx context.Context) (integrationctl.Result, error) {
		return integrationctl.Result{}, domain.NewError(domain.ErrMutationApply, "update failed after partial progress; degraded state persisted", nil)
	})
	if err == nil {
		t.Fatal("expected error")
	}

	output := stdout.String() + stderr.String()
	for _, want := range []string{
		`⏳ Updating managed integration "notion"...`,
		`❌ Update for "notion" failed after partial progress.`,
		"💡 Run `plugin-kit-ai integrations doctor` to inspect degraded targets and open operations.",
		"💡 Then run `plugin-kit-ai repair notion`.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("formatted partial-progress output missing %q:\n%s", want, output)
		}
	}
}

func TestRunIntegrationResultActionPrintsBlockedPlan(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := runIntegrationResultAction(cmd, `Updating managed integration "notion"...`, integrationFailureContext{
		Action: "update",
		Name:   "notion",
	}, func(ctx context.Context) (integrationctl.Report, error) {
		return integrationctl.Report{
			OperationID: `plan_update_version_notion_123`,
			Summary:     `Dry-run update_version plan for "notion".`,
			Targets: []domain.TargetReport{
				{
					IntegrationID:           "notion",
					TargetID:                "claude",
					DeliveryKind:            "claude-marketplace-plugin",
					ActionClass:             "update_version",
					State:                   "degraded",
					ActivationState:         "reload_pending",
					EnvironmentRestrictions: []string{"source_tool_missing"},
					ManualSteps:             []string{"Claude Code CLI is not available on PATH; install/configure Claude Code and rerun from a shell where `claude` works"},
				},
			},
		}, nil
	}, func(ctx context.Context) (integrationctl.Result, error) {
		return integrationctl.Result{}, domain.NewError(domain.ErrMutationApply, "planned mutation is blocked for target claude; rerun with --dry-run to inspect manual steps", nil)
	})
	if err == nil {
		t.Fatal("expected error")
	}

	output := stdout.String() + stderr.String()
	for _, want := range []string{
		`⏳ Updating managed integration "notion"...`,
		`❌ Update for "notion" is blocked before changes.`,
		"🔎 Review the blocked plan below.",
		`🧭 Dry-run update_version plan for "notion".`,
		"🚫 claude - will update Claude plugin",
		"  current - partially installed and may need repair",
		"  next - Claude Code CLI is not available on PATH; install/configure Claude Code and rerun from a shell where `claude` works",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("formatted blocked-mutation output missing %q:\n%s", want, output)
		}
	}
}
