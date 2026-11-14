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

// tempRoot returns a symlink-free temp directory: native root checks demand
// canonical paths, which t.TempDir() does not guarantee on macOS (/var vs
// /private/var).
func tempRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
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
			root, ambient := tempRoot(t), tempRoot(t)
			t.Setenv("CODEX_HOME", ambient)
			t.Setenv("NODE_OPTIONS", "--require=/untrusted")
			runner := &profileRunner{fail: scenario.fail}
			env := clients.Env{Runner: runner}
			client := domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: root}
			plan := domain.DeliveryPlan{ClientID: domain.ClientCodex, PhysicalArtifactID: "artifact", DeclaredName: "demo", NativeRegistryRoot: root, NativeRegistryExecutable: executable}
			artifact := tempRoot(t)
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
				finding, err := New().InspectNativeRegistry(context.Background(), env, client, plan, &domain.ClientBinding{NativeProfileRoot: root})
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

func TestFreshCodexProfileIsReadOnlyUntilActivation(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(tempRoot(t), ".codex")
	runner := &profileRunner{}
	env := clients.Env{Runner: runner}
	client := domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: root}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCodex, PhysicalArtifactID: "artifact", DeclaredName: "demo", NativeRegistryRoot: root, NativeRegistryExecutable: executable}
	finding, err := New().InspectNativeRegistry(context.Background(), env, client, plan, nil)
	if err != nil || finding != clients.RegistryClear || len(runner.commands) != 0 {
		t.Fatalf("fresh profile preflight = %v, %v; commands = %+v", finding, err, runner.commands)
	}

	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("preflight created profile: %v", err)
	}
	outcome, err := New().Activate(context.Background(), env, domain.ActivationRequest{
		Client: client, Plan: plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: tempRoot(t)},
		DeclaredName: "demo", BackendExecutable: executable,
	})
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		t.Fatalf("fresh profile activation = %+v, %v", outcome, err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("profile was not created: %v", err)
	}
}

func TestExistingCodexProfileRejectsUnsupportedCLIWithoutMutation(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(tempRoot(t), ".codex")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	runner := &profileRunner{fail: "plugin list"}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCodex, PhysicalArtifactID: "artifact", DeclaredName: "demo", NativeRegistryRoot: root, NativeRegistryExecutable: executable}
	finding, err := New().InspectNativeRegistry(context.Background(), clients.Env{Runner: runner}, domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: root}, plan, nil)
	if err == nil || finding != clients.RegistryIndeterminate || len(runner.commands) != 1 {
		t.Fatalf("unsupported CLI preflight = %v, %v; commands = %+v", finding, err, runner.commands)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("unsupported CLI preflight mutated selected profile: %v %v", entries, err)
	}
}

func TestEnvironmentFailsClosed(t *testing.T) {
	root := tempRoot(t)
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

func TestWindowsInstallationMetadataIsValidatedButNotInherited(t *testing.T) {
	root := tempRoot(t)
	baseline, err := codexEnvironment(root, []string{"PATH=/bin"})
	if err != nil {
		t.Fatal(err)
	}
	ambient := []string{"ProgramFiles(x86)=ignored", "CommonProgramFiles(x86)=ignored", "PATH=/bin"}
	got, err := codexEnvironment(root, ambient)
	if err != nil || !reflect.DeepEqual(got, baseline) {
		t.Fatalf("Windows metadata affected child environment: %v %v", got, err)
	}
	for _, entries := range [][]string{
		{"ProgramFiles(x86)=first", "PROGRAMFILES(X86)=second"},
		{"CommonProgramFiles(x86)=first", "commonprogramfiles(x86)=second"},
		{"ProgramFiles(x86)=bad\x00value"},
		{"ProgramFiles(x86)=later", "BROKEN(NAME)=ignored"},
	} {
		if _, err := codexEnvironment(root, entries); err == nil {
			t.Fatal("accepted malformed or duplicate ambient metadata")
		}
	}
}

func TestInvalidProfilesNeverRun(t *testing.T) {
	root := tempRoot(t)
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
	root := tempRoot(t)
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
