package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// codexDiscoveredSkills reads the skills/list answer of the Codex app-server
// and returns the enabled skills attributed to pluginID, keyed by the base
// skill name (Codex shows plugin-name:skill-name), with the path Codex loads.
func codexDiscoveredSkills(listing []byte, pluginID string) (map[string]string, error) {
	var decoded struct {
		Data []struct {
			Skills []struct {
				Name     string `json:"name"`
				Path     string `json:"path"`
				PluginID string `json:"pluginId"`
				Enabled  bool   `json:"enabled"`
			} `json:"skills"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listing, &decoded); err != nil {
		return nil, err
	}
	found := map[string]string{}
	prefix := commandsPluginName + ":"
	for _, group := range decoded.Data {
		for _, skill := range group.Skills {
			if skill.PluginID != pluginID {
				continue
			}
			name, qualified := strings.CutPrefix(skill.Name, prefix)
			if !qualified || !skill.Enabled || found[name] != "" {
				return nil, fmt.Errorf("unexpected plugin skill entry %+v", skill)
			}
			found[name] = skill.Path
		}
	}
	return found, nil
}

// TestAgentpluginsCodexNativeCommandsLifecycle drives the real Codex CLI and
// app-server through the portable command lifecycle: the ordinary command
// arrives as a plugin skill, the explicit-only command is withheld, and
// update, repair and remove keep the rendered skill in step with the
// package. Discovery uses skills/list only; no thread, model, login or OAuth.
func TestAgentpluginsCodexNativeCommandsLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CODEX_NATIVE_E2E") != "1" {
		t.Skip("opt-in native client execution")
	}
	nativeRequireDisposable(t)
	if _, err := os.Lstat("/etc/codex"); !os.IsNotExist(err) {
		t.Fatal("system Codex config must be absent in disposable image")
	}
	client := nativeBinary(t, "AGENTPLUGINS_CODEX_BIN")
	installer := nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if expected := os.Getenv("AGENTPLUGINS_CODEX_SHA256"); len(expected) != 64 || nativeSHA(t, client) != expected {
		t.Fatal("Codex binary pin mismatch")
	}
	f := newNativeFixture(t, true)
	f.ClientBinDir = filepath.Dir(client)
	stages := commandsStages()
	evidence := map[string]any{"started_utc": time.Now().UTC().Format(time.RFC3339Nano), "client": "codex", "client_sha256": nativeSHA(t, client), "installer_sha256": nativeSHA(t, installer), "acquisition": "local_directory", "native_surface": "codex plugin list --json and app-server skills/list; installed SKILL.md bytes"}
	defer commandsWriteEvidence(t, f.Root, evidence, stages)
	identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	evidence["source_identity"] = identity
	nativeWrite(t, filepath.Join(f.CodexHome, "config.toml"), []byte("cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n"), 0o600)
	version, err := nativeCommand(t, f, client, "client-version", "--version")
	evidence["client_version_measured"] = strings.TrimSpace(string(version))
	if want := os.Getenv("AGENTPLUGINS_CODEX_VERSION"); err != nil || want == "" || strings.TrimSpace(string(version)) != "codex-cli "+want {
		t.Fatalf("wrong pinned client: %s %v", version, err)
	}
	evidence["scanner"] = nativeProvisionScanner(t, f)

	listed := func(t *testing.T, label string) bool {
		t.Helper()
		b, err := nativeCommand(t, f, client, label, "plugin", "list", "--json")
		if err != nil {
			t.Fatalf("codex plugin list: %v %s", err, b)
		}
		return nativeHasPluginID(t, b, f.PluginID)
	}
	commandsNativeLifecycle(t, commandsNativeClient{
		target: "codex", pkg: f.Package, statePath: filepath.Join(f.Root, "installer-state", "state-v2.json"),
		run: func(t *testing.T, label string, args ...string) (map[string]any, error) {
			t.Helper()
			result, err := commandsRunInstaller(t, f.Root, f.Project, f.env(filepath.Dir(client)), installer, label, args...)
			if physical := nativeFindString(result, "physical_artifact_id"); f.PluginID == "" && physical != "" {
				f.PluginID = commandsPluginName + "@" + shared.ManagedMarketplaceName(physical)
				evidence["native_plugin_id"] = f.PluginID
			}
			return result, err
		},
		skills: func(t *testing.T, label string) map[string]string {
			t.Helper()
			present := listed(t, label+"-plugin-list")
			r := startNativeRPC(t, f, client, label)
			defer r.close()
			r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_native_fixture", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
			if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
				t.Fatal(err)
			}
			found, err := codexDiscoveredSkills(r.must(t, "skills/list", map[string]any{"cwds": []string{f.Project}, "forceReload": true}), f.PluginID)
			if err != nil {
				t.Fatal(err)
			}
			if present != (len(found) > 0) {
				t.Fatalf("plugin list (installed=%v) disagrees with app-server skills %v", present, found)
			}
			for _, path := range found {
				nativeRequireContained(t, f.Root, path)
				if !strings.Contains(filepath.ToSlash(path), "/plugins/cache/") {
					t.Fatalf("skill is not loaded from the installed Codex cache: %s", path)
				}
			}
			return found
		},
		dropManaged: func(t *testing.T, latest map[string]any) string {
			t.Helper()
			return commandsDropManagedArtifact(t, f.Root, filepath.Join(f.Root, "installer-state"), "codex", latest)
		},
		removeArgs: []string{"--external-uninstalled"},
	}, stages)
}

func TestCodexDiscoveredSkills(t *testing.T) {
	const id = "uap-commands-fixture@agentplugins-test"
	const listing = `{"data":[{"skills":[{"name":"uap-commands-fixture:fixture-guide","path":"a","pluginId":"uap-commands-fixture@agentplugins-test","enabled":true},{"name":"uap-commands-fixture:uap-commands-fixture-echo","path":"b","pluginId":"uap-commands-fixture@agentplugins-test","enabled":true},{"name":"foreign:skill","path":"c","pluginId":"foreign@market","enabled":true}]}]}`
	found, err := codexDiscoveredSkills([]byte(listing), id)
	if err != nil || len(found) != 2 || found[commandsSkill("echo")] != "b" || found[commandsGuideSkill] != "a" {
		t.Fatalf("exact plugin skills: %v %v", found, err)
	}
	for name, input := range map[string]string{
		"disabled":    strings.Replace(listing, `"path":"b","pluginId":"uap-commands-fixture@agentplugins-test","enabled":true`, `"path":"b","pluginId":"uap-commands-fixture@agentplugins-test","enabled":false`, 1),
		"unqualified": strings.Replace(listing, `"uap-commands-fixture:uap-commands-fixture-echo"`, `"uap-commands-fixture-echo"`, 1),
		"duplicate":   strings.Replace(listing, `"uap-commands-fixture:fixture-guide"`, `"uap-commands-fixture:uap-commands-fixture-echo"`, 1),
	} {
		if found, err := codexDiscoveredSkills([]byte(input), id); err == nil {
			t.Fatalf("%s accepted: %v", name, found)
		}
	}
}
