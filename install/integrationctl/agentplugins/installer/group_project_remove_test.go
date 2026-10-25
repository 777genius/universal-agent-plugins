package installer

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func projectRemovalGroup(t *testing.T) *projectFilesFixture {
	t.Helper()
	f := newProjectFilesFixture(t, "cline", "disabled-with-runner")
	codexUser := filepath.Join(f.base, "user", ".codex")
	deathWrite(t, filepath.Join(codexUser, "keep.txt"), "original Codex profile")
	f.req.Targets = []ClientTarget{{ClientID: "cline", ClientConfigRoot: f.user}, {ClientID: "codex", ClientConfigRoot: codexUser}}
	f.reopen(t)
	f.apply(t, OpInstall)
	return f
}

func TestProjectGroupRemovalAbsentTargetsAreReadOnly(t *testing.T) {
	for _, firstAbsent := range []bool{false, true} {
		t.Run(map[bool]string{false: "both present", true: "one absent"}[firstAbsent], func(t *testing.T) {
			f := projectRemovalGroup(t)
			other := f.untouched(t)
			if firstAbsent {
				targets := f.req.Targets
				f.req.Targets = nil
				f.apply(t, OpRemove)
				f.req.Targets = targets
			}
			f.reopen(t)
			removed := f.apply(t, OpRemove)
			if removed.Outcome != OutcomeCompleted || !removed.Mutated || len(removed.Targets) != 2 {
				t.Fatalf("remove %+v", removed)
			}
			if firstAbsent && removed.Targets[0].Materialization != string(domain.MaterializationAbsent) {
				t.Fatal("absent target was not preserved", removed.Targets)
			}
			before := deathTree(t, f.cfg.StateRoot)
			repeated := f.apply(t, OpRemove)
			if repeated.Outcome != OutcomeUnchanged || repeated.Mutated || !repeated.NoChange || repeated.Reason != "already_absent" || !reflect.DeepEqual(before, deathTree(t, f.cfg.StateRoot)) {
				t.Fatalf("absent group mutated state/journal %+v", repeated)
			}
			f.assertUntouched(t, other)
		})
	}
}

func TestProjectGroupRemovalRefusalsPreserveAuthority(t *testing.T) {
	for _, kind := range []string{"managed drift", "foreign native", "other project", "mixed scope", "missing project", "unsupported target", "missing project hooks"} {
		t.Run(kind, func(t *testing.T) {
			f := projectRemovalGroup(t)
			switch kind {
			case "managed drift":
				deathWrite(t, filepath.Join(f.plan.ActivePath, "skills", "docs", "SKILL.md"), "changed managed package")
			case "foreign native":
				deathWrite(t, f.skill, "foreign native bytes")
			case "other project":
				f.req.ProjectRoot = filepath.Join(f.base, "B")
			case "mixed scope":
				f.req.Scope = domain.ScopeUser
			case "missing project":
				f.req.ProjectRoot = ""
			case "unsupported target":
				f.req.Targets[1].ClientID = "not-a-client"
			case "missing project hooks":
				registry, err := clients.NewRegistry(&handoffCaptureAdapter{id: domain.ClientCline}, codex.New())
				if err != nil {
					t.Fatal(err)
				}
				f.cfg.Registry = registry
				f.reopen(t)
			}
			before, skill, other := f.state(t), deathRead(t, f.skill), f.untouched(t)
			removed, err := f.execute(t, OpRemove)
			if err == nil || removed.Mutated || f.state(t) != before || deathRead(t, f.skill) != skill {
				t.Fatalf("refusal %+v %v", removed, err)
			}
			f.assertUntouched(t, other)
		})
	}
}

func TestProjectGroupRemovalRechecksChangedBinding(t *testing.T) {
	f := projectRemovalGroup(t)
	req := f.req
	req.Operation, req.OperationID = OpRemove, "frozen-project-group-removal"
	handle, err := f.engine.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	state, err := f.engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		if binding.ClientID == "cline" {
			binding.NativeProfileRoot = filepath.Join(f.project, "foreign-profile")
			state.Installations[0].Clients[key] = binding
		}
	}
	if err := f.engine.store.Save(state); err != nil {
		t.Fatal(err)
	}
	before, skill, other := f.state(t), deathRead(t, f.skill), f.untouched(t)
	removed, err := f.engine.Apply(t.Context(), handle, Decision{Confirmed: true})
	if err == nil || removed.Mutated || f.state(t) != before || deathRead(t, f.skill) != skill {
		t.Fatalf("changed binding removed %+v %v", removed, err)
	}
	f.assertUntouched(t, other)
}

func TestProjectGroupRemovalKeepsHistoricalUserGuard(t *testing.T) {
	f := projectRemovalGroup(t)
	// Exercise the original non-project planning branch with a retained
	// installation. A missing target must not fabricate removal authority.
	engine := *f.engine
	engine.cfg.ProjectRoot = ""
	req := f.req
	req.Scope, req.ProjectRoot, req.Operation = domain.ScopeUser, "", OpRemove
	before, other := f.state(t), f.untouched(t)
	_, err := engine.planRemoveGroup(t.Context(), &PreparedOperation{req: req}, domain.Installation{InstallationID: "retained"})
	if !errors.Is(err, ErrUnsupported) || !strings.Contains(err.Error(), "historical group removal requires Claude/Codex") || f.state(t) != before {
		t.Fatalf("user guard changed %v", err)
	}
	f.assertUntouched(t, other)
}
