package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationCLICodexLocalLifecycleRoundTrip(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	root := RepoRoot(t)
	workDir := t.TempDir()
	dest := filepath.Join(t.TempDir(), "marketplace-root")

	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"displayName":"Demo","defaultPrompt":["Inspect"]}`)
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "skills", "demo", "SKILL.md"), "# Demo\n")

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", workDir, "--platform", "codex-package", "--strict"))

	doctorBefore := exec.Command(pluginKitAIBin, "publication", "doctor", workDir, "--target", "codex-package", "--format", "json")
	doctorBeforeOut, err := doctorBefore.CombinedOutput()
	if err != nil {
		t.Fatalf("publication doctor before materialize: %v\n%s", err, doctorBeforeOut)
	}
	var before map[string]any
	if err := json.Unmarshal(doctorBeforeOut, &before); err != nil {
		t.Fatalf("parse doctor before materialize: %v\n%s", err, doctorBeforeOut)
	}
	if before["status"] != "ready" || before["ready"] != true {
		t.Fatalf("doctor before materialize = %+v", before)
	}

	publishDryRun := exec.Command(pluginKitAIBin, "publish", workDir, "--channel", "codex-marketplace", "--dest", dest, "--dry-run", "--format", "json")
	publishDryRunOut, err := publishDryRun.CombinedOutput()
	if err != nil {
		t.Fatalf("publish dry-run: %v\n%s", err, publishDryRunOut)
	}
	var publishDry map[string]any
	if err := json.Unmarshal(publishDryRunOut, &publishDry); err != nil {
		t.Fatalf("parse publish dry-run: %v\n%s", err, publishDryRunOut)
	}
	if publishDry["format"] != "plugin-kit-ai/publish-report" || publishDry["workflow_class"] != "local_marketplace_root" || publishDry["ready"] != true {
		t.Fatalf("publish dry-run = %+v", publishDry)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugins", "demo")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write package root: %v", err)
	}

	runCmd(t, root, exec.Command(pluginKitAIBin, "publish", workDir, "--channel", "codex-marketplace", "--dest", dest))

	doctorLocal := exec.Command(pluginKitAIBin, "publication", "doctor", workDir, "--target", "codex-package", "--dest", dest, "--format", "json")
	doctorLocalOut, err := doctorLocal.CombinedOutput()
	if err != nil {
		t.Fatalf("publication doctor local root: %v\n%s", err, doctorLocalOut)
	}
	var local map[string]any
	if err := json.Unmarshal(doctorLocalOut, &local); err != nil {
		t.Fatalf("parse doctor local root: %v\n%s", err, doctorLocalOut)
	}
	if local["status"] != "ready" || local["ready"] != true {
		t.Fatalf("doctor local root = %+v", local)
	}
	localRoot, ok := local["local_root"].(map[string]any)
	if !ok || localRoot["ready"] != true || localRoot["status"] != "ready" {
		t.Fatalf("local_root = %+v", local["local_root"])
	}

	body, err := os.ReadFile(filepath.Join(dest, ".agents", "plugins", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"path": "./plugins/demo"`) {
		t.Fatalf("materialized marketplace missing plugin path:\n%s", body)
	}

	runCmd(t, root, exec.Command(pluginKitAIBin, "publication", "remove", workDir, "--target", "codex-package", "--dest", dest))

	body, err = os.ReadFile(filepath.Join(dest, ".agents", "plugins", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"name":"demo"`) || strings.Contains(string(body), `"name": "demo"`) {
		t.Fatalf("materialized marketplace still references demo:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugins", "demo")); !os.IsNotExist(err) {
		t.Fatalf("package root still present after remove: %v", err)
	}
}

func TestPublicationCLIClaudeLocalLifecycleRoundTrip(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	root := RepoRoot(t)
	workDir := t.TempDir()
	dest := filepath.Join(t.TempDir(), "marketplace-root")

	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"claude\"]\n")
	mustWriteRepoFile(t, workDir, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "launcher.yaml"), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "claude", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "publish", "claude", "marketplace.yaml"), "api_version: v1\nmarketplace_name: team-tools\nowner_name: Team\nsource_root: ./\n")

	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir))
	runCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir, "--check"))
	runCmd(t, root, exec.Command(pluginKitAIBin, "validate", workDir, "--platform", "claude", "--strict"))

	doctorBefore := exec.Command(pluginKitAIBin, "publication", "doctor", workDir, "--target", "claude", "--format", "json")
	doctorBeforeOut, err := doctorBefore.CombinedOutput()
	if err != nil {
		t.Fatalf("publication doctor before materialize: %v\n%s", err, doctorBeforeOut)
	}
	var before map[string]any
	if err := json.Unmarshal(doctorBeforeOut, &before); err != nil {
		t.Fatalf("parse doctor before materialize: %v\n%s", err, doctorBeforeOut)
	}
	if before["status"] != "ready" || before["ready"] != true {
		t.Fatalf("doctor before materialize = %+v", before)
	}

	publishDryRun := exec.Command(pluginKitAIBin, "publish", workDir, "--channel", "claude-marketplace", "--dest", dest, "--dry-run", "--format", "json")
	publishDryRunOut, err := publishDryRun.CombinedOutput()
	if err != nil {
		t.Fatalf("publish dry-run: %v\n%s", err, publishDryRunOut)
	}
	var publishDry map[string]any
	if err := json.Unmarshal(publishDryRunOut, &publishDry); err != nil {
		t.Fatalf("parse publish dry-run: %v\n%s", err, publishDryRunOut)
	}
	if publishDry["format"] != "plugin-kit-ai/publish-report" || publishDry["workflow_class"] != "local_marketplace_root" || publishDry["ready"] != true {
		t.Fatalf("publish dry-run = %+v", publishDry)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugins", "demo")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write package root: %v", err)
	}

	runCmd(t, root, exec.Command(pluginKitAIBin, "publish", workDir, "--channel", "claude-marketplace", "--dest", dest))

	doctorLocal := exec.Command(pluginKitAIBin, "publication", "doctor", workDir, "--target", "claude", "--dest", dest, "--format", "json")
	doctorLocalOut, err := doctorLocal.CombinedOutput()
	if err != nil {
		t.Fatalf("publication doctor local root: %v\n%s", err, doctorLocalOut)
	}
	var local map[string]any
	if err := json.Unmarshal(doctorLocalOut, &local); err != nil {
		t.Fatalf("parse doctor local root: %v\n%s", err, doctorLocalOut)
	}
	if local["status"] != "ready" || local["ready"] != true {
		t.Fatalf("doctor local root = %+v", local)
	}
	localRoot, ok := local["local_root"].(map[string]any)
	if !ok || localRoot["ready"] != true || localRoot["status"] != "ready" {
		t.Fatalf("local_root = %+v", local["local_root"])
	}

	body, err := os.ReadFile(filepath.Join(dest, ".claude-plugin", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"source": "./plugins/demo"`) {
		t.Fatalf("materialized marketplace missing plugin source:\n%s", body)
	}

	runCmd(t, root, exec.Command(pluginKitAIBin, "publication", "remove", workDir, "--target", "claude", "--dest", dest))

	body, err = os.ReadFile(filepath.Join(dest, ".claude-plugin", "marketplace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `"name":"demo"`) || strings.Contains(string(body), `"name": "demo"`) {
		t.Fatalf("materialized marketplace still references demo:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(dest, "plugins", "demo")); !os.IsNotExist(err) {
		t.Fatalf("package root still present after remove: %v", err)
	}
}

func TestPublicationCLIGeminiDryRunReportsNeedsRepository(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := t.TempDir()

	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"gemini\"]\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: git_repository\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: repository_root\n")
	mustWriteRepoFile(t, workDir, "gemini-extension.json", "{}\n")

	cmd := exec.Command(pluginKitAIBin, "publish", workDir, "--channel", "gemini-gallery", "--dry-run", "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("publish gemini dry-run: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse publish gemini dry-run: %v\n%s", err, out)
	}
	if payload["status"] != "needs_repository" || payload["ready"] != false || payload["workflow_class"] != "repository_release_plan" {
		t.Fatalf("payload = %+v", payload)
	}
	if payload["issue_count"] == float64(0) {
		t.Fatalf("expected repository issues: %+v", payload)
	}

	doctor := exec.Command(pluginKitAIBin, "publication", "doctor", workDir, "--target", "gemini", "--format", "json")
	doctorOut, err := doctor.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 1 {
		t.Fatalf("publication doctor exit = %d\n%s", exitErr.ExitCode(), doctorOut)
	} else if err != nil && !ok {
		t.Fatalf("publication doctor gemini: %v\n%s", err, doctorOut)
	}
	var doctorPayload map[string]any
	if err := json.Unmarshal(doctorOut, &doctorPayload); err != nil {
		t.Fatalf("parse publication doctor gemini: %v\n%s", err, doctorOut)
	}
	if doctorPayload["status"] != "needs_repository" || doctorPayload["ready"] != false {
		t.Fatalf("doctor payload = %+v", doctorPayload)
	}
}

func TestPublicationCLIGeminiDryRunReadyWithGitHubOrigin(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := t.TempDir()

	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"gemini\"]\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: github_release\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: release_archive_root\n")
	mustWriteRepoFile(t, workDir, "gemini-extension.json", "{}\n")
	if out, err := exec.Command("git", "-C", workDir, "init").CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", workDir, "remote", "add", "origin", "https://github.com/acme/demo.git").CombinedOutput(); err != nil {
		t.Fatalf("git remote add origin: %v\n%s", err, out)
	}

	cmd := exec.Command(pluginKitAIBin, "publish", workDir, "--channel", "gemini-gallery", "--dry-run", "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("publish gemini ready dry-run: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse publish gemini ready dry-run: %v\n%s", err, out)
	}
	if payload["status"] != "ready" || payload["ready"] != true || payload["issue_count"] != float64(0) {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestPublicationCLIAllDryRunReportsNoAuthoredChannels(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := t.TempDir()

	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"displayName":"Demo","defaultPrompt":["Inspect"]}`)

	cmd := exec.Command(pluginKitAIBin, "publish", workDir, "--all", "--dry-run", "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("publish --all no channels: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse publish --all no channels: %v\n%s", err, out)
	}
	if payload["workflow_class"] != "multi_channel_plan" || payload["status"] != "needs_channels" || payload["ready"] != false || payload["channel_count"] != float64(0) {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestPublicationCLIAllDryRunRequiresDestWhenLocalChannelsAreAuthored(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := t.TempDir()

	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\", \"gemini\"]\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"displayName":"Demo","defaultPrompt":["Inspect"]}`)
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: git_repository\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: repository_root\n")
	mustWriteRepoFile(t, workDir, "gemini-extension.json", "{}\n")

	cmd := exec.Command(pluginKitAIBin, "publish", workDir, "--all", "--dry-run", "--format", "json")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "requires --dest") {
		t.Fatalf("expected missing --dest error, got err=%v\n%s", err, out)
	}
}

func TestPublicationCLIAllDryRunAggregatesAuthoredChannels(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := t.TempDir()
	dest := filepath.Join(t.TempDir(), "marketplace-root")

	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\", \"claude\", \"gemini\"]\n")
	mustWriteRepoFile(t, workDir, "go.mod", "module example.com/demo\n\ngo 1.24.0\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "launcher.yaml"), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"displayName":"Demo","defaultPrompt":["Inspect"]}`)
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "claude", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\nsource_root: ./\ncategory: Productivity\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "publish", "claude", "marketplace.yaml"), "api_version: v1\nmarketplace_name: team-tools\nowner_name: Team\nsource_root: ./\n")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: git_repository\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: repository_root\n")
	mustWriteRepoFile(t, workDir, "gemini-extension.json", "{}\n")

	cmd := exec.Command(pluginKitAIBin, "publish", workDir, "--all", "--dry-run", "--dest", dest, "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("publish --all aggregate: %v\n%s", err, out)
	}
	var payload map[string]any
	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("parse publish --all aggregate: %v\n%s", err, out)
	}
	if payload["workflow_class"] != "multi_channel_plan" || payload["status"] != "needs_attention" || payload["channel_count"] != float64(3) {
		t.Fatalf("payload = %+v", payload)
	}
	channels, ok := payload["channels"].([]any)
	if !ok || len(channels) != 3 {
		t.Fatalf("channels = %+v", payload["channels"])
	}
	got := make([]string, 0, len(channels))
	for _, raw := range channels {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("channel item = %+v", raw)
		}
		got = append(got, item["channel"].(string))
	}
	want := []string{"codex-marketplace", "claude-marketplace", "gemini-gallery"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("channel order = %v want %v", got, want)
	}
}
