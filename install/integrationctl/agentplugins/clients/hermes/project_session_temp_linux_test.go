//go:build linux

package hermes

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestProjectSessionLinuxTempOverrideUsesOnlyMissingInspectSelectors(t *testing.T) {
	t.Setenv("TMPDIR", "/ambient-must-not-be-read")
	for _, test := range []struct {
		name      string
		base      []string
		inspect   bool
		defaulted bool
	}{
		{name: "missing", inspect: true, defaulted: true},
		{name: "explicit empty base", base: []string{}, inspect: true, defaulted: true},
		{name: "full session", inspect: false},
		{name: "caller TMPDIR", base: []string{"TMPDIR=/chosen"}, inspect: true},
		{name: "caller TMP", base: []string{"TMP=/chosen"}, inspect: true},
		{name: "caller TEMP", base: []string{"TEMP=/chosen"}, inspect: true},
		{name: "caller ordering", base: []string{"KEEP=first", "TEMP=/chosen", "Z=last"}, inspect: true},
		{name: "blank", base: []string{"TMPDIR="}, inspect: true},
		{name: "duplicates", base: []string{"TMPDIR=/one", "TMPDIR=/two"}, inspect: true},
		{name: "scratch", base: []string{"HERMES_SCRATCH_DIR=/chosen"}, inspect: true},
		{name: "blank scratch", base: []string{"HERMES_SCRATCH_DIR="}, inspect: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := slices.Clone(test.base)
			overrides := sessionEnvironmentOverrides("/project", test.inspect, test.base)
			wanted := map[string]string{"HERMES_HOME": projectSessionRoot("/project")}
			if test.inspect {
				wanted["PYTHONDONTWRITEBYTECODE"] = "1"
			}
			if test.defaulted {
				wanted["TMPDIR"] = "/tmp"
			}
			if !maps.Equal(overrides, wanted) || !slices.Equal(test.base, original) {
				t.Fatal("caller selectors changed or ambient default adopted", overrides)
			}
			plan := clients.SessionPlan{Environment: overrides}
			first, second := plan.ChildEnvironment(test.base), plan.ChildEnvironment(test.base)
			wantedChild := append(slices.Clone(test.base), "HERMES_HOME="+projectSessionRoot("/project"))
			if test.defaulted {
				wantedChild = []string{"HERMES_HOME=" + projectSessionRoot("/project"), "PYTHONDONTWRITEBYTECODE=1", "TMPDIR=/tmp"}
			}
			if test.inspect && !test.defaulted {
				wantedChild = append(wantedChild, "PYTHONDONTWRITEBYTECODE=1")
			}
			if !slices.Equal(first, second) || !slices.Equal(first, wantedChild) {
				t.Fatal("child override order is not deterministic", first, second)
			}
		})
	}
}

// Remove only the four policy selectors from this test-owned frozen base.
func sessionWithoutTempSelectors(base []string) []string {
	result := make([]string, 0, len(base))
	for _, item := range base {
		key, _, _ := strings.Cut(item, "=")
		switch key {
		case "TMPDIR", "TMP", "TEMP", "HERMES_SCRATCH_DIR":
		default:
			result = append(result, item)
		}
	}
	return result
}

func TestProjectSessionLinuxTempRefusesPruningAndAmbiguousSelectors(t *testing.T) {
	for _, selectors := range [][]string{
		{"TMPDIR="},
		{"TMPDIR= \t"},
		{"TMPDIR=/one", "TMPDIR=/two"},
		{"HERMES_SCRATCH_DIR="},
		{"HERMES_SCRATCH_DIR=/tmp"},
		{"TMPDIR=/tmp", "HERMES_SCRATCH_DIR=/tmp"},
	} {
		t.Run(strings.Join(selectors, ";"), func(t *testing.T) {
			fixture := newSessionHistoryFixture(t)
			fixture.env.SessionEnvironment = append(sessionWithoutTempSelectors(fixture.env.SessionEnvironment), selectors...)
			original := slices.Clone(fixture.env.SessionEnvironment)
			runner := &sessionInventoryRunner{}
			fixture.env.Runner = runner
			if _, err := New().PlanSession(t.Context(), fixture.env, fixture.req); err == nil || len(runner.calls) != 0 {
				t.Fatal("ambiguous or pruning selectors reached native execution", err)
			}
			if !slices.Equal(original, fixture.env.SessionEnvironment) {
				t.Fatal("refusal changed supplied selectors")
			}
		})
	}
}

func TestProjectSessionLinuxTempRejectsForgedPlanOverrides(t *testing.T) {
	fixture := newSessionHistoryFixture(t)
	fixture.env.SessionEnvironment = sessionWithoutTempSelectors(fixture.env.SessionEnvironment)
	plan := fixture.plan(t)
	for _, change := range []string{"foreign temporary", "missing default", "extra key", "full session"} {
		t.Run(change, func(t *testing.T) {
			forged := plan
			forged.Environment = maps.Clone(plan.Environment)
			switch change {
			case "foreign temporary":
				forged.Environment["TMPDIR"] = "/foreign"
			case "missing default":
				delete(forged.Environment, "TMPDIR")
			case "extra key":
				forged.Environment["TEMP"] = "/tmp"
			case "full session":
				forged.Arguments = nil
			}
			runner := &sessionInventoryRunner{}
			env := fixture.env
			env.Runner = runner
			if err := New().VerifySession(t.Context(), env, forged); err == nil || len(runner.calls) != 0 {
				t.Fatal("forged overrides reached a child", err)
			}
		})
	}
}

func TestProjectSessionLinuxTempHistoryRetainsEffectiveFourCommandEnvironment(t *testing.T) {
	fixture := newSessionHistoryFixture(t)
	fixture.env.SessionEnvironment = sessionWithoutTempSelectors(fixture.env.SessionEnvironment)
	original := slices.Clone(fixture.env.SessionEnvironment)
	plan := fixture.plan(t)
	if !maps.Equal(plan.Environment, map[string]string{"HERMES_HOME": fixture.root, "PYTHONDONTWRITEBYTECODE": "1", "TMPDIR": "/tmp"}) {
		t.Fatal("missing selectors did not choose the exact inspection override", plan.Environment)
	}
	runner := &sessionInventoryRunner{plan: plan}
	fixture.env.Runner = sessionHistoryRunner{runner, fixture.response}
	execution, err := New().BeginSessionExecution(t.Context(), fixture.env, plan)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := execution.Close(); err != nil {
			t.Error(err)
		}
	})
	// Mutating the caller's slice after capture cannot replace the frozen base.
	fixture.env.SessionEnvironment[0] = "CALLER_MUTATED=1"
	env := fixture.env
	env.SessionEnvironment, env.Runner = original, execution.Runner()
	if err := New().VerifySession(execution.Context(), env, plan); err != nil || len(runner.calls) != 3 {
		t.Fatal("verification did not use the original effective base", err)
	}
	history := execution.(*sessionInspectHistory)
	if history.state.passive == nil || !sessionCapturedTemp(history.state.passive.selectors) {
		t.Fatal("passive source did not commit the effective temporary selector")
	}
	check, err := New().PlanSession(execution.Context(), env, fixture.req)
	if err != nil || !reflect.DeepEqual(check, plan) {
		t.Fatal("unchanged final replan differs", err)
	}
	assertSessionTempFinalChild(t, execution, plan, original, runner)
	changed := env
	changed.SessionEnvironment = append(slices.Clone(original), "TMPDIR=/tmp")
	if err := New().VerifySession(execution.Context(), changed, plan); err == nil || len(runner.calls) != 4 {
		t.Fatal("later supplied selector replaced the captured base", err)
	}
}

func sessionCapturedTemp(selectors []string) bool {
	for index := 0; index+1 < len(selectors); index += 2 {
		if selectors[index] == "TMPDIR" {
			return selectors[index+1] == "/tmp"
		}
	}
	return false
}

func assertSessionTempFinalChild(t *testing.T, execution clients.SessionExecution, plan clients.SessionPlan, original []string, runner *sessionInventoryRunner) {
	t.Helper()
	wanted := plan.ChildEnvironment(original)
	forged := clients.SessionPlan{Environment: map[string]string{"TMPDIR": "/foreign"}}.ChildEnvironment(wanted)
	command := legacyports.Command{Argv: append([]string{plan.Executable}, plan.Arguments...), Dir: plan.Directory, Env: forged, StdoutLimitBytes: 1 << 20}
	if _, err := execution.Runner().Run(execution.Context(), command); err == nil || len(runner.calls) != 3 {
		t.Fatal("changed final child environment was delegated", err)
	}
	command.Env = wanted
	if _, err := execution.Runner().Run(execution.Context(), command); err != nil || len(runner.calls) != 4 {
		t.Fatal("original final child failed", err)
	}
	for _, actual := range runner.environments {
		if !slices.Equal(actual, wanted) {
			t.Fatal("metadata and final child used different effective bases")
		}
	}
}
