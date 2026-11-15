package pluginkitairepo_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type cancellationWorkflow struct {
	On          map[string]any    `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Concurrency struct {
		Group  string `yaml:"group"`
		Cancel string `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Jobs map[string]struct {
		If    string `yaml:"if"`
		Steps []struct {
			Run string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func readCancellationWorkflow(t *testing.T, name string) cancellationWorkflow {
	t.Helper()
	var workflow cancellationWorkflow
	if err := yaml.Unmarshal([]byte(readRepoFile(t, RepoRoot(t), ".github", "workflows", name)), &workflow); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return workflow
}

func TestCISupersession_RegularPRChecksOnly(t *testing.T) {
	for name, prefix := range map[string]string{
		"agentplugins-windows-firebase-capture-probe.yml": "firebase-capture-",
		"docs-canary.yml": "docs-canary-",
		"remotion-windows-source-diagnostic.yml": "remotion-win-diagnostic-",
		"remotion-windows-source-longtemp-e2e.yml": "remotion-win-longtemp-",
		"scaffold-node-lock-e2e.yml": "scaffold-node-lock-",
	} {
		t.Run(name, func(t *testing.T) {
			w := readCancellationWorkflow(t, name)
			if _, ok := w.On["pull_request"]; !ok {
				t.Fatal("missing PR trigger")
			}
			if !strings.HasPrefix(w.Concurrency.Group, prefix) ||
				!strings.Contains(w.Concurrency.Group, "github.event_name == 'pull_request' && github.event.pull_request.number || github.run_id") ||
				w.Concurrency.Cancel != "${{ github.event_name == 'pull_request' }}" {
				t.Fatalf("%s lost PR-scoped cancellation or exact-source preservation: %+v", name, w.Concurrency)
			}
		})
	}
}

func TestCISupersession_ClosedPRCleanupCannotCancelOtherIdentities(t *testing.T) {
	const name = "cancel-closed-pr-checks.yml"
	w := readCancellationWorkflow(t, name)
	if len(w.On) != 1 || w.On["pull_request_target"] == nil ||
		w.Permissions["actions"] != "write" || w.Permissions["pull-requests"] != "read" {
		t.Fatal("closed-PR cleanup must run only from trusted base with minimum actions permission")
	}
	job, ok := w.Jobs["cancel-obsolete"]
	if !ok || !strings.Contains(job.If, "github.event.pull_request.base.repo.full_name == github.repository") || len(job.Steps) != 1 {
		t.Fatal("missing closed-PR repository identity gate")
	}
	script := job.Steps[0].Run
	for _, required := range []string{
		"set -euo pipefail", "gh api --paginate", "event=pull_request",
		".event == \"pull_request\"", ".head_repository.id == $head",
		".number == $pr and .head.repo.id == $head and .base.repo.id == $base",
		"sort -un", "--method POST", "current", "completed",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("closed-PR cleanup lost guard %q", required)
		}
	}
	for _, forbidden := range []string{"actions/checkout", "pull_request.head.sha", "git checkout"} {
		if strings.Contains(readRepoFile(t, RepoRoot(t), ".github", "workflows", name), forbidden) {
			t.Fatalf("closed-PR cleanup must not execute untrusted source: %s", forbidden)
		}
	}
}
