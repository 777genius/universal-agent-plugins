package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// The assessment is a routing fixture. Acquisition, CLI parsing, composition,
// planning, state, native configuration and filesystem transactions stay real.
type updateAllSecurityEvaluator struct{ blockedCalls int }

func (e *updateAllSecurityEvaluator) Evaluate(_ context.Context, input domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	if input.PluginName != "beta" {
		return domain.SecurityAssessment{}, nil
	}
	e.blockedCalls++
	return domain.SecurityAssessment{
		Scanner:  domain.SecurityScanner{ID: domain.SecurityScannerID, Version: domain.SecurityScannerVersion},
		Outcome:  domain.SecurityBlockingFindings,
		Counts:   domain.SecurityCounts{Blocking: 1, Total: 1},
		Findings: []domain.SecurityFinding{{Code: "SEC330", Disposition: "blocking", Path: "mcp.json", Message: "inert update authorization fixture"}},
	}, nil
}

type updateAllSecurityActivator struct {
	providers.Activator
	calls *int
}

func (a updateAllSecurityActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	(*a.calls)++
	return a.Activator.Activate(ctx, request)
}

func (a updateAllSecurityActivator) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	(*a.calls)++
	return a.Activator.Deactivate(ctx, request)
}

func (a updateAllSecurityActivator) PrepareIndependentActivation(ctx context.Context, request domain.ActivationRequest) (func(context.Context) (domain.ActivationOutcome, error), error) {
	effect, err := a.Activator.PrepareIndependentActivation(ctx, request)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) (domain.ActivationOutcome, error) {
		(*a.calls)++
		return effect(ctx)
	}, nil
}

func configureUpdateAllSecurity(app agentpluginscli.App, evaluator *updateAllSecurityEvaluator, calls *int) agentpluginscli.App {
	app.SecurityEvaluator = evaluator
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return evaluator }
	app.Lifecycle.Activator = updateAllSecurityActivator{Activator: app.Lifecycle.Activator.(providers.Activator), calls: calls}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureUpdateAllSecurity(selected, evaluator, calls), nil
	}
	return app
}

type updateAllSecurityJourney struct {
	journey  projectJourney
	security *updateAllSecurityEvaluator
	calls    int
	mixed    bool
	oldLock  string
	savedID  string
	data     []string
}

func newUpdateAllSecurityJourney(t *testing.T, mixed bool) *updateAllSecurityJourney {
	t.Helper()
	source := newProjectSkillsSource(t, "https")
	f := &updateAllSecurityJourney{journey: newProjectJourney(t, source.transport), mixed: mixed}
	if mixed {
		f.journey.run(t, "", "skills", "add", source.input, "--skill", "docs", "--target", "gemini", "--no-ui", "--yes")
		f.oldLock = projectJourneyRead(t, filepath.Join(f.journey.cache, "uap.lock"))
		installation, _ := f.journey.owned(t, "")
		f.savedID = installation.InstallationID
	}
	for _, name := range []string{"alpha", "beta"} {
		root := projectJourneyTemp(t)
		writeUpdateAllSecurityPackage(t, root, name, "1.0.0")
		f.journey.run(t, "", "add", root, "--no-save", "--target", "gemini")
		writeUpdateAllSecurityPackage(t, root, name, "2.0.0")
	}
	for _, installation := range f.journey.state(t, "").Installations {
		for _, receipt := range installation.DataReceipts {
			path := filepath.Join(receipt.Locator, "keep.txt")
			projectJourneyWrite(t, path, "retained update data\n")
			f.data = append(f.data, path)
		}
	}
	if len(f.data) < 2 {
		t.Fatal("imperative fixture did not allocate persistent data")
	}
	if mixed {
		source.advance()
	}
	f.security = &updateAllSecurityEvaluator{}
	f.journey.app = configureUpdateAllSecurity(f.journey.app, f.security, &f.calls)
	return f
}

// Snapshot complete persistent UAP and native profile roots. The source
// acquisition fixture uses home as its disposable temp root; those temporary
// acquisitions are outside both authoritative trees. Keep the whole UAP tree,
// including store, journals, state/receipts, agreements, managed packages and data.
func (f *updateAllSecurityJourney) snapshot(t *testing.T) map[string]map[string]string {
	t.Helper()
	return map[string]map[string]string{
		"persistent-uap": securityFixtureTree(t, f.journey.cache),
		"native-profile": securityFixtureTree(t, f.journey.nativeRoot("")),
	}
}

func writeUpdateAllSecurityPackage(t *testing.T, root, name, version string) {
	t.Helper()
	projectJourneyWrite(t, filepath.Join(root, "plugin.json"), fmt.Sprintf(
		`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":%q,"version":%q}`, name, version))
	projectJourneyWrite(t, filepath.Join(root, "skills", name, "SKILL.md"), fmt.Sprintf(
		"---\nname: %s\ndescription: Inert update fixture\n---\nIndependent %s payload %s.\n", name, name, version))
	projectJourneyWrite(t, filepath.Join(root, "mcp.json"), fmt.Sprintf(
		`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{%q:{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}","REVISION":%q}}}}`, name, version))
}

func (f *updateAllSecurityJourney) execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var output, stderr bytes.Buffer
	app := f.journey.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &output, &stderr, false
	command := agentpluginscli.NewRoot(app)
	command.SetArgs(append(args, "--format", "json"))
	err := command.ExecuteContext(t.Context())
	if stderr.Len() != 0 {
		t.Logf("CLI stderr: %s", stderr.String())
	}
	return output.String(), err
}

type updateAllSecurityJSON struct {
	Result string `json:"result"`
	Data   struct {
		Status        string `json:"status"`
		Planned       int    `json:"planned"`
		Updated       int    `json:"updated"`
		Failed        int    `json:"failed"`
		Installations []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Reason string `json:"reason"`
			Saved  bool   `json:"saved"`
			Plan   *struct {
				Targets []struct {
					Output struct {
						Security *domain.SecurityAssessment `json:"security"`
					} `json:"output"`
				} `json:"targets"`
			} `json:"plan"`
		} `json:"installations"`
	} `json:"data"`
}

func decodeUpdateAllSecurityJSON(t *testing.T, output string) updateAllSecurityJSON {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(output))
	var result updateAllSecurityJSON
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("decode update response: %v\n%s", err, output)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("update must emit exactly one JSON value, second decode=%v\n%s", err, output)
	}
	return result
}

func TestComposedUpdateAllSecurityPreflight(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		name := map[bool]string{false: "imperative", true: "mixed-saved"}[mixed]
		t.Run(name, func(t *testing.T) {
			runComposedUpdateAllSecurityPreflight(t, mixed)
		})
	}
}

func runComposedUpdateAllSecurityPreflight(t *testing.T, mixed bool) {
	t.Helper()
	t.Run("refusal-and-preview", func(t *testing.T) {
		f := newUpdateAllSecurityJourney(t, mixed)
		before := f.snapshot(t)
		preview, err := f.execute(t, "update", "--all", "--dry-run")
		result := decodeUpdateAllSecurityJSON(t, preview)
		want := 2
		if mixed {
			want++
		}
		if err != nil || result.Result != "success" || result.Data.Status != "planned" || result.Data.Planned != want {
			t.Fatalf("changed batch preview: %v\n%s", err, preview)
		}
		if !reflect.DeepEqual(before, f.snapshot(t)) || f.calls != 0 {
			t.Fatal("dry-run changed installed, saved, native or data files, or activated a client")
		}
		// Establish parity with targeted update's existing security refusal.
		targeted, err := f.execute(t, "update", "beta", "--target", "gemini", "--yes")
		if err == nil || !strings.Contains(err.Error(), "--accept-security-risk") {
			t.Fatalf("targeted update bypassed risk: %v\n%s", err, targeted)
		}
		decodeUpdateAllSecurityJSON(t, targeted)
		if !reflect.DeepEqual(before, f.snapshot(t)) || f.calls != 0 {
			t.Fatal("targeted refusal changed installed, saved, native or data files, or activated a client")
		}
		output, err := f.execute(t, "update", "--all", "--yes")
		result = decodeUpdateAllSecurityJSON(t, output)
		if err == nil || result.Result != "failure" || result.Data.Status != "preflight_failed" || result.Data.Updated != 0 || result.Data.Failed == 0 {
			t.Errorf("unauthorized batch was not refused in preflight: %v\n%s", err, output)
		}
		blocked, savedPlanned, assessmentReported := false, false, false
		for _, row := range result.Data.Installations {
			blocked = blocked || (row.Name == "beta" && row.Status == "preflight_failed" && strings.Contains(row.Reason, "--accept-security-risk"))
			savedPlanned = savedPlanned || (row.Saved && row.Status == "planned")
			if row.Name == "beta" && row.Plan != nil {
				for _, target := range row.Plan.Targets {
					assessmentReported = assessmentReported || (target.Output.Security != nil &&
						target.Output.Security.Counts.Blocking == 1 && len(target.Output.Security.Findings) == 1)
				}
			}
		}
		if !blocked || !assessmentReported || (mixed && !savedPlanned) || f.security.blockedCalls == 0 {
			t.Errorf("missing blocked imperative or genuinely pending saved preparation: %s", output)
		}
		if !reflect.DeepEqual(before, f.snapshot(t)) || f.calls != 0 {
			t.Error("refused batch changed state, manifest, lock, managed/native bytes, data or receipts, or activated a client")
		}
	})
	t.Run("explicit-risk-acceptance", func(t *testing.T) {
		f := newUpdateAllSecurityJourney(t, mixed)
		output, err := f.execute(t, "update", "--all", "--yes", "--accept-security-risk")
		result := decodeUpdateAllSecurityJSON(t, output)
		want := 2
		if mixed {
			want++
		}
		if err != nil || result.Result != "success" || result.Data.Status != "completed" || result.Data.Updated != want || f.calls == 0 || f.security.blockedCalls == 0 {
			t.Fatalf("accepted batch did not apply: %v\n%s", err, output)
		}
		f.assertUpdatedAndRemove(t)
	})
}

func (f *updateAllSecurityJourney) assertUpdatedAndRemove(t *testing.T) {
	t.Helper()
	state := f.journey.state(t, "")
	expectedNames := map[string]bool{"alpha": false, "beta": false}
	if f.mixed {
		expectedNames["docs"] = false
	}
	if len(state.Installations) != len(expectedNames) {
		t.Fatalf("updated batch installation count=%d, want %d", len(state.Installations), len(expectedNames))
	}
	for _, installation := range state.Installations {
		name := installation.DeclaredName
		if f.mixed && installation.InstallationID == f.savedID {
			name = "docs"
		}
		if seen, expected := expectedNames[name]; !expected || seen {
			t.Fatalf("unexpected or duplicate updated installation: %+v", installation)
		}
		expectedNames[name] = true
		expected := ""
		if name == "alpha" || name == "beta" {
			if installation.Package.Version != "2.0.0" {
				t.Fatal(installation)
			}
			expected = fmt.Sprintf("---\nname: %s\ndescription: Inert update fixture\n---\nIndependent %s payload 2.0.0.\n", name, name)
		} else if name == "docs" && f.mixed {
			expected = projectJourneyAdvancedSkill
		}
		if expected == "" {
			t.Fatalf("unexpected package: %+v", installation)
		}
		for _, binding := range installation.Clients {
			for _, root := range []string{binding.TargetLocator, f.journey.nativeRoot("")} {
				if projectJourneyRead(t, filepath.Join(root, "skills", name, "SKILL.md")) != expected {
					t.Fatal("accepted update did not install independently expected payload")
				}
			}
		}
	}
	for name, present := range expectedNames {
		if !present {
			t.Fatalf("updated batch omitted %s", name)
		}
	}
	if f.mixed {
		savedReceipt := false
		for _, receipt := range state.EnvironmentReceipts {
			if receipt.InstallationID == f.savedID && receipt.Alias == "docs" {
				savedReceipt = true
			}
		}
		if !savedReceipt {
			t.Fatal("updated saved Skill lost its original installation/alias authority")
		}
	}
	if f.mixed && f.oldLock == projectJourneyRead(t, filepath.Join(f.journey.cache, "uap.lock")) {
		t.Fatal("saved fixture was a no-op: lock did not advance")
	}
	for _, path := range f.data {
		if projectJourneyRead(t, path) != "retained update data\n" {
			t.Fatal("update replaced data")
		}
	}
	for _, name := range []string{"alpha", "beta"} {
		f.journey.run(t, "", "remove", name, "--target", "gemini")
	}
	if f.mixed {
		f.journey.run(t, "", "remove", "docs", "--target", "gemini")
	}
	removed := f.journey.state(t, "")
	if len(removed.EnvironmentReceipts) != 0 {
		t.Fatal("cleanup retained saved ownership")
	}
	for _, installation := range removed.Installations {
		if len(installation.Clients) != 0 {
			t.Fatal("cleanup retained an installed binding")
		}
	}
	if projectJourneyRead(t, filepath.Join(f.journey.nativeRoot(""), "skills", "foreign", "SKILL.md")) != "foreign skill\n" ||
		!strings.Contains(projectJourneyRead(t, filepath.Join(f.journey.nativeRoot(""), "settings.json")), "foreign.test") {
		t.Fatal("cleanup changed foreign native files")
	}
	for _, path := range f.data {
		if projectJourneyRead(t, path) != "retained update data\n" {
			t.Fatal("cleanup consumed retained data")
		}
	}
}
