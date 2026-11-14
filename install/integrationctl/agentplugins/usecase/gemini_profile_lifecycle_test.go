package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

func geminiTESTProfile(t *testing.T, name string) domain.DetectedClient {
	t.Helper()
	physical, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(physical, "TEST-"+name, ".gemini")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sentinel"), []byte(name), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte(`{"TEST":"foreign settings"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return domain.DetectedClient{ClientID: domain.ClientGemini, Status: domain.DetectionDetected, ConfigRoot: root}
}

// The resilience CLI package has no skills or MCP servers. Its freshly chosen
// root must still survive update/repair without inventing legacy ownership.
func TestGeminiEmptyInventoryRecordsProfileAndSupportsLifecycle(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(fmt.Sprintf("grouped=%t", grouped), func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
			profile := geminiTESTProfile(t, "empty-A")
			input := addInput(t, profile, "https://example.com/TEST-gemini-empty")
			input.Confirmed = true
			if _, err := service.Add(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := onlyBinding(state.Installations[0])
			if binding.NativeProfileRoot != profile.ConfigRoot {
				t.Fatalf("fresh empty-inventory profile root = %q, want %q", binding.NativeProfileRoot, profile.ConfigRoot)
			}
			if len(binding.NativeObjects) != 1 || binding.NativeObjects[0].Kind != "managed_package_directory" {
				t.Fatalf("empty package invented native ownership: %+v", binding.NativeObjects)
			}
			for _, action := range []string{"update", "repair", "remove"} {
				if err := geminiLifecycleCall(service, input, action, grouped); err != nil {
					t.Fatalf("%s: %v", action, err)
				}
			}
		})
	}
}

// Exercise the real planner, native identity observer, activator and durable
// store. A changed native home cannot grant B authority over recorded A.
func TestGeminiRecordedProfileCannotBeRedirectedByAmbientHome(t *testing.T) {
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	profileA, profileB := geminiTESTProfile(t, "native-A"), geminiTESTProfile(t, "native-B")
	input := kiroPowerInput(t, profileA, "1.0.0", "sha256:TEST-gemini-profile-tree", "sha256:TEST-gemini-profile-manifest")
	input.Confirmed = true
	added, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if onlyBinding(before.Installations[0]).NativeProfileRoot != profileA.ConfigRoot {
		t.Fatal("selected A was not recorded")
	}
	t.Setenv("GEMINI_CLI_HOME", filepath.Dir(profileB.ConfigRoot))
	nativeA := filepath.Join(profileA.ConfigRoot, "skills", "docs", "SKILL.md")
	settingsA := filepath.Join(profileA.ConfigRoot, "settings.json")
	wrong := input
	wrong.Client = profileB
	geminiAssertRefusedLifecycle(t, service, wrong, func() {
		after, loadErr := store.Load()
		if loadErr != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("wrong profile changed durable state: %v", loadErr)
		}
	}, nativeA, settingsA, filepath.Join(profileB.ConfigRoot, "sentinel"), filepath.Join(profileB.ConfigRoot, "settings.json"), filepath.Join(added.Plan.ActivePath, "plugin.json"))
	for _, action := range []string{"update", "repair", "remove"} {
		if err := geminiLifecycleCall(service, input, action, false); err != nil {
			t.Fatalf("recorded A with ambient B, %s: %v", action, err)
		}
	}
	if _, err := os.Stat(nativeA); !os.IsNotExist(err) {
		t.Fatalf("A native skill survived successful removal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(profileB.ConfigRoot, "skills")); !os.IsNotExist(err) {
		t.Fatalf("ambient B received native writes: %v", err)
	}
}

func TestGeminiUnrecordedEmptyLegacyBindingRemainsUnproven(t *testing.T) {
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	input := addInput(t, geminiTESTProfile(t, "legacy-empty"), "https://example.com/TEST-gemini-legacy")
	input.Confirmed = true
	added, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.NativeProfileRoot = ""
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	geminiAssertRefusedLifecycle(t, service, input, func() {
		after, loadErr := store.Load()
		if loadErr != nil || !reflect.DeepEqual(state, after) {
			t.Fatalf("unproven legacy binding was migrated: %v", loadErr)
		}
	}, filepath.Join(input.Client.ConfigRoot, "sentinel"), filepath.Join(added.Plan.ActivePath, "plugin.json"))
}

func geminiAssertRefusedLifecycle(t *testing.T, service Service, input AddInput, checkState func(), paths ...string) {
	t.Helper()
	before := make([][]byte, len(paths))
	for index, path := range paths {
		var err error
		before[index], err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, grouped := range []bool{false, true} {
		for _, action := range []string{"update", "repair", "remove"} {
			if err := geminiLifecycleCall(service, input, action, grouped); err == nil {
				t.Fatalf("unproven profile accepted %s, grouped=%t", action, grouped)
			}
			checkState()
			for index, path := range paths {
				body, err := os.ReadFile(path)
				if err != nil || string(body) != string(before[index]) {
					t.Fatalf("refused %s changed %s: %v", action, path, err)
				}
			}
		}
	}
}

func geminiLifecycleCall(service Service, input AddInput, action string, grouped bool) error {
	ctx := context.Background()
	input.OperationID = "TEST-gemini-" + action
	group := GroupInput{Targets: []AddInput{input}, Confirmed: true, OperationGroupID: input.OperationID}
	remove := RemoveInput{Selector: input.InstallationID, Client: input.Client, Scope: input.Scope, Confirmed: true, ExternalUninstalled: true, OperationID: input.OperationID}
	var err error
	switch action {
	case "update":
		if grouped {
			_, err = service.UpdateGroup(ctx, group)
		} else {
			_, err = service.Update(ctx, input)
		}
	case "repair":
		if grouped {
			_, err = service.RepairGroup(ctx, group)
		} else {
			_, err = service.Repair(ctx, input)
		}
	case "remove":
		if grouped {
			_, err = service.RemoveGroup(ctx, RemoveGroupInput{Selector: input.InstallationID, Targets: []RemoveInput{remove}, Confirmed: true})
		} else {
			_, err = service.Remove(ctx, remove)
		}
	}
	return err
}
