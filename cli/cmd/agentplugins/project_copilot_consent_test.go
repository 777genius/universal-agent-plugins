package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
)

// Called by the composed saved lifecycle after A and B have owned packages.
// Missing trust must follow ordinary add consent without changing either pin.
func projectCopilotRestoreTrust(t *testing.T, journey projectJourney, trustPath string) {
	t.Helper()
	original := `{"trustedFolders":["/foreign-project"],"allowedTools":["shell(git status)"],"theme":"dark"}`
	projectJourneyWrite(t, trustPath, original)
	userBefore := securityFixtureTree(t, filepath.Dir(trustPath))
	aBefore, bBefore := securityFixtureTree(t, journey.a), securityFixtureTree(t, journey.b)
	unchanged := func() {
		t.Helper()
		if !reflect.DeepEqual(userBefore, securityFixtureTree(t, filepath.Dir(trustPath))) ||
			!reflect.DeepEqual(aBefore, securityFixtureTree(t, journey.a)) ||
			!reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
			t.Fatal("unconfirmed or read-only Copilot flow changed trust, ownership or project bytes")
		}
	}
	journey.run(t, journey.a, "add", journey.transport.endpoint, "--alias", "docs", "--target", "copilot", "--dry-run")
	unchanged()
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	unchanged()
	if _, err := journey.execute(journey.a, "add", journey.transport.endpoint, "--alias", "docs"); err == nil {
		t.Fatal("automated targetless saved add bypassed target selection")
	}
	unchanged()
	projectCopilotInteractiveAdd(t, journey, "n\n")
	unchanged()
	before, binding := projectCopilotOwned(t, journey, journey.a, projectJourneySkill)
	journey.run(t, journey.a, "add", journey.transport.endpoint, "--alias", "docs", "--target", "copilot")
	projectCopilotTrust(t, trustPath, []string{"/foreign-project", journey.a})
	projectJourneyWrite(t, trustPath, original)
	projectCopilotInteractiveAdd(t, journey, "y\n")
	projectCopilotTrust(t, trustPath, []string{"/foreign-project", journey.a})
	after, current := projectCopilotOwned(t, journey, journey.a, projectJourneySkill)
	if !reflect.DeepEqual(before.Package, after.Package) || !reflect.DeepEqual(before.Source, after.Source) ||
		!reflect.DeepEqual(before.DataReceipts, after.DataReceipts) || current.TargetLocator != binding.TargetLocator ||
		!reflect.DeepEqual(current.NativeObjects, binding.NativeObjects) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
		t.Fatal("restoring selected Copilot trust replaced package authority or changed project B")
	}
}

func projectCopilotInteractiveAdd(t *testing.T, journey projectJourney, answer string) {
	t.Helper()
	var output, stderr bytes.Buffer
	app := journey.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(answer), &output, &stderr, true
	root := agentpluginscli.NewRoot(app)
	root.SetArgs([]string{"add", journey.transport.endpoint, "--alias", "docs", "--scope", "project", "--project-root", journey.a, "--format", "human", "--plain"})
	if err := root.Execute(); err != nil {
		t.Fatalf("interactive saved Copilot add failed: %v\n%s%s", err, output.String(), stderr.String())
	}
	if !strings.Contains(output.String()+stderr.String(), "Apply the preflighted dependency sync?") {
		t.Fatal("interactive saved Copilot add did not request the existing confirmation")
	}
}
