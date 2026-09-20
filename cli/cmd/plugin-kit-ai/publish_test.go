package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
)

type fakePublishRunner struct {
	result app.PluginPublishResult
	err    error
	opts   app.PluginPublishOptions
}

func (f *fakePublishRunner) Publish(opts app.PluginPublishOptions) (app.PluginPublishResult, error) {
	f.opts = opts
	return f.result, f.err
}

func TestPublishDelegatesToRunner(t *testing.T) {
	t.Parallel()
	runner := &fakePublishRunner{
		result: app.PluginPublishResult{Lines: []string{"published"}},
	}
	cmd := newPublishCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{".", "--channel", "codex-marketplace", "--dest", "/tmp/market", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.opts.Channel != "codex-marketplace" || runner.opts.Dest != "/tmp/market" || runner.opts.Root != "." || !runner.opts.DryRun {
		t.Fatalf("opts = %+v", runner.opts)
	}
	if !strings.Contains(buf.String(), "published") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestPublishAllowsGeminiDryRunWithoutDest(t *testing.T) {
	t.Parallel()
	runner := &fakePublishRunner{
		result: app.PluginPublishResult{Lines: []string{"planned"}},
	}
	cmd := newPublishCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{".", "--channel", "gemini-gallery", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.opts.Channel != "gemini-gallery" || runner.opts.Dest != "" || !runner.opts.DryRun {
		t.Fatalf("opts = %+v", runner.opts)
	}
	if !strings.Contains(buf.String(), "planned") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestPublishAllDryRunDelegatesToRunner(t *testing.T) {
	t.Parallel()
	runner := &fakePublishRunner{
		result: app.PluginPublishResult{Lines: []string{"planned all"}},
	}
	cmd := newPublishCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{".", "--all", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !runner.opts.All || !runner.opts.DryRun || runner.opts.Channel != "" {
		t.Fatalf("opts = %+v", runner.opts)
	}
	if !strings.Contains(buf.String(), "planned all") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestPublishJSONEmitsVersionedContract(t *testing.T) {
	t.Parallel()
	runner := &fakePublishRunner{
		result: app.PluginPublishResult{
			Channel:       "gemini-gallery",
			Target:        "gemini",
			Ready:         false,
			Status:        "needs_repository",
			Mode:          "dry-run",
			WorkflowClass: "repository_release_plan",
			Details: map[string]string{
				"distribution": "github_release",
			},
			Issues: []app.PluginPublishIssue{
				{Code: "gemini_origin_remote_missing", Message: "missing origin"},
			},
			NextSteps: []string{"use gemini extensions link <path>"},
		},
	}
	cmd := newPublishCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{".", "--channel", "gemini-gallery", "--dry-run", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v\n%s", err, buf.Bytes())
	}
	if payload["format"] != "plugin-kit-ai/publish-report" {
		t.Fatalf("format = %+v", payload["format"])
	}
	if payload["schema_version"] != float64(1) {
		t.Fatalf("schema_version = %+v", payload["schema_version"])
	}
	if payload["workflow_class"] != "repository_release_plan" {
		t.Fatalf("workflow_class = %+v", payload["workflow_class"])
	}
	if payload["channel"] != "gemini-gallery" || payload["target"] != "gemini" || payload["mode"] != "dry-run" {
		t.Fatalf("payload = %+v", payload)
	}
	if payload["ready"] != false || payload["status"] != "needs_repository" {
		t.Fatalf("status payload = %+v", payload)
	}
	if payload["detail_count"] != float64(1) || payload["next_step_count"] != float64(1) || payload["issue_count"] != float64(1) {
		t.Fatalf("counts = %+v", payload)
	}
}

func TestPublishAllJSONEmitsMultiChannelContract(t *testing.T) {
	t.Parallel()
	runner := &fakePublishRunner{
		result: app.PluginPublishResult{
			Ready:         false,
			Status:        "needs_attention",
			Mode:          "dry-run",
			WorkflowClass: "multi_channel_plan",
			Warnings:      []string{"dest ignored for repository-only channels"},
			NextSteps:     []string{"review per-channel steps"},
			Channels: []app.PluginPublishResult{
				{Channel: "codex-marketplace", Target: "codex-package", Ready: true, Status: "ready", Mode: "dry-run", WorkflowClass: "local_marketplace_root", Details: map[string]string{"catalog_artifact": ".agents/plugins/marketplace.json"}, Issues: []app.PluginPublishIssue{}, NextSteps: []string{"run codex"}},
				{Channel: "gemini-gallery", Target: "gemini", Ready: false, Status: "needs_repository", Mode: "dry-run", WorkflowClass: "repository_release_plan", Details: map[string]string{"distribution": "git_repository"}, Issues: []app.PluginPublishIssue{{Code: "gemini_origin_remote_missing", Message: "missing origin"}}, NextSteps: []string{"add origin"}},
			},
		},
	}
	cmd := newPublishCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{".", "--all", "--dry-run", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v\n%s", err, buf.Bytes())
	}
	if payload["workflow_class"] != "multi_channel_plan" || payload["status"] != "needs_attention" || payload["ready"] != false {
		t.Fatalf("payload = %+v", payload)
	}
	if payload["channel_count"] != float64(2) || payload["warning_count"] != float64(1) {
		t.Fatalf("counts = %+v", payload)
	}
	channels, ok := payload["channels"].([]any)
	if !ok || len(channels) != 2 {
		t.Fatalf("channels = %+v", payload["channels"])
	}
	first, ok := channels[0].(map[string]any)
	if !ok || first["channel"] != "codex-marketplace" {
		t.Fatalf("first channel = %+v", channels[0])
	}
}

func TestPublishRejectsUnknownFormat(t *testing.T) {
	t.Parallel()
	runner := &fakePublishRunner{
		result: app.PluginPublishResult{Channel: "codex-marketplace", Target: "codex-package", Mode: "dry-run"},
	}
	cmd := newPublishCmd(runner)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{".", "--channel", "codex-marketplace", "--dest", "/tmp/market", "--format", "yaml"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `unsupported publish output format "yaml"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildPublishOptionsDefaultsRootAndCopiesFlags(t *testing.T) {
	t.Parallel()

	options, err := newPublishOptions(publishFlags{
		Channel:     "codex-marketplace",
		Dest:        "/tmp/market",
		PackageRoot: "plugins/demo",
		DryRun:      true,
		Format:      "json",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if options.Root != "." || options.Channel != "codex-marketplace" || options.Dest != "/tmp/market" || options.PackageRoot != "plugins/demo" || !options.DryRun {
		t.Fatalf("options = %+v", options)
	}
}

func TestPublishRejectsAllWithoutDryRun(t *testing.T) {
	t.Parallel()
	cmd := newPublishCmd(&fakePublishRunner{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{".", "--all"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "publish --all currently supports only --dry-run planning") {
		t.Fatalf("err = %v", err)
	}
}

func TestPublishRejectsAllWithChannel(t *testing.T) {
	t.Parallel()
	cmd := newPublishCmd(&fakePublishRunner{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{".", "--all", "--channel", "codex-marketplace", "--dry-run"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "publish --all cannot be combined with --channel") {
		t.Fatalf("err = %v", err)
	}
}

func TestPublishHelpMentionsBoundedChannels(t *testing.T) {
	t.Parallel()
	cmd := newPublishCmd(&fakePublishRunner{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		`publish channel ("codex-marketplace", "claude-marketplace", or "gemini-gallery")`,
		"plan across all authored publication channels (dry-run only)",
		"destination marketplace root directory for local Codex/Claude marketplace flows",
		"preview the materialized publish result without writing changes",
		`output format ("text" or "json")`,
		"codex-marketplace",
		"claude-marketplace",
		"gemini-gallery (dry-run plan only)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}
