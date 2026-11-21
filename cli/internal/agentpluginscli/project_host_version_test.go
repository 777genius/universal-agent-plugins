package agentpluginscli

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type versionBoundaryCodex struct {
	*codex.Adapter
	observed *string
	surfaces *[]domain.ClientSurface
}

func (adapter *versionBoundaryCodex) CheckPlanPrecondition(in clients.PlanInput, _ *domain.DeliveryPlan) error {
	*adapter.observed = in.Client.Version
	if adapter.surfaces != nil {
		*adapter.surfaces = append([]domain.ClientSurface(nil), in.Client.Surfaces...)
	}
	return errors.New("inert host-version boundary reached")
}

// Windsurf's direct no-save path already retains the domain client. This proves
// the existing qualified facade builder forwards its own selected metadata too.
func TestProjectHostVersionCrossesExistingNoSaveFacadeBoundary(t *testing.T) {
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	client.Version = "selected-host-version"
	client.Surfaces = []domain.ClientSurface{{ID: "selected-cli", Detected: true, Evidence: "selected-host-only"}}
	f := newCLIFixture(t, []domain.DetectedClient{client})
	observed := ""
	var surfaces []domain.ClientSurface
	registry, err := clients.NewRegistry(&versionBoundaryCodex{Adapter: codex.New(), observed: &observed, surfaces: &surfaces})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: filepath.Join(f.root, "facade"), Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Installer = engine
	f.app.Lifecycle.Planner = nil
	_, _, err = f.execute(false, "add", writeCLIPlugin(t), "--no-save", "--target", "codex", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "inert host-version boundary reached") || observed != client.Version || !reflect.DeepEqual(surfaces, client.Surfaces) {
		t.Fatalf("selected host evidence=%q,%v", observed, err)
	}
}

func TestProjectDependencyKnownFactsRetainOnlyUnselectedSiblings(t *testing.T) {
	root := t.TempDir()
	sibling := domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: filepath.Join(root, "gemini"), ExecutablePath: filepath.Join(root, "gemini-cli")}
	adapter := dependencyInstaller{app: App{Scope: domain.ScopeProject, ProjectRoot: root}, detected: map[domain.ClientID]domain.DetectedClient{domain.ClientGemini: sibling}}
	view := installer.Inspection{Installations: []installer.InspectedInstallation{{InstallationID: "selected", Bindings: []installer.InspectedBinding{{ClientID: "windsurf", BindingID: "selected-binding"}, {ClientID: "gemini", BindingID: "sibling-binding"}}}}}
	request := installer.Request{ClientID: "windsurf", ClientVersion: "3000.11.205"}
	if err := adapter.populateDependencyTargets(view, "selected", &request); err != nil {
		t.Fatal(err)
	}
	if len(request.KnownTargets) != 1 || request.KnownTargets[0] != (installer.TargetFacts{ClientID: "gemini", BindingID: "sibling-binding", ConfigRoot: sibling.ConfigRoot, Executable: sibling.ExecutablePath}) {
		t.Fatalf("selected/sibling facts mixed: %+v", request.KnownTargets)
	}
	if request.ClientVersion != "3000.11.205" || request.ClientConfigRoot != "" {
		t.Fatal("primary host evidence changed")
	}
	delete(adapter.detected, domain.ClientGemini)
	if err := adapter.populateDependencyTargets(view, "selected", &installer.Request{ClientID: "windsurf"}); err == nil {
		t.Fatal("unselected sibling missing facts accepted")
	}
}
