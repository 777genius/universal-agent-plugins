package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

type fakePublicationRunner struct {
	fakeInspectRunner
	result       app.PluginPublicationMaterializeResult
	removeResult app.PluginPublicationRemoveResult
	verifyResult app.PluginPublicationVerifyRootResult
	err          error
	opts         app.PluginPublicationMaterializeOptions
	removeOpts   app.PluginPublicationRemoveOptions
	verifyOpts   app.PluginPublicationVerifyRootOptions
}

func (f *fakePublicationRunner) PublicationMaterialize(opts app.PluginPublicationMaterializeOptions) (app.PluginPublicationMaterializeResult, error) {
	f.opts = opts
	return f.result, f.err
}

func (f *fakePublicationRunner) PublicationRemove(opts app.PluginPublicationRemoveOptions) (app.PluginPublicationRemoveResult, error) {
	f.removeOpts = opts
	return f.removeResult, f.err
}

func (f *fakePublicationRunner) PublicationVerifyRoot(opts app.PluginPublicationVerifyRootOptions) (app.PluginPublicationVerifyRootResult, error) {
	f.verifyOpts = opts
	return f.verifyResult, f.err
}

func TestPublicationTextShowsPackagesAndChannels(t *testing.T) {
	t.Parallel()
	cmd := newPublicationCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion:  "v1",
					Name:        "demo",
					Version:     "0.1.0",
					Description: "demo plugin",
				},
				Packages: []publicationmodel.Package{
					{
						Target:          "codex-package",
						PackageFamily:   "codex-plugin",
						ChannelFamilies: []string{"codex-marketplace"},
						AuthoredInputs:  []string{"plugin/plugin.yaml", "plugin/publish/codex/marketplace.yaml"},
						ManagedArtifacts: []string{
							".codex-plugin/plugin.json",
							".agents/plugins/marketplace.json",
						},
					},
				},
				Channels: []publicationmodel.Channel{
					{
						Family:         "codex-marketplace",
						Path:           "plugin/publish/codex/marketplace.yaml",
						PackageTargets: []string{"codex-package"},
						Details: map[string]string{
							"marketplace_name":      "local-repo",
							"source_root":           "./",
							"category":              "Productivity",
							"installation_policy":   "AVAILABLE",
							"authentication_policy": "ON_INSTALL",
						},
					},
				},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--format", "text", "."})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"publication demo 0.1.0 api_version=v1",
		"packages: 1 channels: 1",
		"package[codex-package]: family=codex-plugin channels=codex-marketplace inputs=2 managed=2",
		"channel[codex-marketplace]: path=plugin/publish/codex/marketplace.yaml targets=codex-package",
		"details=authentication_policy=ON_INSTALL,category=Productivity,installation_policy=AVAILABLE,marketplace_name=local-repo,source_root=./",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("publication output missing %q:\n%s", want, output)
		}
	}
}

func TestPublicationJSONEmitsPublicationModelOnly(t *testing.T) {
	t.Parallel()
	cmd := newPublicationCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion:  "v1",
					Name:        "gemini-publish",
					Version:     "0.1.0",
					Description: "gemini publish",
				},
				Packages: []publicationmodel.Package{
					{
						Target:          "gemini",
						PackageFamily:   "gemini-extension",
						ChannelFamilies: []string{"gemini-gallery"},
					},
				},
				Channels: []publicationmodel.Channel{
					{
						Family:         "gemini-gallery",
						Path:           "plugin/publish/gemini/gallery.yaml",
						PackageTargets: []string{"gemini"},
						Details: map[string]string{
							"distribution":          "github_release",
							"repository_visibility": "public",
							"github_topic":          "gemini-cli-extension",
							"manifest_root":         "release_archive_root",
						},
					},
				},
			},
		},
		warnings: []pluginmanifest.Warning{
			{Message: "plugin/publish/gemini/gallery.yaml is discoverable"},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--format", "json", "."})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v\n%s", err, buf.Bytes())
	}
	if payload["format"] != "plugin-kit-ai/publication-report" {
		t.Fatalf("format = %+v", payload["format"])
	}
	if payload["schema_version"] != float64(1) {
		t.Fatalf("schema_version = %+v", payload["schema_version"])
	}
	if payload["warning_count"] != float64(1) {
		t.Fatalf("warning_count = %+v", payload["warning_count"])
	}
	warnings, ok := payload["warnings"].([]any)
	if !ok || len(warnings) != 1 || warnings[0] != "plugin/publish/gemini/gallery.yaml is discoverable" {
		t.Fatalf("warnings = %+v", payload["warnings"])
	}
	publication, ok := payload["publication"].(map[string]any)
	if !ok || publication["core"] == nil || publication["packages"] == nil || publication["channels"] == nil {
		t.Fatalf("publication payload = %+v", payload)
	}
	if _, found := payload["core"]; found {
		t.Fatalf("publication json should use envelope shape, not raw model: %+v", payload)
	}
}

func TestPublicationHelpMentionsSupportedTargets(t *testing.T) {
	t.Parallel()
	cmd := newPublicationCmd(fakeInspectRunner{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		`publication target ("all", "claude", "codex-package", or "gemini")`,
		`output format: text or json`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestPublicationHelpMentionsMaterialize(t *testing.T) {
	t.Parallel()
	runner := &fakePublicationRunner{}
	cmd := newPublicationCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "materialize") {
		t.Fatalf("help missing materialize subcommand:\n%s", output)
	}
	if !strings.Contains(output, "remove") {
		t.Fatalf("help missing remove subcommand:\n%s", output)
	}
	if !strings.Contains(output, "materialize") || !strings.Contains(output, "remove") {
		t.Fatalf("unexpected help output:\n%s", output)
	}
}

func TestPublicationDoctorReturnsExitCodeOneWhenChannelsAreMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gemini-extension.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newPublicationDoctorCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion: "v1",
					Name:       "demo",
					Version:    "0.1.0",
				},
				Packages: []publicationmodel.Package{
					{
						Target:          "gemini",
						PackageFamily:   "gemini-extension",
						ChannelFamilies: []string{"gemini-gallery"},
						ManagedArtifacts: []string{
							"gemini-extension.json",
						},
					},
				},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{root})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	output := buf.String()
	for _, want := range []string{
		"Issue[missing_channel]: target gemini requires authored gemini-gallery at plugin/publish/gemini/gallery.yaml",
		"Status: needs_channels",
		"Next:",
		"add plugin/publish/gemini/gallery.yaml",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("publication doctor output missing %q:\n%s", want, output)
		}
	}
}

func TestPublicationDoctorReportsReadyWhenChannelsExist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationTestFile(t, root, filepath.Join(".codex-plugin", "plugin.json"), "{}\n")
	mustWritePublicationTestFile(t, root, filepath.Join(".agents", "plugins", "marketplace.json"), "{}\n")
	cmd := newPublicationDoctorCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion: "v1",
					Name:       "demo",
					Version:    "0.1.0",
				},
				Packages: []publicationmodel.Package{
					{
						Target:           "codex-package",
						PackageFamily:    "codex-plugin",
						ChannelFamilies:  []string{"codex-marketplace"},
						ManagedArtifacts: []string{".codex-plugin/plugin.json", ".agents/plugins/marketplace.json"},
					},
				},
				Channels: []publicationmodel.Channel{
					{
						Family:         "codex-marketplace",
						Path:           "plugin/publish/codex/marketplace.yaml",
						PackageTargets: []string{"codex-package"},
						Details: map[string]string{
							"marketplace_name": "local-repo",
						},
					},
				},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"Status: ready",
		"Channel[codex-marketplace]: path=plugin/publish/codex/marketplace.yaml targets=codex-package",
		"run plugin-kit-ai validate . --strict",
		"run plugin-kit-ai publication . --format json",
		"run plugin-kit-ai publication materialize . --target codex-package --dest <marketplace-root> --dry-run",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("publication doctor output missing %q:\n%s", want, output)
		}
	}
}

func TestPublicationDoctorReportsGeminiReadyHints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationTestFile(t, root, "gemini-extension.json", "{}\n")
	if err := exec.Command("git", "-C", root, "init").Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	if err := exec.Command("git", "-C", root, "remote", "add", "origin", "https://github.com/acme/demo.git").Run(); err != nil {
		t.Fatal(err)
	}
	cmd := newPublicationDoctorCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion: "v1",
					Name:       "demo",
					Version:    "0.1.0",
				},
				Packages: []publicationmodel.Package{
					{
						Target:           "gemini",
						PackageFamily:    "gemini-extension",
						ChannelFamilies:  []string{"gemini-gallery"},
						ManagedArtifacts: []string{"gemini-extension.json"},
					},
				},
				Channels: []publicationmodel.Channel{
					{
						Family:         "gemini-gallery",
						Path:           "plugin/publish/gemini/gallery.yaml",
						PackageTargets: []string{"gemini"},
						Details: map[string]string{
							"distribution":          "github_release",
							"github_topic":          "gemini-cli-extension",
							"manifest_root":         "release_archive_root",
							"repository_visibility": "public",
						},
					},
				},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target", "gemini", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"Status: ready",
		"confirm the GitHub repository stays public and tagged with the gemini-cli-extension topic",
		"ensure GitHub release archives keep gemini-extension.json at the archive root",
		"use gemini extensions link <path> for live Gemini CLI verification before publishing",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("publication doctor output missing %q:\n%s", want, output)
		}
	}
}

func TestPublicationDoctorReportsGeminiRepositoryIssues(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationTestFile(t, root, "gemini-extension.json", "{}\n")
	cmd := newPublicationDoctorCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion: "v1",
					Name:       "demo",
					Version:    "0.1.0",
				},
				Packages: []publicationmodel.Package{
					{
						Target:           "gemini",
						PackageFamily:    "gemini-extension",
						ChannelFamilies:  []string{"gemini-gallery"},
						ManagedArtifacts: []string{"gemini-extension.json"},
					},
				},
				Channels: []publicationmodel.Channel{
					{
						Family:         "gemini-gallery",
						Path:           "plugin/publish/gemini/gallery.yaml",
						PackageTargets: []string{"gemini"},
						Details: map[string]string{
							"distribution":          "git_repository",
							"github_topic":          "gemini-cli-extension",
							"manifest_root":         "repository_root",
							"repository_visibility": "public",
						},
					},
				},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target", "gemini", root})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	output := buf.String()
	for _, want := range []string{
		"Issue[gemini_git_repository_missing]",
		"Issue[gemini_origin_remote_missing]",
		"Status: needs_repository",
		"initialize a Git repository for this plugin before publishing to the Gemini gallery",
		"add a GitHub origin remote for this plugin repository before publishing",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("publication doctor output missing %q:\n%s", want, output)
		}
	}
}

func TestPublicationDoctorGeminiReadyInGitHubRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationTestFile(t, root, "gemini-extension.json", "{}\n")
	if err := exec.Command("git", "-C", root, "init").Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	if err := exec.Command("git", "-C", root, "remote", "add", "origin", "https://github.com/acme/demo.git").Run(); err != nil {
		t.Fatal(err)
	}
	cmd := newPublicationDoctorCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion: "v1",
					Name:       "demo",
					Version:    "0.1.0",
				},
				Packages: []publicationmodel.Package{
					{
						Target:           "gemini",
						PackageFamily:    "gemini-extension",
						ChannelFamilies:  []string{"gemini-gallery"},
						ManagedArtifacts: []string{"gemini-extension.json"},
					},
				},
				Channels: []publicationmodel.Channel{
					{
						Family:         "gemini-gallery",
						Path:           "plugin/publish/gemini/gallery.yaml",
						PackageTargets: []string{"gemini"},
						Details: map[string]string{
							"distribution":          "git_repository",
							"github_topic":          "gemini-cli-extension",
							"manifest_root":         "repository_root",
							"repository_visibility": "public",
						},
					},
				},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target", "gemini", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if strings.Contains(output, "needs_repository") || strings.Contains(output, "Issue[gemini_") {
		t.Fatalf("unexpected repository issues:\n%s", output)
	}
}

func TestPublicationDoctorHelpIncludesReadOnlyReadinessCheck(t *testing.T) {
	t.Parallel()
	cmd := newPublicationDoctorCmd(fakeInspectRunner{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"Read-only publication readiness check",
		`publication target ("all", "claude", "codex-package", or "gemini")`,
		"output format: text or json",
		"optional materialized marketplace root to verify for local codex-package or claude publication flows",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestPublicationDoctorJSONIncludesLocalRootVerification(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationTestFile(t, root, filepath.Join(".codex-plugin", "plugin.json"), "{}\n")
	mustWritePublicationTestFile(t, root, filepath.Join(".agents", "plugins", "marketplace.json"), "{}\n")
	runner := &fakePublicationRunner{
		fakeInspectRunner: fakeInspectRunner{
			report: pluginmanifest.Inspection{
				Publication: publicationmodel.Model{
					Core: publicationmodel.Core{
						APIVersion:  "v1",
						Name:        "demo",
						Version:     "0.1.0",
						Description: "demo plugin",
					},
					Packages: []publicationmodel.Package{
						{
							Target:           "codex-package",
							PackageFamily:    "codex-plugin",
							ChannelFamilies:  []string{"codex-marketplace"},
							ManagedArtifacts: []string{".codex-plugin/plugin.json", ".agents/plugins/marketplace.json"},
						},
					},
					Channels: []publicationmodel.Channel{
						{
							Family:         "codex-marketplace",
							Path:           "plugin/publish/codex/marketplace.yaml",
							PackageTargets: []string{"codex-package"},
						},
					},
				},
			},
		},
		verifyResult: app.PluginPublicationVerifyRootResult{
			Ready:       false,
			Status:      "needs_sync",
			Dest:        "/tmp/market",
			PackageRoot: "plugins/demo",
			CatalogPath: ".agents/plugins/marketplace.json",
			IssueCount:  1,
			Issues: []app.PluginPublicationRootIssue{{
				Code:    "missing_materialized_catalog_entry",
				Path:    "plugins",
				Message: "catalog entry for plugin demo is missing",
			}},
			NextSteps: []string{
				"run plugin-kit-ai publication materialize . --target codex-package --dest /tmp/market",
			},
		},
	}
	cmd := newPublicationDoctorCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--format", "json", "--target", "codex-package", "--dest", "/tmp/market", root})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if runner.verifyOpts.Target != "codex-package" || runner.verifyOpts.Dest != "/tmp/market" || runner.verifyOpts.Root != root {
		t.Fatalf("verify opts = %+v", runner.verifyOpts)
	}
	var payload map[string]any
	if parseErr := json.Unmarshal(buf.Bytes(), &payload); parseErr != nil {
		t.Fatalf("json parse: %v\n%s", parseErr, buf.Bytes())
	}
	if payload["status"] != "needs_sync" || payload["ready"] != false {
		t.Fatalf("payload status = %+v ready = %+v", payload["status"], payload["ready"])
	}
	if payload["issue_count"] != float64(1) {
		t.Fatalf("issue_count = %+v", payload["issue_count"])
	}
	localRoot, ok := payload["local_root"].(map[string]any)
	if !ok {
		t.Fatalf("local_root = %+v", payload["local_root"])
	}
	if localRoot["status"] != "needs_sync" || localRoot["dest"] != "/tmp/market" || localRoot["package_root"] != "plugins/demo" {
		t.Fatalf("local_root = %+v", localRoot)
	}
	issues, ok := localRoot["issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("local_root issues = %+v", localRoot["issues"])
	}
}

func TestPublicationMaterializeDelegatesToRunner(t *testing.T) {
	t.Parallel()
	runner := &fakePublicationRunner{
		result: app.PluginPublicationMaterializeResult{
			Lines: []string{"ok"},
		},
	}
	cmd := newPublicationMaterializeCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{".", "--target", "codex-package", "--dest", "/tmp/demo", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.opts.Target != "codex-package" || runner.opts.Dest != "/tmp/demo" || runner.opts.Root != "." || !runner.opts.DryRun {
		t.Fatalf("opts = %+v", runner.opts)
	}
	if !strings.Contains(buf.String(), "ok") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestPublicationRemoveDelegatesToRunner(t *testing.T) {
	t.Parallel()
	runner := &fakePublicationRunner{
		removeResult: app.PluginPublicationRemoveResult{
			Lines: []string{"removed"},
		},
	}
	cmd := newPublicationRemoveCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{".", "--target", "claude", "--dest", "/tmp/demo", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.removeOpts.Target != "claude" || runner.removeOpts.Dest != "/tmp/demo" || runner.removeOpts.Root != "." || !runner.removeOpts.DryRun {
		t.Fatalf("opts = %+v", runner.removeOpts)
	}
	if !strings.Contains(buf.String(), "removed") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestPublicationDoctorJSONEmitsStableReportForMissingChannels(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "gemini-extension.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newPublicationDoctorCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion:  "v1",
					Name:        "demo",
					Version:     "0.1.0",
					Description: "demo plugin",
				},
				Packages: []publicationmodel.Package{
					{
						Target:           "gemini",
						PackageFamily:    "gemini-extension",
						ChannelFamilies:  []string{"gemini-gallery"},
						ManagedArtifacts: []string{"gemini-extension.json"},
					},
				},
			},
		},
		warnings: []pluginmanifest.Warning{
			{Message: "plugin/publish/gemini/gallery.yaml is not authored yet"},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--format", "json", "--target", "gemini", root})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	var payload map[string]any
	if parseErr := json.Unmarshal(buf.Bytes(), &payload); parseErr != nil {
		t.Fatalf("json parse: %v\n%s", parseErr, buf.Bytes())
	}
	if payload["format"] != "plugin-kit-ai/publication-doctor-report" {
		t.Fatalf("format = %+v", payload["format"])
	}
	if payload["schema_version"] != float64(1) {
		t.Fatalf("schema_version = %+v", payload["schema_version"])
	}
	if payload["requested_target"] != "gemini" {
		t.Fatalf("requested_target = %+v", payload["requested_target"])
	}
	if payload["ready"] != false {
		t.Fatalf("ready = %+v", payload["ready"])
	}
	if payload["status"] != "needs_channels" {
		t.Fatalf("status = %+v", payload["status"])
	}
	if payload["warning_count"] != float64(1) {
		t.Fatalf("warning_count = %+v", payload["warning_count"])
	}
	if payload["issue_count"] != float64(1) {
		t.Fatalf("issue_count = %+v", payload["issue_count"])
	}
	warnings, ok := payload["warnings"].([]any)
	if !ok || len(warnings) != 1 || warnings[0] != "plugin/publish/gemini/gallery.yaml is not authored yet" {
		t.Fatalf("warnings = %+v", payload["warnings"])
	}
	issues, ok := payload["issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("issues = %+v", payload["issues"])
	}
	issue, ok := issues[0].(map[string]any)
	if !ok {
		t.Fatalf("issue = %+v", issues[0])
	}
	if issue["code"] != "missing_channel" || issue["target"] != "gemini" || issue["channel_family"] != "gemini-gallery" || issue["path"] != "plugin/publish/gemini/gallery.yaml" {
		t.Fatalf("issue = %+v", issue)
	}
	nextSteps, ok := payload["next_steps"].([]any)
	if !ok || len(nextSteps) == 0 {
		t.Fatalf("next_steps = %+v", payload["next_steps"])
	}
	missingTargets, ok := payload["missing_package_targets"].([]any)
	if !ok || len(missingTargets) != 1 || missingTargets[0] != "gemini" {
		t.Fatalf("missing_package_targets = %+v", payload["missing_package_targets"])
	}
	publication, ok := payload["publication"].(map[string]any)
	if !ok || publication["core"] == nil || publication["packages"] == nil || publication["channels"] == nil {
		t.Fatalf("publication = %+v", payload["publication"])
	}
}

func TestPublicationDoctorJSONReportsReadyState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationTestFile(t, root, filepath.Join(".codex-plugin", "plugin.json"), "{}\n")
	mustWritePublicationTestFile(t, root, filepath.Join(".agents", "plugins", "marketplace.json"), "{}\n")
	cmd := newPublicationDoctorCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion:  "v1",
					Name:        "demo",
					Version:     "0.1.0",
					Description: "demo plugin",
				},
				Packages: []publicationmodel.Package{
					{
						Target:           "codex-package",
						PackageFamily:    "codex-plugin",
						ChannelFamilies:  []string{"codex-marketplace"},
						ManagedArtifacts: []string{".codex-plugin/plugin.json", ".agents/plugins/marketplace.json"},
					},
				},
				Channels: []publicationmodel.Channel{
					{
						Family:         "codex-marketplace",
						Path:           "plugin/publish/codex/marketplace.yaml",
						PackageTargets: []string{"codex-package"},
						Details: map[string]string{
							"marketplace_name": "local-repo",
						},
					},
				},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--format", "json", root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if parseErr := json.Unmarshal(buf.Bytes(), &payload); parseErr != nil {
		t.Fatalf("json parse: %v\n%s", parseErr, buf.Bytes())
	}
	if payload["status"] != "ready" {
		t.Fatalf("status = %+v", payload["status"])
	}
	if payload["ready"] != true {
		t.Fatalf("ready = %+v", payload["ready"])
	}
	if payload["issue_count"] != float64(0) {
		t.Fatalf("issue_count = %+v", payload["issue_count"])
	}
	issues, ok := payload["issues"].([]any)
	if !ok || len(issues) != 0 {
		t.Fatalf("issues = %+v", payload["issues"])
	}
	if _, found := payload["missing_package_targets"]; found {
		t.Fatalf("missing_package_targets should be omitted for ready payload: %+v", payload)
	}
}

func TestPublicationDoctorReportsNeedsRenderWhenGeneratedArtifactsAreMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := newPublicationDoctorCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion:  "v1",
					Name:        "demo",
					Version:     "0.1.0",
					Description: "demo plugin",
				},
				Packages: []publicationmodel.Package{
					{
						Target:           "codex-package",
						PackageFamily:    "codex-plugin",
						ChannelFamilies:  []string{"codex-marketplace"},
						ManagedArtifacts: []string{".codex-plugin/plugin.json", ".agents/plugins/marketplace.json"},
					},
				},
				Channels: []publicationmodel.Channel{
					{
						Family:         "codex-marketplace",
						Path:           "plugin/publish/codex/marketplace.yaml",
						PackageTargets: []string{"codex-package"},
					},
				},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{root})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	output := buf.String()
	for _, want := range []string{
		"Issue[missing_channel_artifact]",
		"Issue[missing_package_artifact]",
		"Status: needs_generate",
		"run plugin-kit-ai generate . to regenerate package and publication artifacts",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("publication doctor output missing %q:\n%s", want, output)
		}
	}
}

func TestPublicationDoctorJSONReportsNeedsRenderIssues(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cmd := newPublicationDoctorCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Publication: publicationmodel.Model{
				Core: publicationmodel.Core{
					APIVersion: "v1",
					Name:       "demo",
					Version:    "0.1.0",
				},
				Packages: []publicationmodel.Package{
					{
						Target:          "claude",
						PackageFamily:   "claude-plugin",
						ChannelFamilies: []string{"claude-marketplace"},
					},
				},
				Channels: []publicationmodel.Channel{
					{
						Family:         "claude-marketplace",
						Path:           "plugin/publish/claude/marketplace.yaml",
						PackageTargets: []string{"claude"},
					},
				},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--format", "json", root})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	var payload map[string]any
	if parseErr := json.Unmarshal(buf.Bytes(), &payload); parseErr != nil {
		t.Fatalf("json parse: %v\n%s", parseErr, buf.Bytes())
	}
	if payload["status"] != "needs_generate" {
		t.Fatalf("status = %+v", payload["status"])
	}
	if payload["issue_count"] != float64(2) {
		t.Fatalf("issue_count = %+v", payload["issue_count"])
	}
	issues, ok := payload["issues"].([]any)
	if !ok || len(issues) != 2 {
		t.Fatalf("issues = %+v", payload["issues"])
	}
}

func TestPublicationDoctorReportsNeedsRenderWhenGeneratedArtifactsAreOutOfSync(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationRepoFile(t, root, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWritePublicationRepoFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationRepoFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWritePublicationRepoFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")
	mustWritePublicationRepoFile(t, root, filepath.Join(".codex-plugin", "plugin.json"), "{}\n")
	mustWritePublicationRepoFile(t, root, filepath.Join(".agents", "plugins", "marketplace.json"), "{}\n")

	cmd := newPublicationDoctorCmd(pluginService)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{root})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	output := buf.String()
	for _, want := range []string{
		"Issue[drifted_channel_artifact]",
		"Issue[drifted_package_artifact]",
		"Status: needs_generate",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("publication doctor output missing %q:\n%s", want, output)
		}
	}
}

func mustWritePublicationTestFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWritePublicationRepoFile(t *testing.T, root, rel, body string) {
	t.Helper()
	mustWritePublicationTestFile(t, root, rel, body)
}
