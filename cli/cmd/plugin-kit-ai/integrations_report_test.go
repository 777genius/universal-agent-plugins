package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/spf13/cobra"
)

func TestPrintIntegrationReportUsesHumanReadableTargetStatus(t *testing.T) {
	t.Parallel()

	report := integrationctl.Report{
		OperationID: "plan_add_notion_123",
		Summary:     `Dry-run plan for integration "notion" at version 0.1.1.`,
		Targets: []domain.TargetReport{
			{
				TargetID:          "claude",
				DeliveryKind:      "claude-marketplace-plugin",
				ActionClass:       "install_missing",
				State:             "installed",
				ActivationState:   "reload_pending",
				ManualSteps:       []string{"run /reload-plugins in Claude Code if the current session should pick up the new plugin immediately"},
				EvidenceKey:       "target.claude.native_surface",
				CapabilitySurface: []string{"hooks"},
			},
			{
				TargetID:                "codex",
				DeliveryKind:            "codex-marketplace-plugin",
				ActionClass:             "install_missing",
				State:                   "degraded",
				ActivationState:         "native_activation_pending",
				EnvironmentRestrictions: []string{"native_activation_required", "new_thread_required"},
				ManualSteps: []string{
					"open Codex Plugin Directory and install notion from the prepared personal marketplace",
					"after installation, start a new Codex thread before using the plugin",
				},
			},
			{
				TargetID:        "gemini",
				DeliveryKind:    "gemini-extension",
				ActionClass:     "install_missing",
				State:           "removed",
				ActivationState: "restart_pending",
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	printIntegrationReport(cmd, report)

	output := buf.String()
	for _, want := range []string{
		"🆔 Operation: plan_add_notion_123",
		`🧭 Dry-run plan for integration "notion" at version 0.1.1.`,
		"📊 Preview: 1 target(s) already present, 1 need attention, 1 would be newly installed",
		"✅ claude - will adopt existing Claude plugin",
		"  current - already installed",
		"  activation - reload the target app or plugin list after applying changes",
		"🟡 codex - will finish preparing Codex plugin",
		"  current - partially prepared and still needs activation in the target app",
		"  activation - complete the native install in the target app, then start a new thread or session",
		"  next - open Codex Plugin Directory and install notion from the prepared personal marketplace",
		"⬇️ gemini - will install Gemini extension",
		"  current - not currently installed",
		"  activation - restart the target app after applying changes",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("integration report missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "state=") || strings.Contains(output, "activation=") || strings.Contains(output, "evidence=") || strings.Contains(output, "restriction -") {
		t.Fatalf("integration report still exposes raw internal fields:\n%s", output)
	}
}

func TestPrintIntegrationReportUsesAppliedLanguageForCompletedAdd(t *testing.T) {
	t.Parallel()

	report := integrationctl.Report{
		OperationID: `add_heroku_123`,
		Summary:     `Installed integration "heroku" at version 0.1.1.`,
		Targets: []domain.TargetReport{
			{
				TargetID:        "claude",
				DeliveryKind:    "claude-marketplace-plugin",
				ActionClass:     "install_missing",
				State:           "installed",
				ActivationState: "reload_pending",
				ManualSteps:     []string{"run /reload-plugins in Claude Code if the current session should pick up the new plugin immediately"},
			},
			{
				TargetID:                "codex",
				DeliveryKind:            "codex-marketplace-plugin",
				ActionClass:             "install_missing",
				State:                   "activation_pending",
				ActivationState:         "native_activation_pending",
				EnvironmentRestrictions: []string{"native_activation_required", "new_thread_required"},
				ManualSteps: []string{
					"open Codex Plugin Directory and install heroku from the prepared personal marketplace",
					"after installation, start a new Codex thread before using the plugin",
				},
			},
			{
				TargetID:        "cursor",
				DeliveryKind:    "cursor-mcp",
				ActionClass:     "install_missing",
				State:           "installed",
				ActivationState: "not_required",
			},
			{
				TargetID:        "gemini",
				DeliveryKind:    "gemini-extension",
				ActionClass:     "install_missing",
				State:           "installed",
				ActivationState: "restart_pending",
				ManualSteps:     []string{"restart Gemini CLI to load the updated extension and merged configuration"},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	printIntegrationReport(cmd, report)

	output := buf.String()
	for _, want := range []string{
		`✅ Installed integration "heroku" at version 0.1.1.`,
		"📊 Progress: [████████] 4/4 target(s) changed successfully",
		"📝 Changes were written successfully, but some targets are only prepared until you finish the steps below.",
		"🚀 Ready now: claude, cursor",
		"🔄 Restart or reload: gemini",
		"🧩 Finish setup: codex",
		"✅ claude - installed Claude plugin",
		"  ready - available now",
		"  tip - run /reload-plugins in Claude Code if the current session should pick up the new plugin immediately",
		"🟡 codex - prepared Codex plugin for in-app install",
		"  finish - open Codex Plugin Directory and install heroku from the prepared personal marketplace",
		"  finish - after installation, start a new Codex thread before using the plugin",
		"✅ cursor - installed Cursor MCP setup",
		"  ready - available now",
		"🟡 gemini - installed Gemini extension",
		"  restart - restart Gemini CLI to load the updated extension and merged configuration",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("applied integration report missing %q:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{"current -", "activation -", "next -", "state=", "activation=", "restriction -"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("applied integration report still exposes planning/internal wording %q:\n%s", unwanted, output)
		}
	}
}

func TestPrintIntegrationReportUsesHumanReadablePartialInstallSummary(t *testing.T) {
	t.Parallel()

	report := integrationctl.Report{
		OperationID:          `add_linear_123`,
		Summary:              `Installed integration "linear" at version 0.1.1 on 4 targets.`,
		RequestedTargetCount: 5,
		SkippedTargets:       []string{"claude"},
		Warnings: []string{
			`Skipped "claude" - Claude Code CLI is not available on PATH; install/configure Claude Code and rerun from a shell where ` + "`claude` works.",
		},
		Targets: []domain.TargetReport{
			{
				TargetID:                "codex",
				DeliveryKind:            "codex-marketplace-plugin",
				ActionClass:             "install_missing",
				State:                   "activation_pending",
				ActivationState:         "native_activation_pending",
				EnvironmentRestrictions: []string{"native_activation_required", "new_thread_required"},
				ManualSteps: []string{
					"open Codex Plugin Directory and install linear from the prepared personal marketplace",
					"after installation, start a new Codex thread before using the plugin",
				},
			},
			{
				TargetID:        "cursor",
				DeliveryKind:    "cursor-mcp",
				ActionClass:     "install_missing",
				State:           "installed",
				ActivationState: "not_required",
			},
			{
				TargetID:        "gemini",
				DeliveryKind:    "gemini-extension",
				ActionClass:     "install_missing",
				State:           "installed",
				ActivationState: "restart_pending",
				ManualSteps:     []string{"restart Gemini CLI to load the updated extension and merged configuration"},
			},
			{
				TargetID:        "opencode",
				DeliveryKind:    "opencode-plugin",
				ActionClass:     "install_missing",
				State:           "installed",
				ActivationState: "restart_pending",
				ManualSteps:     []string{"restart OpenCode to pick up updated config and projected files"},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	printIntegrationReport(cmd, report)

	output := buf.String()
	for _, want := range []string{
		`✅ Installed integration "linear" at version 0.1.1 on 4 targets.`,
		"📊 Applied changes: [██████░░] 4/5 requested target(s)",
		"⏭️ Skipped: claude",
		"🚀 Ready now: cursor",
		"🔄 Restart or reload: gemini, opencode",
		"🧩 Finish setup: codex",
		`⚠️ Warning: Skipped "claude" - Claude Code CLI is not available on PATH; install/configure Claude Code and rerun from a shell where ` + "`claude` works.",
		"🟡 codex - prepared Codex plugin for in-app install",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("partial install report missing %q:\n%s", want, output)
		}
	}
}

func TestPrintIntegrationReportGroupsNeutralTargetsByIntegration(t *testing.T) {
	t.Parallel()

	report := integrationctl.Report{
		Summary: `3 managed integration(s) in state.`,
		Targets: []domain.TargetReport{
			{
				IntegrationID:   "gitlab",
				TargetID:        "claude",
				DeliveryKind:    "claude-marketplace-plugin",
				State:           "installed",
				ActivationState: "reload_pending",
			},
			{
				IntegrationID:   "gitlab",
				TargetID:        "cursor",
				DeliveryKind:    "cursor-mcp",
				State:           "installed",
				ActivationState: "not_required",
			},
			{
				IntegrationID:   "notion",
				TargetID:        "claude",
				DeliveryKind:    "claude-marketplace-plugin",
				State:           "degraded",
				ActivationState: "reload_pending",
				ManualSteps:     []string{"run plugin-kit-ai integrations repair notion"},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	printIntegrationReport(cmd, report)

	output := buf.String()
	for _, want := range []string{
		`ℹ️ 3 managed integration(s) in state.`,
		"📦 gitlab",
		"  ✅ claude - Claude plugin",
		"    current - already installed",
		"  ✅ cursor - Cursor MCP setup",
		"📦 notion",
		"  🟡 claude - Claude plugin",
		"    current - partially installed and may need repair",
		"    next - run plugin-kit-ai integrations repair notion",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("grouped integration report missing %q:\n%s", want, output)
		}
	}
}

func TestPrintIntegrationReportUsesFinishLabelForRemoveManualSteps(t *testing.T) {
	t.Parallel()

	report := integrationctl.Report{
		OperationID: `remove_linear_123`,
		Summary:     `Removed managed targets from integration "linear".`,
		Targets: []domain.TargetReport{
			{
				TargetID:        "codex",
				DeliveryKind:    "codex-marketplace-plugin",
				ActionClass:     "remove_orphaned_target",
				State:           "removed",
				ActivationState: "native_activation_pending",
				ManualSteps: []string{
					"if linear was already installed in Codex, uninstall it from the Codex Plugin Directory",
					"restart Codex after removing the plugin bundle",
				},
			},
		},
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	printIntegrationReport(cmd, report)

	output := buf.String()
	for _, want := range []string{
		`🗑️ Removed managed targets from integration "linear".`,
		"🟡 codex - removed Codex plugin",
		"  finish - if linear was already installed in Codex, uninstall it from the Codex Plugin Directory",
		"  finish - restart Codex after removing the plugin bundle",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("remove integration report missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "  activate -") {
		t.Fatalf("remove integration report still uses activate label:\n%s", output)
	}
}
