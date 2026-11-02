package vscodelocal_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

func linuxTarget() vscodelocalhooks.Target {
	return vscodelocalhooks.Target{Shell: vscodelocalhooks.LinuxSH}
}
func hookPath() string { return filepath.FromSlash(vscodelocalhooks.PluginPath) }
func localSpecs(executable string) []vscodelocalhooks.Spec {
	return []vscodelocalhooks.Spec{{Event: vscodelocalhooks.Stop, Executable: executable, Args: []string{"--TEST-argv", "${PLUGIN_ROOT}/plugin.json", "${PLUGIN_DATA}/locator", "literal $(touch sentinel) & ; ' Ω", ""}, TimeoutSeconds: 5}}
}
func declaredHooks(t *testing.T, c vscode.LocalConfig) []byte {
	t.Helper()
	specs := append([]vscodelocalhooks.Spec(nil), c.HookSpecs...)
	specs[0].Args = append([]string(nil), specs[0].Args...)
	replace := strings.NewReplacer("${PLUGIN_ROOT}", "/TEST-declared-plugin", "${PLUGIN_DATA}", "/TEST-declared-data")
	for i, arg := range specs[0].Args {
		specs[0].Args[i] = replace.Replace(arg)
	}
	body, err := vscodelocalhooks.Render(c.TargetShell, specs)
	must(t, err)
	return []byte(strings.NewReplacer("/TEST-declared-plugin", "${PLUGIN_ROOT}", "/TEST-declared-data", "${PLUGIN_DATA}").Replace(string(body)))
}

// Red: sanitizer drops native namespace, projection retains optional components,
// confuses future active/data roots with staging or expands/executes fixed argv.
// Nearest boundary: actual public stager plus real /bin/sh and inert recorder.
func TestLocalStagerNativeOnlyWithoutHelper(t *testing.T) {
	for _, optional := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent-MCP", true: "unselected-MCP-missing-helper"}[optional], func(t *testing.T) {
			f := freshLocal(t, optional)
			env, plan, staged := f.stage(t)
			if staged.ArtifactDigest == env.TreeDigest {
				t.Fatal("source digest reused as projection digest")
			}
			for _, path := range []string{"hooks/root.json", "mcp.json", "skills/notify/SKILL.md"} {
				if _, err := os.Lstat(filepath.Join(staged.StagingPath, filepath.FromSlash(path))); !os.IsNotExist(err) {
					t.Fatalf("unselected path survives: %s, %v", path, err)
				}
			}
			if string(readLocal(t, filepath.Join(staged.StagingPath, "com.example.opaque", "preserved.txt"))) != "TEST opaque namespace bytes" {
				t.Fatal("opaque namespace changed")
			}
			if string(readLocal(t, filepath.Join(f.pkg, hookPath()))) != string(declaredHooks(t, f.config)) {
				t.Fatal("canonical hook source changed")
			}
			body := readLocal(t, filepath.Join(staged.StagingPath, hookPath()))
			var hooks struct {
				Hooks map[string][]struct {
					Linux   string
					Timeout int
					Type    string
				}
			}
			must(t, json.Unmarshal(body, &hooks))
			entries := hooks.Hooks["Stop"]
			if len(hooks.Hooks) != 1 || len(entries) != 1 || entries[0].Timeout != 5 || entries[0].Type != "command" {
				t.Fatal("native flat seconds/Stop schema differs")
			}
			command := exec.CommandContext(t.Context(), "/bin/sh", "-c", entries[0].Linux)
			command.Dir = f.root
			command.Env = []string{"HOME=" + f.root, "USERPROFILE=" + f.root, "PATH=/usr/bin:/bin"}
			output, err := runLocalProcess(t, command)
			must(t, err)
			var args []string
			must(t, json.Unmarshal(output, &args))
			want := []string{plan.ActivePath + "/plugin.json", filepath.Join(f.root, "TEST plugin data") + "/locator", "literal $(touch sentinel) & ; ' Ω", ""}
			if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
				t.Fatalf("projected argv differs: %q", args)
			}
			if _, err := os.Lstat(filepath.Join(f.root, "sentinel")); !os.IsNotExist(err) {
				t.Fatal("shell injection sentinel exists")
			}
		})
	}
}

// Red: public read-only active projection accepts edited hooks or writes a
// replacement using current config. One real stager digest boundary proves it.
func TestLocalDamagedProjectionRefusesReadOnlyRepair(t *testing.T) {
	f := freshLocal(t, false)
	env, plan, staged := f.stage(t)
	must(t, os.Rename(staged.StagingPath, plan.ActivePath))
	body := readLocal(t, filepath.Join(plan.ActivePath, hookPath()))
	writeLocal(t, filepath.Join(plan.ActivePath, hookPath()), append(body, []byte("TEST damage")...), 0600)
	before := readLocal(t, filepath.Join(plan.ActivePath, hookPath()))
	_, err := (providers.Stager{Registry: f.registry, Paths: pathpolicy.Policy{}}).ProjectActiveNative(t.Context(), env, plan, staged.ArtifactDigest, filepath.Join(f.root, "TEST plugin data"))
	if err == nil {
		t.Fatal("damaged projection accepted")
	}
	if string(before) != string(readLocal(t, filepath.Join(plan.ActivePath, hookPath()))) {
		t.Fatal("readonly projection mutated damaged bytes")
	}
}

var _ clients.ActiveNativeProjector = (*vscode.LocalAdapter)(nil)

// Red: the initial real adapter wrote selected settings even for prepare intent.
// Boundary: genuine staged/sealed package and public lifecycle, since the 040
// Engine request does not expose intent. Preparation must confer no native effect.
func TestLocalPrepareIntentRetainsNativeBytes(t *testing.T) {
	f := freshLocal(t, false)
	_, plan, staged := f.stage(t)
	must(t, os.Rename(staged.StagingPath, plan.ActivePath))
	plan.InstallIntent = domain.InstallIntentPrepare
	before := readLocal(t, f.settings)
	request := domain.ActivationRequest{Plan: plan, Delivery: staged, Client: domain.DetectedClient{ClientID: domain.ClientVSCode, ConfigRoot: filepath.Dir(f.settings)}}
	out, err := f.adapter.Activate(t.Context(), clients.Env{NativeConfig: nativeconfig.New()}, request)
	must(t, err)
	if out.NativeEffect != domain.NativeEffectUnchanged || len(out.NativeObjects) != 0 || string(before) != string(readLocal(t, f.settings)) {
		t.Fatal("prepare intent changed native registration")
	}
}
