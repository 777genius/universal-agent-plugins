package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestOpenCodeImportPreservesUnsupportedInlineCommandInConfigExtra(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeOpenCodeImportFile(t, filepath.Join(root, "opencode.json"), `{
  "command": {
    "demo": {
      "template": "echo demo",
      "temperature": 1
    }
  }
}
`)

	imported, err := (opencodeAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo"},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	warnings := warningsText(imported.Warnings)
	if !strings.Contains(warnings, `preserved OpenCode inline command "demo"`) {
		t.Fatalf("warnings missing inline preservation notice:\n%s", warnings)
	}
	configExtra, ok := artifactBody(imported.Artifacts, filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "config.extra.json"))
	if !ok {
		t.Fatal("expected targets/opencode/config.extra.json artifact")
	}
	if !strings.Contains(configExtra, `"temperature": 1`) {
		t.Fatalf("config.extra.json missing preserved command payload:\n%s", configExtra)
	}
}

func TestOpenCodeImportNormalizesInlineAgentToolsToPermission(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeOpenCodeImportFile(t, filepath.Join(root, "opencode.json"), `{
  "agent": {
    "planner": {
      "description": "Plan work",
      "tools": {
        "web": true,
        "shell": false
      },
      "prompt": "Do the plan"
    }
  }
}
`)

	imported, err := (opencodeAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo"},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	agentBody, ok := artifactBody(imported.Artifacts, filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "agents", "planner.md"))
	if !ok {
		t.Fatal("expected planner agent artifact")
	}
	if !strings.Contains(agentBody, "permission:") || !strings.Contains(agentBody, "web: true") || !strings.Contains(agentBody, "shell: false") {
		t.Fatalf("agent markdown missing normalized permission tools frontmatter:\n%s", agentBody)
	}
	if strings.Contains(agentBody, "\ntools:") {
		t.Fatalf("agent markdown unexpectedly preserved deprecated tools frontmatter:\n%s", agentBody)
	}
	if !strings.Contains(agentBody, "Do the plan") {
		t.Fatalf("agent markdown missing normalized prompt body:\n%s", agentBody)
	}
}

func TestOpenCodeImportCarriesWorkspacePackageJSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeOpenCodeImportFile(t, filepath.Join(root, "opencode.json"), `{"$schema":"https://opencode.ai/config.json"}`)
	writeOpenCodeImportFile(t, filepath.Join(root, ".opencode", "package.json"), `{"name":"demo-opencode","private":true}`)

	imported, err := (opencodeAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo"},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	packageBody, ok := artifactBody(imported.Artifacts, filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "package.json"))
	if !ok {
		t.Fatal("expected imported package.json artifact")
	}
	if !strings.Contains(packageBody, `"name":"demo-opencode"`) {
		t.Fatalf("package.json artifact missing workspace payload:\n%s", packageBody)
	}
}

func TestOpenCodeImportRejectsCompatSkillDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeOpenCodeImportFile(t, filepath.Join(root, "opencode.json"), `{"$schema":"https://opencode.ai/config.json"}`)
	writeOpenCodeImportFile(t, filepath.Join(root, ".agents", "skills", "legacy", "SKILL.md"), "# Legacy\n")

	_, err := (opencodeAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo"},
	})
	if err == nil {
		t.Fatal("expected compat skills rejection")
	}
	if !strings.Contains(err.Error(), "unsupported OpenCode native skill path .agents/skills: use skills/**") {
		t.Fatalf("error = %v", err)
	}
}

func writeOpenCodeImportFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func artifactBody(artifacts []pluginmodel.Artifact, want string) (string, bool) {
	want = filepath.ToSlash(want)
	for _, artifact := range artifacts {
		if filepath.ToSlash(artifact.RelPath) == want {
			return string(artifact.Content), true
		}
	}
	return "", false
}
