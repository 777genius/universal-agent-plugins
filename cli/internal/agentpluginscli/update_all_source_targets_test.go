package agentpluginscli

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestUpdateAllCopilotOwnDirectoryGrant(t *testing.T) {
	rollout := newRolloutDirectoryFixture(t,
		[]domain.ClientID{domain.ClientCopilot, domain.ClientVSCode},
		[]domain.ClientID{domain.ClientCopilot})
	rollout.directory.bundle.Snapshot.Distributions[0].ReleasePolicies[0].Targets = []domain.DirectoryTarget{{
		Client: domain.ClientCopilot, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed",
	}}
	if _, _, err := rollout.cli.execute(false, "add", "--no-save", "rollout-demo", "--target", "copilot"); err != nil {
		t.Fatal(err)
	}
	rollout.publishV2()
	stdout, stderr, err := rollout.cli.execute(false, "update", "--all", "--format", "json")
	if err != nil {
		t.Fatalf("Copilot-own-grant update --all failed: %v; stdout=%s; stderr=%s", err, stdout, stderr)
	}
	result := decodeUpdateAllSourceResult(t, stdout)
	if result.Status != "completed" || result.Planned != 1 || result.Updated != 1 || result.Failed != 0 {
		t.Fatalf("unexpected signed batch outcome: %+v", result)
	}
	state, err := rollout.cli.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("batch changed physical installation/binding count: %+v", state.Installations)
	}
	binding := onlyCLIClient(state.Installations[0])
	if binding.ClientID != string(domain.ClientCopilot) || binding.PackageRevision.ReleaseSequence != 2 ||
		!reflect.DeepEqual(binding.AffectedSurfaces, []string{"copilot", "vscode"}) || len(binding.Receipts) != 2 ||
		state.Installations[0].Directory.DesiredReleaseSequence != 2 {
		t.Fatalf("batch lost signed revision, native owner, physical surfaces or receipts: %+v", state.Installations[0])
	}
}

func TestUpdateAllRetainsExistingVSCodeOwnerDirectoryGrant(t *testing.T) {
	rollout := newRolloutDirectoryFixture(t,
		[]domain.ClientID{domain.ClientCopilot, domain.ClientVSCode},
		[]domain.ClientID{domain.ClientCopilot})
	if _, _, err := rollout.cli.execute(false, "add", "--no-save", "rollout-demo", "--target", "vscode"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(rollout.cli.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	rollout.acquirer.verifiedCalls = 0
	rollout.publishV2()
	stdout, _, err := rollout.cli.execute(false, "update", "--all", "--format", "json")
	if err == nil {
		t.Fatalf("batch dropped existing VS Code owner's signed grant: %v; output=%s", err, stdout)
	}
	result := decodeUpdateAllSourceResult(t, stdout)
	if result.Status != "preflight_failed" || result.Planned != 0 || result.Updated != 0 || result.Failed != 1 ||
		len(result.Installations) != 1 || !strings.Contains(result.Installations[0].Reason,
		"release does not support complete target set; missing vscode") {
		t.Fatalf("refused signed batch outcome: %+v", result)
	}
	after, err := os.ReadFile(rollout.cli.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if rollout.acquirer.verifiedCalls != 0 || string(before) != string(after) {
		t.Fatalf("refused batch acquired or mutated state/owner/receipt/desired release: acquisitions=%d", rollout.acquirer.verifiedCalls)
	}
}

func TestUpdateAllLocalCopilotKeepsPhysicalCompatibilityTargets(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCopilot), fixtureClient(t, domain.ClientVSCode),
	})
	plugin := writeNamedCLIPlugin(t, "alpha", "1.0.0")
	if _, _, err := fixture.execute(false, "add", "--no-save", plugin, "--target", "copilot"); err != nil {
		t.Fatal(err)
	}
	writeNamedCLIPluginVersion(t, plugin, "alpha", "2.0.0")
	stdout, _, err := fixture.execute(false, "update", "--all", "--format", "json")
	if err != nil {
		t.Fatalf("local shared batch failed: %v; output=%s", err, stdout)
	}
	result := decodeUpdateAllSourceResult(t, stdout)
	if result.Status != "completed" || result.Planned != 1 || result.Updated != 1 || result.Failed != 0 ||
		len(result.Installations) != 1 || result.Installations[0].Plan == nil {
		t.Fatalf("unexpected local batch outcome: %+v", result)
	}
	selected := map[string]bool{}
	for _, target := range result.Installations[0].Plan.Targets {
		if target.Selected {
			selected[target.Target] = true
		}
	}
	if !reflect.DeepEqual(selected, map[string]bool{"copilot": true, "vscode": true}) {
		t.Fatalf("local batch narrowed physical compatibility targets: %v", selected)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || state.Installations[0].Package.Version != "2.0.0" ||
		!reflect.DeepEqual(onlyCLIClient(state.Installations[0]).AffectedSurfaces, []string{"copilot", "vscode"}) {
		t.Fatalf("local batch lost revision or shared physical surfaces: %+v", state.Installations)
	}
}

func decodeUpdateAllSourceResult(t *testing.T, output string) updateAllResult {
	t.Helper()
	var envelope struct {
		Data updateAllResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}
