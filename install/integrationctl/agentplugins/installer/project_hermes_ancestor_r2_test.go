package installer_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/hermes"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// Diagnostic regression: the real SDK creates the receipt; no digest or state
// proof is fabricated and every runner callback is inert.
func TestProjectHermesR2RejectsSubstitutedProjectAncestor(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, pkg := hermesConflictPackage(t, base)
	fixture := newHermesSDKFixture(t, filepath.Join(base, "project"), user, pkg)
	fixture.apply(t, installer.OpInstall, "ancestor-proof-install")
	binding := fixture.binding(t)
	statePath := filepath.Join(fixture.project, ".uap", "state-v2.json")
	root := filepath.Join(fixture.project, ".hermes", "uap-session")
	before := map[string]os.FileInfo{}
	for _, path := range []string{fixture.project, filepath.Dir(root), root, statePath} {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path], err = file.Stat()
		if closeErr := file.Close(); err != nil || closeErr != nil {
			t.Fatalf("capture original identity: %v %v", err, closeErr)
		}
	}
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	store := statev2.Store{Path: statePath, ProjectRoot: fixture.project}
	publisher := transaction.ProjectRootPublisher{Kernel: transaction.Kernel{StateStore: store}}
	var owner domain.NativeObjectOwnership
	for _, object := range binding.NativeObjects {
		if object.Kind == "hermes_project_session_root" {
			owner = object
		}
	}
	if err := publisher.VerifyProjectRoot(t.Context(), root, owner); err != nil {
		t.Fatal("original publication was not valid", err)
	}
	request := clients.SessionRequest{ProjectRoot: fixture.project, Inspect: true, Bindings: []domain.ClientBinding{binding},
		Client: domain.DetectedClient{ClientID: domain.ClientHermes, ConfigRoot: fixture.user,
			ExecutablePath: fixture.request.ClientExecutable, Status: domain.DetectionDetected,
			Surfaces: []domain.ClientSurface{{ID: "hermes_cli", Detected: true}}}}
	runner := &hermesSessionSwapRunner{t: t, root: root}
	adapter, env := hermes.New(), clients.Env{ProjectRoots: publisher, Runner: runner}
	originalPlan, err := adapter.PlanSession(t.Context(), env, request)
	if err != nil {
		t.Fatal("original session was not valid", err)
	}
	original := fixture.project + "-original"
	if err := os.Rename(fixture.project, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(fixture.project, before[fixture.project].Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	children, err := os.ReadDir(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range children {
		if err := os.Rename(filepath.Join(original, child.Name()), filepath.Join(fixture.project, child.Name())); err != nil {
			t.Fatal(err)
		}
	}
	for path, previous := range before {
		current, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(previous, current) != (path != fixture.project) {
			t.Fatalf("fixture did not replace only Project ancestor: %s", path)
		}
	}
	currentBytes, err := os.ReadFile(statePath)
	if err != nil || !bytes.Equal(stateBytes, currentBytes) {
		t.Fatalf("fixture changed original persisted state: %v", err)
	}
	if named, err := filepath.EvalSymlinks(root); err != nil || named != root {
		t.Fatalf("fixture changed named path or introduced alias: %s %v", named, err)
	}
	if err := publisher.VerifyProjectRoot(t.Context(), root, owner); err == nil {
		t.Error("VerifyProjectRoot accepted substituted Project with original parent/root/state inodes")
	}
	if _, err := adapter.PlanSession(t.Context(), env, request); err == nil {
		t.Error("PlanSession accepted substituted Project ancestor")
	}
	if err := adapter.VerifySession(t.Context(), env, originalPlan); err == nil || runner.calls != 0 {
		t.Errorf("VerifySession accepted substituted Project or started observation: %v (%d calls)", err, runner.calls)
	}
	fixture.request.Operation, fixture.request.OperationID = installer.OpInstall, "ancestor-repeat-preflight"
	handle, err := fixture.engine.Prepare(t.Context(), fixture.request)
	if err != nil {
		t.Log("Engine.Prepare independently refused", err)
	} else {
		t.Error("Engine.Prepare accepted the substituted Project; no Apply performed")
		if err := handle.Close(); err != nil {
			t.Fatal(err)
		}
	}
	cold := newHermesSDKFixture(t, fixture.project, fixture.user, pkg)
	cold.request.Operation, cold.request.OperationID = installer.OpInstall, "cold-ancestor-repeat-preflight"
	coldHandle, coldErr := cold.engine.Prepare(t.Context(), cold.request)
	if coldErr != nil {
		t.Log("fresh Engine.Prepare independently refused", coldErr)
	} else {
		t.Error("fresh Engine.Prepare accepted the substituted Project; no Apply performed")
		if err := coldHandle.Close(); err != nil {
			t.Fatal(err)
		}
	}
	currentBytes, err = os.ReadFile(statePath)
	if err != nil || !bytes.Equal(stateBytes, currentBytes) || fixture.runner.calls != 0 {
		t.Fatal("refusal changed persisted state or invoked an actual native runner", err)
	}
	for path, previous := range before {
		if path == fixture.project {
			continue
		}
		current, err := os.Stat(path)
		if err != nil || !os.SameFile(previous, current) || previous.Mode() != current.Mode() || !previous.ModTime().Equal(current.ModTime()) {
			t.Fatalf("refusal changed original object at %s: %v", path, err)
		}
	}
}
