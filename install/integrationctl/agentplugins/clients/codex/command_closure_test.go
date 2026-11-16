package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type commandClosureRunner struct {
	closedFail string
	fail       string
	cause      error
	// raw reports the failure like processadapter.OS does when its own
	// cleanup could not be proved: a plain error with CleanupComplete false.
	raw   bool
	calls []string
}

func (r *commandClosureRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	args := command.Argv[1:]
	key := strings.Join(args[:2], " ")
	if args[1] == "marketplace" {
		key = strings.Join(args[:3], " ")
	}
	r.calls = append(r.calls, key)
	if key == r.closedFail {
		return legacyports.CommandResult{CleanupComplete: true}, errors.New("closed command failure")
	}
	if key == r.fail {
		return legacyports.CommandResult{Stdout: []byte("not configured or installed"), CleanupComplete: !r.raw}, r.cause
	}
	return legacyports.CommandResult{CleanupComplete: true}, nil
}

func TestCodexUnclosedCommandStopsActivationFollowups(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, fail        string
		closedFail        string
		replacing, verify bool
		want              []string
	}{
		{name: "registration", fail: "plugin marketplace add", want: []string{"plugin marketplace add"}},
		{name: "update no fallback", fail: "plugin marketplace update", replacing: true, want: []string{"plugin marketplace update"}},
		{name: "plugin add no rollback", fail: "plugin add", want: []string{"plugin marketplace add", "plugin add"}},
		{name: "rollback closure", fail: "plugin marketplace remove", closedFail: "plugin add", want: []string{"plugin marketplace add", "plugin add", "plugin marketplace remove"}},
		{name: "fallback closure", fail: "plugin marketplace add", closedFail: "plugin marketplace update", replacing: true, want: []string{"plugin marketplace update", "plugin marketplace add"}},
		{name: "listing", fail: "plugin list", verify: true, want: []string{"plugin list"}},
	} {
		for _, report := range closureReports() {
			t.Run(tc.name+"/"+report.name, func(t *testing.T) {
				cause := errors.New("owned process closure unproved")
				runner := &commandClosureRunner{fail: tc.fail, closedFail: tc.closedFail, cause: report.cause(cause), raw: report.raw}
				profile := tempRoot(t)
				outcome, err := New().Activate(context.Background(), clients.Env{Runner: runner}, domain.ActivationRequest{
					Client:       domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: profile},
					Plan:         domain.DeliveryPlan{ClientID: domain.ClientCodex, PhysicalArtifactID: "artifact", NativeRegistryRoot: profile},
					Delivery:     domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: tempRoot(t)},
					DeclaredName: "demo", BackendExecutable: executable, Replacing: tc.replacing, VerifyOnly: tc.verify,
				})
				if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) || outcome.NativeEffect != domain.NativeEffectUncertain {
					t.Fatalf("global-stop closure evidence lost: outcome=%+v err=%v", outcome, err)
				}
				if !reflect.DeepEqual(runner.calls, tc.want) {
					t.Fatalf("followup after unclosed command: got %v want %v", runner.calls, tc.want)
				}
			})
		}
	}
}

type closureReport struct {
	name  string
	raw   bool
	cause func(error) error
}

// closureReports covers both ways a runner reports unproven process closure:
// the typed duplex error, and the ordinary OS runner's plain error whose
// result leaves CleanupComplete false (for example after a failed group kill).
func closureReports() []closureReport {
	return []closureReport{
		{name: "typed", cause: func(cause error) error { return &legacyports.ProcessClosureError{Cause: cause} }},
		{name: "raw runner result", raw: true, cause: func(cause error) error {
			return fmt.Errorf("terminate supervised process: %w", cause)
		}},
	}
}

func TestCodexRemovalNeverTreatsUnclosedCommandAsAlreadyAbsent(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"plugin remove", "plugin marketplace remove", "plugin list"} {
		for _, report := range closureReports() {
			t.Run(target+"/"+report.name, func(t *testing.T) {
				profile, artifact := tempRoot(t), tempRoot(t)
				marketplace := shared.ManagedMarketplaceName("artifact")
				config := fmt.Sprintf("[marketplaces.%s]\nsource = %q\nsource_type = \"local\"\n", marketplace, artifact)
				if err := os.WriteFile(filepath.Join(profile, "config.toml"), []byte(config), 0600); err != nil {
					t.Fatal(err)
				}
				cause := errors.New("removal process closure unproved")
				runner := &commandClosureRunner{fail: target, cause: report.cause(cause), raw: report.raw}
				outcome, err := New().Deactivate(context.Background(), clients.Env{Runner: runner}, domain.DeactivationRequest{
					Client:       domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: profile},
					DeclaredName: "demo", PhysicalArtifactID: "artifact", ManagedArtifactPath: artifact, BackendExecutable: executable, Confirmed: true,
				})
				if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) || outcome.NativeEffect != domain.NativeEffectUncertain || outcome.ArtifactRemovalAllowed || outcome.ExternalRemovalComplete {
					t.Fatalf("unsafe removal outcome=%+v err=%v", outcome, err)
				}
				want := []string{"plugin remove"}
				if target != "plugin remove" {
					want = append(want, "plugin marketplace remove")
				}
				if target == "plugin list" {
					want = append(want, "plugin list")
				}
				if !reflect.DeepEqual(runner.calls, want) {
					t.Fatalf("followup after unclosed removal: got %v want %v", runner.calls, want)
				}
				if _, err := os.Stat(artifact); err != nil {
					t.Fatalf("managed artifact lost: %v", err)
				}
			})
		}
	}
}

func TestCodexClosedAlreadyAbsentRemovalRemainsIdempotent(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"plugin remove", "plugin marketplace remove"} {
		runner := &commandClosureRunner{fail: target, cause: errors.New("closed absent command")}
		env := clients.Env{Runner: runner}
		var removeErr error
		if target == "plugin remove" {
			removeErr = removeCodexPlugin(context.Background(), env, tempRoot(t), executable, "demo@marketplace")
		} else {
			removeErr = removeCodexMarketplace(context.Background(), env, tempRoot(t), executable, "marketplace")
		}
		if removeErr != nil {
			t.Fatalf("closed already-absent %s: %v", target, removeErr)
		}
	}
}
