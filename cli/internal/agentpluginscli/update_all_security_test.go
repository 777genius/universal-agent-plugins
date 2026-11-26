package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type updateAllBlockingSecurity struct {
	name  string
	calls int
}

func (e *updateAllBlockingSecurity) Evaluate(_ context.Context, input domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	if input.PluginName == e.name {
		e.calls++
		return *testSecurityAssessment(1, 0), nil
	}
	return domain.SecurityAssessment{}, nil
}

func updateAllSecurityFixture(t *testing.T) (cliFixture, *updateAllBlockingSecurity, *cliObservedActivator) {
	t.Helper()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	for _, name := range []string{"alpha", "beta"} {
		root := writeNamedCLIPlugin(t, name, "1.0.0")
		if _, _, err := fixture.execute(false, "add", root, "--target", "cursor", "--no-save"); err != nil {
			t.Fatal(err)
		}
		writeNamedCLIPluginVersion(t, root, name, "2.0.0")
	}
	evaluator := &updateAllBlockingSecurity{name: "beta"}
	fixture.app.SecurityEvaluator = evaluator
	fixture.app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return evaluator }
	activator := &cliObservedActivator{outcome: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid}}
	fixture.app.Lifecycle.Activator = activator
	return fixture, evaluator, activator
}

func TestUpdateAllSecurityPreflight(t *testing.T) {
	fixture, evaluator, activator := updateAllSecurityFixture(t)
	before := savedCheckFiles(t, fixture.root)
	output, _, err := fixture.execute(false, "update", "--all", "--yes", "--format", "json")
	var envelope struct {
		Result string          `json:"result"`
		Data   updateAllResult `json:"data"`
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	if decodeErr := decoder.Decode(&envelope); decodeErr != nil {
		t.Fatalf("decode response: %v\n%s", decodeErr, output)
	}
	var extra any
	if decodeErr := decoder.Decode(&extra); !errors.Is(decodeErr, io.EOF) {
		t.Errorf("batch emitted more than one JSON response: %v\n%s", decodeErr, output)
	}
	if err == nil || envelope.Result != "failure" || envelope.Data.Status != "preflight_failed" || envelope.Data.Updated != 0 || envelope.Data.Failed != 1 {
		t.Errorf("blocking assessment did not stop batch before effects: %v\n%s", err, output)
	}
	found := false
	for _, row := range envelope.Data.Installations {
		if row.Name == "beta" && row.Status == "preflight_failed" && strings.Contains(row.Reason, "--accept-security-risk") {
			found = true
		}
	}
	if !found || evaluator.calls == 0 {
		t.Errorf("batch did not retain the blocked preparation: %s", output)
	}
	if !reflect.DeepEqual(before, savedCheckFiles(t, fixture.root)) || activator.calls != 0 {
		t.Error("unauthorized batch changed installed state, managed files or receipts, or called native activation")
	}
}

func TestUpdateAllSecurityHumanConsent(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		name := map[bool]string{false: "imperative", true: "mixed-saved"}[mixed]
		t.Run(name, func(t *testing.T) {
			for _, accept := range []bool{false, true} {
				t.Run(map[bool]string{false: "reject", true: "accept"}[accept], func(t *testing.T) {
					fixture, evaluator, activator := updateAllSecurityHumanFixture(t, mixed)
					before := savedCheckFiles(t, fixture.root)
					input := "no\n"
					if accept {
						input = "yes\nyes\n"
					} // Security consent, then saved graph confirmation when present.
					output, stderr, err := fixture.executeInput(true, input, "update", "--all", "--format", "human", "--plain", "--color", "never")
					visible := output + stderr
					if strings.Count(visible, "Continue anyway?") != 1 || !strings.Contains(visible, "not a guarantee of safety") || evaluator.calls == 0 {
						t.Errorf("security review must remain visible and prompt once through original input/output: %v\n%s", err, visible)
					}
					if !accept {
						if err == nil || !strings.Contains(visible, "preflight_failed") {
							t.Errorf("declined risk was not refused during preflight: %v\n%s", err, visible)
						}
						if !reflect.DeepEqual(before, savedCheckFiles(t, fixture.root)) || activator.calls != 0 {
							t.Error("declined risk changed installed/saved agreement or called native activation")
						}
						return
					}
					if err != nil || !strings.Contains(visible, "Update all: completed") || activator.calls == 0 {
						t.Fatalf("interactive acceptance did not apply batch: %v\n%s", err, visible)
					}
					state, loadErr := fixture.store.Load()
					if loadErr != nil {
						t.Fatal(loadErr)
					}
					for _, installation := range state.Installations {
						if installation.Package.Version != "2.0.0" {
							t.Fatal(installation)
						}
					}
					if mixed {
						_, lock := fixture.app.dependencyPaths()
						relative, relErr := filepath.Rel(fixture.root, lock)
						if relErr != nil {
							t.Fatal(relErr)
						}
						if before[relative] == dependencyRead(t, lock) {
							t.Fatal("mixed saved fixture did not publish a changed lock")
						}
					}
				})
			}
		})
	}
}

func updateAllSecurityHumanFixture(t *testing.T, mixed bool) (cliFixture, *updateAllBlockingSecurity, *cliObservedActivator) {
	t.Helper()
	if !mixed {
		return updateAllSecurityFixture(t)
	}
	j, _, _ := savedMixedUpdateJourney(t)
	evaluator := &updateAllBlockingSecurity{name: "imperative"}
	j.cli.app.SecurityEvaluator = evaluator
	j.cli.app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return evaluator }
	activator := &cliObservedActivator{outcome: domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid}}
	j.cli.app.Lifecycle.Activator = activator
	return j.cli, evaluator, activator
}
