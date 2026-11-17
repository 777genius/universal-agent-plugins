package pluginkitairepo_test

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// heavyPRLaneGate is the one Tier 1 rule from docs/CI_RUNNERS.md: draft pull
// requests skip the lane, while ready pull requests, pushes, schedules,
// workflow runs and manual dispatches keep it.
const heavyPRLaneGate = "github.event_name != 'pull_request' || github.event.pull_request.draft == false"

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

// A newer push supersedes the run of the same pull request; other events never
// cancel a running heavy lane.
const heavyPRLaneCancel = "${{ github.event_name == 'pull_request' }}"

// Without labeled or edited, a label or title change on a ready pull request
// cannot cancel and restart a running heavy lane.
var heavyPRLaneTypes = []string{"opened", "ready_for_review", "reopened", "synchronize"}

type heavyPRLane struct {
	// triggers is the exact set of events. pull_request_target, issue_comment
	// and similar events would pass the gate, because it only checks for
	// pull_request.
	triggers []string
	// group is the exact concurrency group. Pull request events must share one
	// group per pull request number, or cancel-in-progress has nothing to
	// supersede.
	group string
}

// heavyPRLaneWorkflows run native clients or wide GitHub-hosted matrices on
// pull requests. Release-called and authoring workflows keep their own rules.
var heavyPRLaneWorkflows = map[string]heavyPRLane{
	"agentplugins-published-catalog-audit.yml": {
		[]string{"pull_request", "schedule", "workflow_dispatch", "workflow_run"},
		"published-catalog-audit-${{ github.event.pull_request.number || github.ref }}",
	},
	"agentplugins-published-other-clients-audit.yml": {
		[]string{"pull_request", "schedule", "workflow_dispatch", "workflow_run"},
		"published-other-clients-${{ github.event.pull_request.number || github.ref }}",
	},
	"cline-managed-session.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"cline-managed-session-${{ github.event_name }}-${{ github.event_name == 'pull_request' && github.event.pull_request.number || github.run_id }}",
	},
	"hermes-noauth-tool-call.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"hermes-noauth-call-${{ github.event.pull_request.number || github.ref }}",
	},
	"kimi252-noauth-call.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"kimi252-noauth-call-${{ github.event_name }}-${{ github.event.pull_request.number || github.ref }}",
	},
	"kiro-no-auth-installation.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"kiro-no-auth-${{ github.event.pull_request.number || github.ref }}",
	},
	"live-install-audit.yml": {
		[]string{"pull_request", "workflow_dispatch"},
		"live-install-audit-${{ github.event_name }}-${{ github.event_name == 'pull_request' && github.event.pull_request.number || github.run_id }}",
	},
	"live-install-remaining-clients.yml": {
		[]string{"pull_request", "workflow_dispatch"},
		"remaining-client-audit-${{ github.event.pull_request.number || github.ref }}",
	},
	"marketing-kit-project-release-e2e.yml": {
		[]string{"pull_request", "push", "workflow_dispatch"},
		"published-marketing-project-${{ github.event.pull_request.number || github.ref }}",
	},
	"project-native-runtime.yml": {
		[]string{"pull_request", "workflow_dispatch"},
		"project-native-runtime-${{ github.event_name }}-${{ github.event.pull_request.number || github.run_id }}",
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
			if w.Concurrency.Group != lane.group || w.Concurrency.Cancel != heavyPRLaneCancel {
				t.Fatalf("concurrency = %+v, want group %q and cancel-in-progress %q", w.Concurrency, lane.group, heavyPRLaneCancel)
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
