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
	// Published audits also wait for a successful npm publish run.
	"${{ (github.event_name != 'workflow_run' || github.event.workflow_run.conclusion == 'success') && (" + heavyPRLaneGate + ") }}",
}

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
// pull requests. Release-called and authoring workflows keep their own rules.
var heavyPRLaneWorkflows = map[string]heavyPRLane{
	"agentplugins-published-catalog-audit.yml": {
		[]string{"pull_request", "schedule", "workflow_dispatch", "workflow_run"},
		"published-catalog-audit-", "github.ref",
	},
	"agentplugins-published-other-clients-audit.yml": {
		[]string{"pull_request", "schedule", "workflow_dispatch", "workflow_run"},
		"published-other-clients-", "github.ref",
	},
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
	Jobs map[string]struct {
		If    string    `yaml:"if"`
		Needs yaml.Node `yaml:"needs"`
	} `yaml:"jobs"`
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

			needs := map[string][]string{}
			for job, spec := range w.Jobs {
				needs[job] = decodeNeeds(t, spec.Needs)
			}
			var gated func(job string, seen map[string]bool) bool
			gated = func(job string, seen map[string]bool) bool {
				if seen[job] {
					return false
				}
				seen[job] = true
				condition := strings.TrimSpace(w.Jobs[job].If)
				if slices.Contains(heavyPRLaneConditions, condition) {
					return true
				}
				// Without an explicit status function GitHub adds success(), so
				// a job is skipped with its skipped needs. Any status function
				// replaces that, so such a job must carry the gate itself.
				if len(needs[job]) == 0 || readsStatusFunction(condition) {
					return false
				}
				for _, parent := range needs[job] {
					if gated(parent, seen) {
						return true
					}
				}
				return false
			}
			for job := range w.Jobs {
				if !gated(job, map[string]bool{}) {
					t.Errorf("job %s can run on a draft pull request; use one of the conditions %q", job, heavyPRLaneConditions)
				}
			}
		})
	}
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
	for _, function := range []string{"always()", "cancelled()", "failure()", "success()"} {
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
// always() and contains(), with GitHub's loose equality and truthiness.
// Other functions, !, comparisons such as < and object filters fail the test.
func evalExpression(t *testing.T, source string, ctx map[string]any) any {
	t.Helper()
	tokens := regexp.MustCompile(`\|\||&&|==|!=|\(|\)|,|'[^']*'|[A-Za-z_][A-Za-z0-9_.*-]*`).FindAllString(source, -1)
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
	comparison := func() any {
		left := primary()
		for peek() == "==" || peek() == "!=" {
			operator, right := next(), primary()
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
