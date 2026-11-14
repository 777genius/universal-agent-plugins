package installer

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCodexProjectPreparedReplayAndUpdateReportNoChange(t *testing.T) {
	f := newProjectFilesFixture(t, "codex", "disabled-with-runner")
	f.apply(t, OpInstall)
	binding := f.binding(t)
	before, foreign := f.state(t), f.untouched(t)
	managed := deathTree(t, binding.TargetLocator)
	skills := deathTree(t, filepath.Dir(f.skill))
	config := deathRead(t, f.config)
	for _, operation := range []Operation{OpInstall, OpUpdate} {
		out := f.apply(t, operation)
		if !out.NoChange || out.Mutated || f.state(t) != before {
			t.Fatalf("%s rewrote or failed to classify verified Prepared replay: %+v", operation, out)
		}
		current := f.binding(t)
		if current.Activation != domain.ActivationPrepared || current.Verification != domain.VerificationInstalled || current.LifecycleConverged() {
			t.Fatalf("%s promoted Prepared lifecycle: %+v", operation, current)
		}
		if !reflect.DeepEqual(managed, deathTree(t, binding.TargetLocator)) || !reflect.DeepEqual(skills, deathTree(t, filepath.Dir(f.skill))) || deathRead(t, f.config) != config {
			t.Fatal("unchanged operation changed owned package/native bytes")
		}
		f.assertUntouched(t, foreign)
	}
}
