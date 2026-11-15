package transaction

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type rootPreflightState struct {
	StateStore
	loads int
}

func (state *rootPreflightState) Load() (domain.StateFileV2, error) {
	state.loads++
	if state.loads == 2 {
		return domain.StateFileV2{}, errors.New("state unavailable at kernel ownership preflight")
	}
	return state.StateStore.Load()
}

func TestProjectRootKernelPreflightRefusalCleansExactUnjournaledStage(t *testing.T) {
	publisher, request, spec, store := projectRootFixture(t)
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	publisher.Kernel.StateStore = &rootPreflightState{StateStore: store}
	if err := publisher.PublishProjectRoot(t.Context(), request, spec); err == nil {
		t.Fatal("kernel preflight refusal was ignored")
	}
	entries, err := os.ReadDir(request.Plan.TargetContext.ProjectRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".uap-native-root-") {
			t.Fatal("kernel preflight retained an unjournaled staging directory")
		}
	}
	if _, err := os.Lstat(spec.Path); !os.IsNotExist(err) {
		t.Fatalf("preflight published root: %v", err)
	}
	after, err := store.Load()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("preflight changed state: %v", err)
	}
}

func TestProjectRootOpenIntentRetainsStagingForExistingRecovery(t *testing.T) {
	publisher, request, spec, store := projectRootFixture(t)
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	publisher.Kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseActivationPending || phase == dirswap.PhaseRollbackPending {
			return errors.New("interrupted directory operation")
		}
		return nil
	}
	if err := publisher.PublishProjectRoot(t.Context(), request, spec); err == nil {
		t.Fatal("interrupted publication reported success")
	}
	open, err := publisher.Kernel.Directory.ListOpen()
	if err != nil || len(open) != 1 {
		t.Fatalf("open intent lost: %+v %v", open, err)
	}
	if _, err := os.Lstat(filepath.Join(open[0].StagingPath, filepath.Base(spec.Path), filepath.Base(spec.Marker.Path))); err != nil {
		t.Fatalf("publisher removed staging owned by open journal: %v", err)
	}
	publisher.Kernel.Directory.Fault = nil
	if err := publisher.Kernel.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, err := store.Load()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("recovery changed uncommitted state: %v", err)
	}
}
