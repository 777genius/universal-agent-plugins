package agentpluginscli

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type versionBoundaryCodex struct {
	*codex.Adapter
	observed *string
}

func (adapter *versionBoundaryCodex) CheckPlanPrecondition(in clients.PlanInput, _ *domain.DeliveryPlan) error {
	*adapter.observed = in.Client.Version
	return errors.New("inert host-version boundary reached")
}

// Windsurf's direct no-save path already retains the domain client. This proves
// the existing qualified facade builder forwards its own selected metadata too.
func TestProjectHostVersionCrossesExistingNoSaveFacadeBoundary(t *testing.T) {
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	client.Version = "selected-host-version"
	f := newCLIFixture(t, []domain.DetectedClient{client})
	observed := ""
	registry, err := clients.NewRegistry(&versionBoundaryCodex{Adapter: codex.New(), observed: &observed})
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
	if err == nil || !strings.Contains(err.Error(), "inert host-version boundary reached") || observed != client.Version {
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
	if len(request.KnownTargets) != 1 || !reflect.DeepEqual(request.KnownTargets[0], installer.TargetFacts{ClientID: "gemini", BindingID: "sibling-binding", ConfigRoot: sibling.ConfigRoot, Executable: sibling.ExecutablePath}) {
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

func TestDependencyKnownFactsRetainNativeOnlyUserSibling(t *testing.T) {
	root := t.TempDir()
	registry, err := clients.NewRegistry(windsurf.New())
	if err != nil {
		t.Fatal(err)
	}
	sibling := domain.DetectedClient{ClientID: domain.ClientWindsurf,
		NativeConfigRoot: filepath.Join(root, "original", "devin"),
		ExecutablePath:   filepath.Join(root, "bin", "devin"), Version: "3000.12.57"}
	adapter := dependencyInstaller{app: App{Scope: domain.ScopeUser, ClientRegistry: registry},
		detected: map[domain.ClientID]domain.DetectedClient{domain.ClientWindsurf: sibling}}
	view := installer.Inspection{Installations: []installer.InspectedInstallation{{InstallationID: "same-installation",
		Bindings: []installer.InspectedBinding{{ClientID: "cline", BindingID: "selected-binding"}, {ClientID: "windsurf", BindingID: "native-binding"}}}}}
	request := installer.Request{ClientID: "cline"}
	if err := adapter.populateDependencyTargets(view, "same-installation", &request); err != nil {
		t.Fatal(err)
	}
	if len(request.KnownTargets) != 1 || !reflect.DeepEqual(request.KnownTargets[0], installer.TargetFacts{ClientID: "windsurf", BindingID: "native-binding",
		NativeConfigRoot: sibling.NativeConfigRoot, Executable: sibling.ExecutablePath, Version: sibling.Version}) {
		t.Fatal("CLI dropped native-only sibling facts or invented a legacy root", request.KnownTargets)
	}
}
