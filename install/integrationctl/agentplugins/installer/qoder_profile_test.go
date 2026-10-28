package installer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/qoder"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Keep separate native registries so the public installer's persisted binding,
// rather than an adapter request assembled by the test, must select its owner.
type qoderUserRegistry struct {
	versions map[string]string
	commands []legacyports.Command
}

func (r *qoderUserRegistry) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.commands = append(r.commands, command)
	args := command.Argv
	if len(args) < 6 || args[1] != "--config-dir" || args[3] != "plugins" {
		return legacyports.CommandResult{}, fmt.Errorf("unexpected Qoder fixture command: %v", args)
	}
	root := args[2]
	switch args[4] {
	case "list":
		body := []byte("[]")
		if version := r.versions[root]; version != "" {
			body, _ = json.Marshal([]map[string]string{{"id": "profile-demo@local", "version": version}})
		}
		return legacyports.CommandResult{Stdout: body}, nil
	case "install":
		body, err := os.ReadFile(filepath.Join(args[5], ".qoder-plugin", "plugin.json"))
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		var manifest struct{ Version string }
		if err := json.Unmarshal(body, &manifest); err != nil {
			return legacyports.CommandResult{}, err
		}
		r.versions[root] = manifest.Version
	case "uninstall":
		delete(r.versions, root)
	default:
		return legacyports.CommandResult{}, fmt.Errorf("unsupported Qoder fixture command: %v", args)
	}
	return legacyports.CommandResult{}, nil
}

func TestQoderUserProfilePersistsAcrossLifecycleAndRefusesOtherProfile(t *testing.T) {
	for _, observe := range []bool{false, true} {
		t.Run(fmt.Sprintf("native_observer_%t", observe), func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			selected, foreign := filepath.Join(base, "selected profile"), filepath.Join(base, "foreign profile")
			pkg, stateRoot := filepath.Join(base, "package"), filepath.Join(base, "state")
			writeManifest := func(version string) {
				claudeSDKWrite(t, filepath.Join(pkg, "plugin.json"), fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"profile-demo","version":%q}`, version))
			}
			writeManifest("1.0.0")
			claudeSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: User profile fixture\n---\nFixture only.\n")
			registry, err := clients.NewRegistry(qoder.New())
			if err != nil {
				t.Fatal(err)
			}
			helper, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			runner := &qoderUserRegistry{versions: map[string]string{foreign: "0.9.0"}}
			config := installer.Config{StateRoot: stateRoot, Registry: registry, Runner: runner, EnableNativeObserver: observe, TrustedLocalPackages: true, HelperExecutable: helper}
			newEngine := func() *installer.Engine {
				e, err := installer.New(config)
				if err != nil {
					t.Fatal(err)
				}
				return e
			}
			e := newEngine()
			req := installer.Request{Scope: domain.ScopeUser, Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "qoder", ClientConfigRoot: selected, ClientExecutable: helper, InstallationID: "00000000-0000-4000-8000-000000000078", RequiredComponents: []string{"skills"}}
			apply := func(req installer.Request) installer.Result {
				h, err := e.Prepare(t.Context(), req)
				if err != nil {
					t.Fatalf("prepare %s: %v", req.Operation, err)
				}
				defer func() { _ = h.Close() }()
				out, err := e.Apply(t.Context(), h, installer.Decision{Confirmed: true})
				if err != nil || out.Outcome != installer.OutcomeCompleted {
					t.Fatalf("apply %s: %+v, %v", req.Operation, out, err)
				}
				return out
			}
			statePath := filepath.Join(stateRoot, "state-v2.json")
			assertBindingProfile := func() {
				state, err := (statev2.Store{Path: statePath}).Load()
				if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
					t.Fatalf("persisted user binding: %+v, %v", state, err)
				}
				for _, binding := range state.Installations[0].Clients {
					if binding.NativeProfileRoot != selected {
						t.Fatalf("persisted profile = %q, want %q", binding.NativeProfileRoot, selected)
					}
				}
			}
			req.OperationID = "qoder-user-install"
			apply(req)
			assertBindingProfile()
			// Reopen the facade to ensure update relies on persisted ownership.
			e = newEngine()
			writeManifest("2.0.0")
			req.Operation, req.OperationID = installer.OpUpdate, "qoder-user-update"
			apply(req)
			assertBindingProfile()
			if runner.versions[selected] != "2.0.0" {
				t.Fatalf("selected profile was not updated: %v", runner.versions)
			}
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			calls := len(runner.commands)
			req.Operation, req.OperationID, req.Selector = installer.OpRemove, "qoder-user-remove-foreign", req.InstallationID
			req.ClientConfigRoot = foreign
			h, err := e.Prepare(t.Context(), req)
			if err == nil {
				_, err = e.Apply(t.Context(), h, installer.Decision{Confirmed: true})
				_ = h.Close()
			}
			if err == nil {
				t.Fatal("removal accepted a different user profile")
			}
			after, readErr := os.ReadFile(statePath)
			if readErr != nil || !bytes.Equal(before, after) || len(runner.commands) != calls || runner.versions[foreign] != "0.9.0" {
				t.Fatalf("profile refusal changed state or native registrations: calls %d -> %d, versions %v, read error %v", calls, len(runner.commands), runner.versions, readErr)
			}
			req.ClientConfigRoot, req.OperationID = selected, "qoder-user-remove-selected"
			apply(req)
			if runner.versions[selected] != "" || runner.versions[foreign] != "0.9.0" {
				t.Fatalf("removal changed the wrong profile: %v", runner.versions)
			}
		})
	}
}
