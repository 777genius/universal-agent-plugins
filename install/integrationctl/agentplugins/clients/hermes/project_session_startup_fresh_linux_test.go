//go:build linux

package hermes

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type freshSessionStartupEntry struct {
	path  string
	info  os.FileInfo
	body  []byte
	names []string
}

type freshSessionStartupFixture struct {
	req    clients.SessionRequest
	owner  domain.NativeObjectOwnership
	env    clients.Env
	runner *sessionInventoryRunner
	before []freshSessionStartupEntry
}

func newFreshSessionStartupFixture(t *testing.T) freshSessionStartupFixture {
	t.Helper()
	req, owner := newProjectSessionFixture(t)
	userConfig := filepath.Join(req.Client.ConfigRoot, "config.yaml")
	historyWrite(t, userConfig, []byte("model: original-user\n"), 0o600)
	runner := &sessionInventoryRunner{}
	env := sessionInventoryEnvironment(t, clients.SessionPlan{Directory: req.ProjectRoot}, runner)
	// Only this test-owned copy loses the selectors varied below. Ambient
	// environment and the shared fixture are unchanged.
	env.SessionEnvironment = make([]string, 0)
	for _, item := range sessionWithoutTempSelectors(os.Environ()) {
		key, _, _ := strings.Cut(item, "=")
		if key != "HERMES_INSTALL_ROOT" && key != "HERMES_GATEWAY_LOCK_DIR" {
			env.SessionEnvironment = append(env.SessionEnvironment, item)
		}
	}
	root := projectSessionRoot(req.ProjectRoot)
	paths := []string{req.ProjectRoot, filepath.Dir(root), root, filepath.Join(root, ".uap-owner"),
		filepath.Join(root, "config.yaml"), req.Client.ConfigRoot, userConfig}
	fixture := freshSessionStartupFixture{req: req, owner: owner, env: env, runner: runner}
	for _, path := range paths {
		fixture.before = append(fixture.before, readFreshSessionStartupEntry(t, path))
	}
	return fixture
}

func readFreshSessionStartupEntry(t *testing.T, path string) freshSessionStartupEntry {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	row := freshSessionStartupEntry{path: path, info: info}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			row.names = append(row.names, entry.Name())
		}
	} else {
		row.body, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	return row
}

func (fixture freshSessionStartupFixture) assertOriginal(t *testing.T) {
	t.Helper()
	for _, before := range fixture.before {
		after := readFreshSessionStartupEntry(t, before.path)
		if !os.SameFile(before.info, after.info) || before.info.Mode() != after.info.Mode() ||
			before.info.Size() != after.info.Size() || !before.info.ModTime().Equal(after.info.ModTime()) ||
			!bytes.Equal(before.body, after.body) || !slices.Equal(before.names, after.names) {
			t.Fatalf("fresh startup changed original fixture: %s", before.path)
		}
	}
	if err := verifyProjectSessionRoot(t.Context(), fixture.req.ProjectRoot, fixture.owner); err != nil {
		t.Fatal("fresh startup changed original ownership", err)
	}
}

func TestProjectSessionFreshLinuxStartupRetainsEffectiveEnvironment(t *testing.T) {
	for _, supplied := range []bool{false, true} {
		name := "missing-selectors"
		if supplied {
			name = "original-distinct-TEMP"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newFreshSessionStartupFixture(t)
			env := fixture.env
			wanted := map[string]string{"HERMES_HOME": projectSessionRoot(fixture.req.ProjectRoot)}
			if supplied {
				env.SessionEnvironment = append(slices.Clone(env.SessionEnvironment), "TEMP="+t.TempDir())
			} else {
				wanted["TMPDIR"] = "/tmp"
			}
			original := slices.Clone(env.SessionEnvironment)
			plan, err := New().PlanSession(t.Context(), env, fixture.req)
			if err != nil || !maps.Equal(plan.Environment, wanted) || len(fixture.runner.calls) != 0 {
				t.Fatal("fresh planning did not preserve exact startup policy", err, plan.Environment)
			}
			fixture.runner.plan = plan
			assertFreshSessionStartupCommands(t, fixture, env, plan)
			if !slices.Equal(original, env.SessionEnvironment) {
				t.Fatal("fresh startup changed the original supplied environment")
			}
			fixture.assertOriginal(t)
		})
	}
}

// The adapter reads the real original source; the existing recording runner
// observes its commands without executing an official native binary.
func assertFreshSessionStartupCommands(t *testing.T, fixture freshSessionStartupFixture, env clients.Env, plan clients.SessionPlan) {
	t.Helper()
	execution, err := New().BeginSessionExecution(t.Context(), env, plan)
	if err != nil || execution == nil {
		t.Fatal("fresh execution capture failed", err)
	}
	t.Cleanup(func() {
		if err := execution.Close(); err != nil {
			t.Error(err)
		}
	})
	history, ok := execution.(*sessionInspectHistory)
	if !ok || history.state == nil || history.state.passive != nil || len(history.state.admitted()) != 0 {
		t.Fatal("fresh fixture unexpectedly acquired runtime history")
	}
	env.Runner = execution.Runner()
	if err := New().VerifySession(execution.Context(), env, plan); err != nil || len(fixture.runner.calls) != 3 {
		t.Fatal("fresh verification did not retain its original source", err)
	}
	wanted := plan.ChildEnvironment(env.SessionEnvironment)
	command := legacyports.Command{Argv: append([]string{plan.Executable}, plan.Arguments...),
		Dir: plan.Directory, Env: wanted, StdoutLimitBytes: 1 << 20}
	if _, err := execution.Runner().Run(execution.Context(), command); err != nil || len(fixture.runner.calls) != 4 {
		t.Fatal("fresh final command did not retain original inputs", err)
	}
	for _, actual := range fixture.runner.environments {
		if !slices.Equal(actual, wanted) {
			t.Fatal("fresh metadata or final command changed the effective environment")
		}
	}
	if err := execution.Revalidate(); err != nil {
		t.Fatal(err)
	}
	if err := execution.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectSessionFreshLinuxStartupRefusesBeforeFirstChild(t *testing.T) {
	const pruning = "hermes passive startup could activate scratch pruning"
	const ambiguous = "hermes passive environment is ambiguous"
	const recovery = "hermes passive startup has a different recovery authority"
	for _, test := range []struct {
		name      string
		selectors []string
		want      string
	}{
		{"blank", []string{"TMPDIR="}, pruning},
		{"all-blank", []string{"TMPDIR=", "TMP=", "TEMP="}, pruning},
		{"python-strip", []string{"TMPDIR=\x1c\x1f\u0085\u3000"}, pruning},
		{"all-marker", []string{"TMPDIR=/scratch", "TMP=/scratch", "TEMP=/scratch", "HERMES_SCRATCH_DIR=/scratch"}, pruning},
		{"duplicate", []string{"TMPDIR=/one", "TMPDIR=/two"}, ambiguous},
		{"nul", []string{"TMPDIR=/one\x00tail"}, ambiguous},
		{"install-recovery", []string{"TEMP=/distinct", "HERMES_INSTALL_ROOT=/foreign"}, recovery},
		{"gateway-recovery", []string{"TEMP=/distinct", "HERMES_GATEWAY_LOCK_DIR=/foreign"}, recovery},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newFreshSessionStartupFixture(t)
			safe := fixture.env
			safe.SessionEnvironment = append(slices.Clone(safe.SessionEnvironment), "TEMP="+t.TempDir())
			plan, err := New().PlanSession(t.Context(), safe, fixture.req)
			if err != nil || len(fixture.runner.calls) != 0 {
				t.Fatal("original safe fresh plan failed", err)
			}
			fixture.runner.plan = plan
			hostile := fixture.env
			hostile.SessionEnvironment = append(slices.Clone(hostile.SessionEnvironment), test.selectors...)
			original := slices.Clone(hostile.SessionEnvironment)
			_, err = New().PlanSession(t.Context(), hostile, fixture.req)
			fixture.assertRefusal(t, "plan", test.want, err, hostile, original)
			err = New().VerifySession(t.Context(), hostile, plan)
			fixture.assertRefusal(t, "verify", test.want, err, hostile, original)
			execution, err := New().BeginSessionExecution(t.Context(), hostile, plan)
			if execution != nil {
				closeErr := execution.Close()
				t.Fatal("hostile fresh base retained execution authority", err, closeErr)
			}
			fixture.assertRefusal(t, "begin", test.want, err, hostile, original)
		})
	}
}

func (fixture freshSessionStartupFixture) assertRefusal(t *testing.T, phase, wanted string, err error, env clients.Env, original []string) {
	t.Helper()
	if err == nil || err.Error() != wanted || len(fixture.runner.calls) != 0 {
		t.Fatalf("%s did not refuse the exact startup cause before native: %v (calls=%d)", phase, err, len(fixture.runner.calls))
	}
	if !slices.Equal(env.SessionEnvironment, original) {
		t.Fatalf("%s changed original supplied selectors", phase)
	}
	fixture.assertOriginal(t)
}
