package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestOpenCodeValidateRejectsToolHelperWithoutPackageDependency(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeOpenCodeValidateFile(t, filepath.Join(root, "opencode.json"), "{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n")
	writeOpenCodeValidateFile(t, filepath.Join(root, "plugin", "targets", "opencode", "tools", "demo.ts"), "import { tool } from \"@opencode-ai/plugin\"\nexport default tool({})\n")

	state := pluginmodel.NewTargetState("opencode")
	state.AddComponent("tools", filepath.Join("plugin", "targets", "opencode", "tools", "demo.ts"))

	diagnostics, err := (opencodeAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Portable: pluginmodel.NewPortableComponents(),
	}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	joined := diagnosticsText(diagnostics)
	if !strings.Contains(joined, `must declare that dependency in plugin/targets/opencode/package.json`) {
		t.Fatalf("diagnostics missing dependency failure:\n%s", joined)
	}
}

func TestOpenCodeValidateRejectsLegacyPluginScaffoldShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeOpenCodeValidateFile(t, filepath.Join(root, "opencode.json"), "{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n")
	writeOpenCodeValidateFile(t, filepath.Join(root, "plugin", "targets", "opencode", "plugins", "index.ts"), "export default { setup() { return {} } }\n")

	state := pluginmodel.NewTargetState("opencode")
	state.AddComponent("local_plugin_code", filepath.Join("plugin", "targets", "opencode", "plugins", "index.ts"))

	diagnostics, err := (opencodeAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Portable: pluginmodel.NewPortableComponents(),
	}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	joined := diagnosticsText(diagnostics)
	if !strings.Contains(joined, "uses the old scaffold shape") {
		t.Fatalf("diagnostics missing legacy scaffold failure:\n%s", joined)
	}
}

func TestOpenCodeValidateRejectsCaseInsensitiveToolCollisions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeOpenCodeValidateFile(t, filepath.Join(root, "opencode.json"), "{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n")
	writeOpenCodeValidateFile(t, filepath.Join(root, "plugin", "targets", "opencode", "tools", "Demo.ts"), "export default {}\n")
	writeOpenCodeValidateFile(t, filepath.Join(root, "plugin", "targets", "opencode", "tools", "demo.ts"), "export default {}\n")

	state := pluginmodel.NewTargetState("opencode")
	state.AddComponent("tools", filepath.Join("plugin", "targets", "opencode", "tools", "Demo.ts"))
	state.AddComponent("tools", filepath.Join("plugin", "targets", "opencode", "tools", "demo.ts"))

	diagnostics, err := (opencodeAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Portable: pluginmodel.NewPortableComponents(),
	}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	joined := diagnosticsText(diagnostics)
	if !strings.Contains(joined, "collide on case-insensitive filesystems") {
		t.Fatalf("diagnostics missing collision failure:\n%s", joined)
	}
}

func TestValidateOpenCodeAgentFilesRejectsDeprecatedToolsField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rel := filepath.ToSlash(filepath.Join("plugin", "targets", "opencode", "agents", "reviewer.md"))
	writeOpenCodeValidateFile(t, filepath.Join(root, rel), "---\ndescription: Review code\ntools:\n  - read\n---\nPrompt body\n")

	diagnostics := validateOpenCodeAgentFiles(root, []string{rel})
	joined := diagnosticsText(diagnostics)
	if !strings.Contains(joined, `frontmatter field "tools" is deprecated; use "permission" instead`) {
		t.Fatalf("diagnostics missing deprecated tools failure:\n%s", joined)
	}
}

func writeOpenCodeValidateFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
