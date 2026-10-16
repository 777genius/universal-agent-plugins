package report

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/readiness"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	commandsCore  = `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`
	guideSkill    = "---\nname: guide\ndescription: Guide\n---\nRead it.\n"
	explicitEntry = "---\ndescription: Wizard\ndisable-model-invocation: true\n---\nRun it.\n"
)

func readCommandProject(t *testing.T, files map[string]string) project.Result {
	t.Helper()
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64", "windows/amd64", "windows/arm64":
	default:
		if runtime.GOOS != "linux" {
			t.Skip("writable native authoring requires Linux, Darwin arm64 APFS or Windows amd64/arm64 NTFS")
		}
	}
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Service{Scratch: t.TempDir()}.Read(context.Background(), root)
	if err != nil || p.Facts.Package == nil {
		t.Fatalf("read project: %+v, %v", p.Facts, err)
	}
	return p
}

func findingCodes(r Report, layer string) map[string]string {
	codes := map[string]string{}
	for _, finding := range r.Findings {
		if finding.Layer == layer {
			codes[finding.Code] = finding.Severity
		}
	}
	return codes
}

// A broken command blocks readiness through its own installer-policy
// findings and leaves normative conformance and host safety as they were.
func TestInvalidCommandFailsReadinessOnly(t *testing.T) {
	t.Parallel()
	base := map[string]string{"plugin.json": commandsCore, "skills/guide/SKILL.md": guideSkill}
	clean := Build("validate", "test", readCommandProject(t, base), false)
	base[commandext.RelativeDirectory+"/bad.md"] = "no header $ARGUMENTS\n"
	broken := Build("validate", "test", readCommandProject(t, base), false)
	if broken.Readiness.Status != Fail || broken.Conformance.Status != clean.Conformance.Status || broken.HostSafety.Status != clean.HostSafety.Status {
		t.Fatalf("readiness = %s, conformance = %s (was %s), host safety = %s (was %s)", broken.Readiness.Status, broken.Conformance.Status, clean.Conformance.Status, broken.HostSafety.Status, clean.HostSafety.Status)
	}
	codes := findingCodes(broken, "installer_policy")
	if codes["command_frontmatter_invalid"] != "error" || codes["command_argument_placeholder"] != "error" || len(codes) != 2 {
		t.Fatalf("installer policy findings = %v", codes)
	}
	for _, finding := range broken.Findings {
		if finding.Layer == "installer_policy" && (finding.Location != commandext.RelativeDirectory || finding.ItemID == "bad") {
			t.Fatalf("command finding exposes its entry: %+v", finding)
		}
	}
}

// Withholding a command is the safe outcome: a warning while the client still
// receives the package's skill, a failure when nothing is left. Claude
// withholds an explicit-only command; the github-copilot family withholds
// every command by its registry entry.
func TestWithheldCommandCompatibility(t *testing.T) {
	t.Parallel()
	const ordinaryEntry = "---\ndescription: Echo\ndisable-model-invocation: false\n---\nEcho it.\n"
	cases := map[domain.ClientID]string{domain.ClientClaude: explicitEntry, domain.ClientCopilot: ordinaryEntry, domain.ClientVSCode: ordinaryEntry}
	for client, entry := range cases {
		compatibility := func(files map[string]string) Report {
			p := readCommandProject(t, files)
			r := Build("validate", "test", p, false)
			clients, err := readiness.Compatibility(all.Default(), p, []domain.ClientID{client})
			if err != nil {
				t.Fatal(err)
			}
			r.AddCompatibility(clients)
			return r
		}
		withSkill := compatibility(map[string]string{"plugin.json": commandsCore, "skills/guide/SKILL.md": guideSkill, commandext.RelativeDirectory + "/withheld.md": entry})
		if withSkill.Compatibility.Status != Pass || findingCodes(withSkill, "compatibility")["target_command_withheld"] != "warning" {
			t.Fatalf("%s: skill plus withheld command: %s %v", client, withSkill.Compatibility.Status, findingCodes(withSkill, "compatibility"))
		}
		only := compatibility(map[string]string{"plugin.json": commandsCore, commandext.RelativeDirectory + "/withheld.md": entry})
		if only.Compatibility.Status != Fail || findingCodes(only, "compatibility")["target_command_withheld"] != "error" {
			t.Fatalf("%s: withheld command only: %s %v", client, only.Compatibility.Status, findingCodes(only, "compatibility"))
		}
	}
}
