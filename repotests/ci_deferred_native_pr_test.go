package pluginkitairepo_test

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const draftDeferredNativeGate = "${{ github.event_name != 'pull_request' || github.event.pull_request.draft != true }}"

type draftDeferredWorkflow struct {
	On   map[string]yaml.Node `yaml:"on"`
	Jobs map[string]struct {
		If    string    `yaml:"if"`
		Needs yaml.Node `yaml:"needs"`
	} `yaml:"jobs"`
}

func TestCIDeferredNativePR_ReadyAndReleaseBoundary(t *testing.T) {
	root := RepoRoot(t)
	cases := []struct {
		file               string
		jobs               []string
		gated              []string
		manual, push, call bool
	}{
		{"hermes-native-e2e.yml", []string{"native-platform", "native-windows-x64", "aggregate-release-native"}, []string{"native-platform", "native-windows-x64"}, true, false, true},
		{"physical-profile-native.yml", []string{"native"}, []string{"native"}, true, false, false},
		{"source-transports.yml", []string{"native-ssh"}, []string{"native-ssh"}, true, true, false},
		{"grok-native-research.yml", []string{"research"}, []string{"research"}, true, false, false},
		{"remotion-windows-source-longtemp-e2e.yml", []string{"remotion-long-temp"}, []string{"remotion-long-temp"}, true, false, false},
		{"scaffold-node-lock-e2e.yml", []string{"generated-node-stdio"}, []string{"generated-node-stdio"}, false, true, false},
		{"transaction-recovery-e2e.yml", []string{"transaction-recovery"}, []string{"transaction-recovery"}, false, true, false},
		{"release-pipeline-rehearsal.yml", []string{"rehearsal"}, []string{"rehearsal"}, true, false, false},
		{"authoring-milestone-a-e2e.yml", []string{"prepare", "e2e"}, []string{"prepare"}, true, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			source := readRepoFile(t, root, ".github", "workflows", tc.file)
			var w draftDeferredWorkflow
			if err := yaml.Unmarshal([]byte(source), &w); err != nil {
				t.Fatalf("invalid workflow YAML: %v", err)
			}
			if _, ok := w.On["pull_request"]; !ok {
				t.Fatal("PR E2E trigger removed")
			}
			var pr struct {
				Types []string `yaml:"types"`
			}
			prEvent := w.On["pull_request"]
			if err := prEvent.Decode(&pr); err != nil {
				t.Fatal(err)
			}
			slices.Sort(pr.Types)
			expected := []string{"opened", "ready_for_review", "reopened", "synchronize"}
			if !slices.Equal(pr.Types, expected) {
				t.Fatalf("must re-run real E2E on Ready: %v", pr.Types)
			}
			for trigger, need := range map[string]bool{
				"workflow_dispatch": tc.manual,
				"push":              tc.push,
				"workflow_call":     tc.call,
			} {
				_, exists := w.On[trigger]
				if exists != need {
					t.Fatalf("%s event changed: got %t, want %t", trigger, exists, need)
				}
			}
			actualJobs := make([]string, 0, len(w.Jobs))
			for name := range w.Jobs {
				actualJobs = append(actualJobs, name)
			}
			slices.Sort(actualJobs)
			slices.Sort(tc.jobs)
			if !slices.Equal(actualJobs, tc.jobs) {
				t.Fatalf("changed native/release job closure: got %v, want %v", actualJobs, tc.jobs)
			}
			for _, job := range tc.gated {
				if w.Jobs[job].If != draftDeferredNativeGate {
					t.Errorf("%s: missing exact PR-only Draft gate", job)
				}
			}
			// Existing release aggregation remains independent of Draft:
			// reusable workflows and manual dispatch must run fully.
			if tc.file == "hermes-native-e2e.yml" {
				if w.Jobs["aggregate-release-native"].If != "${{ always() && inputs.release_tag != '' }}" {
					t.Fatal("release aggregate condition changed")
				}
			}
			if tc.file == "authoring-milestone-a-e2e.yml" {
				if w.Jobs["e2e"].If != "" || !strings.Contains(source, "    needs: prepare") {
					t.Fatal("exact 4-job Milestone A graph changed")
				}
			}
		})
	}
}
