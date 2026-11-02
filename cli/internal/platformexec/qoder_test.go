package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestQoderDetectNativeMatchesPluginManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".qoder-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".qoder-plugin", "plugin.json"), []byte(`{"name":"qoder-demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !(qoderAdapter{}).DetectNative(root) {
		t.Fatal("DetectNative did not match .qoder-plugin/plugin.json")
	}
}

func TestQoderDetectNativeIgnoresStandaloneRootFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Shared agents\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if (qoderAdapter{}).DetectNative(root) {
		t.Fatal("DetectNative unexpectedly matched standalone root files")
	}
}

func TestQoderPackagedGenerateWritesManagedPluginArtifacts(t *testing.T) {
	t.Parallel()
	root, graph := qoderGenerateFixture(t)
	artifacts, err := (qoderAdapter{}).Generate(root, graph, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	var sawManifest, sawMCP, sawSkill bool
	for _, artifact := range artifacts {
		switch artifact.RelPath {
		case qoderPluginManifestPath:
			sawManifest = true
			for _, want := range []string{`"name": "qoder-demo"`, `"author": {`, `"name": "Ada"`, `"skills": "./skills/"`, `"mcpServers": "./.mcp.json"`} {
				if !strings.Contains(string(artifact.Content), want) {
					t.Fatalf("plugin manifest missing %q:\n%s", want, artifact.Content)
				}
			}
		case ".mcp.json":
			sawMCP = true
			if !strings.Contains(string(artifact.Content), `"mcpServers": {`) || !strings.Contains(string(artifact.Content), `"docs"`) {
				t.Fatalf("packaged Qoder MCP sidecar should use the wrapped shape:\n%s", artifact.Content)
			}
		case filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")):
			sawSkill = true
		}
	}
	if !sawManifest || !sawMCP || !sawSkill {
		t.Fatalf("artifacts = %+v", artifacts)
	}
}

func TestQoderPackagedGenerateValidateRoundTrip(t *testing.T) {
	t.Parallel()
	root, graph := qoderGenerateFixture(t)
	artifacts, err := (qoderAdapter{}).Generate(root, graph, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	writeQoderArtifacts(t, root, artifacts)
	diagnostics, err := (qoderAdapter{}).Validate(root, graph, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedImportRoundTrip(t *testing.T) {
	t.Parallel()
	root, graph := qoderGenerateFixture(t)
	artifacts, err := (qoderAdapter{}).Generate(root, graph, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	writeQoderArtifacts(t, root, artifacts)
	imported, err := (qoderAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo", Targets: []string{"qoder"}},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if imported.Manifest.Name != "qoder-demo" || imported.Manifest.Version != "0.1.0" || imported.Manifest.Description != "qoder demo" {
		t.Fatalf("imported manifest = %+v", imported.Manifest)
	}
	if imported.Manifest.Author == nil || imported.Manifest.Author.Name != "Ada" {
		t.Fatalf("imported author = %+v", imported.Manifest.Author)
	}
	paths := map[string]bool{}
	for _, artifact := range imported.Artifacts {
		paths[artifact.RelPath] = true
	}
	for _, want := range []string{
		filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")),
		filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")),
	} {
		if !paths[want] {
			t.Fatalf("imported artifacts missing %q: %+v", want, imported.Artifacts)
		}
	}
	if text := warningsText(imported.Warnings); text != "" {
		t.Fatalf("unexpected warnings: %s", text)
	}
}

func TestQoderPackagedImportCarriesHooksCommandsAndBin(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo"}`)
	for _, asset := range []string{"commands", "agents", "hooks", "output-styles", "bin"} {
		if err := os.MkdirAll(filepath.Join(root, asset), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeQoderTestFile(t, filepath.Join(root, "hooks", "hooks.json"), `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"${QODER_PLUGIN_ROOT}/bin/hook","args":["stop"]}]}]}}`+"\n")
	writeQoderTestFile(t, filepath.Join(root, "commands", "ship.md"), "---\ndescription: ship it\n---\n\n# ship\n")
	writeQoderTestFile(t, filepath.Join(root, "bin", "hook"), "#!/bin/sh\nexit 0\n")

	imported, err := (qoderAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo", Targets: []string{"qoder"}},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	paths := map[string]bool{}
	for _, artifact := range imported.Artifacts {
		paths[artifact.RelPath] = true
	}
	for _, want := range []string{
		filepath.ToSlash(filepath.Join("targets", "qoder", "hooks", "hooks.json")),
		filepath.ToSlash(filepath.Join("targets", "qoder", "commands", "ship.md")),
		filepath.ToSlash(filepath.Join("targets", "qoder", "bin", "hook")),
	} {
		if !paths[want] {
			t.Fatalf("imported artifacts missing %q: %+v", want, imported.Artifacts)
		}
	}
	text := warningsText(imported.Warnings)
	for _, asset := range []string{"agents", "output-styles"} {
		if !strings.Contains(text, "ignored unsupported import asset: "+asset) {
			t.Fatalf("warnings missing dropped kind %q: %s", asset, text)
		}
	}
	if strings.Contains(text, "bin") {
		t.Fatalf("bin must be imported, not dropped: %s", text)
	}
	for _, artifact := range imported.Artifacts {
		if strings.HasPrefix(artifact.RelPath, "agents/") {
			t.Fatalf("unsupported asset imported: %+v", artifact)
		}
	}
}

func TestQoderPackagedImportRequiresPluginManifest(t *testing.T) {
	t.Parallel()
	_, err := (qoderAdapter{}).Import(t.TempDir(), ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo", Targets: []string{"qoder"}},
	})
	if err == nil || !strings.Contains(err.Error(), qoderPluginManifestPath) {
		t.Fatalf("Import error = %v", err)
	}
}

func TestQoderPackagedValidateRequiresManifestName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"version":"0.1.0","description":"qoder demo"}`)
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, `must define required field "name"`) {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateChecksAuthorIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","author":{"name":"Eve"}}`)
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Author: &pluginmodel.Author{Name: "Ada"}, Targets: []string{"qoder"}},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "does not match author from plugin.yaml") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateRequiresManagedSkillsRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo"}`)
	writeQoderTestFile(t, filepath.Join(root, "plugin", "skills", "release-checks", "SKILL.md"), "---\nname: release-checks\ndescription: release checks\n---\n\nUse this skill.\n")
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
		},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, `must reference "./skills/"`) {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateRejectsSkillsWithoutPortableSkills(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","skills":"./skills/"}`)
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "may not define skills when portable skills are absent") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateChecksGeneratedSkillProjection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","skills":"./skills/"}`)
	writeQoderTestFile(t, filepath.Join(root, "plugin", "skills", "release-checks", "SKILL.md"), "---\nname: release-checks\ndescription: release checks\n---\n\nSource.\n")
	writeQoderTestFile(t, filepath.Join(root, "skills", "release-checks", "SKILL.md"), "---\nname: release-checks\ndescription: release checks\n---\n\nGenerated drift.\n")
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
		},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnosticsText(diagnostics), "does not match authored portable skill") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateRequiresManagedMCPRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","mcpServers":"./config/mcp.json"}`)
	parsed, err := pluginmodel.ParsePortableMCP("plugin/mcp/servers.yaml", []byte(`api_version: v1

servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
`))
	if err != nil {
		t.Fatal(err)
	}
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
		Portable: pluginmodel.PortableComponents{
			MCP: &pluginmodel.PortableMCP{
				Path:    filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")),
				Servers: parsed.Servers,
				File:    parsed.File,
			},
		},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, `must use "./.mcp.json"`) {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateRejectsMCPServersWithoutPortableMCP(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","mcpServers":"./.mcp.json"}`)
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "may not define mcpServers when portable MCP is absent") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

const qoderHooksBody = `{"description":"demo hooks","hooks":{"Stop":[{"async":true,"hooks":[{"type":"command","command":"${QODER_PLUGIN_ROOT}/bin/demo-hook","args":["stop"],"name":"demo-stop","timeout":2}]}]}}` + "\n"

const qoderCommandBody = "---\ndescription: Ship the demo\n---\nShip it.\n"

const qoderHookScriptBody = "#!/bin/sh\nexit 0\n"

func TestQoderPackagedGenerateMirrorsAuthoredHooksAndCommands(t *testing.T) {
	t.Parallel()
	root, graph, state := qoderHooksFixture(t)
	artifacts, err := (qoderAdapter{}).Generate(root, graph, state)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, artifact := range artifacts {
		byPath[artifact.RelPath] = string(artifact.Content)
	}
	if byPath["hooks/hooks.json"] != qoderHooksBody {
		t.Fatalf("hooks mirror = %q", byPath["hooks/hooks.json"])
	}
	if byPath["commands/ship.md"] != qoderCommandBody {
		t.Fatalf("commands mirror = %q", byPath["commands/ship.md"])
	}
	if byPath["bin/demo-hook"] != qoderHookScriptBody {
		t.Fatalf("bin mirror = %q", byPath["bin/demo-hook"])
	}
}

func TestQoderPackagedGenerateValidateRoundTripWithHooksAndCommands(t *testing.T) {
	t.Parallel()
	root, graph, state := qoderHooksFixture(t)
	artifacts, err := (qoderAdapter{}).Generate(root, graph, state)
	if err != nil {
		t.Fatal(err)
	}
	writeQoderArtifacts(t, root, artifacts)
	diagnostics, err := (qoderAdapter{}).Validate(root, graph, state)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateHooksShapeContract(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, body, want string }{
		{"invalid document", `{"hooks":`, "Qoder hooks document is invalid"},
		{"missing hooks key", `{"description":"demo"}`, "Qoder hooks document is invalid"},
		{"unrecognized event", `{"hooks":{"NotAnEvent":[]}}`, `unrecognized hook event "NotAnEvent"`},
		{"null hook groups", `{"hooks":{"Stop":null}}`, `hook event "Stop" has no hook groups`},
		{"empty hook groups", `{"hooks":{"Stop":[]}}`, `hook event "Stop" has no hook groups`},
		{"empty group", `{"hooks":{"Stop":[{}]}}`, "group 0 has no hook commands"},
		{"wrong type", `{"hooks":{"Stop":[{"hooks":[{"type":"prompt","command":"demo-hook"}]}]}}`, `must use type "command"`},
		{"unbundled command", `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"/usr/bin/env demo"}]}]}}`, "command must be a bare runtime or bundled under"},
		{"traversal command", `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"${QODER_PLUGIN_ROOT}/../../evil"}]}]}}`, "command must be a bare runtime or bundled under"},
		{"dot segment command", `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"${QODER_PLUGIN_ROOT}/bin/./x"}]}]}}`, "command must be a bare runtime or bundled under"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := qoderManifestFixture(t)
			writeQoderTestFile(t, filepath.Join(root, "plugin", "targets", "qoder", "hooks", "hooks.json"), tc.body)
			state := pluginmodel.NewTargetState("qoder")
			state.AddComponent("hooks", filepath.ToSlash(filepath.Join("plugin", "targets", "qoder", "hooks", "hooks.json")))
			diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), state)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(diagnosticsText(diagnostics), tc.want) {
				t.Fatalf("diagnostics = %+v, want %q", diagnostics, tc.want)
			}
		})
	}
}

func TestQoderPackagedValidateHooksMirrorContract(t *testing.T) {
	t.Parallel()
	t.Run("missing mirror", func(t *testing.T) {
		t.Parallel()
		root, state := qoderAuthoredHooksFixture(t, qoderHooksBody)
		diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), state)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diagnosticsText(diagnostics), "hooks/hooks.json is missing; run generate") {
			t.Fatalf("diagnostics = %+v", diagnostics)
		}
	})
	t.Run("drifted mirror", func(t *testing.T) {
		t.Parallel()
		root, state := qoderAuthoredHooksFixture(t, qoderHooksBody)
		writeQoderTestFile(t, filepath.Join(root, "hooks", "hooks.json"), `{"hooks":{}}`+"\n")
		diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), state)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diagnosticsText(diagnostics), "does not match authored") {
			t.Fatalf("diagnostics = %+v", diagnostics)
		}
	})
	t.Run("generated without authored", func(t *testing.T) {
		t.Parallel()
		root := qoderManifestFixture(t)
		writeQoderTestFile(t, filepath.Join(root, "hooks", "hooks.json"), qoderHooksBody)
		diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), pluginmodel.NewTargetState("qoder"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diagnosticsText(diagnostics), "exists without authored targets/qoder/hooks sources") {
			t.Fatalf("diagnostics = %+v", diagnostics)
		}
	})
	t.Run("missing bundled command", func(t *testing.T) {
		t.Parallel()
		root, state := qoderAuthoredHooksFixture(t, qoderHooksBody)
		writeQoderTestFile(t, filepath.Join(root, "hooks", "hooks.json"), qoderHooksBody)
		diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), state)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diagnosticsText(diagnostics), "references missing bundled file bin/demo-hook") {
			t.Fatalf("diagnostics = %+v", diagnostics)
		}
	})
}

func TestQoderPackagedValidateBinMirrorContract(t *testing.T) {
	t.Parallel()
	t.Run("missing mirror", func(t *testing.T) {
		t.Parallel()
		root := qoderManifestFixture(t)
		state := pluginmodel.NewTargetState("qoder")
		qoderAuthoredBinState(t, root, state, "demo-hook", qoderHookScriptBody)
		diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), state)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diagnosticsText(diagnostics), "bin/demo-hook is missing; run generate") {
			t.Fatalf("diagnostics = %+v", diagnostics)
		}
	})
	t.Run("drifted mirror", func(t *testing.T) {
		t.Parallel()
		root := qoderManifestFixture(t)
		state := pluginmodel.NewTargetState("qoder")
		qoderAuthoredBinState(t, root, state, "demo-hook", qoderHookScriptBody)
		writeQoderTestFile(t, filepath.Join(root, "bin", "demo-hook"), "#!/bin/sh\nexit 1\n")
		diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), state)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diagnosticsText(diagnostics), "bin/demo-hook does not match authored") {
			t.Fatalf("diagnostics = %+v", diagnostics)
		}
	})
	t.Run("generated without authored", func(t *testing.T) {
		t.Parallel()
		root := qoderManifestFixture(t)
		writeQoderTestFile(t, filepath.Join(root, "bin", "demo-hook"), qoderHookScriptBody)
		diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), pluginmodel.NewTargetState("qoder"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diagnosticsText(diagnostics), "exists without authored targets/qoder/bin sources") {
			t.Fatalf("diagnostics = %+v", diagnostics)
		}
	})
}

func TestQoderPackagedValidateRejectsGeneratedSurplusMirrors(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{"hooks", "commands", "bin"} {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			root, graph, state := qoderHooksFixture(t)
			artifacts, err := (qoderAdapter{}).Generate(root, graph, state)
			if err != nil {
				t.Fatal(err)
			}
			writeQoderArtifacts(t, root, artifacts)
			diagnostics, err := (qoderAdapter{}).Validate(root, graph, state)
			if err != nil {
				t.Fatal(err)
			}
			if len(diagnostics) != 0 {
				t.Fatalf("round-trip diagnostics = %+v", diagnostics)
			}
			surplus := filepath.Join(root, dir, "surplus.json")
			writeQoderTestFile(t, surplus, "{}\n")
			diagnostics, err = (qoderAdapter{}).Validate(root, graph, state)
			if err != nil {
				t.Fatal(err)
			}
			want := dir + "/surplus.json has no authored source"
			if !strings.Contains(diagnosticsText(diagnostics), want) {
				t.Fatalf("diagnostics = %+v, want %q", diagnostics, want)
			}
		})
	}
}

func TestQoderPackagedGenerateKeepsNestedCommandNamespaces(t *testing.T) {
	t.Parallel()
	root := qoderManifestFixture(t)
	body := "---\ndescription: ship run\n---\n\n# run\n"
	writeQoderTestFile(t, filepath.Join(root, "plugin", "targets", "qoder", "commands", "ship", "run.md"), body)
	state := pluginmodel.NewTargetState("qoder")
	state.AddComponent("commands", filepath.ToSlash(filepath.Join("plugin", "targets", "qoder", "commands", "ship", "run.md")))
	artifacts, err := (qoderAdapter{}).Generate(root, qoderManifestGraph(), state)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, artifact := range artifacts {
		byPath[artifact.RelPath] = string(artifact.Content)
	}
	if byPath["commands/ship/run.md"] != body {
		t.Fatalf("commands mirror = %q, want nested namespace kept", byPath["commands/ship/run.md"])
	}
	writeQoderArtifacts(t, root, artifacts)
	diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), state)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateCommandsContract(t *testing.T) {
	t.Parallel()
	t.Run("missing description", func(t *testing.T) {
		t.Parallel()
		root, state := qoderAuthoredCommandsFixture(t, "---\nargument-hint: anything\n---\nBody.\n")
		diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), state)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diagnosticsText(diagnostics), "must begin with frontmatter defining a non-empty description") {
			t.Fatalf("diagnostics = %+v", diagnostics)
		}
	})
	t.Run("drifted mirror", func(t *testing.T) {
		t.Parallel()
		root, state := qoderAuthoredCommandsFixture(t, qoderCommandBody)
		writeQoderTestFile(t, filepath.Join(root, "commands", "ship.md"), "---\ndescription: Ship the demo\n---\nDrifted.\n")
		diagnostics, err := (qoderAdapter{}).Validate(root, qoderManifestGraph(), state)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diagnosticsText(diagnostics), "does not match authored") {
			t.Fatalf("diagnostics = %+v", diagnostics)
		}
	})
}

func qoderManifestGraph() pluginmodel.PackageGraph {
	return pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
	}
}

func qoderManifestFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo"}`)
	return root
}

func qoderHooksFixture(t *testing.T) (string, pluginmodel.PackageGraph, pluginmodel.TargetState) {
	t.Helper()
	root, graph := qoderGenerateFixture(t)
	state := qoderAuthoredHooksState(t, root, qoderHooksBody)
	qoderAuthoredCommandsState(t, root, state, qoderCommandBody)
	qoderAuthoredBinState(t, root, state, "demo-hook", qoderHookScriptBody)
	return root, graph, state
}

func qoderAuthoredHooksFixture(t *testing.T, hooksBody string) (string, pluginmodel.TargetState) {
	t.Helper()
	root := qoderManifestFixture(t)
	return root, qoderAuthoredHooksState(t, root, hooksBody)
}

func qoderAuthoredCommandsFixture(t *testing.T, commandBody string) (string, pluginmodel.TargetState) {
	t.Helper()
	root := qoderManifestFixture(t)
	state := pluginmodel.NewTargetState("qoder")
	qoderAuthoredCommandsState(t, root, state, commandBody)
	return root, state
}

func qoderAuthoredHooksState(t *testing.T, root, hooksBody string) pluginmodel.TargetState {
	t.Helper()
	writeQoderTestFile(t, filepath.Join(root, "plugin", "targets", "qoder", "hooks", "hooks.json"), hooksBody)
	state := pluginmodel.NewTargetState("qoder")
	state.AddComponent("hooks", filepath.ToSlash(filepath.Join("plugin", "targets", "qoder", "hooks", "hooks.json")))
	return state
}

func qoderAuthoredCommandsState(t *testing.T, root string, state pluginmodel.TargetState, commandBody string) {
	t.Helper()
	writeQoderTestFile(t, filepath.Join(root, "plugin", "targets", "qoder", "commands", "ship.md"), commandBody)
	state.AddComponent("commands", filepath.ToSlash(filepath.Join("plugin", "targets", "qoder", "commands", "ship.md")))
}

func qoderAuthoredBinState(t *testing.T, root string, state pluginmodel.TargetState, name, scriptBody string) {
	t.Helper()
	writeQoderTestFile(t, filepath.Join(root, "plugin", "targets", "qoder", "bin", name), scriptBody)
	state.AddComponent("bin", filepath.ToSlash(filepath.Join("plugin", "targets", "qoder", "bin", name)))
}

func qoderGenerateFixture(t *testing.T) (string, pluginmodel.PackageGraph) {
	t.Helper()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, "plugin", "skills", "release-checks", "SKILL.md"), "---\nname: release-checks\ndescription: release checks\n---\n\nUse this skill.\n")
	parsed, err := pluginmodel.ParsePortableMCP("plugin/mcp/servers.yaml", []byte(`api_version: v1

servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
`))
	if err != nil {
		t.Fatal(err)
	}
	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			Name:        "qoder-demo",
			Version:     "0.1.0",
			Description: "qoder demo",
			Author:      &pluginmodel.Author{Name: "Ada"},
			Targets:     []string{"qoder"},
		},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
			MCP: &pluginmodel.PortableMCP{
				Path:    filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")),
				Servers: parsed.Servers,
				File:    parsed.File,
			},
		},
	}
	return root, graph
}

func writeQoderArtifacts(t *testing.T, root string, artifacts []pluginmodel.Artifact) {
	t.Helper()
	for _, artifact := range artifacts {
		writeQoderTestFile(t, filepath.Join(root, filepath.FromSlash(artifact.RelPath)), string(artifact.Content))
	}
}

func writeQoderTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
