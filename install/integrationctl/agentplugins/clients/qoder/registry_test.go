package qoder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// fixtureListingRunner changes only the executable/fixture environment. The
// adapter's context and stdout limit reach the real OS process transport intact.
// No client executable, user profile, shell, or native registry is touched.
type fixtureListingRunner struct {
	executable, root, mode string
	result                 legacyports.CommandResult
	calls                  int
}

func (runner *fixtureListingRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return runner.run(ctx, command, 0)
}

func (runner *fixtureListingRunner) RunWithTreeExitGrace(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	return runner.run(ctx, command, grace)
}

func (runner *fixtureListingRunner) run(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	runner.calls++
	if len(command.Argv) != 6 || command.Argv[0] != runner.executable || command.Argv[1] != "--config-dir" || !reflect.DeepEqual(command.Argv[3:], []string{"plugins", "list", "--json"}) {
		return legacyports.CommandResult{}, fmt.Errorf("fixture refuses non-listing command: %v", command.Argv)
	}
	command.Argv = []string{runner.executable, "-test.run=^TestQoderListingProcessFixture$", "--", runner.mode, filepath.Join(runner.root, "completed")}
	command.Dir = runner.root
	command.Env = []string{"HOME=" + runner.root, "USERPROFILE=" + runner.root, "TMPDIR=" + runner.root, "TMP=" + runner.root, "TEMP=" + runner.root}
	var result legacyports.CommandResult
	var err error
	if grace > 0 {
		result, err = (process.OS{}).RunWithTreeExitGrace(ctx, command, grace)
	} else {
		result, err = (process.OS{}).Run(ctx, command)
	}
	runner.result = result
	return result, err
}

func newFixtureListingRunner(t *testing.T, mode string) *fixtureListingRunner {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &fixtureListingRunner{executable: executable, root: root, mode: mode}
}

func TestQoderListingProcessFixture(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	if len(args) != 2 {
		os.Exit(81)
	}
	switch args[0] {
	case "overflow":
		// A complete valid empty listing when unbounded. A truncated listing
		// must remain a transport failure, never absence/collision evidence.
		chunk := []byte(strings.Repeat(" ", 4096))
		for range 512 {
			if _, err := os.Stdout.Write(chunk); err != nil {
				os.Exit(82)
			}
		}
		if _, err := fmt.Fprint(os.Stdout, "[]"); err != nil {
			os.Exit(82)
		}
	case "stall":
		if _, err := fmt.Fprint(os.Stdout, "[]"); err != nil {
			os.Exit(82)
		}
		time.Sleep(qoderListTimeout + 2*time.Second)
	case "valid":
		if _, err := fmt.Fprint(os.Stdout, `[{"id":"demo@local","version":"2.0.0"}]`); err != nil {
			os.Exit(82)
		}
	default:
		os.Exit(83)
	}
	if err := os.WriteFile(args[1], []byte("completed"), 0o600); err != nil {
		os.Exit(84)
	}
	os.Exit(0)
}

func TestQoderListingOSRunnerPreservesRecognizedOutput(t *testing.T) {
	runner := newFixtureListingRunner(t, "valid")
	entries, err := listPlugins(context.Background(), clients.Env{Runner: runner}, runner.root, runner.executable)
	if err != nil || len(entries) != 1 || entries[0].ID != "demo@local" || entries[0].Version != "2.0.0" {
		t.Fatalf("native fixture listing = %+v, %v", entries, err)
	}
}

func TestQoderListingOverflowCannotProveRegistryActivationOrRemoval(t *testing.T) {
	for _, operation := range []string{"registry", "activate", "remove"} {
		t.Run(operation, func(t *testing.T) {
			runner := newFixtureListingRunner(t, "overflow")
			env := clients.Env{Runner: runner}
			r := qoderRequest(t)
			r.BackendExecutable = runner.executable
			r.Plan.NativeRegistryExecutable = runner.executable
			var err error
			switch operation {
			case "registry":
				var finding clients.RegistryFinding
				finding, err = New().InspectNativeRegistry(context.Background(), env, r.Client, r.Plan, nil)
				if finding != clients.RegistryIndeterminate {
					t.Fatalf("overflow registry finding = %v, want indeterminate", finding)
				}
			case "activate":
				// Even explicit user attestation cannot replace a transport failure.
				r.VerifyOnly, r.ActivationComplete = true, true
				var outcome domain.ActivationOutcome
				outcome, err = New().Activate(context.Background(), env, r)
				if outcome.Activation != domain.ActivationFailed || outcome.Verification != domain.VerificationFailed {
					t.Fatalf("overflow activation = %+v, want failed", outcome)
				}
			case "remove":
				var outcome domain.DeactivationOutcome
				outcome, err = New().Deactivate(context.Background(), env, domain.DeactivationRequest{
					Client: r.Client, DeclaredName: r.DeclaredName, BackendExecutable: runner.executable,
					ManagedArtifactPath: r.Delivery.ActivePath, Confirmed: true,
				})
				if outcome.ExternalRemovalComplete {
					t.Fatal("overflow listing proved external removal")
				}
			}
			if !errors.Is(err, process.ErrStdoutLimitExceeded) {
				t.Fatalf("overflow error = %v, want stdout limit failure", err)
			}
			if len(runner.result.Stdout) > 1<<20 || runner.calls != 1 {
				t.Fatalf("overflow retained %d bytes, made %d commands", len(runner.result.Stdout), runner.calls)
			}
		})
	}
}

func TestQoderListingLocalDeadlineStopsOSProcess(t *testing.T) {
	runner := newFixtureListingRunner(t, "stall")
	// This background context models the unbounded lifecycle session. The
	// adapter must supply its own deadline rather than waiting for session exit.
	entries, err := listPlugins(context.Background(), clients.Env{Runner: runner}, runner.root, runner.executable)
	if !errors.Is(err, context.DeadlineExceeded) || entries != nil {
		body, _ := json.Marshal(entries)
		t.Fatalf("stalled listing = %s, %v; want deadline failure", body, err)
	}
	if _, err := os.Stat(filepath.Join(runner.root, "completed")); !os.IsNotExist(err) {
		t.Fatalf("stalled producer completed after deadline: %v", err)
	}
}

// Genuine unchanged bytes from run40135008690/source2b0ef181, the preserved
// B-installed-project-A-list stream. Global inventory has repeated ids with
// different scope/projectPath/version; legacy user mutation must not collapse it.
func TestQoderGenuineScopedGlobalInventoryCannotBecomeUniqueUserOwnership(t *testing.T) {
	body, err := os.ReadFile("testdata/qoder-1.1.78-scoped-global-list.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil || len(rows) != 3 {
		t.Fatalf("raw fixture: %v %v", rows, err)
	}
	if rows[0]["scope"] != "user" || rows[1]["scope"] != "project" || rows[2]["scope"] != "project" || rows[1]["projectPath"] == rows[2]["projectPath"] || rows[0]["id"] != rows[2]["id"] {
		t.Fatal("genuine inventory fixture lost global scoped shape")
	}
	if _, err := parseList(body); !errors.Is(err, errUnknownList) {
		t.Fatalf("global repeated ids became one owned user entry: %v", err)
	}
}
