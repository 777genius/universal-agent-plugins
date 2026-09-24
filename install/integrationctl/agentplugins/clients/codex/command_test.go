package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type profileRunner struct {
	commands []legacyports.Command
	fail     string
	failText string
	tree     bool
}

func (r *profileRunner) Run(ctx context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	r.commands = append(r.commands, c)
	if r.fail != "" && strings.Contains(strings.Join(c.Argv[1:], " "), r.fail) {
		text := r.failText
		if text == "" {
			text = "not configured or installed"
		}
		return legacyports.CommandResult{ExitCode: 1, Stdout: []byte(text)}, errors.New("command failed")
	}
	cmd := exec.CommandContext(ctx, c.Argv[0], append([]string{"-test.run=^TestProfileProcess$", "--"}, c.Argv[1:]...)...)
	cmd.Env = c.Env
	body, err := cmd.Output()
	return legacyports.CommandResult{Stdout: body}, err
}
func (r *profileRunner) RunWithTreeExitGrace(ctx context.Context, c legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	r.tree = grace > 0
	return r.Run(ctx, c)
}

// A real child observes only the supplied environment and touches only its
// CODEX_HOME. The test executable stands in for Codex; no shell is involved.
func TestProfileProcess(t *testing.T) {
	index := -1
	for i, a := range os.Args {
		if a == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	if len(args) > 1 && args[1] == "list" {
		fmt.Printf(`{"installed":[{"pluginId":"demo@%s","name":"demo","marketplaceName":"%s","installed":true,"enabled":true}]}`, shared.ManagedMarketplaceName("artifact"), shared.ManagedMarketplaceName("artifact"))
	} else {
		if err := os.WriteFile(filepath.Join(os.Getenv("CODEX_HOME"), "mutation"), []byte(strings.Join(args, " ")), 0600); err != nil {
			os.Exit(2)
		}
	}
	os.Exit(0)
}

func TestEveryCommandUsesExplicitProfile(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name, fail                         string
		replacing, verify, remove, inspect bool
		want                               []string
	}{
		{name: "activation", want: []string{"plugin marketplace add", "plugin add", "plugin list"}},
		{name: "update", replacing: true, want: []string{"plugin marketplace update", "plugin add", "plugin list"}},
		{name: "update fallback", replacing: true, fail: "marketplace update", want: []string{"plugin marketplace update", "plugin marketplace add", "plugin add", "plugin list"}},
		{name: "rollback", fail: "plugin add", want: []string{"plugin marketplace add", "plugin add", "plugin marketplace remove"}},
		{name: "verify only", verify: true, want: []string{"plugin list"}},
		{name: "removal", remove: true, want: []string{"plugin remove", "plugin marketplace remove"}},
		{name: "registry", inspect: true, want: []string{"plugin list"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root, ambient := t.TempDir(), t.TempDir()
			t.Setenv("CODEX_HOME", ambient)
			t.Setenv("NODE_OPTIONS", "--require=/untrusted")
			runner := &profileRunner{fail: scenario.fail}
			env := clients.Env{Runner: runner}
			client := domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: root}
			plan := domain.DeliveryPlan{ClientID: domain.ClientCodex, PhysicalArtifactID: "artifact", DeclaredName: "demo", NativeRegistryRoot: root, NativeRegistryExecutable: executable}
			artifact := t.TempDir()
			marketplace := shared.ManagedMarketplaceName("artifact")
			if scenario.remove {
				config := fmt.Sprintf("[marketplaces.%s]\nsource = %q\nsource_type = \"local\"\n", marketplace, artifact)
				if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(config), 0600); err != nil {
					t.Fatal(err)
				}
				out, err := New().Deactivate(context.Background(), env, domain.DeactivationRequest{Client: client, DeclaredName: "demo", PhysicalArtifactID: "artifact", BackendExecutable: executable, ManagedArtifactPath: artifact, Confirmed: true, ExternalUninstalled: true})
				if err != nil || !out.ExternalRemovalComplete {
					t.Fatalf("%+v %v", out, err)
				}
			} else if scenario.inspect {
				finding, err := New().InspectNativeRegistry(context.Background(), env, client, plan, &domain.ClientBinding{})
				if err != nil || finding != clients.RegistryExpected || !runner.tree {
					t.Fatalf("%v %v tree=%v", finding, err, runner.tree)
				}
			} else {
				out, err := New().Activate(context.Background(), env, domain.ActivationRequest{Client: client, Plan: plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: artifact}, DeclaredName: "demo", BackendExecutable: executable, Replacing: scenario.replacing, VerifyOnly: scenario.verify})
				if scenario.name == "rollback" {
					if err == nil {
						t.Fatal("expected failure")
					}
				} else if err != nil || out.Verification != domain.VerificationInstalled {
					t.Fatalf("%+v %v", out, err)
				}
			}
			if len(runner.commands) != len(scenario.want) {
				t.Fatalf("commands: %+v", runner.commands)
			}
			for i, c := range runner.commands {
				if c.Argv[0] != executable || !strings.HasPrefix(strings.Join(c.Argv[1:], " "), scenario.want[i]+" ") {
					t.Fatalf("command: %+v", c)
				}
				homes := 0
				for _, e := range c.Env {
					if strings.HasPrefix(e, "CODEX_HOME=") {
						homes++
						if e != "CODEX_HOME="+root {
							t.Fatal(e)
						}
					}
					if strings.HasPrefix(e, "NODE_OPTIONS=") {
						t.Fatal(e)
					}
				}
				if homes != 1 {
					t.Fatalf("environment: %v", c.Env)
				}
			}
			entries, err := os.ReadDir(ambient)
			if err != nil || len(entries) != 0 {
				t.Fatalf("ambient mutated: %v %v", entries, err)
			}
			_, err = os.Stat(filepath.Join(root, "mutation"))
			if scenario.inspect || scenario.verify {
				if !os.IsNotExist(err) {
					t.Fatal("read-only command mutated profile")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestEnvironmentFailsClosed(t *testing.T) {
	root := t.TempDir()
	for _, entries := range [][]string{{"CODEX_HOME=a", "CODEX_HOME=b"}, {"PATH=a", "Path=b"}, {"IGNORED=a", "IGNORED=b"}, {"broken"}, {"=value"}, {"BAD-NAME=x"}, {"X=a\x00b"}, {strings.Repeat("X", 128*1024+1) + "=v"}} {
		if _, err := codexCommand(root, filepath.Join(root, "codex"), entries, "plugin", "list"); err == nil {
			t.Fatalf("accepted malformed environment %q", entries)
		}
	}
	a, err := codexEnvironment(root, []string{"SECRET=no", "PATH=/bin", "CODEX_HOME=/elsewhere", "HOME=/home/user"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := codexEnvironment(root, []string{"HOME=/home/user", "CODEX_HOME=/other", "PATH=/bin"})
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("%v %v %v", a, b, err)
	}
}

func TestInvalidProfilesNeverRun(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "relative", root + "/../alias", root + "/", root + "\n", root + "\x00", string(filepath.Separator), file} {
		r := &profileRunner{}
		env := clients.Env{Runner: r}
		client := domain.DetectedClient{ConfigRoot: bad}
		plan := domain.DeliveryPlan{NativeRegistryExecutable: filepath.Join(root, "codex"), NativeRegistryRoot: bad}
		finding, err := New().InspectNativeRegistry(context.Background(), env, client, plan, nil)
		if err == nil || finding != clients.RegistryIndeterminate {
			t.Fatalf("registry accepted %q", bad)
		}
		_, err = New().Activate(context.Background(), env, domain.ActivationRequest{Client: client, BackendExecutable: plan.NativeRegistryExecutable})
		if err == nil {
			t.Fatalf("activation accepted %q", bad)
		}
		_, err = New().Deactivate(context.Background(), env, domain.DeactivationRequest{Client: client, BackendExecutable: plan.NativeRegistryExecutable, PhysicalArtifactID: "artifact", Confirmed: true, ExternalUninstalled: true})
		if err == nil || len(r.commands) != 0 {
			t.Fatalf("removal accepted %q, commands %v", bad, r.commands)
		}
	}
	if err := validateProfile(root, t.TempDir()); err == nil {
		t.Fatal("accepted conflicting plan")
	}
	for _, bad := range []string{"codex", "", root + "/../codex", root + "\x00"} {
		if _, err := codexCommand(root, bad, nil); err == nil {
			t.Fatalf("accepted executable %q", bad)
		}
	}
}

func TestCommandFailuresAreNotAbsence(t *testing.T) {
	root := t.TempDir()
	r := &profileRunner{fail: "plugin", failText: "permission denied"}
	env := clients.Env{Runner: r}
	plan := domain.DeliveryPlan{NativeRegistryRoot: root, NativeRegistryExecutable: filepath.Join(root, "codex")}
	finding, err := New().InspectNativeRegistry(context.Background(), env, domain.DetectedClient{ConfigRoot: root}, plan, nil)
	if err == nil || finding != clients.RegistryIndeterminate {
		t.Fatalf("%v %v", finding, err)
	}
	request := domain.ActivationRequest{Client: domain.DetectedClient{ConfigRoot: root}, BackendExecutable: plan.NativeRegistryExecutable, VerifyOnly: true}
	out, err := New().Activate(context.Background(), env, request)
	if err == nil || out.AuthoritativeObservation || errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
		t.Fatalf("%+v %v", out, err)
	}
	if err := removeCodexPlugin(context.Background(), env, root, plan.NativeRegistryExecutable, "demo@market"); err == nil {
		t.Fatal("failure treated as absence")
	}
	if err := removeCodexMarketplace(context.Background(), env, root, plan.NativeRegistryExecutable, "market"); err == nil {
		t.Fatal("failure treated as absence")
	}
}

type removedBackupListRunner struct {
	calls     int
	failCount int
	failPath  string
}

func (r *removedBackupListRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	if r.calls <= r.failCount {
		return legacyports.CommandResult{}, &os.PathError{
			Op: "fork/exec", Path: r.failPath,
			Err: os.ErrNotExist,
		}
	}
	if got := strings.Join(command.Argv[1:], " "); got != "plugin list --json" {
		return legacyports.CommandResult{}, fmt.Errorf("unexpected command %q", got)
	}
	marketplace := shared.ManagedMarketplaceName("artifact")
	body := fmt.Sprintf(`{"installed":[{"pluginId":"demo@%s","name":"demo","marketplaceName":"%s","installed":true,"enabled":true}]}`, marketplace, marketplace)
	return legacyports.CommandResult{Stdout: []byte(body)}, nil
}

func TestVerifyCodexPluginRetriesRemovedBackupProcessPath(t *testing.T) {
	root := t.TempDir()
	runner := &removedBackupListRunner{failCount: 1, failPath: filepath.Join(root, "plugins", "cache", "agentplugins-test", "plugin-backup-test", "agent-notify", "bin", "claude-notifications")}
	request := domain.ActivationRequest{
		Client:       domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: root},
		Plan:         domain.DeliveryPlan{ClientID: domain.ClientCodex, PhysicalArtifactID: "artifact", NativeRegistryRoot: root},
		Delivery:     domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: t.TempDir()},
		DeclaredName: "demo", BackendExecutable: filepath.Join(root, "codex"), VerifyOnly: true,
	}
	outcome, err := New().Activate(context.Background(), clients.Env{Runner: runner}, request)
	if err != nil || outcome.Verification != domain.VerificationInstalled || runner.calls != 2 {
		t.Fatalf("removed-backup listing should recover on one read-only retry: outcome=%+v err=%v calls=%d", outcome, err, runner.calls)
	}
}

func TestVerifyCodexPluginBackupRetryIsBoundedAndSpecific(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		failCount  int
		wantCalls  int
	}{
		{name: "persistent removed backup", path: "plugin-backup-test/agent-notify/bin/claude-notifications", failCount: 3, wantCalls: 3},
		{name: "missing active executable", path: "agent-notify/bin/claude-notifications", failCount: 1, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			runner := &removedBackupListRunner{failCount: tc.failCount, failPath: filepath.Join(root, tc.path)}
			request := domain.ActivationRequest{
				Client:       domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: root},
				Plan:         domain.DeliveryPlan{ClientID: domain.ClientCodex, PhysicalArtifactID: "artifact", NativeRegistryRoot: root},
				Delivery:     domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: t.TempDir()},
				DeclaredName: "demo", BackendExecutable: filepath.Join(root, "codex"), VerifyOnly: true,
			}
			_, err := New().Activate(context.Background(), clients.Env{Runner: runner}, request)
			if err == nil || runner.calls != tc.wantCalls {
				t.Fatalf("retry escaped its bound or masked missing active executable: err=%v calls=%d", err, runner.calls)
			}
		})
	}
}
