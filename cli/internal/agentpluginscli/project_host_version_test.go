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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
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

// Verify host evidence through the no-save CLI and saved dependency builders.
// Skills shares the latter's real Prepare/installer path.
func TestProjectHostEvidenceCrossesExistingInstallerBoundaries(t *testing.T) {
	for _, route := range []string{"no-save", "dependency"} {
		t.Run(route, func(t *testing.T) {
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
			if route == "dependency" {
				app, pin, _ := dependencyStagingFixture(t)
				app.Installer, app.SecurityEvaluator = engine, dependencyTestScanner{}
				adapter := dependencyInstaller{app: app, opts: &options{},
					detected: map[domain.ClientID]domain.DetectedClient{client.ClientID: client}}
				_, err = adapter.Prepare(t.Context(), environment.PreparedBinding{
					Alias: pin.Alias, ClientID: string(client.ClientID), Action: environment.ActionInstall, Package: &pin})
			} else {
				f.app.Installer = engine
				f.app.Lifecycle.Planner = nil
				_, _, err = f.execute(false, "add", writeCLIPlugin(t), "--no-save", "--target", "codex", "--format", "json")
			}
			if err == nil || !strings.Contains(err.Error(), "inert host-version boundary reached") || observed != client.Version || !reflect.DeepEqual(surfaces, client.Surfaces) {
				t.Fatalf("selected host evidence: version=%q surfaces=%+v error=%v", observed, surfaces, err)
			}
		})
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
