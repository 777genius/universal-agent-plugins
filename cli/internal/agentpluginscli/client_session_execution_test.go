package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var (
	errSessionHistoryChanged = errors.New("original session history changed")
	errSessionExecutionStep  = errors.New("session execution step failed")
	errSessionExecutionClose = errors.New("session execution close failed")
)

// This is a dispatcher fixture, not a native ownership receipt. The real empty
// state store exercises sessionBindings. Default observations are inert; the
// separate retained-process tests inject the real final process runner.
type sessionExecutionFixture struct {
	plan                               clients.SessionPlan
	path, mode                         string
	ctx                                context.Context
	cancel                             context.CancelFunc
	file, original                     *os.File
	info                               os.FileInfo
	plans, begins, verifies, calls     int
	starts, releases, closes, rechecks int
	ordinary                           int
	finalRunner                        ports.CommandRunner
	finalEdit                          func([]string)
}

func (*sessionExecutionFixture) ID() domain.ClientID { return domain.ClientHermes }

func (fixture *sessionExecutionFixture) PlanSession(ctx context.Context, env clients.Env, _ clients.SessionRequest) (clients.SessionPlan, error) {
	fixture.plans++
	if err := ctx.Err(); err != nil {
		return clients.SessionPlan{}, err
	}
	plan := fixture.plan
	if fixture.plans == 2 {
		if fixture.finalEdit != nil {
			fixture.finalEdit(env.SessionEnvironment)
		}
		switch fixture.mode {
		case "replan-error":
			return clients.SessionPlan{}, errSessionExecutionStep
		case "replan-change":
			plan.ConfigDigest = "different"
		case "before-start":
			if err := os.WriteFile(fixture.path, []byte("changed"), 0o600); err != nil {
				return clients.SessionPlan{}, err
			}
		}
	}
	return plan, nil
}

func (fixture *sessionExecutionFixture) BeginSessionExecution(ctx context.Context, _ clients.Env, _ clients.SessionPlan) (clients.SessionExecution, error) {
	fixture.begins++
	if fixture.mode == "nil" {
		fixture.ctx = ctx
		return nil, nil
	}
	fixture.ctx, fixture.cancel = context.WithTimeout(ctx, time.Minute)
	var err error
	fixture.file, err = os.Open(fixture.path)
	if err != nil {
		fixture.cancel()
		return nil, err
	}
	fixture.original = fixture.file
	fixture.info, err = fixture.file.Stat()
	if err != nil {
		return nil, errors.Join(err, fixture.Close())
	}
	if fixture.mode == "begin" {
		return fixture, errSessionExecutionStep
	}
	return fixture, nil
}

func (fixture *sessionExecutionFixture) VerifySession(ctx context.Context, env clients.Env, _ clients.SessionPlan) error {
	fixture.verifies++
	if fixture.releases != 0 || ctx != fixture.ctx || fixture.mode != "nil" && env.Runner != fixture {
		return errors.New("original verifier lost the lock, context or wrapped runner")
	}
	if fixture.mode == "verify" {
		return errSessionExecutionStep
	}
	for range 3 {
		if _, err := env.Runner.Run(ctx, legacyports.Command{}); err != nil {
			return err
		}
	}
	return nil
}

func (fixture *sessionExecutionFixture) Context() context.Context {
	if fixture.mode == "nil-context" {
		return nil
	}
	return fixture.ctx
}

func (fixture *sessionExecutionFixture) Runner() ports.CommandRunner {
	if fixture.mode == "nil-runner" {
		return nil
	}
	return fixture
}

func (fixture *sessionExecutionFixture) Revalidate() error {
	fixture.rechecks++
	if err := fixture.ctx.Err(); err != nil {
		return err
	}
	named, err := os.Stat(fixture.path)
	if err != nil || !os.SameFile(fixture.info, named) || fixture.info.Mode() != named.Mode() ||
		fixture.info.Size() != named.Size() || !fixture.info.ModTime().Equal(named.ModTime()) {
		return errSessionHistoryChanged
	}
	body, err := os.ReadFile(fixture.path)
	if err != nil || string(body) != "original" {
		return errSessionHistoryChanged
	}
	return nil
}

func (fixture *sessionExecutionFixture) Close() error {
	fixture.closes++
	err := fixture.file.Close()
	fixture.file = nil
	fixture.cancel()
	if fixture.mode == "unlock" || fixture.mode == "close" {
		err = errors.Join(err, errSessionExecutionClose)
	}
	return err
}

func (fixture *sessionExecutionFixture) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	fixture.calls++
	if ctx != fixture.ctx {
		return legacyports.CommandResult{}, errors.New("native call renewed the original context")
	}
	if fixture.calls == 4 {
		fixture.starts++
		if fixture.releases != 1 || fixture.plans != 2 {
			return legacyports.CommandResult{}, errors.New("final command ran before release and replan")
		}
		if fixture.finalRunner != nil {
			return fixture.finalRunner.Run(ctx, command)
		}
		return legacyports.CommandResult{CleanupComplete: true, Stdout: []byte("inspected\n")}, fixture.final(ctx)
	}
	return legacyports.CommandResult{CleanupComplete: true}, ctx.Err()
}

type sessionExecutionOriginalRunner struct{ fixture *sessionExecutionFixture }

func (runner sessionExecutionOriginalRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return runner.fixture.Run(ctx, command)
}

func newSessionExecutionFixture(t *testing.T, mode string) (App, *sessionExecutionFixture) {
	t.Helper()
	base := newCLIFixture(t, []domain.DetectedClient{{ClientID: domain.ClientHermes, Status: domain.DetectionDetected}})
	fixture := &sessionExecutionFixture{path: filepath.Join(base.root, "history"), mode: mode,
		plan: clients.SessionPlan{ClientID: domain.ClientHermes, Directory: base.root, Executable: filepath.Join(base.root, "inert-hermes"),
			Arguments: []string{"config", "get", "mcp_servers", "--json"}, ConfigDigest: "original"}}
	if err := os.WriteFile(fixture.path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if fixture.file != nil {
			_ = fixture.file.Close()
		}
		if fixture.cancel != nil {
			fixture.cancel()
		}
	})
	app := base.app
	app.Scope, app.ProjectRoot = domain.ScopeProject, base.root
	var err error
	app.ClientRegistry, err = clients.NewRegistry(fixture)
	if err != nil {
		t.Fatal(err)
	}
	app.SessionEnv.Runner = sessionExecutionOriginalRunner{fixture}
	app.Lifecycle.Lock = migrationLockFunc(func(context.Context) (ports.UnlockFunc, error) {
		return func() error {
			fixture.releases++
			if mode == "unlock" {
				return errSessionExecutionStep
			}
			return nil
		}, nil
	})
	app.StartNativeSession = fixture.start
	return app, fixture
}

func (fixture *sessionExecutionFixture) start(ctx context.Context, _ clients.SessionPlan, _ []string) error {
	fixture.ordinary++
	_, err := fixture.Run(ctx, legacyports.Command{})
	return err
}

func (fixture *sessionExecutionFixture) final(ctx context.Context) error {
	switch fixture.mode {
	case "fourth-mutation":
		return os.WriteFile(fixture.path, []byte("changed"), 0o600)
	case "nonzero":
		return errSessionExecutionStep
	case "canceled":
		fixture.cancel()
		return context.Canceled
	}
	return ctx.Err()
}

func TestSessionExecutionGuardSpansFourthCommand(t *testing.T) {
	for _, mode := range []string{"clean", "fourth-mutation", "nonzero", "canceled", "close", "nil"} {
		t.Run(mode, func(t *testing.T) {
			app, fixture := newSessionExecutionFixture(t, mode)
			command := &cobra.Command{}
			var output bytes.Buffer
			command.SetOut(&output)
			err := runClientSession(t.Context(), command, app, &options{target: "hermes", format: "human"}, true, true)
			expected := map[string]error{"fourth-mutation": errSessionHistoryChanged, "nonzero": errSessionExecutionStep, "canceled": context.Canceled, "close": errSessionExecutionClose}[mode]
			if expected == nil && err != nil || expected != nil && !errors.Is(err, expected) {
				t.Fatalf("final execution = %v, want %v", err, expected)
			}
			if fixture.begins != 1 || fixture.verifies != 1 || fixture.calls != 4 || fixture.starts != 1 || fixture.plans != 2 || fixture.releases != 1 {
				t.Fatal("original four-command execution order changed", fixture)
			}
			if mode == "nil" {
				if fixture.closes != 0 || fixture.rechecks != 0 || fixture.ordinary != 1 {
					t.Fatal("nil execution changed the ordinary flow")
				}
				return
			}
			if fixture.ordinary != 0 || mode == "clean" && output.String() != "inspected\n" || mode != "clean" && output.Len() != 0 {
				t.Fatal("retained execution used process replacement or emitted unvalidated output")
			}
			if fixture.closes != 1 || fixture.rechecks < 2 {
				t.Fatal("execution did not settle exactly once")
			}
			if err := fixture.original.Close(); !errors.Is(err, os.ErrClosed) {
				t.Fatal("original descriptor survived final execution", err)
			}
		})
	}
}

func TestSessionExecutionGuardClosesOnEarlyFailures(t *testing.T) {
	for _, mode := range []string{"begin", "verify", "unlock", "replan-error", "replan-change", "before-start"} {
		t.Run(mode, func(t *testing.T) {
			app, fixture := newSessionExecutionFixture(t, mode)
			err := runClientSession(t.Context(), &cobra.Command{}, app, &options{target: "hermes", format: "human"}, true, true)
			if err == nil || fixture.starts != 0 || fixture.closes != 1 || fixture.releases != 1 || fixture.rechecks == 0 {
				t.Fatal("early refusal bypassed settlement", mode, err, fixture)
			}
			wantPlans, wantCalls, wantVerify := 2, 3, 1
			switch mode {
			case "begin":
				wantPlans, wantCalls, wantVerify = 1, 0, 0
			case "verify":
				wantPlans, wantCalls = 1, 0
			case "unlock":
				wantPlans = 1
			}
			if fixture.plans != wantPlans || fixture.calls != wantCalls || fixture.verifies != wantVerify {
				t.Fatal("failure reached a later native stage", mode, fixture)
			}
			if mode == "unlock" && (!errors.Is(err, errSessionExecutionStep) || !errors.Is(err, errSessionExecutionClose)) {
				t.Fatal("unlock or close error was discarded", err)
			}
			if err := fixture.original.Close(); !errors.Is(err, os.ErrClosed) {
				t.Fatal("early failure leaked the captured descriptor", err)
			}
		})
	}
}

func TestSessionExecutionCaptureRequiresConsentAndNonDryRun(t *testing.T) {
	for _, dry := range []bool{false, true} {
		app, fixture := newSessionExecutionFixture(t, "clean")
		command := &cobra.Command{}
		var output bytes.Buffer
		command.SetOut(&output)
		err := runClientSession(t.Context(), command, app, &options{target: "hermes", format: "human", dryRun: dry}, true, false)
		if dry && err != nil || !dry && err == nil {
			t.Fatal("consent/dry-run behavior changed", err)
		}
		if fixture.begins != 0 || fixture.verifies != 0 || fixture.calls != 0 || fixture.starts != 0 || fixture.closes != 0 {
			t.Fatal("read-only planning or missing consent captured execution authority")
		}
		if dry && (fixture.plans != 1 || fixture.releases != 1) || !dry && (fixture.plans != 0 || fixture.releases != 0) {
			t.Fatal("dry-run or consent changed the original planning/lock boundary")
		}
	}
}
