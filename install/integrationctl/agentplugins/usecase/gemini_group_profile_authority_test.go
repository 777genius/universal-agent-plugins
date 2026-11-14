package usecase

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

// Every incoming authority must be checked, including targets whose package
// path collides. A refusal must leave all durable and native bytes unchanged.
func TestGeminiGroupRejectsConflictingRecordedProfiles(t *testing.T) {
	for _, skills := range []bool{false, true} {
		for _, action := range []string{"update", "repair"} {
			for _, conflictingFirst := range []bool{false, true} {
				t.Run(fmt.Sprintf("skills=%t/%s/conflictingFirst=%t", skills, action, conflictingFirst), func(t *testing.T) {
					service, store, _ := serviceFixture(t)
					service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
					// Pin only coordination metadata so a full tree comparison includes the real lock.
					lock := service.Lock.(processlock.Lock)
					lock.Now = service.Now
					service.Lock = lock
					profileA, profileB := geminiTESTProfile(t, "group-A"), geminiTESTProfile(t, "group-B")
					input := geminiGroupInput(t, profileA, skills)
					if _, err := service.Add(context.Background(), input); err != nil {
						t.Fatal(err)
					}
					before, err := store.Load()
					if err != nil || onlyBinding(before.Installations[0]).NativeProfileRoot != profileA.ConfigRoot {
						t.Fatalf("fixture did not record A: %v", err)
					}
					if action == "update" {
						setEnvelopeVersion(t, &input.Envelope, "2.0.0", "sha256:TEST-group-v2", "sha256:TEST-group-manifest-v2")
					}
					wrong := input
					wrong.Client = profileB
					targets := []AddInput{input, wrong}
					if conflictingFirst {
						targets = []AddInput{wrong, input}
					}
					roots := []string{filepath.Dir(filepath.Dir(store.Path)), profileA.ConfigRoot, profileB.ConfigRoot}
					bytesBefore := geminiGroupTrees(t, roots...)
					progressEvents := 0
					result, err := geminiGroupCall(service, targets, action, func(GroupProgressEvent) { progressEvents++ })
					if err == nil || result.Mutated || len(result.Receipts) != 0 || result.Phase != GroupPhasePlanned || progressEvents != 0 {
						t.Fatalf("conflicting group passed mutation preflight: result=%+v, err=%v", result, err)
					}
					after, loadErr := store.Load()
					if loadErr != nil || !reflect.DeepEqual(before, after) {
						t.Fatalf("refusal changed complete durable state: %v", loadErr)
					}
					if !reflect.DeepEqual(bytesBefore, geminiGroupTrees(t, roots...)) {
						t.Fatal("refusal changed managed/staged/journal/state or either TEST profile tree")
					}
				})
			}
		}
	}
}

// With no recorded binding, the selected authorities must still agree before
// a collision can discard one. This exercises the merge guard independently.
func TestGeminiGroupRejectsConflictingNewProfiles(t *testing.T) {
	for _, conflictingFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("conflictingFirst=%t", conflictingFirst), func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
			profileA, profileB := geminiTESTProfile(t, "new-A"), geminiTESTProfile(t, "new-B")
			input := geminiGroupInput(t, profileA, false)
			wrong := input
			wrong.Client = profileB
			targets := []AddInput{input, wrong}
			if conflictingFirst {
				targets = []AddInput{wrong, input}
			}
			before := geminiGroupTrees(t, profileA.ConfigRoot, profileB.ConfigRoot)
			progressEvents := 0
			result, err := geminiGroupCall(service, targets, "add", func(GroupProgressEvent) { progressEvents++ })
			if err == nil || result.Mutated || len(result.Receipts) != 0 || result.Phase != GroupPhasePlanned || progressEvents != 0 {
				t.Fatalf("conflicting new profiles coalesced: result=%+v, err=%v", result, err)
			}
			state, loadErr := store.Load()
			if loadErr != nil || len(state.Installations) != 0 {
				t.Fatalf("refusal created durable installation: %v", loadErr)
			}
			if !reflect.DeepEqual(before, geminiGroupTrees(t, profileA.ConfigRoot, profileB.ConfigRoot)) {
				t.Fatal("refusal changed TEST profiles")
			}
			if _, statErr := os.Lstat(filepath.Join(filepath.Dir(filepath.Dir(store.Path)), "managed")); !os.IsNotExist(statErr) {
				t.Fatalf("refusal created managed/staging tree: %v", statErr)
			}
		})
	}
}

func TestGeminiGroupSameProfileCoalescesAcrossLifecycle(t *testing.T) {
	for _, skills := range []bool{false, true} {
		t.Run(fmt.Sprintf("skills=%t", skills), func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
			profile := geminiTESTProfile(t, "same-profile")
			input := geminiGroupInput(t, profile, skills)
			for _, action := range []string{"add", "noop", "update", "repair"} {
				if action == "update" {
					setEnvelopeVersion(t, &input.Envelope, "2.0.0", "sha256:TEST-same-v2", "sha256:TEST-same-manifest-v2")
				}
				result, err := geminiGroupCall(service, []AddInput{input, input}, action, nil)
				if err != nil {
					t.Fatalf("%s: %v", action, err)
				}
				geminiAssertCoalescedResult(t, action, result)
				state, loadErr := store.Load()
				if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
					t.Fatalf("%s did not retain one durable physical binding: %v", action, loadErr)
				}
				binding := onlyBinding(state.Installations[0])
				if binding.NativeProfileRoot != profile.ConfigRoot || binding.PackageRevision.Version != input.Envelope.Manifest.Version {
					t.Fatalf("%s lost authority or replacement version: %+v", action, binding)
				}
			}
			if err := geminiLifecycleCall(service, input, "remove", true); err != nil {
				t.Fatal(err)
			}
			if body, err := os.ReadFile(filepath.Join(profile.ConfigRoot, "settings.json")); err != nil || string(body) != `{"TEST":"foreign settings"}` {
				t.Fatalf("lifecycle changed foreign settings: %v", err)
			}
		})
	}
}

func geminiGroupInput(t *testing.T, profile domain.DetectedClient, skills bool) AddInput {
	t.Helper()
	input := addInput(t, profile, "https://example.test/TEST-group-profile")
	if skills {
		input = kiroPowerInput(t, profile, "1.0.0", "sha256:TEST-group-skills", "sha256:TEST-group-skills-manifest")
	}
	input.Confirmed = true
	return input
}

func geminiGroupCall(service Service, targets []AddInput, action string, progress func(GroupProgressEvent)) (GroupResult, error) {
	group := GroupInput{Targets: targets, Confirmed: true, Progress: progress, OperationGroupID: "TEST-profile-" + action}
	switch action {
	case "add":
		return service.AddGroup(context.Background(), group)
	case "repair":
		return service.RepairGroup(context.Background(), group)
	default:
		return service.UpdateGroup(context.Background(), group)
	}
}

func geminiAssertCoalescedResult(t *testing.T, action string, result GroupResult) {
	t.Helper()
	if len(result.Targets) != 2 || result.Targets[0].Plan.ActivePath != result.Targets[1].Plan.ActivePath {
		t.Fatalf("%s lost coalesced result indexes: %+v", action, result)
	}
	if action == "noop" {
		if result.Mutated || len(result.Receipts) != 0 || !result.Targets[0].NoChange || !result.Targets[1].NoChange || result.Phase != GroupPhaseCompleted {
			t.Fatalf("coalesced no-op = %+v", result)
		}
		return
	}
	if !result.Mutated || len(result.Receipts) != 1 || result.Phase != GroupPhaseCompleted {
		t.Fatalf("%s did not mutate exactly one physical target: %+v", action, result)
	}
	for _, target := range result.Targets {
		if target.GroupPhase != GroupTargetExternalCompleted || target.Activation.Activation != result.Targets[0].Activation.Activation || target.NoChange {
			t.Fatalf("%s failed to complete a coalesced result index: %+v", action, target)
		}
	}
}

// Capture entry kinds, permissions, regular bytes and symlink destinations;
// inspecting only settings.json would miss publication, staging or skill edits.
func geminiGroupTrees(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	entries := map[string]string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			var body []byte
			if info.Mode().IsRegular() {
				body, err = os.ReadFile(path)
			} else if info.Mode()&os.ModeSymlink != 0 {
				var destination string
				destination, err = os.Readlink(path)
				body = []byte(destination)
			}
			entries[path] = info.Mode().String() + ":" + string(body)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return entries
}
