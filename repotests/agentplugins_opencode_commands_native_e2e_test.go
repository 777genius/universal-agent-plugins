package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// openCodeDiscoveredSkills reads `opencode debug skill` and returns the
// skills OpenCode loads from the global skills root, keyed by name, with the
// file it loads. Built-in skills and other roots are not the plugin's.
func openCodeDiscoveredSkills(listing []byte, skillsRoot string) (map[string]string, error) {
	var skills []struct {
		Name     string `json:"name"`
		Location string `json:"location"`
	}
	if err := json.Unmarshal(listing, &skills); err != nil {
		return nil, fmt.Errorf("decode opencode debug skill: %w", err)
	}
	found := map[string]string{}
	for _, skill := range skills {
		if !openCodeSamePath(skill.Location, filepath.Join(skillsRoot, skill.Name, "SKILL.md")) {
			continue
		}
		if found[skill.Name] != "" {
			return nil, fmt.Errorf("opencode lists skill %q twice", skill.Name)
		}
		found[skill.Name] = skill.Location
	}
	return found, nil
}

func openCodeSamePath(a, b string) bool {
	a, b = filepath.ToSlash(filepath.Clean(a)), filepath.ToSlash(filepath.Clean(b))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func openCodeDebugSkills(t *testing.T, f *openCodeNativeFixture, client, label string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), openCodeNativeCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "debug", "skill")
	cmd.Env = f.env(filepath.Dir(client))
	cmd.Dir = f.Project
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	f.record(t, label, out)
	f.record(t, label+"-stderr", stderr.Bytes())
	if ctx.Err() != nil || err != nil {
		t.Fatalf("opencode debug skill: %v\n%s", err, stderr.Bytes())
	}
	return out
}

// TestAgentpluginsOpenCodeNativeCommandsLifecycle drives the real OpenCode CLI
// through the portable command lifecycle: the ordinary command arrives as a
// global skill, the explicit-only command is withheld, and update, repair and
// remove keep the copied skill in step with the package. The unchanged update
// and the intact repair go through OpenCode's read-only native projection,
// which must see the rendered command skill. No model, login or OAuth is used.
func TestAgentpluginsOpenCodeNativeCommandsLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_OPENCODE_NATIVE_E2E") != "1" {
		t.Skip("opt-in native client execution")
	}
	client := openCodeNativeBinary(t, "AGENTPLUGINS_OPENCODE_BIN")
	installer := openCodeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f := newOpenCodeNativeFixture(t)
	stages := commandsStages()
	evidence := map[string]any{"started_utc": time.Now().UTC().Format(time.RFC3339Nano), "client": "opencode", "client_sha256": openCodeSHA256(t, client), "installer_sha256": openCodeSHA256(t, installer), "acquisition": "local_directory", "native_surface": "opencode debug skill; installed SKILL.md bytes", "network_dependency": nativeNetworkEvidence()}
	defer commandsWriteEvidence(t, f.Root, evidence, stages)
	evidence["scanner"] = openCodePrepareNative(t, f, client)
	identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	evidence["source_identity"] = identity
	measured := openCodeVersionString(t, f, client)
	evidence["client_version_measured"] = measured
	if pinned := os.Getenv("AGENTPLUGINS_OPENCODE_VERSION"); measured != pinned {
		t.Fatalf("measured opencode version %q does not match pinned %q", measured, pinned)
	}

	skillsRoot := filepath.Join(f.XDGConfig, "opencode", "skills")
	commandsNativeLifecycle(t, commandsNativeClient{
		target: "opencode", pkg: f.PackageRoot, statePath: filepath.Join(f.StateHome, "state-v2.json"),
		run: func(t *testing.T, label string, args ...string) (map[string]any, error) {
			t.Helper()
			return commandsRunInstaller(t, f.Root, f.Root, f.env(filepath.Dir(client)), installer, label, args...)
		},
		skills: func(t *testing.T, label string) map[string]string {
			t.Helper()
			found, err := openCodeDiscoveredSkills(openCodeDebugSkills(t, f, client, label), skillsRoot)
			if err != nil {
				t.Fatal(err)
			}
			if len(found) == 0 {
				for _, name := range []string{commandsGuideSkill, commandsSkill("echo"), commandsSkill("wizard")} {
					if _, statErr := os.Lstat(filepath.Join(skillsRoot, name)); !os.IsNotExist(statErr) {
						t.Fatalf("OpenCode lists no plugin skill, but %s is still on disk: %v", name, statErr)
					}
				}
			}
			return found
		},
		dropManaged: func(t *testing.T, latest map[string]any) string {
			t.Helper()
			return commandsDropManagedArtifact(t, f.Root, f.StateHome, "opencode", latest)
		},
	}, stages)
}

func TestOpenCodeDiscoveredSkills(t *testing.T) {
	root := filepath.Join(t.TempDir(), "xdg-config", "opencode", "skills")
	entry := func(name, location string) map[string]string {
		return map[string]string{"name": name, "description": "d", "location": location, "content": "c"}
	}
	listing, err := json.Marshal([]map[string]string{
		entry("customize-opencode", "<built-in>"),
		entry(commandsGuideSkill, filepath.Join(root, commandsGuideSkill, "SKILL.md")),
		entry(commandsSkill("echo"), filepath.Join(root, commandsSkill("echo"), "SKILL.md")),
		entry("project-skill", filepath.Join(t.TempDir(), "project", ".opencode", "skills", "project-skill", "SKILL.md")),
	})
	if err != nil {
		t.Fatal(err)
	}
	found, err := openCodeDiscoveredSkills(listing, root)
	if err != nil || len(found) != 2 || found[commandsSkill("echo")] != filepath.Join(root, commandsSkill("echo"), "SKILL.md") {
		t.Fatalf("plugin skills: %v %v", found, err)
	}
	if _, err := openCodeDiscoveredSkills([]byte("Error: unknown command"), root); err == nil {
		t.Fatal("non-JSON listing accepted")
	}
	duplicate, err := json.Marshal([]map[string]string{entry(commandsGuideSkill, filepath.Join(root, commandsGuideSkill, "SKILL.md")), entry(commandsGuideSkill, filepath.Join(root, commandsGuideSkill, "SKILL.md"))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openCodeDiscoveredSkills(duplicate, root); err == nil {
		t.Fatal("duplicate skill accepted")
	}
}
