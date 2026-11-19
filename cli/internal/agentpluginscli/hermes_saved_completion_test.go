package agentpluginscli

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// The actual mixed package and binding cannot gain an optional trust-completion
// repair, including when the native version is unavailable. Preserve genuine
// owner-requested repairs independently of that optional decision.
func TestHermesCompletionSavedMixedKeepsNoopWithoutVersionProbe(t *testing.T) {
	origin := hermesCompletionArchive(t, true)
	journey := newSavedJourney(t, t.TempDir())
	journey.cli.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: origin.Client()}
	runner := setHermesCompletionHost(t, &journey)
	journey.run(t, "add", origin.URL+"/demo.zip", "--target", "hermes", "--scope", "project",
		"--project-root", journey.cli.app.ProjectRoot, "--yes", "--format", "json")
	clients, err := journey.cli.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	clients[0].Version = ""
	detector := &observedProbingDetector{clients: clients}
	journey.cli.app.Detector = detector
	runner.grants = 0
	before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
	command := &cobra.Command{Use: "add"}
	command.SetContext(t.Context())
	var output bytes.Buffer
	command.SetOut(&output)
	opts := &options{scope: "project", target: "hermes", yes: true, format: "json"}
	snapshot, err := snapshotEnvironment(command, journey.cli.app)
	if err != nil {
		t.Fatal(err)
	}
	request, err := prepareSavedSync(command, journey.cli.app, opts, snapshot, snapshot.Requirements, snapshot.Lock,
		snapshot.ManifestBytes, []string{"demo"}, nil, []string{"hermes"})
	if err != nil {
		t.Fatalf("ineligible unchanged package required native setup: %v", err)
	}
	rechecked, err := request.recheck(command, journey.cli.app, opts)
	if err != nil || !reflect.DeepEqual(request.prepared, rechecked) {
		t.Fatalf("mixed no-op changed during recheck: %+v %v", rechecked, err)
	}
	if len(rechecked) != 1 || rechecked[0].Action != environment.ActionUnchanged || detector.targetedCalls != 0 || runner.grants != 0 {
		t.Fatalf("optional completion reached native preflight: actions=%+v probes=%d grants=%d", rechecked, detector.targetedCalls, runner.grants)
	}
	state := journey.state(t)
	repairs, err := buildDependencyRepairs(t.Context(), journey.cli.app, state, state.Installations[0], []domain.ClientID{domain.ClientHermes})
	if err != nil {
		t.Fatal(err)
	}
	retained, err := reviewedSavedAddActions(command, journey.cli.app, opts, []string{"hermes"}, repairs)
	if err != nil || len(retained) != 1 || retained[0].Action != "repair" || !reflect.DeepEqual(retained, repairs) {
		t.Fatalf("real repair was filtered by optional completion eligibility: %+v %v", retained, err)
	}
	if !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("optional setup preflight changed installed state or intent")
	}
}
