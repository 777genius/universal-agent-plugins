package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectVSCodeDefaultSDKRefusesForeignNativeBeforePublication(t *testing.T) {
	for _, collision := range []string{"skill", "mcp", "malformed"} {
		t.Run(collision, func(t *testing.T) {
			f := newProjectFilesFixture(t, "vscode", "default")
			path, body := f.skill, "foreign skill"
			if collision != "skill" {
				path, body = f.config, `{"servers":{"docs":{"type":"http","url":"https://foreign.test/mcp"}}}`
				if collision == "malformed" {
					body = `{"servers":`
				}
			}
			deathWrite(t, path, body)
			other, before := f.untouched(t), f.state(t)
			out, err := f.execute(t, OpInstall)
			if err == nil || out.Mutated || f.state(t) != before || deathRead(t, path) != body {
				t.Fatalf("foreign workspace publication: %+v %v", out, err)
			}
			if _, err := os.Lstat(filepath.Join(f.cfg.StateRoot, "managed")); !os.IsNotExist(err) {
				t.Fatal("native refusal published a managed workspace package", err)
			}
			f.assertUntouched(t, other)
		})
	}
}

func TestProjectVSCodeDefaultRegistryRefusesBeforeSourceAcquisition(t *testing.T) {
	f := newProjectFilesFixture(t, "vscode", "default")
	f.cfg.Registry = all.Default()
	f.reopen(t)
	f.req.PackageRoot = filepath.Join(f.base, "source-that-must-not-be-read")
	other := f.untouched(t)
	if out, err := f.execute(t, OpInstall); !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "project scope is not qualified for vscode") || out.Mutated || f.state(t) != "" {
		t.Fatalf("default registry reached project acquisition: %+v %v", out, err)
	}
	for _, path := range []string{f.config, f.skill, filepath.Join(f.cfg.StateRoot, "managed"), filepath.Join(f.cfg.StateRoot, "data")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("default registry published project output %s: %v", path, err)
		}
	}
	f.assertUntouched(t, other)
}

func TestProjectVSCodeManagedStdioRequiresExactPackageDataAndLauncher(t *testing.T) {
	f := newProjectFilesFixture(t, "vscode", "disabled-with-runner")
	deathWrite(t, filepath.Join(f.pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js","${PLUGIN_DATA}"],"env":{"TASK_MODE":"inert"},"cwd":"${PLUGIN_DATA}"}}}`)
	deathWrite(t, filepath.Join(f.pkg, "server.js"), "// Inert fixture; never executed.\n")
	prepared, err := f.engine.Prepare(t.Context(), f.req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := prepared.Close(); err != nil {
			t.Error(err)
		}
	}()
	if len(prepared.deliveries) != 1 {
		t.Fatal("missing selected workspace delivery")
	}
	envelope, plan := prepared.envelope, prepared.deliveries[0].plan
	result, err := f.engine.Apply(t.Context(), prepared, Decision{Confirmed: true})
	if err != nil || !result.Mutated || result.Client.Activation != string(domain.ActivationPrepared) {
		t.Fatalf("stdio preparation: %+v %v", result, err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	binding := f.binding(t)
	root, data := binding.TargetLocator, result.Binding.DataRoot
	if data == "" || result.Binding.DataReceiptID == "" {
		t.Fatal("stdio preparation omitted selected data authority")
	}
	deathWrite(t, filepath.Join(data, "keep.txt"), "retained plugin data")
	deathWrite(t, filepath.Join(f.base, "unowned-data", "keep"), "foreign data")
	adapter := vscode.NewProject()
	objects, err := adapter.ProjectActiveNative(t.Context(), root, envelope, plan, data)
	if err != nil || len(objects) != 2 {
		t.Fatalf("exact active stdio projection: %+v %v", objects, err)
	}
	config, state, other := deathRead(t, f.config), f.state(t), f.untouched(t)
	var native struct {
		Servers map[string]struct {
			Command string
			Args    []string
			Env     map[string]string
			CWD     string
		}
	}
	if err := json.Unmarshal([]byte(config), &native); err != nil {
		t.Fatal(err)
	}
	server := native.Servers["docs"]
	if server.Env["PLUGIN_ROOT"] != root || server.Env["PLUGIN_DATA"] != data || server.Env["TASK_MODE"] != "inert" || len(server.Args) < 7 || server.Args[1] != root || server.Args[2] != data || server.Args[3] != data || server.Args[4] != "data" || server.CWD != "" || deathRead(t, server.Command) != deathRead(t, f.cfg.HelperExecutable) {
		t.Fatalf("stdio launcher lost exact immutable package/data/helper binding: %+v", server)
	}
	managed := filepath.Join(root, "mcp.json")
	original := deathRead(t, managed)
	for _, mismatch := range []string{"data", "package", "launcher"} {
		t.Run(mismatch, func(t *testing.T) {
			selectedRoot, selectedData := root, data
			switch mismatch {
			case "data":
				selectedData = filepath.Join(f.base, "unowned-data")
			case "package":
				selectedRoot = filepath.Join(f.base, "unowned-package")
			case "launcher":
				var document map[string]any
				if err := json.Unmarshal([]byte(original), &document); err != nil {
					t.Fatal(err)
				}
				entry := document["mcpServers"].(map[string]any)["docs"].(map[string]any)
				entry["command"] = filepath.Join(f.base, "foreign-launcher")
				body, err := json.Marshal(document)
				if err != nil {
					t.Fatal(err)
				}
				deathWrite(t, managed, string(body))
				t.Cleanup(func() { deathWrite(t, managed, original) })
			}
			if _, err := adapter.ProjectActiveNative(t.Context(), selectedRoot, envelope, plan, selectedData); err == nil || mismatch != "package" && !errors.Is(err, domain.ErrProjectionRefreshRequired) {
				t.Fatalf("mismatched %s gained projection authority: %v", mismatch, err)
			}
			if deathRead(t, f.config) != config || f.state(t) != state {
				t.Fatal("projection refusal changed native files or state")
			}
		})
	}
	if restored, err := adapter.ProjectActiveNative(t.Context(), root, envelope, plan, data); err != nil || !reflect.DeepEqual(restored, objects) {
		t.Fatalf("restored exact projection differs: %+v %v", restored, err)
	}
	f.apply(t, OpRemove)
	if deathRead(t, filepath.Join(data, "keep.txt")) != "retained plugin data" {
		t.Fatal("workspace removal changed plugin data")
	}
	f.assertUntouched(t, other)
}

func TestProjectVSCodeRefusesMissingOrDuplicateManagedOwnership(t *testing.T) {
	for _, malformed := range []string{"missing", "duplicate"} {
		t.Run(malformed, func(t *testing.T) {
			f := newProjectFilesFixture(t, "vscode", "disabled-with-runner")
			f.apply(t, OpInstall)
			state, err := f.engine.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			for key, binding := range state.Installations[0].Clients {
				var objects []domain.NativeObjectOwnership
				for _, object := range binding.NativeObjects {
					if object.Kind == "managed_package_directory" {
						if malformed == "duplicate" {
							objects = append(objects, object, object)
						}
					} else {
						objects = append(objects, object)
					}
				}
				binding.NativeObjects = objects
				state.Installations[0].Clients[key] = binding
			}
			body, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			deathWrite(t, filepath.Join(f.cfg.StateRoot, "state-v2.json"), string(body))
			before := deathTree(t, f.project)
			f.reopen(t)
			if out, err := f.execute(t, OpRemove); err == nil || out.Mutated {
				t.Fatalf("%s managed ownership admitted removal: %+v %v", malformed, out, err)
			}
			deathAssertTree(t, f.project, before)
			if f.runner.calls != 0 {
				t.Fatal("malformed ownership invoked a native process")
			}
		})
	}
}

func TestProjectVSCodeRejectsAttestationAndCopilotSharedSkillAdoption(t *testing.T) {
	f := newProjectFilesFixture(t, "vscode", "disabled-with-runner")
	f.cfg.Registry = all.Project()
	f.reopen(t)
	other := f.untouched(t)
	f.req.ActivationComplete = true
	if out, err := f.execute(t, OpInstall); err == nil || out.Mutated || f.state(t) != "" {
		t.Fatalf("workspace attestation admitted before installation: %+v %v", out, err)
	}
	f.req.ActivationComplete = false
	f.apply(t, OpInstall)
	before, skill, config := f.state(t), deathRead(t, f.skill), deathRead(t, f.config)
	f.req.ActivationComplete = true
	if out, err := f.execute(t, OpRepair); err == nil || out.Mutated || f.state(t) != before || deathRead(t, f.skill) != skill || deathRead(t, f.config) != config {
		t.Fatalf("workspace attestation changed installed files: %+v %v", out, err)
	}
	f.req.ActivationComplete = false
	f.req.ClientID = "copilot"
	f.req.ClientConfigRoot = filepath.Join(f.base, "copilot-user")
	deathWrite(t, filepath.Join(f.req.ClientConfigRoot, "keep"), "original Copilot user")
	if out, err := f.execute(t, OpInstall); err == nil || out.Mutated || f.state(t) != before || deathRead(t, f.skill) != skill {
		t.Fatalf("Copilot adopted the same-byte VS Code skill: %+v %v", out, err)
	}
	f.assertUntouched(t, other)
}
