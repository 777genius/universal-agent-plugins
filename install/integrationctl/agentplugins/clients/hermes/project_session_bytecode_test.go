package hermes

import (
	"maps"
	"slices"
	"strings"
	"testing"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestProjectSessionInspectSuppressesBytecodeWithoutChangingCallerEnvironment(t *testing.T) {
	for _, selectors := range [][]string{nil, {"PYTHONDONTWRITEBYTECODE=0"}, {"PYTHONDONTWRITEBYTECODE="}, {"PYTHONDONTWRITEBYTECODE=0", "PYTHONDONTWRITEBYTECODE="}} {
		t.Run(strings.Join(selectors, ","), func(t *testing.T) {
			fixture := newSessionHistoryFixture(t)
			base := slices.DeleteFunc(slices.Clone(fixture.env.SessionEnvironment), func(value string) bool { return strings.HasPrefix(value, "PYTHONDONTWRITEBYTECODE=") })
			base = append(base, selectors...)
			fixture.env.SessionEnvironment = slices.Clone(base)
			plan := fixture.plan(t)
			if plan.Environment["PYTHONDONTWRITEBYTECODE"] != "1" || !slices.Equal(base, fixture.env.SessionEnvironment) {
				t.Fatal("inspection changed caller base or omitted bytecode suppression")
			}
			wanted := plan.ChildEnvironment(base)
			count := 0
			for _, value := range wanted {
				if strings.HasPrefix(value, "PYTHONDONTWRITEBYTECODE=") {
					count++
					if value != "PYTHONDONTWRITEBYTECODE=1" {
						t.Fatal("caller value reached child", value)
					}
				}
			}
			if count != 1 {
				t.Fatal("bytecode selector was not unique", wanted)
			}
			runner := &sessionInventoryRunner{plan: plan}
			env := fixture.env
			env.Runner = sessionHistoryRunner{runner, fixture.response}
			execution, err := New().BeginSessionExecution(t.Context(), env, plan)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := execution.Close(); err != nil {
					t.Error(err)
				}
			})
			frozen := env
			frozen.Runner = execution.Runner()
			if err := New().VerifySession(execution.Context(), frozen, plan); err != nil {
				t.Fatal(err)
			}
			command := legacyports.Command{Argv: append([]string{plan.Executable}, plan.Arguments...), Dir: plan.Directory, Env: wanted, StdoutLimitBytes: 1 << 20}
			if _, err := execution.Runner().Run(execution.Context(), command); err != nil {
				t.Fatal(err)
			}
			if len(runner.calls) != 4 {
				t.Fatal("metadata command count", len(runner.calls))
			}
			for _, environment := range runner.environments {
				if !slices.Equal(environment, wanted) {
					t.Fatal("metadata child environment diverged")
				}
			}
			interactive := sessionEnvironmentOverrides(plan.Directory, false, base)
			if _, exists := interactive["PYTHONDONTWRITEBYTECODE"]; exists {
				t.Fatal("interactive session acquired inspection policy")
			}
			for _, forgedValue := range []string{"missing", "", "0"} {
				forged := plan
				forged.Environment = maps.Clone(plan.Environment)
				if forgedValue == "missing" {
					delete(forged.Environment, "PYTHONDONTWRITEBYTECODE")
				} else {
					forged.Environment["PYTHONDONTWRITEBYTECODE"] = forgedValue
				}
				rejected := &sessionInventoryRunner{}
				env.Runner = sessionHistoryRunner{rejected, fixture.response}
				if err := New().VerifySession(t.Context(), env, forged); err == nil || len(rejected.calls) != 0 {
					t.Fatal("forged inspection selector reached child", forgedValue, err)
				}
			}
		})
	}
}
