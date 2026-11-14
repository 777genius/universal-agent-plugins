//go:build linux

package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectGrokSDKColdPrototypeRemovalAttemptRetainsSavedOwner(t *testing.T) {
	f := newGrokPrototypeSDKFixture(t)
	state, err := f.engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := f.binding(t)
	binding.NativeActivationAttempt = "prototype-remove"
	state.Installations[0].Clients[binding.ClientBindingID] = binding
	if err := f.engine.store.Save(state); err != nil {
		t.Fatal(err)
	}
	f.engine = f.reopen(t, f.home)
	view, err := f.engine.Inspect(t.Context())
	if err == nil || !view.Recovery.Required || len(view.EnvironmentReceipts) != 1 || view.EnvironmentReceipts[0].EnvironmentID != "project-default" {
		t.Fatalf("prototype pending state lost its saved owner: %+v %v", view, err)
	}
	unchanged := snapshotGrokSDKFiles(t, f, f.config, filepath.Join(f.project, ".uap", "state-v2.json"), filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"))
	for _, owner := range []string{"foreign-environment", "project-default"} {
		if result, err := f.engine.RecoverEnvironment(t.Context(), view, owner); err == nil {
			t.Fatalf("prototype removal decision was acknowledged: %+v", result)
		}
		unchanged()
	}
	if f.binding(t).NativeActivationAttempt != binding.NativeActivationAttempt {
		t.Fatal("pending prototype marker was cleared")
	}
}

func TestProjectGrokSDKInspectionDoesNotCleanPartialNativePublication(t *testing.T) {
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	stage := filepath.Join(f.engine.cfg.OperationsDir, ".grok-enable-stage-foreign")
	deathWrite(t, stage, "partial private publication")
	before, err := os.Stat(stage)
	if err != nil {
		t.Fatal(err)
	}
	state := deathRead(t, filepath.Join(f.project, ".uap", "state-v2.json"))
	view, err := f.engine.Inspect(t.Context())
	if err == nil || !view.Recovery.Required {
		t.Fatalf("partial publication was silently adopted: %+v %v", view, err)
	}
	after, err := os.Stat(stage)
	if err != nil || !os.SameFile(before, after) || deathRead(t, stage) != "partial private publication" || deathRead(t, filepath.Join(f.project, ".uap", "state-v2.json")) != state || f.native.duplex != 0 {
		t.Fatal("read-only inspection cleaned or changed native authority", err)
	}
}
