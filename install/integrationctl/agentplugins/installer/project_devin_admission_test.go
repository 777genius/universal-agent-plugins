package installer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func devinSDKRemotePackage(t *testing.T, root string) {
	t.Helper()
	devinSDKWrite(t, filepath.Join(root, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-devin","version":"1.0.0"}`)
	devinSDKWrite(t, filepath.Join(root, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Inert admission fixture\n---\nText only.\n")
	devinSDKWrite(t, filepath.Join(root, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"sse","url":"https://managed.test/events"}}}`)
}

func TestProjectDevinPublicSDKAbsentLegacyProfileAndVersionAdmission(t *testing.T) {
	for _, evidence := range []string{"3000.11.205", "", "3000.3.0", "3000.11.205-alpha", "3000.11.205.1", "devin 3000.11.205 (6dea39440ae3)", "other-primary"} {
		t.Run("version="+evidence, func(t *testing.T) {
			base := devinSDKTemp(t)
			pkg := filepath.Join(base, "package")
			devinSDKRemotePackage(t, pkg)
			f := newDevinSDKFixture(t, filepath.Join(base, "A"), "", pkg, "")
			f.request.ClientVersion = evidence
			if evidence == "other-primary" {
				f.request.ClientVersion = "3000.11.205"
				f.request.ClientExecutable = filepath.Join(base, "windsurf")
			}
			if evidence == "3000.11.205" {
				f.apply(t, installer.OpInstall, "current-only-no-profile")
				native := filepath.Join(f.project, ".devin", "mcp_config.json")
				if body := devinSDKRead(t, native); !strings.Contains(body, `"transport"`) || !strings.Contains(body, `"sse"`) {
					t.Fatalf("SSE transport lost: %s", body)
				}
				f.request.ClientVersion = ""
				f.request.ClientExecutable = ""
				f.apply(t, installer.OpRemove, "remove-no-current-harness")
				return
			}
			if _, err := f.engine.Prepare(t.Context(), f.request); err == nil || !strings.Contains(err.Error(), "explicitly selected primary") {
				t.Fatalf("unqualified host evidence admitted: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(f.project, ".devin")); !os.IsNotExist(err) {
				t.Fatalf("version refusal changed native files: %v", err)
			}
		})
	}
}

func TestProjectDevinPortableManifestCannotSupplyVersionEvidence(t *testing.T) {
	base := devinSDKTemp(t)
	pkg := filepath.Join(base, "package")
	devinSDKRemotePackage(t, pkg)
	devinSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-devin","version":"1.0.0","clientVersion":"3000.11.205"}`)
	f := newDevinSDKFixture(t, filepath.Join(base, "A"), "", pkg, "")
	f.request.ClientVersion = ""
	if _, err := f.engine.Prepare(t.Context(), f.request); err == nil {
		t.Fatal("portable manifest supplied operational host evidence")
	}
	if _, err := os.Lstat(filepath.Join(f.project, ".devin")); !os.IsNotExist(err) {
		t.Fatalf("injected source changed native files: %v", err)
	}
}
