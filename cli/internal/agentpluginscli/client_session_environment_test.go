package agentpluginscli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// This exercises the public run worker's actual prepare/verify/unlock/recheck
// sequence. The inert planner commits one selector; it is not native admission.
type sessionHandoffPlanner struct {
	plans, verifies int
	initial         []string
	finalEdit       func([]string)
	verifyEdit      func()
}

func (*sessionHandoffPlanner) ID() domain.ClientID { return domain.ClientHermes }

func (planner *sessionHandoffPlanner) PlanSession(_ context.Context, env clients.Env, req clients.SessionRequest) (clients.SessionPlan, error) {
	planner.plans++
	if env.SessionEnvironment == nil || !req.Inspect || len(req.Bindings) != 0 {
		return clients.SessionPlan{}, fmt.Errorf("handoff fixture received the wrong preparation")
	}
	digest := handoffEnvironmentValue(env.SessionEnvironment, "TMPDIR")
	if planner.plans == 1 {
		planner.initial = append([]string{}, env.SessionEnvironment...)
	} else if planner.plans == 2 && planner.finalEdit != nil {
		planner.finalEdit(env.SessionEnvironment)
	}
	return clients.SessionPlan{ClientID: domain.ClientHermes, Directory: req.ProjectRoot,
		Executable: "inert-handoff", Arguments: []string{"config", "get", "mcp_servers", "--json"},
		Environment:  map[string]string{"HERMES_HOME": filepath.Join(req.ProjectRoot, ".hermes", "uap-session")},
		ConfigDigest: digest, Servers: []string{"inert-owned"}}, nil
}

func (planner *sessionHandoffPlanner) VerifySession(_ context.Context, env clients.Env, _ clients.SessionPlan) error {
	planner.verifies++
	if planner.plans != 1 || !reflect.DeepEqual(env.SessionEnvironment, planner.initial) {
		return fmt.Errorf("verification did not receive the original environment")
	}
	if planner.verifyEdit != nil {
		planner.verifyEdit()
	}
	return nil
}

func handoffEnvironmentValue(environment []string, name string) string {
	for _, item := range environment {
		key, value, _ := strings.Cut(item, "=")
		if key == name {
			return value
		}
	}
	return ""
}

func TestSessionHandoffRetainsFinalEnvironment(t *testing.T) {
	for _, change := range []string{"ambient", "adapter-slice"} {
		t.Run(change, func(t *testing.T) {
			project, original := t.TempDir(), t.TempDir()
			t.Setenv("TMPDIR", original)
			planner := &sessionHandoffPlanner{finalEdit: func(base []string) {
				if change == "ambient" {
					t.Setenv("TMPDIR", "")
					return
				}
				for index, entry := range base {
					if strings.HasPrefix(entry, "TMPDIR=") {
						base[index] = "TMPDIR="
					}
				}
			}}
			starts := 0
			app := handoffSessionApp(t, project, planner)
			app.StartNativeSession = func(_ context.Context, plan clients.SessionPlan, base []string) error {
				starts++
				child := plan.ChildEnvironment(base)
				if handoffEnvironmentValue(child, "TMPDIR") != original ||
					handoffEnvironmentValue(child, "HERMES_HOME") != filepath.Join(project, ".hermes", "uap-session") {
					return fmt.Errorf("final start adopted a later environment")
				}
				return nil
			}
			err := runClientSession(t.Context(), &cobra.Command{}, app, &options{target: "hermes", format: "human"}, true, true)
			if err != nil || starts != 1 || planner.plans != 2 || planner.verifies != 1 {
				t.Fatalf("original handoff failed: %v; starts=%d plans=%d verifies=%d", err, starts, planner.plans, planner.verifies)
			}
		})
	}
}

func TestSessionHandoffRejectsChangedCommittedSelector(t *testing.T) {
	project, original, changed := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", original)
	planner := &sessionHandoffPlanner{verifyEdit: func() { t.Setenv("TMPDIR", changed) }}
	app := handoffSessionApp(t, project, planner)
	starts := 0
	app.StartNativeSession = func(context.Context, clients.SessionPlan, []string) error {
		starts++
		return nil
	}
	err := runClientSession(t.Context(), &cobra.Command{}, app, &options{target: "hermes", format: "human"}, true, true)
	if err == nil || !strings.Contains(err.Error(), "client startup plan changed") || starts != 0 || planner.plans != 2 || planner.verifies != 1 {
		t.Fatalf("changed original selector was not refused: %v; starts=%d plans=%d verifies=%d", err, starts, planner.plans, planner.verifies)
	}
	if os.Getenv("TMPDIR") != changed {
		t.Fatal("refusal rewrote the ambient environment")
	}
}

func handoffSessionApp(t *testing.T, project string, planner *sessionHandoffPlanner) App {
	t.Helper()
	registry, err := clients.NewRegistry(planner)
	if err != nil {
		t.Fatal(err)
	}
	return App{Scope: domain.ScopeProject, ProjectRoot: project, ClientRegistry: registry,
		StateStore: scopedFloorTestStore{}, Detector: staticDetector{clients: []domain.DetectedClient{{ClientID: domain.ClientHermes}}},
		Lifecycle: usecase.Service{Lock: migrationLockFunc(func(context.Context) (ports.UnlockFunc, error) {
			return func() error { return nil }, nil
		})}}
}
