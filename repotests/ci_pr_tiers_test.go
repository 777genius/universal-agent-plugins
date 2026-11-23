package pluginkitairepo_test

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// heavyPRLaneRealRun is true for the pull request events that run the lane
// for the current head: pushes, open, reopen, ready_for_review, and adding
// ci:full to a draft. Every other label event, including ci:full on a ready
// pull request, is not one of them.
const heavyPRLaneRealRun = "github.event_name == 'pull_request' && " +
	"(github.event.action != 'labeled' || (github.event.label.name == 'ci:full' && github.event.pull_request.draft == true))"

// heavyPRLaneGate is the one Tier 1 rule from docs/CI_RUNNERS.md: a draft pull
// request skips the lane unless it has the ci:full label, a label event never
// starts it except ci:full on a draft, and ready pull requests, pushes,
// schedules, workflow runs and manual dispatches keep it.
const heavyPRLaneGate = "github.event_name != 'pull_request' || " +
	"(github.event.action != 'labeled' || (github.event.label.name == 'ci:full' && github.event.pull_request.draft == true)) && " +
	"(github.event.pull_request.draft == false || contains(github.event.pull_request.labels.*.name, 'ci:full'))"

// heavyPRLaneConditions are the only job conditions that carry the gate. They
// are matched exactly: a condition that merely contains the gate, such as a
// negation or an alternative with ||, could still start a job on a draft.
var heavyPRLaneConditions = []string{
	"${{ " + heavyPRLaneGate + " }}",
	// An aggregate that reads always() runs after skipped needs, so it needs
	// the gate itself.
	"${{ always() && (" + heavyPRLaneGate + ") }}",
	// Native audits retain classifier-failure fallback while still refusing
	// cancellation and carrying the complete tier gate themselves.
	heavyPRNativePilot,
	heavyPRRemainingSelection,
	// Published audits also wait for a successful npm publish run.
	"${{ (github.event_name != 'workflow_run' || github.event.workflow_run.conclusion == 'success') && (" + heavyPRLaneGate + ") }}",
}

// The evaluator's synthetic context key pins GitHub's exact cancellation name.
// Reuse that name for function calls and job results; local state uses Go spelling.
const tierCanceledContext = "status.cancelled"

var tierCanceledName = strings.TrimPrefix(tierCanceledContext, "status.")
var tierNotCanceled = "!" + tierCanceledName + "()"

var heavyPRNativePilot = "${{ " + tierNotCanceled + " && (" + heavyPRLaneGate + ") }}"
var heavyPRRemainingSelection = "${{ " + tierNotCanceled + " && " +
	"(needs.changes.result != 'success' || needs.changes.outputs.remaining_clients != '[]') && (" + heavyPRLaneGate + ") }}"

// This status-bearing matrix has no direct tier gate. Only this exact
// conjunction may inherit one through its mandatory successful native pilot.
var heavyPRNativeMatrix = "${{ " + tierNotCanceled + " && needs.native-pilot.result == 'success' && " +
	"(needs.changes.result != 'success' || needs.changes.outputs.live_clients != '[]') }}"

// A newer real run supersedes the run of the same pull request; label events
// that do not start the lane and all other events never cancel a running one.
const heavyPRLaneCancel = "${{ " + heavyPRLaneRealRun + " }}"

// labeled is allowed only together with the pinned gate, cancel rule and
// groups above, which keep every label event except ci:full on a draft away
// from the lane. Without edited or unlabeled, a title change or removing
// ci:full cannot cancel and restart a running heavy lane.
var heavyPRLaneTypes = []string{"labeled", "opened", "ready_for_review", "reopened", "synchronize"}

// Real runs share one group per pull request. Other pull request events get a
// group per run, so they can neither cancel nor replace a pending real run.
// Pushes and other events keep the key each workflow had before.
func heavyPRLaneGroup(prefix, otherEvents string) string {
	if otherEvents == "github.run_id" {
		return prefix + "${{ " + heavyPRLaneRealRun + " && github.event.pull_request.number || github.run_id }}"
	}
	return prefix + "${{ " + heavyPRLaneRealRun + " && github.event.pull_request.number || github.event_name == 'pull_request' && github.run_id || " + otherEvents + " }}"
}

type heavyPRLane struct {
	// triggers is the exact set of events. pull_request_target, issue_comment
	// and similar events would pass the gate, because it only checks for
	// pull_request.
	triggers []string
	// prefix and otherEvents build the exact concurrency group with
	// heavyPRLaneGroup: real runs share prefix+number, other pull request
	// events get prefix+run_id, and every other event keeps prefix+otherEvents.
	prefix, otherEvents string
}

// heavyPRLaneWorkflows run native clients or wide GitHub-hosted matrices on
// pull requests. Published npm audits (now postpublish/weekly/manual only)
// are asserted by scripts/test_package_manager_ci_policy.py, not this PR-tier map.
var heavyPRLaneWorkflows = map[string]heavyPRLane{
	"cline-managed-session.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"cline-managed-session-${{ github.event_name }}-", "github.run_id",
	},
	"hermes-noauth-tool-call.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"hermes-noauth-call-", "github.ref",
	},
	"kimi252-noauth-call.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"kimi252-noauth-call-${{ github.event_name }}-", "github.ref",
	},
	"kiro-no-auth-installation.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"kiro-no-auth-", "github.ref",
	},
	"live-install-audit.yml": {
		[]string{"pull_request", "workflow_dispatch"},
		"live-install-audit-${{ github.event_name }}-", "github.run_id",
	},
	"live-install-remaining-clients.yml": {
		[]string{"pull_request", "workflow_dispatch"},
		"remaining-client-audit-", "github.ref",
	},
	"marketing-kit-project-release-e2e.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"published-marketing-project-", "github.ref",
	},
	"project-native-runtime.yml": {
		[]string{"pull_request", "workflow_dispatch"},
		"project-native-runtime-${{ github.event_name }}-", "github.run_id",
	},
}

type tierWorkflow struct {
	On          map[string]yaml.Node `yaml:"on"`
	Concurrency struct {
		Group  string `yaml:"group"`
		Cancel string `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Jobs map[string]tierJob `yaml:"jobs"`
}

type tierJob struct {
	If    string    `yaml:"if"`
	Needs yaml.Node `yaml:"needs"`
}

func TestCITiers_HeavyPRLanesSkipDraftsAndRunWhenReady(t *testing.T) {
	for name, lane := range heavyPRLaneWorkflows {
		t.Run(name, func(t *testing.T) {
			var w tierWorkflow
			if err := yaml.Unmarshal([]byte(readRepoFile(t, RepoRoot(t), ".github", "workflows", name)), &w); err != nil {
				t.Fatal(err)
			}
			var triggers []string
			for event := range w.On {
				triggers = append(triggers, event)
			}
			slices.Sort(triggers)
			// workflow_dispatch keeps every lane available on demand.
			if !slices.Equal(triggers, lane.triggers) {
				t.Fatalf("triggers = %v, want exactly %v", triggers, lane.triggers)
			}
			var trigger struct {
				Types []string `yaml:"types"`
			}
			pr := w.On["pull_request"]
			if err := pr.Decode(&trigger); err != nil {
				t.Fatal(err)
			}
			// ready_for_review starts the full run on the head that becomes
			// mergeable.
			types := slices.Clone(trigger.Types)
			slices.Sort(types)
			if !slices.Equal(types, heavyPRLaneTypes) {
				t.Fatalf("pull_request types = %v, want exactly %v", trigger.Types, heavyPRLaneTypes)
			}
			group := heavyPRLaneGroup(lane.prefix, lane.otherEvents)
			if w.Concurrency.Group != group || w.Concurrency.Cancel != heavyPRLaneCancel {
				t.Fatalf("concurrency = %+v, want group %q and cancel-in-progress %q", w.Concurrency, group, heavyPRLaneCancel)
			}

			gated := heavyPRGatedJobs(t, w)
			for job := range w.Jobs {
				if !gated[job] {
					t.Errorf("job %s can run on a draft pull request; use one of the conditions %q", job, heavyPRLaneConditions)
				}
			}
		})
	}
}

// Validate the whole dependency graph before recognizing any direct gate:
// an exact gate must not hide a missing dependency or a cycle behind it.
func validTierNeeds(needs map[string][]string) bool {
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(job string) bool {
		parents, exists := needs[job]
		if !exists || visiting[job] {
			return false
		}
		if done[job] {
			return true
		}
		visiting[job] = true
		for _, parent := range parents {
			if !visit(parent) {
				return false
			}
		}
		delete(visiting, job)
		done[job] = true
		return true
	}
	for job := range needs {
		if !visit(job) {
			return false
		}
	}
	return true
}

func heavyPRGatedJobs(t *testing.T, w tierWorkflow) map[string]bool {
	t.Helper()
	needs, result := map[string][]string{}, map[string]bool{}
	for job, spec := range w.Jobs {
		needs[job] = decodeNeeds(t, spec.Needs)
	}
	if !validTierNeeds(needs) {
		return result
	}
	var gated func(string) bool
	gated = func(job string) bool {
		condition := strings.TrimSpace(w.Jobs[job].If)
		if slices.Contains(heavyPRLaneConditions, condition) {
			return true
		}
		if condition == heavyPRNativeMatrix {
			return slices.Contains(needs[job], "native-pilot") &&
				slices.Contains(needs[job], "changes") && gated("native-pilot")
		}
		// Without a status function, GitHub implicitly requires success() of
		// all needs. One gated required parent is sufficient to prevent a
		// draft run. Status-bearing jobs cannot inherit through this rule.
		if readsStatusFunction(condition) {
			return false
		}
		for _, parent := range needs[job] {
			if gated(parent) {
				return true
			}
		}
		return false
	}
	for job := range w.Jobs {
		result[job] = gated(job)
	}
	return result
}

// Evaluate only the job-level conditions used here. Without a status function,
// GitHub supplies success(): cancellation or a non-successful required job skips
// the job. An explicit status function replaces that implicit prerequisite.
func evalTierJob(t *testing.T, w tierWorkflow, job string, ctx map[string]any) bool {
	t.Helper()
	spec, exists := w.Jobs[job]
	if !exists {
		t.Fatalf("unknown job %q", job)
	}
	if !readsStatusFunction(spec.If) {
		canceled, ok := ctx[tierCanceledContext].(bool)
		if !ok {
			t.Fatal("implicit success() requires explicit boolean workflow cancellation status")
		}
		if canceled {
			return false
		}
		for _, parent := range decodeNeeds(t, spec.Needs) {
			result, ok := ctx["needs."+parent+".result"]
			if !ok {
				t.Fatalf("unknown needs result for %q", parent)
			}
			if result != "success" {
				return false
			}
		}
	}
	return strings.TrimSpace(spec.If) == "" || truthy(evalTemplate(t, spec.If, ctx))
}

func decodeNeeds(t *testing.T, node yaml.Node) []string {
	t.Helper()
	switch node.Kind {
	case 0:
		return nil
	case yaml.ScalarNode:
		return []string{node.Value}
	default:
		var list []string
		if err := node.Decode(&list); err != nil {
			t.Fatal(err)
		}
		return list
	}
}

func readsStatusFunction(condition string) bool {
	for _, function := range []string{"always()", tierCanceledName + "()", "failure()", "success()"} {
		if strings.Contains(condition, function) {
			return true
		}
	}
	return false
}

// TestCITiers_LabelEventsBehavior evaluates every gated job condition, the
// cancel rule and the group of each Tier 1 workflow, as written in the
// workflow files, for the events that matter. Events other than pull_request
// are checked only for workflows that list them.
func TestCITiers_LabelEventsBehavior(t *testing.T) {
	type event struct {
		name, action, label, conclusion string
		draft                           bool
		labels                          []string
	}
	const (
		prGroup    = "the pull request group"
		ownGroup   = "a group of its own"
		nonPRGroup = "the group the workflow had before"
	)
	cases := []struct {
		about       string
		event       event
		run, cancel bool
		group       string
	}{
		{"draft opened", event{"pull_request", "opened", "", "", true, nil}, false, true, prGroup},
		{"draft opened with ci:full", event{"pull_request", "opened", "", "", true, []string{"ci:full"}}, true, true, prGroup},
		{"ready pull request opened", event{"pull_request", "opened", "", "", false, nil}, true, true, prGroup},
		{"ready pull request reopened", event{"pull_request", "reopened", "", "", false, nil}, true, true, prGroup},
		{"draft reopened", event{"pull_request", "reopened", "", "", true, nil}, false, true, prGroup},
		{"push to a draft", event{"pull_request", "synchronize", "", "", true, nil}, false, true, prGroup},
		{"ci:full added to a draft", event{"pull_request", "labeled", "ci:full", "", true, []string{"ci:full"}}, true, true, prGroup},
		{"push to a ci:full draft", event{"pull_request", "synchronize", "", "", true, []string{"ci:full"}}, true, true, prGroup},
		{"other label on a ci:full draft", event{"pull_request", "labeled", "docs", "", true, []string{"ci:full", "docs"}}, false, false, ownGroup},
		{"push to a draft after ci:full was removed", event{"pull_request", "synchronize", "", "", true, []string{"docs"}}, false, true, prGroup},
		{"ci:full added to a ready pull request", event{"pull_request", "labeled", "ci:full", "", false, []string{"ci:full"}}, false, false, ownGroup},
		{"other label on a ready pull request", event{"pull_request", "labeled", "docs", "", false, []string{"docs"}}, false, false, ownGroup},
		{"marked ready", event{"pull_request", "ready_for_review", "", "", false, nil}, true, true, prGroup},
		{"marked ready with ci:full", event{"pull_request", "ready_for_review", "", "", false, []string{"ci:full"}}, true, true, prGroup},
		{"push to a ready pull request", event{"pull_request", "synchronize", "", "", false, nil}, true, true, prGroup},
		{"push to main", event{name: "push"}, true, false, nonPRGroup},
		{"manual dispatch", event{name: "workflow_dispatch"}, true, false, nonPRGroup},
		{"schedule", event{name: "schedule"}, true, false, nonPRGroup},
		{"npm publish succeeded", event{name: "workflow_run", conclusion: "success"}, true, false, nonPRGroup},
		{"npm publish failed", event{name: "workflow_run", conclusion: "failure"}, false, false, nonPRGroup},
		{"npm publish timed out", event{name: "workflow_run", conclusion: "timed_out"}, false, false, nonPRGroup},
	}
	const runID, number, mainRef = 7001, 42, "refs/heads/main"
	context := func(e event) map[string]any {
		ctx := map[string]any{
			tierCanceledContext:                       false,
			"needs.changes.result":                    "success",
			"needs.changes.outputs.remaining_clients": `["cursor"]`,
			"github.event_name":                       e.name,
			"github.event.action":                     nil,
			"github.event.label.name":                 nil,
			"github.event.pull_request.draft":         nil,
			"github.event.pull_request.number":        nil,
			"github.event.pull_request.labels.*.name": []string(nil),
			"github.event.workflow_run.conclusion":    nil,
			"github.run_id":                           runID,
			"github.ref":                              mainRef,
		}
		switch e.name {
		case "pull_request":
			ctx["github.event.action"] = e.action
			ctx["github.event.pull_request.draft"] = e.draft
			ctx["github.event.pull_request.number"] = number
			ctx["github.event.pull_request.labels.*.name"] = e.labels
			ctx["github.ref"] = fmt.Sprintf("refs/pull/%d/merge", number)
			if e.action == "labeled" {
				ctx["github.event.label.name"] = e.label
			}
		case "workflow_run":
			ctx["github.event.workflow_run.conclusion"] = e.conclusion
		}
		return ctx
	}
	for name, lane := range heavyPRLaneWorkflows {
		var w tierWorkflow
		if err := yaml.Unmarshal([]byte(readRepoFile(t, RepoRoot(t), ".github", "workflows", name)), &w); err != nil {
			t.Fatal(err)
		}
		var conditions []string
		for _, job := range w.Jobs {
			if strings.Contains(job.If, "github.event.pull_request.draft") {
				conditions = append(conditions, job.If)
			}
		}
		if len(conditions) == 0 {
			t.Fatalf("%s: no gated job", name)
		}
		for _, tc := range cases {
			if !slices.Contains(lane.triggers, tc.event.name) {
				continue
			}
			t.Run(name+"/"+tc.about, func(t *testing.T) {
				ctx := context(tc.event)
				for _, condition := range conditions {
					if got := truthy(evalTemplate(t, condition, ctx)); got != tc.run {
						t.Errorf("job condition %q = %v, want %v", condition, got, tc.run)
					}
				}
				if got := truthy(evalTemplate(t, w.Concurrency.Cancel, ctx)); got != tc.cancel {
					t.Errorf("cancel-in-progress = %v, want %v", got, tc.cancel)
				}
				prefix := renderTemplate(t, lane.prefix, ctx)
				want := map[string]string{
					prGroup:  prefix + strconv.Itoa(number),
					ownGroup: prefix + strconv.Itoa(runID),
				}[tc.group]
				if tc.group == nonPRGroup {
					want = prefix + mainRef
					if lane.otherEvents == "github.run_id" {
						want = prefix + strconv.Itoa(runID)
					}
				}
				if got := fmt.Sprint(evalTemplate(t, w.Concurrency.Group, ctx)); got != want {
					t.Errorf("group %q, want %s %q", got, tc.group, want)
				}
			})
		}
	}
}

func readTierWorkflow(t *testing.T, name string) tierWorkflow {
	t.Helper()
	var w tierWorkflow
	if err := yaml.Unmarshal([]byte(readRepoFile(t, RepoRoot(t), ".github", "workflows", name)), &w); err != nil {
		t.Fatal(err)
	}
	return w
}

// Draft-safety inheritance alone does not prove classifier-failure fallback:
// removing a status function restores implicit success() and can lose fallback
// while remaining draft-safe. Pin these three deployed native conditions too.
func exactTierConditions(w tierWorkflow, expected map[string]string) bool {
	for job, condition := range expected {
		spec, exists := w.Jobs[job]
		if !exists || strings.TrimSpace(spec.If) != condition {
			return false
		}
	}
	return true
}

var nativeAuditConditions = map[string]string{
	"native-pilot": heavyPRNativePilot, "real-native-cli-matrix": heavyPRNativeMatrix,
}

func TestCITiers_NativeClassifierAndCancellationConditions(t *testing.T) {
	live := readTierWorkflow(t, "live-install-audit.yml")
	remaining := readTierWorkflow(t, "live-install-remaining-clients.yml")
	if !exactTierConditions(live, nativeAuditConditions) ||
		!exactTierConditions(remaining, map[string]string{"remaining-client-install": heavyPRRemainingSelection}) {
		t.Fatal("native audit condition differs from exact cancellation, pilot and selection contract")
	}
	cases := []struct {
		name, classifier, output, pilot string
		canceled, selection, smoke      bool
	}{
		{"selected clients", "success", `["claude"]`, "success", false, true, true},
		{"successful empty selection", "success", "[]", "success", false, false, true},
		{"failed classifier missing output", "failure", "", "success", false, true, true},
		{"failed classifier partial output", "failure", `["claude"]`, "success", false, true, true},
		{"failed classifier empty output", "failure", "[]", "success", false, true, true},
		{"skipped classifier", "skipped", "", "success", false, true, true},
		{"pilot failed", "success", `["claude"]`, "failure", false, true, false},
		{"pilot skipped", "success", `["claude"]`, "skipped", false, true, false},
		{"pilot canceled", "success", `["claude"]`, tierCanceledName, false, true, false},
		{"workflow canceled after successful pilot", "success", `["claude"]`, "success", true, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := map[string]any{
				tierCanceledContext: tc.canceled, "github.event_name": "pull_request",
				"github.event.action": "synchronize", "github.event.label.name": nil,
				"github.event.pull_request.draft": false, "github.event.pull_request.labels.*.name": []string(nil),
				"needs.changes.result": tc.classifier, "needs.native-pilot.result": tc.pilot,
				"needs.changes.outputs.live_clients": tc.output, "needs.changes.outputs.remaining_clients": tc.output,
			}
			want := map[string]bool{
				// A job's own future result cannot alter its admission.
				"changes":                !tc.canceled,
				"native-pilot":           !tc.canceled,
				"published-cli-smoke":    tc.smoke,
				"real-native-cli-matrix": tc.selection && tc.pilot == "success",
			}
			for job, expected := range want {
				if got := evalTierJob(t, live, job, ctx); got != expected {
					t.Errorf("live %s admitted=%v, want %v", job, got, expected)
				}
			}
			if got := evalTierJob(t, remaining, "remaining-client-install", ctx); got != tc.selection {
				t.Errorf("remaining admitted=%v, want %v", got, tc.selection)
			}
		})
	}
}

func TestCITiers_NativeGateMutationRefusals(t *testing.T) {
	for _, mutation := range []string{"pilot-no-tier-gate", "pilot-or-tier-gate", "pilot-no-cancellation", "matrix-no-cancellation", "matrix-no-success",
		"matrix-failure-result", "matrix-or-success", "matrix-no-selection", "matrix-nondependency-pilot", "matrix-no-classifier-dependency",
		"matrix-indirect-pilot", "matrix-or-alternative", "missing-pilot", "cycle-behind-direct-gate",
		"missing-dependency-behind-direct-gate"} {
		t.Run(mutation, func(t *testing.T) {
			w := readTierWorkflow(t, "live-install-audit.yml")
			pilot, matrix, changes := w.Jobs["native-pilot"], w.Jobs["real-native-cli-matrix"], w.Jobs["changes"]
			switch mutation {
			case "pilot-no-tier-gate":
				pilot.If = "${{ " + tierNotCanceled + " }}"
			case "pilot-or-tier-gate":
				pilot.If = "${{ " + tierNotCanceled + " || (" + heavyPRLaneGate + ") }}"
			case "pilot-no-cancellation":
				pilot.If = "${{ " + heavyPRLaneGate + " }}"
			case "matrix-no-cancellation":
				matrix.If = strings.Replace(matrix.If, tierNotCanceled+" && ", "", 1)
			case "matrix-no-success":
				matrix.If = strings.Replace(matrix.If, "needs.native-pilot.result == 'success' && ", "", 1)
			case "matrix-failure-result":
				matrix.If = strings.Replace(matrix.If, "needs.native-pilot.result == 'success'", "needs.native-pilot.result == 'failure'", 1)
			case "matrix-or-success":
				matrix.If = "${{ " + tierNotCanceled + " && (needs.native-pilot.result == 'success' || needs.changes.outputs.live_clients != '[]') }}"
			case "matrix-no-selection":
				matrix.If = "${{ " + tierNotCanceled + " && needs.native-pilot.result == 'success' }}"
			case "matrix-nondependency-pilot":
				matrix.Needs = yaml.Node{Kind: yaml.ScalarNode, Value: "changes"}
			case "matrix-no-classifier-dependency":
				matrix.Needs = yaml.Node{Kind: yaml.ScalarNode, Value: "native-pilot"}
			case "matrix-indirect-pilot":
				matrix.Needs = yaml.Node{Kind: yaml.ScalarNode, Value: "published-cli-smoke"}
			case "matrix-or-alternative":
				matrix.If = strings.TrimSuffix(matrix.If, " }}") + " || true }}"
			case "missing-pilot":
				delete(w.Jobs, "native-pilot")
			case "cycle-behind-direct-gate":
				changes.Needs = yaml.Node{Kind: yaml.ScalarNode, Value: "native-pilot"}
			case "missing-dependency-behind-direct-gate":
				changes.Needs = yaml.Node{Kind: yaml.ScalarNode, Value: "missing"}
			}
			if mutation != "missing-pilot" {
				w.Jobs["native-pilot"] = pilot
			}
			w.Jobs["real-native-cli-matrix"], w.Jobs["changes"] = matrix, changes
			gated := heavyPRGatedJobs(t, w)["real-native-cli-matrix"]
			// These status-bearing bypasses must fail graph proof independently
			// of the stricter exact-condition check. No-cancellation variants
			// can remain draft-safe through restored implicit success().
			switch mutation {
			case "pilot-no-tier-gate", "pilot-or-tier-gate", "matrix-no-success", "matrix-or-success",
				"matrix-nondependency-pilot", "matrix-indirect-pilot", "matrix-no-classifier-dependency",
				"matrix-or-alternative", "missing-pilot", "cycle-behind-direct-gate", "missing-dependency-behind-direct-gate":
				if gated {
					t.Fatal("unsafe status or dependency mutation retained graph authority")
				}
			}
			if exactTierConditions(w, nativeAuditConditions) && gated {
				t.Fatal("mutated matrix retained exact native condition and tier-gate authority")
			}
		})
	}
	// A selected-client condition cannot weaken the remaining lane's direct
	// tier conjunction into an alternative or omit cancellation.
	for _, condition := range []string{
		strings.Replace(heavyPRRemainingSelection, tierNotCanceled+" && ", "", 1),
		"${{ " + tierNotCanceled + " && (needs.changes.result != 'success' || needs.changes.outputs.remaining_clients != '[]') || (" + heavyPRLaneGate + ") }}",
		"${{ (needs.changes.result != 'success' || needs.changes.outputs.remaining_clients != '[]') && (" + heavyPRLaneGate + ") || true }}",
	} {
		w := readTierWorkflow(t, "live-install-remaining-clients.yml")
		job := w.Jobs["remaining-client-install"]
		job.If = condition
		w.Jobs["remaining-client-install"] = job
		if exactTierConditions(w, map[string]string{"remaining-client-install": heavyPRRemainingSelection}) &&
			heavyPRGatedJobs(t, w)["remaining-client-install"] {
			t.Fatal("mutated remaining lane retained exact native condition and tier-gate authority")
		}
	}
}

// renderTemplate replaces every ${{ }} segment of a string with its value.
func renderTemplate(t *testing.T, template string, ctx map[string]any) string {
	t.Helper()
	var out strings.Builder
	for {
		start := strings.Index(template, "${{")
		if start < 0 {
			return out.String() + template
		}
		end := strings.Index(template[start:], "}}")
		out.WriteString(template[:start])
		fmt.Fprint(&out, evalExpression(t, template[start+3:start+end], ctx))
		template = template[start+end+2:]
	}
}

// evalTemplate evaluates a job condition, cancel rule or group: a single
// ${{ }} segment or a bare expression returns the expression value, any other
// string is rendered.
func evalTemplate(t *testing.T, template string, ctx map[string]any) any {
	t.Helper()
	template = strings.TrimSpace(template)
	if strings.HasPrefix(template, "${{") && strings.HasSuffix(template, "}}") && strings.Count(template, "${{") == 1 {
		return evalExpression(t, template[3:len(template)-2], ctx)
	}
	if !strings.Contains(template, "${{") {
		return evalExpression(t, template, ctx)
	}
	return renderTemplate(t, template, ctx)
}

// truthy follows GitHub: false, 0, NaN, "" and null are falsy.
func truthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case int:
		return v != 0
	case float64:
		return v != 0 && !math.IsNaN(v)
	case string:
		return v != ""
	default:
		return true
	}
}

// looseEqual follows GitHub's expression equality: two strings compare
// case-insensitively; other operands are coerced to numbers (null and false
// to 0, true to 1, a numeric string to its value, any other string to NaN,
// which equals nothing).
func looseEqual(left, right any) bool {
	leftString, leftIsString := left.(string)
	rightString, rightIsString := right.(string)
	if leftIsString && rightIsString {
		return strings.EqualFold(leftString, rightString)
	}
	leftNumber, rightNumber := expressionNumber(left), expressionNumber(right)
	return !math.IsNaN(leftNumber) && leftNumber == rightNumber
}

func expressionNumber(value any) float64 {
	switch v := value.(type) {
	case nil:
		return 0
	case bool:
		if v {
			return 1
		}
		return 0
	case int:
		return float64(v)
	case float64:
		return v
	case string:
		if strings.TrimSpace(v) == "" {
			return 0
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return math.NaN()
		}
		return number
	default:
		return math.NaN()
	}
}

// expressionContains follows GitHub's contains(): an array matches an equal
// item, a string matches a case-insensitive substring.
func expressionContains(search, item any) bool {
	switch v := search.(type) {
	case []string:
		for _, element := range v {
			if looseEqual(element, item) {
				return true
			}
		}
		return false
	case string:
		return strings.Contains(strings.ToLower(v), strings.ToLower(fmt.Sprint(item)))
	default:
		return false
	}
}

// evalExpression implements the GitHub expression subset these rules use:
// ||, &&, ==, !=, parentheses, string and boolean literals, context values,
// unary !, always(), the cancellation function and contains(), with GitHub's
// loose equality and truthiness. Cancellation requires explicit boolean status.
// Other functions, comparisons such as < and object filters fail the test.
func evalExpression(t *testing.T, source string, ctx map[string]any) any {
	t.Helper()
	tokens := regexp.MustCompile(`\|\||&&|==|!=|!|\(|\)|,|'[^']*'|[A-Za-z_][A-Za-z0-9_.*-]*`).FindAllString(source, -1)
	if strings.Join(tokens, "") != strings.Join(strings.Fields(source), "") {
		t.Fatalf("unsupported expression %q", source)
	}
	position := 0
	peek := func() string {
		if position < len(tokens) {
			return tokens[position]
		}
		return ""
	}
	next := func() string { token := peek(); position++; return token }
	expect := func(token string) {
		if next() != token {
			t.Fatalf("expected %q in %q", token, source)
		}
	}
	var or func() any
	primary := func() any {
		token := next()
		switch {
		case token == "(":
			value := or()
			expect(")")
			return value
		case token == "always":
			expect("(")
			expect(")")
			return true
		case token == tierCanceledName:
			expect("(")
			expect(")")
			value, ok := ctx[tierCanceledContext].(bool)
			if !ok {
				t.Fatalf("%s() requires explicit boolean workflow status", tierCanceledName)
			}
			return value
		case token == "contains":
			expect("(")
			haystack := or()
			expect(",")
			needle := or()
			expect(")")
			return expressionContains(haystack, needle)
		case strings.HasPrefix(token, "'"):
			return strings.Trim(token, "'")
		case token == "true" || token == "false":
			return token == "true"
		default:
			value, ok := ctx[token]
			if !ok {
				t.Fatalf("unknown context %q in %q", token, source)
			}
			return value
		}
	}
	var unary func() any
	unary = func() any {
		if peek() == "!" {
			next()
			return !truthy(unary())
		}
		return primary()
	}
	comparison := func() any {
		left := unary()
		for peek() == "==" || peek() == "!=" {
			operator, right := next(), unary()
			left = looseEqual(left, right) == (operator == "==")
		}
		return left
	}
	and := func() any {
		left := comparison()
		for peek() == "&&" {
			next()
			right := comparison()
			if truthy(left) {
				left = right
			}
		}
		return left
	}
	or = func() any {
		left := and()
		for peek() == "||" {
			next()
			right := and()
			if !truthy(left) {
				left = right
			}
		}
		return left
	}
	value := or()
	if position != len(tokens) {
		t.Fatalf("trailing tokens in %q", source)
	}
	return value
}
