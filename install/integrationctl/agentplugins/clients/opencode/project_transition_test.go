package opencode

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func projectPreparedTransition(t *testing.T) (transitionFixtureData, string) {
	t.Helper()
	f := providerTransitionFixtureScope(t, domain.ScopeProject)
	layout, err := openCodeActivationLayout(f.request)
	if err != nil {
		t.Fatal(err)
	}
	p, err := prepareOpenCodeNativeApplyLayout(layout, f.request.Delivery.ActivePath, f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects, nativeconfig.New(), shared.RenameDirectoryExclusive, os.RemoveAll)
	if err != nil {
		t.Fatal(err)
	}
	req, err := openCodeTransitionRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	txn, _, err := stageOpenCodeTransitionSkills(f.request, p)
	if err != nil {
		t.Fatal(err)
	}
	stop := errors.New("TEST stop after durable preparation")
	req.PersistPrepared = func(prepared nativeconfig.PreparedTransition) error {
		if err := freshTransitions(f).PersistPrepared(context.Background(), f.request.NativeAttempt, txn.root, transitionDTO(prepared), f.request.PreviousNativeObjects, f.request.Delivery.NativeObjects); err != nil {
			return err
		}
		return stop
	}
	if _, err := nativeconfig.New().ApplyDialectTransition(req); !errors.Is(err, stop) {
		t.Fatalf("preparation fence: %v", err)
	}
	return f, txn.root
}

func TestProjectOpenCodeTransitionRequiresSelectedContextBeforeRecovery(t *testing.T) {
	f, root := projectPreparedTransition(t)
	stateBefore, err := os.ReadFile(f.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	configBefore, err := os.ReadFile(transitionPaths(f.store.ProjectRoot).JSONC)
	if err != nil {
		t.Fatal(err)
	}
	for _, selected := range []string{"", filepath.Join(t.TempDir(), "B")} {
		transitions := freshTransitions(f)
		transitions.ProjectRoot = selected
		if _, err := transitions.Pending(); err == nil {
			t.Fatalf("discovery accepted mismatched project %q", selected)
		}
		if _, err := transitions.Reconcile(context.Background(), root); err == nil {
			t.Fatalf("recovery accepted mismatched project %q", selected)
		}
	}
	stateAfter, _ := os.ReadFile(f.store.Path)
	configAfter, _ := os.ReadFile(transitionPaths(f.store.ProjectRoot).JSONC)
	if !bytes.Equal(stateBefore, stateAfter) || !bytes.Equal(configBefore, configAfter) {
		t.Fatal("refused recovery changed state or native config")
	}
	pending, err := freshTransitions(f).Pending()
	if err != nil || len(pending) != 1 || pending[0].Root != root {
		t.Fatalf("selected project lost pending transition: %+v, %v", pending, err)
	}
	if err := freshTransitions(f).Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
}
