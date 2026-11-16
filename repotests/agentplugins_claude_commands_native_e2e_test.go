package pluginkitairepo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const claudeCommandsPluginID = commandsPluginName + "@skills-dir"

// claudeDiscoveredSkills reads the fixture skills Claude Code itself
// attributes to the plugin in `plugin details`, with the file it loads each
// one from. The inventory count must match, so an extra skill the fixture
// does not know about cannot hide behind the name checks.
func claudeDiscoveredSkills(details, installPath string) (map[string]string, error) {
	found := map[string]string{}
	for _, name := range []string{commandsGuideSkill, commandsSkill("echo"), commandsSkill("wizard")} {
		if strings.Contains(details, name) {
			found[name] = filepath.Join(installPath, ".agentplugins-runtime", "skills", name, "SKILL.md")
		}
	}
	if count := fmt.Sprintf("Skills (%d)", len(found)); !strings.Contains(details, count) {
		return nil, fmt.Errorf("claude plugin details does not report %q:\n%s", count, details)
	}
	return found, nil
}

// TestAgentpluginsClaudeNativeCommandsLifecycle drives the real Claude Code
// CLI through the portable command lifecycle: the ordinary command arrives as
// a plugin skill, the explicit-only command is withheld, and update, repair
// and remove keep the rendered skill in step with the package. No model,
// login or OAuth is used.
func TestAgentpluginsClaudeNativeCommandsLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLAUDE_NATIVE_E2E") != "1" {
		t.Skip("opt-in native client execution")
	}
	nativeRequireDisposable(t)
	client := claudeNativeBinary(t, "AGENTPLUGINS_CLAUDE_BIN")
	installer := claudeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if expected := os.Getenv("AGENTPLUGINS_CLAUDE_SHA256"); len(expected) != 64 || claudeSHA256(t, client) != expected {
		t.Fatal("Claude binary pin mismatch")
	}
	f := newClaudeNativeFixture(t)
	clientDir := filepath.Dir(client)
	stages := commandsStages()
	evidence := map[string]any{"started_utc": time.Now().UTC().Format(time.RFC3339Nano), "client": "claude", "client_sha256": claudeSHA256(t, client), "installer_sha256": claudeSHA256(t, installer), "acquisition": "local_directory", "native_surface": "claude plugin list --json and plugin details; installed SKILL.md bytes"}
	defer commandsWriteEvidence(t, f.Root, evidence, stages)
	identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	evidence["source_identity"] = identity
	measured := claudeVersionString(t, f, client)
	evidence["client_version_measured"] = measured
	if want := os.Getenv("AGENTPLUGINS_CLAUDE_VERSION"); want == "" || measured != want {
		t.Fatalf("Claude version %q does not match %q", measured, want)
	}
	evidence["scanner"] = nativeProvisionScanner(t, &nativeFixture{Root: f.Root})

	installPath := func(t *testing.T, label string) string {
		t.Helper()
		for _, entry := range claudePluginList(t, f, client, label) {
			if entry.ID == claudeCommandsPluginID {
				if !entry.Enabled {
					t.Fatalf("Claude lists the plugin as disabled: %+v", entry)
				}
				nativeRequireContained(t, f.Root, entry.InstallPath)
				return entry.InstallPath
			}
		}
		return ""
	}
	commandsNativeLifecycle(t, commandsNativeClient{
		target: "claude", pkg: f.PackageRoot, statePath: filepath.Join(f.StateHome, "state-v2.json"),
		run: func(t *testing.T, label string, args ...string) (map[string]any, error) {
			t.Helper()
			result, err := commandsRunInstaller(t, f.Root, f.Root, f.env(clientDir), installer, label, args...)
			if err == nil && len(args) != 0 && args[0] != "remove" {
				if err := validateClaudeSkillsInstallationReady(result); err != nil {
					t.Fatal(err)
				}
			}
			return result, err
		},
		skills: func(t *testing.T, label string) map[string]string {
			t.Helper()
			path := installPath(t, label+"-list")
			if path == "" {
				return nil
			}
			found, err := claudeDiscoveredSkills(claudePluginDetails(t, f, client, claudeCommandsPluginID, label+"-details"), path)
			if err != nil {
				t.Fatal(err)
			}
			return found
		},
		dropManaged: func(t *testing.T, _ map[string]any) string {
			t.Helper()
			path := installPath(t, "list-before-absent-repair")
			if path == "" {
				t.Fatal("Claude does not list the plugin before the absent-directory repair")
			}
			if err := os.RemoveAll(path); err != nil {
				t.Fatal(err)
			}
			return path
		},
	}, stages)
}

func validateClaudeSkillsInstallationReady(result map[string]any) error {
	outcome, err := commandsTargetOutcome(claudePreparedEnvelope(result))
	if err != nil {
		return err
	}
	activation, _ := outcome["activation"].(map[string]any)
	if activation["activation"] != "active" || activation["verification"] != "installation_verified" ||
		activation["authentication"] != "not_required" || activation["policy"] != "allowed" ||
		activation["activation_attested"] == true || activation["authentication_attested"] == true {
		return fmt.Errorf("the Claude Skill installation is not natively ready: %v", activation)
	}
	data, _ := result["data"].(map[string]any)
	output := data
	if targets, ok := data["targets"].([]any); ok && len(targets) == 1 {
		target, _ := targets[0].(map[string]any)
		output, _ = target["output"].(map[string]any)
	}
	// Repair's established JSON schema has no readiness field. Add and update
	// must report the public result as ready, including verified unchanged work.
	readiness, exists := output["readiness"]
	if (exists || result["command"] == "add" || result["command"] == "update") && readiness != "ready" {
		return fmt.Errorf("the Claude Skill public readiness still requires setup: %v", readiness)
	}
	return nil
}

func TestClaudeSkillsInstallationReadyRequiresNativeAndPublicEvidence(t *testing.T) {
	for _, single := range []bool{false, true} {
		for _, condition := range []string{"ready", "prepared", "unverified", "unchecked-auth", "blocked", "attested", "setup-required", "missing-readiness"} {
			t.Run(fmt.Sprintf("single=%t/%s", single, condition), func(t *testing.T) {
				activation := map[string]any{"activation": "active", "verification": "installation_verified", "authentication": "not_required", "policy": "allowed"}
				output := map[string]any{"readiness": "ready", "result": map[string]any{"activation": activation}}
				switch condition {
				case "prepared":
					activation["activation"] = "prepared"
				case "unverified":
					activation["verification"] = "package_validated"
				case "unchecked-auth":
					activation["authentication"] = "not_checked"
				case "blocked":
					activation["policy"] = "blocked"
				case "attested":
					activation["activation_attested"] = true
				case "setup-required":
					output["readiness"] = "needs_action"
				case "missing-readiness":
					delete(output, "readiness")
				}
				data := output
				if !single {
					data = map[string]any{"targets": []any{map[string]any{"output": output}}}
				}
				result := map[string]any{"command": "add", "data": data}
				if err := validateClaudeSkillsInstallationReady(result); (err == nil) != (condition == "ready") {
					t.Fatalf("ready proof accepted wrong evidence: %v", err)
				}
			})
		}
	}
}

func TestClaudeDiscoveredSkills(t *testing.T) {
	const details = "uap-commands-fixture 1.0.0\n  Source: uap-commands-fixture@skills-dir\n\nComponent inventory\n  Skills (2)  fixture-guide, uap-commands-fixture-echo\n  Agents (0)\n  MCP servers (0)\n"
	found, err := claudeDiscoveredSkills(details, "root")
	if err != nil || len(found) != 2 || found[commandsSkill("echo")] != filepath.Join("root", ".agentplugins-runtime", "skills", commandsSkill("echo"), "SKILL.md") {
		t.Fatalf("exact inventory: %v %v", found, err)
	}
	for name, input := range map[string]string{
		"unlisted extra skill":  strings.Replace(details, "Skills (2)", "Skills (3)", 1),
		"withheld skill listed": strings.Replace(details, "Skills (2)  fixture-guide, uap-commands-fixture-echo", "Skills (2)  fixture-guide, uap-commands-fixture-echo, uap-commands-fixture-wizard", 1),
		"no inventory":          "uap-commands-fixture 1.0.0\n",
	} {
		if found, err := claudeDiscoveredSkills(input, "root"); err == nil {
			t.Fatalf("%s accepted: %v", name, found)
		}
	}
}
