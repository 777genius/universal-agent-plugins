package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

type fakeInspectRunner struct {
	report   pluginmanifest.Inspection
	warnings []pluginmanifest.Warning
	err      error
}

func (f fakeInspectRunner) Inspect(app.PluginInspectOptions) (pluginmanifest.Inspection, []pluginmanifest.Warning, error) {
	return f.report, f.warnings, f.err
}

func TestInspectTextShowsLauncherAndGeminiGuidance(t *testing.T) {
	t.Parallel()
	cmd := newInspectCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Manifest: pluginmanifest.Manifest{
				Name:    "demo",
				Version: "0.1.0",
				Targets: []string{"gemini"},
			},
			Launcher: &pluginmanifest.Launcher{Runtime: "go", Entrypoint: "./bin/demo"},
			Targets: []pluginmanifest.InspectTarget{{
				Target:            "gemini",
				TargetClass:       "mcp_extension",
				ProductionClass:   "production-ready extension packaging lane",
				RuntimeContract:   "production-ready extension packaging plus optional production-ready 9-hook Go runtime",
				TargetNativeKinds: []string{"hooks", "contexts"},
				ManagedArtifacts:  []string{"gemini-extension.json", "hooks/hooks.json"},
			}},
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
		"launcher: runtime=go entrypoint=./bin/demo",
		"next=go mod tidy; go test ./...; plugin-kit-ai generate --check .; plugin-kit-ai validate . --platform gemini --strict; gemini extensions link .",
		"runtime_gate=make test-gemini-runtime",
		"live_runtime_gate=make test-gemini-runtime-live",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, output)
		}
	}
}

func TestInspectAuthoringShowsJobFirstSummary(t *testing.T) {
	t.Parallel()
	cmd := newInspectCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Manifest: pluginmanifest.Manifest{
				Name:    "demo",
				Version: "0.1.0",
				Targets: []string{"claude", "codex-package", "gemini", "opencode", "cursor"},
			},
			Portable: pluginmanifest.PortableComponents{
				MCP: &pluginmanifest.PortableMCP{
					File: &pluginmodel.PortableMCPFile{
						Servers: map[string]pluginmodel.PortableMCPServer{
							"service": {
								Type: "remote",
								Remote: &pluginmodel.PortableMCPRemote{
									URL:      "https://example.com/mcp",
									Protocol: "streamable_http",
								},
							},
						},
					},
				},
			},
			Layout: pluginmanifest.InspectLayout{
				AuthoredRoot:     "plugin",
				AuthoredInputs:   []string{"plugin/plugin.yaml", "plugin/mcp/servers.yaml"},
				GeneratedOutputs: []string{"README.md", "CLAUDE.md", "AGENTS.md", "GENERATED.md", ".mcp.json"},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--authoring", "."})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"This repo is set up to connect an online service.",
		"Editable source lives under plugin.",
		"Edit these files:",
		"  - plugin/plugin.yaml",
		"  - plugin/mcp/servers.yaml",
		"Managed guidance files:",
		"  - README.md",
		"  - CLAUDE.md",
		"  - AGENTS.md",
		"  - GENERATED.md",
		"Generated target outputs:",
		"  - .mcp.json",
		"Supported outputs:",
		"  - Claude (claude)",
		"  - Codex package (codex-package)",
		"  - Gemini extension (gemini)",
		"  - OpenCode (opencode)",
		"  - Cursor plugin (cursor)",
		"Next commands:",
		"  - plugin-kit-ai generate .",
		"  - plugin-kit-ai generate --check .",
		"  - plugin-kit-ai validate . --platform claude --strict",
		"  - Then validate any other outputs you plan to ship.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, output)
		}
	}
}

func TestInspectTextShowsGeminiPackagingGuidanceWithoutLauncher(t *testing.T) {
	t.Parallel()
	cmd := newInspectCmd(fakeInspectRunner{
		report: pluginmanifest.Inspection{
			Manifest: pluginmanifest.Manifest{
				Name:    "demo",
				Version: "0.1.0",
				Targets: []string{"gemini"},
			},
			Targets: []pluginmanifest.InspectTarget{{
				Target:            "gemini",
				TargetClass:       "mcp_extension",
				ProductionClass:   "production-ready extension packaging lane",
				RuntimeContract:   "production-ready extension packaging plus optional production-ready 9-hook Go runtime",
				TargetNativeKinds: []string{"commands", "contexts"},
				ManagedArtifacts:  []string{"gemini-extension.json"},
			}},
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
		"managed=gemini-extension.json",
		"next=generate --check + validate --strict keep the packaging lane honest; add --runtime go when you want the Gemini production-ready 9-hook runtime",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "launcher: runtime=") {
		t.Fatalf("inspect output unexpectedly shows launcher:\n%s", output)
	}
}

func TestInspectTextIncludesNativeDocPathsForCodexLanes(t *testing.T) {
	root := t.TempDir()
	manifest := pluginmanifest.Default("codex-inspect", "codex-runtime", "go", "codex inspect", true)
	manifest.Targets = []string{"codex-package", "codex-runtime"}
	if err := pluginmanifest.Save(root, manifest, true); err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.SaveLauncher(root, pluginmanifest.DefaultLauncher("codex-inspect", "go"), true); err != nil {
		t.Fatal(err)
	}
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "codex-runtime", "config.extra.toml"), "approval_policy = \"never\"\n")

	var buf bytes.Buffer
	cmd := rootCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"inspect", root, "--target", "all", "--format", "text"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"docs=interface=plugin/targets/codex-package/interface.json,package_metadata=plugin/targets/codex-package/package.yaml",
		"docs=config_extra=plugin/targets/codex-runtime/config.extra.toml,package_metadata=plugin/targets/codex-runtime/package.yaml",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, output)
		}
	}
}

func TestInspectTextOrdersChannelDetailsAndRemainingDocsDeterministically(t *testing.T) {
	t.Parallel()
	output := renderInspectText(pluginmanifest.Inspection{
		Manifest: pluginmanifest.Manifest{
			Name:    "demo",
			Version: "0.1.0",
			Targets: []string{"codex-package"},
		},
		Publication: publicationmodel.Model{
			Channels: []publicationmodel.Channel{{
				Family:         "codex-marketplace",
				Path:           "plugin/publish/codex/marketplace.yaml",
				PackageTargets: []string{"codex-package"},
				Details: map[string]string{
					"zeta":  "last",
					"alpha": "first",
				},
			}},
		},
		Targets: []pluginmanifest.InspectTarget{{
			Target:            "codex-package",
			TargetClass:       "package",
			ProductionClass:   "bundle",
			RuntimeContract:   "package only",
			ManagedArtifacts:  []string{"plugin.json"},
			TargetNativeKinds: []string{"package_metadata"},
			NativeDocPaths: map[string]string{
				"interface":        "plugin/targets/codex-package/interface.json",
				"package_metadata": "plugin/targets/codex-package/package.yaml",
			},
		}},
	})
	for _, want := range []string{
		"details=alpha=first,zeta=last",
		"docs=package_metadata=plugin/targets/codex-package/package.yaml,interface=plugin/targets/codex-package/interface.json",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, output)
		}
	}
}

func TestInspectJSONUsesPortableContractShape(t *testing.T) {
	root := t.TempDir()
	mustWriteInspectFile(t, root, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"codex-inspect\"\nversion: \"0.1.0\"\ndescription: \"codex inspect\"\ntargets: [\"codex-runtime\"]\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "launcher.yaml"), "runtime: go\nentrypoint: ./bin/codex-inspect\n")

	var buf bytes.Buffer
	cmd := rootCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"inspect", root, "--target", "codex-runtime", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("inspect json parse: %v\n%s", err, buf.Bytes())
	}
	portable, ok := report["portable"].(map[string]any)
	if !ok {
		t.Fatalf("portable payload missing: %+v", report)
	}
	if _, ok := portable["items"].(map[string]any); !ok {
		t.Fatalf("portable.items missing or wrong shape: %+v", portable)
	}
	if _, found := portable["Items"]; found {
		t.Fatalf("portable should not expose removed field name: %+v", portable)
	}
	if sourceFiles, ok := report["source_files"].([]any); !ok || len(sourceFiles) != 2 {
		t.Fatalf("source_files should be a non-null array with plugin and launcher, got %+v", report["source_files"])
	}
	layout, ok := report["layout"].(map[string]any)
	if !ok {
		t.Fatalf("layout payload missing: %+v", report)
	}
	if layout["authored_root"] != "plugin" {
		t.Fatalf("layout.authored_root = %+v", layout["authored_root"])
	}
	if layout["generated_guide"] != "GENERATED.md" {
		t.Fatalf("layout.generated_guide = %+v", layout["generated_guide"])
	}
	if authoredInputs, ok := layout["authored_inputs"].([]any); !ok || len(authoredInputs) != 2 {
		t.Fatalf("layout.authored_inputs should be a non-null array, got %+v", layout["authored_inputs"])
	}
	if generatedOutputs, ok := layout["generated_outputs"].([]any); !ok || !containsInspectString(generatedOutputs, "GENERATED.md") {
		t.Fatalf("layout.generated_outputs missing GENERATED.md: %+v", layout["generated_outputs"])
	}
	generatedByTarget, ok := layout["generated_by_target"].(map[string]any)
	if !ok {
		t.Fatalf("layout.generated_by_target missing: %+v", layout)
	}
	codexRuntimeOutputs, ok := generatedByTarget["codex-runtime"].([]any)
	if !ok || !containsInspectString(codexRuntimeOutputs, "GENERATED.md") {
		t.Fatalf("layout.generated_by_target[codex-runtime] missing GENERATED.md: %+v", generatedByTarget["codex-runtime"])
	}
	publication, ok := report["publication"].(map[string]any)
	if !ok {
		t.Fatalf("publication payload missing: %+v", report)
	}
	core, ok := publication["core"].(map[string]any)
	if !ok || core["api_version"] != "v1" || core["name"] != "codex-inspect" {
		t.Fatalf("publication.core payload = %+v", publication["core"])
	}
	packages, ok := publication["packages"].([]any)
	if !ok || len(packages) != 0 {
		t.Fatalf("publication.packages should be an empty array for codex-runtime-only inspect, got %+v", publication["packages"])
	}
	channels, ok := publication["channels"].([]any)
	if !ok || len(channels) != 0 {
		t.Fatalf("publication.channels should be an empty array for codex-runtime-only inspect, got %+v", publication["channels"])
	}
	targets, ok := report["targets"].([]any)
	if !ok || len(targets) != 1 {
		t.Fatalf("targets payload = %+v", report["targets"])
	}
	target, ok := targets[0].(map[string]any)
	if !ok {
		t.Fatalf("target payload = %+v", targets[0])
	}
	if _, ok := target["native_surface_tiers"].(map[string]any); !ok {
		t.Fatalf("native_surface_tiers missing from inspect json target: %+v", target)
	}
	if kinds, ok := target["target_native_kinds"].([]any); !ok || len(kinds) != 0 {
		t.Fatalf("target_native_kinds should be an empty array, got %+v", target["target_native_kinds"])
	}
	if kinds, ok := target["portable_kinds"].([]any); !ok || len(kinds) != 0 {
		t.Fatalf("portable_kinds should be an empty array, got %+v", target["portable_kinds"])
	}
}

func TestInspectJSONIncludesPublicationPackages(t *testing.T) {
	root := t.TempDir()
	mustWriteInspectFile(t, root, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"codex-package-publish\"\nversion: \"0.1.0\"\ndescription: \"codex package publish\"\ntargets: [\"codex-package\"]\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWriteInspectFile(t, root, filepath.Join("plugin", "skills", "demo", "SKILL.md"), "# Demo\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: /bin/echo\n      args:\n        - ok\n    targets:\n      - codex-package\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\ncategory: Productivity\n")

	var buf bytes.Buffer
	cmd := rootCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"inspect", root, "--target", "codex-package", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("inspect json parse: %v\n%s", err, buf.Bytes())
	}
	publication, ok := report["publication"].(map[string]any)
	if !ok {
		t.Fatalf("publication payload missing: %+v", report)
	}
	packages, ok := publication["packages"].([]any)
	if !ok || len(packages) != 1 {
		t.Fatalf("publication.packages = %+v", publication["packages"])
	}
	pkg, ok := packages[0].(map[string]any)
	if !ok {
		t.Fatalf("publication package = %+v", packages[0])
	}
	if pkg["package_family"] != "codex-plugin" {
		t.Fatalf("package_family = %+v", pkg["package_family"])
	}
	channels, ok := pkg["channel_families"].([]any)
	if !ok || len(channels) != 1 || channels[0] != "codex-marketplace" {
		t.Fatalf("channel_families = %+v", pkg["channel_families"])
	}
	inputs, ok := pkg["authored_inputs"].([]any)
	if !ok {
		t.Fatalf("authored_inputs = %+v", pkg["authored_inputs"])
	}
	for _, want := range []string{
		"plugin/plugin.yaml",
		filepath.ToSlash(filepath.Join("plugin", "targets", "codex-package", "package.yaml")),
		filepath.ToSlash(filepath.Join("plugin", "targets", "codex-package", "interface.json")),
		filepath.ToSlash(filepath.Join("plugin", "skills", "demo", "SKILL.md")),
		filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")),
		filepath.ToSlash(filepath.Join("plugin", "publish", "codex", "marketplace.yaml")),
	} {
		if !containsInspectString(inputs, want) {
			t.Fatalf("authored_inputs missing %q: %+v", want, inputs)
		}
	}
	publicationChannels, ok := publication["channels"].([]any)
	if !ok || len(publicationChannels) != 1 {
		t.Fatalf("publication.channels = %+v", publication["channels"])
	}
	channel, ok := publicationChannels[0].(map[string]any)
	if !ok || channel["family"] != "codex-marketplace" || channel["path"] != filepath.ToSlash(filepath.Join("plugin", "publish", "codex", "marketplace.yaml")) {
		t.Fatalf("publication channel = %+v", publicationChannels[0])
	}
}

func TestInspectTextSeparatesAuthoredAndGeneratedOutputs(t *testing.T) {
	root := t.TempDir()
	mustWriteInspectFile(t, root, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"cursor-inspect\"\nversion: \"0.1.0\"\ndescription: \"cursor inspect\"\ntargets: [\"cursor\"]\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "README.md"), "# Cursor inspect\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: /bin/echo\n      args:\n        - ok\n    targets:\n      - cursor\n")

	var buf bytes.Buffer
	cmd := rootCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"inspect", root, "--target", "cursor", "--format", "text"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"layout: authored_root=plugin boundary_docs=CLAUDE.md,AGENTS.md generated_guide=GENERATED.md",
		"authored_inputs:",
		"  - plugin/plugin.yaml",
		"  - plugin/README.md",
		"  - plugin/mcp/servers.yaml",
		"generated_outputs:",
		"  - GENERATED.md",
		"  - README.md",
		"  - .cursor-plugin/plugin.json",
		"  - .mcp.json",
		"generated_by_target:",
		"  cursor:",
		"    - GENERATED.md",
		"    - README.md",
		"    - .cursor-plugin/plugin.json",
		"    - .mcp.json",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, output)
		}
	}
}

func TestInspectTextIncludesPublicationSummary(t *testing.T) {
	root := t.TempDir()
	mustWriteInspectFile(t, root, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"codex-package-publish\"\nversion: \"0.1.0\"\ndescription: \"codex package publish\"\ntargets: [\"codex-package\"]\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Inspect"]}`)
	mustWriteInspectFile(t, root, filepath.Join("plugin", "skills", "demo", "SKILL.md"), "# Demo\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: /bin/echo\n      args:\n        - ok\n    targets:\n      - codex-package\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\ncategory: Productivity\n")

	var buf bytes.Buffer
	cmd := rootCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"inspect", root, "--target", "codex-package", "--format", "text"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"publication: api_version=v1 packages=1 channels=1",
		"channel[codex-marketplace]: path=plugin/publish/codex/marketplace.yaml targets=codex-package",
		"details=authentication_policy=ON_INSTALL,category=Productivity,installation_policy=AVAILABLE,marketplace_name=local-repo,source_root=./",
		"publish[codex-package]: family=codex-plugin channels=codex-marketplace",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, output)
		}
	}
}

func TestInspectJSONIncludesGeminiPublicationChannelDetails(t *testing.T) {
	root := t.TempDir()
	mustWriteInspectFile(t, root, filepath.Join("plugin", "plugin.yaml"), "api_version: v1\nname: \"gemini-publish\"\nversion: \"0.1.0\"\ndescription: \"gemini publish\"\ntargets: [\"gemini\"]\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "gemini", "package.yaml"), "homepage: https://example.com/gemini\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteInspectFile(t, root, filepath.Join("plugin", "publish", "gemini", "gallery.yaml"), "api_version: v1\ndistribution: github_release\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: release_archive_root\n")

	var buf bytes.Buffer
	cmd := rootCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"inspect", root, "--target", "gemini", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("inspect json parse: %v\n%s", err, buf.Bytes())
	}
	publication, ok := report["publication"].(map[string]any)
	if !ok {
		t.Fatalf("publication payload missing: %+v", report)
	}
	channels, ok := publication["channels"].([]any)
	if !ok || len(channels) != 1 {
		t.Fatalf("publication.channels = %+v", publication["channels"])
	}
	channel, ok := channels[0].(map[string]any)
	if !ok || channel["family"] != "gemini-gallery" {
		t.Fatalf("publication channel = %+v", channels[0])
	}
	details, ok := channel["details"].(map[string]any)
	if !ok {
		t.Fatalf("publication channel details = %+v", channel["details"])
	}
	for key, want := range map[string]any{
		"distribution":          "github_release",
		"repository_visibility": "public",
		"github_topic":          "gemini-cli-extension",
		"manifest_root":         "release_archive_root",
	} {
		if details[key] != want {
			t.Fatalf("details[%s] = %+v", key, details[key])
		}
	}
}

func TestInspectHelpIncludesCursorTarget(t *testing.T) {
	t.Parallel()
	cmd := newInspectCmd(fakeInspectRunner{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"cursor"`) {
		t.Fatalf("help output missing cursor target:\n%s", buf.String())
	}
}

func mustWriteInspectFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsInspectString(items []any, want string) bool {
	for _, item := range items {
		if text, ok := item.(string); ok && text == want {
			return true
		}
	}
	return false
}
