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

// heavyPRLaneWorkflows run native clients or wide GitHub-hosted matrices on
// pull requests. Release-called and authoring workflows keep their own rules.
var heavyPRLaneWorkflows = []string{
	"agentplugins-published-catalog-audit.yml",
	"agentplugins-published-other-clients-audit.yml",
	"cline-managed-session.yml",
	"hermes-noauth-tool-call.yml",
	"kimi252-noauth-call.yml",
	"kiro-no-auth-installation.yml",
	"live-install-audit.yml",
	"live-install-remaining-clients.yml",
	"marketing-kit-project-release-e2e.yml",
	"project-native-runtime.yml",
}

type tierWorkflow struct {
	On          map[string]yaml.Node `yaml:"on"`
	Concurrency struct {
		Cancel string `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Jobs map[string]struct {
		If    string    `yaml:"if"`
		Needs yaml.Node `yaml:"needs"`
	} `yaml:"jobs"`
}

func TestCITiers_HeavyPRLanesSkipDraftsAndRunWhenReady(t *testing.T) {
	for _, name := range heavyPRLaneWorkflows {
		t.Run(name, func(t *testing.T) {
			var w tierWorkflow
			if err := yaml.Unmarshal([]byte(readRepoFile(t, RepoRoot(t), ".github", "workflows", name)), &w); err != nil {
				t.Fatal(err)
			}
			pr, ok := w.On["pull_request"]
			if !ok {
				t.Fatal("missing pull_request trigger")
			}
			var trigger struct {
				Types []string `yaml:"types"`
			}
			if err := pr.Decode(&trigger); err != nil {
				t.Fatal(err)
			}
			// ready_for_review starts the full run on the head that becomes
			// mergeable; the default activity types must stay.
			for _, kind := range []string{"opened", "synchronize", "reopened", "ready_for_review"} {
				if !slices.Contains(trigger.Types, kind) {
					t.Fatalf("pull_request types %v lack %q", trigger.Types, kind)
				}
			}
			if _, ok := w.On["workflow_dispatch"]; !ok {
				t.Fatal("heavy lane must stay available on demand through workflow_dispatch")
			}
			if !strings.Contains(w.Concurrency.Cancel, "pull_request") {
				t.Fatalf("superseded pull request runs must be cancelled: %q", w.Concurrency.Cancel)
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
				condition := w.Jobs[job].If
				if hasGate(condition) {
					return true
				}
				// A job that reads status functions runs after skipped needs, so it
				// must carry the gate itself.
				if len(needs[job]) == 0 || bypassesSkippedNeeds(condition) {
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
					t.Errorf("job %s can run on a draft pull request; add if: ${{ %s }}", job, heavyPRLaneGate)
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

func hasGate(condition string) bool {
	condition = strings.TrimSpace(condition)
	return condition == "${{ "+heavyPRLaneGate+" }}" || strings.Contains(condition, "("+heavyPRLaneGate+")")
}

func bypassesSkippedNeeds(condition string) bool {
	for _, function := range []string{"always()", "cancelled()", "failure()"} {
		if strings.Contains(condition, function) {
			return true
		}
	}
	return false
}
