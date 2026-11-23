package pluginkitairepo_test

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

type mainSupersessionWorkflow struct {
	On map[string]yaml.Node `yaml:"on"`
	Concurrency struct {
		Group string `yaml:"group"`
		Cancel string `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
}

func TestCIMainPushEvidenceIsNeverCancelledByNextMerge(t *testing.T) {
	const suffix = "${{ github.event_name }}-${{ github.event_name == 'pull_request' && (github.event.pull_request.number || github.ref) || github.run_id }}"
	const prOnly = "${{ github.event_name == 'pull_request' }}"
	cases := []struct{
		file, prefix string
		triggers []string
	}{
		{"package-manager-e2e.yml", "package-manager-e2e", []string{"pull_request", "push", "schedule", "workflow_dispatch"}},
		{"source-transports.yml", "source-transports", []string{"pull_request", "push", "workflow_dispatch"}},
		{"landing.yml", "landing", []string{"pull_request", "push", "schedule", "workflow_dispatch"}},
		{"opencode-sdk.yml", "opencode-sdk", []string{"pull_request", "push", "workflow_dispatch"}},
		{"coverage.yml", "coverage", []string{"pull_request", "push"}},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			var w mainSupersessionWorkflow
			if err := yaml.Unmarshal([]byte(readRepoFile(t, RepoRoot(t), ".github", "workflows", tc.file)), &w); err != nil {
				t.Fatal(err)
			}
			actual := make([]string, 0, len(w.On))
			for name := range w.On {
				actual = append(actual, name)
			}
			slices.Sort(actual)
			slices.Sort(tc.triggers)
			if !slices.Equal(actual, tc.triggers) {
				t.Fatalf("original qualification event roster changed: got %v, want %v", actual, tc.triggers)
			}
			if got, want := w.Concurrency.Group, tc.prefix+"-"+suffix; got != want {
				t.Fatalf("main push must retain exact run identity: got %q, want %q", got, want)
			}
			if w.Concurrency.Cancel != prOnly {
				t.Fatalf("only obsolete PR runs may be cancelled: %s", w.Concurrency.Cancel)
			}
		})
	}
}
