package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginServicePublicationMaterializeCodexMarketplaceRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\ndisplay_name: Local Repo\nsource_root: ./\ncategory: Productivity\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "skills", "demo", "SKILL.md"), "# Demo\n")

	result, err := PluginService{}.PublicationMaterialize(PluginPublicationMaterializeOptions{
		Root:   root,
		Target: "codex-package",
		Dest:   dest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) == 0 {
		t.Fatal("expected output lines")
	}
	mustContainFileText(t, filepath.Join(dest, "plugins", "demo", ".codex-plugin", "plugin.json"), `"name": "demo"`)
	mustContainFileText(t, filepath.Join(dest, "plugins", "demo", "skills", "demo", "SKILL.md"), "# Demo")
	mustContainFileText(t, filepath.Join(dest, ".agents", "plugins", "marketplace.json"), `"path": "./plugins/demo"`)
}

func TestPluginServicePublicationMaterializeClaudeMarketplaceMergesExistingCatalog(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"claude\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "launcher.yaml"), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "claude", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "claude", "marketplace.yaml"), "api_version: v1\nmarketplace_name: team-tools\nowner_name: Team\nsource_root: ./\n")
	mustWritePublicationSourceFile(t, dest, filepath.Join(".claude-plugin", "marketplace.json"), `{"name":"team-tools","owner":{"name":"Team"},"plugins":[{"name":"alpha","source":"./plugins/alpha","description":"alpha","version":"1.0.0"}]}`)

	_, err := PluginService{}.PublicationMaterialize(PluginPublicationMaterializeOptions{
		Root:   root,
		Target: "claude",
		Dest:   dest,
	})
	if err != nil {
		t.Fatal(err)
	}
	mustContainFileText(t, filepath.Join(dest, ".claude-plugin", "marketplace.json"), `"name": "demo"`)
}

func TestPluginServicePublicationMaterializeRemovesOldPackageRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")
	mustWritePublicationSourceFile(t, dest, filepath.Join("plugins", "demo", "stale.txt"), "old\n")

	_, err := PluginService{}.PublicationMaterialize(PluginPublicationMaterializeOptions{
		Root:   root,
		Target: "codex-package",
		Dest:   dest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugins", "demo", "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale file still present: %v", err)
	}
}

func TestPluginServicePublicationMaterializeDryRunDoesNotWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")

	result, err := PluginService{}.PublicationMaterialize(PluginPublicationMaterializeOptions{
		Root:   root,
		Target: "codex-package",
		Dest:   dest,
		DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Mode: dry-run") {
		t.Fatalf("lines = %v", result.Lines)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugins", "demo")); !os.IsNotExist(err) {
		t.Fatalf("package root should not exist after dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".agents", "plugins", "marketplace.json")); !os.IsNotExist(err) {
		t.Fatalf("catalog should not exist after dry-run: %v", err)
	}
}

func TestPluginServicePublicationRemoveCodexMarketplaceRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")
	mustWritePublicationSourceFile(t, dest, filepath.Join("plugins", "demo", ".codex-plugin", "plugin.json"), "{}\n")
	mustWritePublicationSourceFile(t, dest, filepath.Join(".agents", "plugins", "marketplace.json"), `{"name":"local-repo","plugins":[{"name":"demo","source":{"source":"local","path":"./plugins/demo"},"policy":{"installation":"AVAILABLE","authentication":"ON_INSTALL"},"category":"Productivity"},{"name":"alpha","source":{"source":"local","path":"./plugins/alpha"},"policy":{"installation":"AVAILABLE","authentication":"ON_INSTALL"},"category":"Productivity"}]}`)

	result, err := PluginService{}.PublicationRemove(PluginPublicationRemoveOptions{
		Root:   root,
		Target: "codex-package",
		Dest:   dest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) == 0 {
		t.Fatal("expected output lines")
	}
	if _, err := os.Stat(filepath.Join(dest, "plugins", "demo")); !os.IsNotExist(err) {
		t.Fatalf("package root still present: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dest, ".agents", "plugins", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, `"name": "demo"`) || strings.Contains(text, `"name":"demo"`) {
		t.Fatalf("demo entry still present:\n%s", text)
	}
	if !strings.Contains(text, `"name":"alpha"`) && !strings.Contains(text, `"name": "alpha"`) {
		t.Fatalf("alpha entry missing:\n%s", text)
	}
}

func TestPluginServicePublicationRemoveIsIdempotentWhenEntryMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"claude\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "launcher.yaml"), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "claude", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "claude", "marketplace.yaml"), "api_version: v1\nmarketplace_name: team-tools\nowner_name: Team\nsource_root: ./\n")
	mustWritePublicationSourceFile(t, dest, filepath.Join(".claude-plugin", "marketplace.json"), `{"name":"team-tools","owner":{"name":"Team"},"plugins":[{"name":"alpha","source":"./plugins/alpha","description":"alpha","version":"1.0.0"}]}`)

	result, err := PluginService{}.PublicationRemove(PluginPublicationRemoveOptions{
		Root:   root,
		Target: "claude",
		Dest:   dest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) == 0 {
		t.Fatal("expected output lines")
	}
	body, err := os.ReadFile(filepath.Join(dest, ".claude-plugin", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "alpha") {
		t.Fatalf("existing alpha entry should remain:\n%s", body)
	}
}

func TestPluginServicePublicationRemoveDryRunDoesNotDelete(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"claude\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "launcher.yaml"), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "claude", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "claude", "marketplace.yaml"), "api_version: v1\nmarketplace_name: team-tools\nowner_name: Team\nsource_root: ./\n")
	mustWritePublicationSourceFile(t, dest, filepath.Join("plugins", "demo", ".claude-plugin", "plugin.json"), "{}\n")
	originalCatalog := `{"name":"team-tools","owner":{"name":"Team"},"plugins":[{"name":"demo","source":"./plugins/demo","description":"demo","version":"0.1.0"}]}`
	mustWritePublicationSourceFile(t, dest, filepath.Join(".claude-plugin", "marketplace.json"), originalCatalog)

	result, err := PluginService{}.PublicationRemove(PluginPublicationRemoveOptions{
		Root:   root,
		Target: "claude",
		Dest:   dest,
		DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(result.Lines, "\n"), "Mode: dry-run") {
		t.Fatalf("lines = %v", result.Lines)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugins", "demo")); err != nil {
		t.Fatalf("package root should remain after dry-run: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dest, ".claude-plugin", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != originalCatalog {
		t.Fatalf("catalog changed during dry-run:\n%s", body)
	}
}

func TestPluginServicePublicationVerifyRootReady(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "skills", "demo", "SKILL.md"), "# Demo\n")
	if _, err := (PluginService{}).PublicationMaterialize(PluginPublicationMaterializeOptions{
		Root:   root,
		Target: "codex-package",
		Dest:   dest,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := PluginService{}.PublicationVerifyRoot(PluginPublicationVerifyRootOptions{
		Root:   root,
		Target: "codex-package",
		Dest:   dest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || result.Status != "ready" || result.IssueCount != 0 {
		t.Fatalf("verify result = %+v", result)
	}
}

func TestPluginServicePublicationVerifyRootReportsDriftedCatalogEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"claude\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "launcher.yaml"), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "claude", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "claude", "marketplace.yaml"), "api_version: v1\nmarketplace_name: team-tools\nowner_name: Team\nsource_root: ./\n")
	if _, err := (PluginService{}).PublicationMaterialize(PluginPublicationMaterializeOptions{
		Root:   root,
		Target: "claude",
		Dest:   dest,
	}); err != nil {
		t.Fatal(err)
	}
	mustWritePublicationSourceFile(t, dest, filepath.Join(".claude-plugin", "marketplace.json"), `{"name":"team-tools","owner":{"name":"Team"},"plugins":[{"name":"demo","source":"./plugins/demo-drift","description":"demo","version":"0.1.0"}]}`)

	result, err := PluginService{}.PublicationVerifyRoot(PluginPublicationVerifyRootOptions{
		Root:   root,
		Target: "claude",
		Dest:   dest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Status != "needs_sync" {
		t.Fatalf("verify result = %+v", result)
	}
	if result.IssueCount != 1 || result.Issues[0].Code != "drifted_materialized_catalog_entry" {
		t.Fatalf("verify issues = %+v", result.Issues)
	}
}

func TestLoadPublicationContextPreservesChannelMetadataHints(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"claude\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "launcher.yaml"), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "claude", "package.yaml"), "homepage: https://example.com/demo\n")

	t.Run("materialize", func(t *testing.T) {
		t.Parallel()
		_, err := loadPublicationContextForMaterialize(PluginPublicationMaterializeOptions{
			Root:   root,
			Target: "claude",
			Dest:   dest,
		})
		if err == nil || !strings.Contains(err.Error(), "target claude requires authored publication channel metadata under plugin/publish/...") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("remove", func(t *testing.T) {
		t.Parallel()
		_, err := loadPublicationContextForRemove(PluginPublicationRemoveOptions{
			Root:   root,
			Target: "claude",
			Dest:   dest,
		})
		if err == nil || !strings.Contains(err.Error(), "target claude requires authored publication channel metadata under publish/...") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestPluginServicePublishDelegatesToLocalCodexMaterialize(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")

	result, err := PluginService{}.Publish(PluginPublishOptions{
		Root:    root,
		Channel: "codex-marketplace",
		Dest:    dest,
		DryRun:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) == 0 || !strings.Contains(strings.Join(result.Lines, "\n"), "Publish channel: codex-marketplace") {
		t.Fatalf("lines = %v", result.Lines)
	}
	if !result.Ready || result.Status != "ready" || result.WorkflowClass != "local_marketplace_root" || result.Target != "codex-package" || result.Mode != "dry-run" {
		t.Fatalf("publish result = %+v", result)
	}
	if result.Dest != dest {
		t.Fatalf("dest = %q", result.Dest)
	}
	if result.PackageRoot != "plugins/demo" {
		t.Fatalf("package_root = %q", result.PackageRoot)
	}
	if result.Details["package_root_action"] == "" || result.Details["catalog_artifact"] == "" || len(result.NextSteps) == 0 {
		t.Fatalf("publish details = %+v next=%v", result.Details, result.NextSteps)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugins", "demo")); !os.IsNotExist(err) {
		t.Fatalf("dry-run publish should not write package root: %v", err)
	}
}

func TestPluginServicePublishGeminiGalleryDryRunPlan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"gemini\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: github_release\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: release_archive_root\n")

	result, err := PluginService{}.Publish(PluginPublishOptions{
		Root:    root,
		Channel: "gemini-gallery",
		DryRun:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Publish channel: gemini-gallery",
		"Mode: dry-run",
		"Publication model: repository/release rooted",
		"Distribution: github_release",
		"Manifest root: release_archive_root",
		"gemini extensions link <path>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("publish plan missing %q:\n%s", want, text)
		}
	}
	if result.Ready || result.Status != "needs_repository" || result.WorkflowClass != "repository_release_plan" || result.Target != "gemini" || result.Mode != "dry-run" {
		t.Fatalf("publish result = %+v", result)
	}
	if result.Details["publication_model"] != "repository_or_release_rooted" || len(result.NextSteps) == 0 || result.IssueCount == 0 {
		t.Fatalf("publish details = %+v next=%v", result.Details, result.NextSteps)
	}
}

func TestPluginServicePublishGeminiGalleryReadyInGitHubRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"gemini\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: git_repository\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: repository_root\n")
	if err := exec.Command("git", "-C", root, "init").Run(); err != nil {
		t.Skipf("git init unavailable: %v", err)
	}
	if err := exec.Command("git", "-C", root, "remote", "add", "origin", "https://github.com/acme/demo.git").Run(); err != nil {
		t.Fatal(err)
	}

	result, err := PluginService{}.Publish(PluginPublishOptions{
		Root:    root,
		Channel: "gemini-gallery",
		DryRun:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready || result.Status != "ready" || result.IssueCount != 0 {
		t.Fatalf("publish result = %+v", result)
	}
}

func TestPluginServicePublishGeminiGalleryRejectsApply(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"gemini\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: git_repository\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: repository_root\n")

	_, err := PluginService{}.Publish(PluginPublishOptions{
		Root:    root,
		Channel: "gemini-gallery",
	})
	if err == nil || !strings.Contains(err.Error(), "supports only --dry-run planning") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginServicePublishAllDryRunNeedsChannelsWhenNoAuthoredPublication(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)

	result, err := PluginService{}.Publish(PluginPublishOptions{
		Root:   root,
		All:    true,
		DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Status != "needs_channels" || result.WorkflowClass != "multi_channel_plan" || result.ChannelCount != 0 {
		t.Fatalf("publish result = %+v", result)
	}
}

func TestPluginServicePublishAllDryRunRequiresDestForLocalChannels(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\", \"gemini\"]\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: git_repository\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: repository_root\n")

	_, err := PluginService{}.Publish(PluginPublishOptions{
		Root:   root,
		All:    true,
		DryRun: true,
	})
	if err == nil || !strings.Contains(err.Error(), "requires --dest") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginServicePublishAllDryRunOrdersChannels(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dest := t.TempDir()
	mustWritePublicationSourceFile(t, root, "plugin/plugin.yaml", "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\", \"claude\", \"gemini\"]\n")
	mustWritePublicationSourceFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.24.0\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "launcher.yaml"), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "claude", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWritePublicationSourceFile(t, root, "gemini-extension.json", "{}\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "claude", "marketplace.yaml"), "api_version: v1\nmarketplace_name: team-tools\nowner_name: Team\nsource_root: ./\n")
	mustWritePublicationSourceFile(t, root, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: git_repository\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: repository_root\n")

	result, err := PluginService{}.Publish(PluginPublishOptions{
		Root:   root,
		All:    true,
		DryRun: true,
		Dest:   dest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Ready || result.Status != "needs_attention" || result.ChannelCount != 3 {
		t.Fatalf("publish result = %+v", result)
	}
	got := []string{result.Channels[0].Channel, result.Channels[1].Channel, result.Channels[2].Channel}
	want := []string{"codex-marketplace", "claude-marketplace", "gemini-gallery"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("channel order = %v want %v", got, want)
	}
	text := strings.Join(result.Lines, "\n")
	for _, wantText := range []string{
		"Authored channels: codex-marketplace, claude-marketplace, gemini-gallery",
		"Channel 1/3: codex-marketplace",
		"Channel 2/3: claude-marketplace",
		"Channel 3/3: gemini-gallery",
	} {
		if !strings.Contains(text, wantText) {
			t.Fatalf("publish text missing %q:\n%s", wantText, text)
		}
	}
}

func mustWritePublicationSourceFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustContainFileText(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), want) {
		t.Fatalf("%s missing %q:\n%s", path, want, body)
	}
}
