package agentpluginscli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type dependencyClientSpec struct {
	id                domain.ClientID
	executable        bool
	activation        string
	externalUninstall bool
}

// Expectations are independent of the capability implementation. Native client
// responses are fixtures; this proves UAP lifecycle contracts, not live clients.
var dependencyClientSpecs = []dependencyClientSpec{
	{domain.ClientCodex, true, "active", true}, {domain.ClientClaude, true, "active", false},
	{domain.ClientCursor, true, "manual_activation_required", false},
	{domain.ClientCopilot, true, "active", false},
	{domain.ClientKiro, true, "active", false}, {domain.ClientGrok, true, "active", false},
	{domain.ClientOpenCode, true, "active", false}, {domain.ClientGemini, false, "active", false},
	{domain.ClientCline, false, "active", false}, {domain.ClientWindsurf, false, "active", false}, {domain.ClientKimi, false, "active", false},
}

func newDependencyHarness(t *testing.T, repo string, clients []domain.DetectedClient, cfg installer.Config) (cliFixture, *dependencyTestSource) {
	t.Helper()
	f := newCLIFixture(t, clients)
	root := filepath.Join(f.root, "data")
	// The native processes and helper are explicit fixtures. These tests verify
	// real helper digest/copy/metadata contracts but never launch the helper or
	// an MCP process; packaged helper execution has separate qualification tests.
	helper := filepath.Join(t.TempDir(), managedstdio.ExecutableName)
	if err := os.WriteFile(helper, []byte("UAP trusted dependency-harness helper fixture; never execute\n"), 0700); err != nil {
		t.Fatal(err)
	}
	var err error
	cfg.StateRoot, cfg.StateFile, cfg.LockFile = root, f.store.Path, filepath.Join(root, "mutation.lock")
	cfg.OperationsDir, cfg.PluginDataBase, cfg.ManagedRoot = f.operations, filepath.Join(root, "plugin-data"), f.app.ManagedRoot
	cfg.TempRoot, cfg.Registry, cfg.HelperExecutable = filepath.Join(root, "installer-tmp"), f.app.ClientRegistry, helper
	f.app.Installer, err = installer.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	f.app.DependenciesRoot, f.app.EnvironmentLockPath, f.app.EnvironmentStoreRoot = root, filepath.Join(root, "environment.lock"), filepath.Join(root, "store")
	f.app.SecurityEvaluator = dependencyTestScanner{}
	source := &dependencyTestSource{Acquirer: sourceacquisition.Acquirer{TempRoot: t.TempDir(), URLForRepo: func(string) string { return repo }}}
	source.Runner = &dependencyGitDiagnosticRunner{delegate: sourceacquisition.OSRunner{}, report: func(phase, class string) {
		t.Logf("dependency Git diagnostic phase=%s class=%s", phase, class)
	}}
	f.app.SourceAcquirer = source
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	return f, source
}

func dependencyGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	command := exec.CommandContext(context.Background(), "git", append([]string{"-c", "commit.gpgsign=false"}, args...)...)
	command.Dir = repo
	command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=iliya", "GIT_AUTHOR_EMAIL=iliyazelenkog@gmail.com", "GIT_COMMITTER_NAME=iliya", "GIT_COMMITTER_EMAIL=iliyazelenkog@gmail.com")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}
func dependencyRepo(t *testing.T) string {
	t.Helper()
	repo := writeCLIPlugin(t)
	// Every client supports stdio. The explicit data placeholder exercises the
	// real PLUGIN_DATA allocation contract without starting the configured server.
	dependencyWrite(t, filepath.Join(repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	if err := os.MkdirAll(filepath.Join(repo, "skills", "demo"), 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(repo, "skills", "demo", "SKILL.md"), "---\nname: demo\ndescription: Fixture skill\n---\nFixture instructions.\n")
	dependencyGit(t, repo, "init", "-b", "main")
	dependencyGit(t, repo, "add", ".")
	dependencyGit(t, repo, "commit", "-m", "test: disposable all-client dependency")
	return repo
}
func dependencyWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func dependencyRun(t *testing.T, f cliFixture, args ...string) string {
	t.Helper()
	out, stderr, err := f.execute(false, args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s\n%s", args, err, out, stderr)
	}
	return out
}
func dependencyRead(t *testing.T, path string) string {
	t.Helper()
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func dependencySpecHarness(t *testing.T, repo string, spec dependencyClientSpec) (cliFixture, *dependencyTestSource, *dependencyFixtureRunner) {
	t.Helper()
	client := fixtureClient(t, spec.id)
	if spec.executable {
		client.ExecutablePath = filepath.Join(t.TempDir(), string(spec.id))
		dependencyWrite(t, client.ExecutablePath, "fixture executable, never launched")
	}
	runner := &dependencyFixtureRunner{id: spec.id, claude: dependencyClaudeListing{root: client.ConfigRoot}}
	cfg := installer.Config{Runner: runner, EnableNativeObserver: true, OpenCodeProbe: func(_ context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		runner.probes++
		return clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: "1.18.34", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "fixture:" + target.Executable}}, nil
	}}
	clients := []domain.DetectedClient{client}
	f, source := newDependencyHarness(t, repo, clients, cfg)
	return f, source, runner
}
func dependencyBinding(t *testing.T, f cliFixture) installer.InspectedBinding {
	t.Helper()
	view, err := f.app.Installer.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Recovery.Required || len(view.EnvironmentReceipts) != 1 || len(view.Installations) != 1 || len(view.Installations[0].Bindings) != 1 {
		t.Fatalf("owned binding: %+v", view)
	}
	binding := view.Installations[0].Bindings[0]
	if binding.Owner == nil || binding.Materialization != "materialized" {
		t.Fatalf("missing owner/materialization: %+v", binding)
	}
	if _, err := os.Stat(binding.TargetPath); err != nil {
		t.Fatal(err)
	}
	return binding
}

func TestDependencyAllClientsLifecycle(t *testing.T) {
	for _, spec := range dependencyClientSpecs {
		t.Run(string(spec.id), func(t *testing.T) {
			// Each case owns its Git repository, profiles, runner, store and data roots.
			// Parallelism preserves real isolated Git transport without multiplying its
			// process-containment startup latency across a serial eleven-client matrix.
			t.Parallel()
			repo := dependencyRepo(t)
			f, source, runner := dependencySpecHarness(t, repo, spec)
			dependencyRun(t, f, "deps", "init", "--format", "json")
			mp, lp := f.app.dependencyPaths()
			manifest := fmt.Sprintf("schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\n%s = [\"demo\"]\n", spec.id)
			// First install goes through save-default add for every real adapter.
			headCommand := exec.CommandContext(context.Background(), "git", "rev-parse", "HEAD")
			headCommand.Dir = repo
			head, err := headCommand.Output()
			if err != nil {
				t.Fatal(err)
			}
			sourceID := githubIdentity("acme/demo", strings.TrimSpace(string(head)), "")
			if spec.id == domain.ClientKiro && runtime.GOOS == "windows" {
				dependencyKiroPlatformRefusal(t, f, runner, sourceID)
				return
			}
			out := dependencyRun(t, f, "add", sourceID, "--target", string(spec.id), "--format", "json")
			var saved struct {
				Data struct {
					Environment struct {
						Saved bool `json:"saved"`
					} `json:"environment"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(out), &saved); err != nil {
				t.Fatal(err)
			}
			if !saved.Data.Environment.Saved {
				t.Fatal("add did not save dependency: " + out)
			}
			// Continue the same installation with a moving ref for targeted updates.
			dependencyWrite(t, mp, manifest)
			dependencyRun(t, f, "deps", "lock", "--format", "json")
			dependencyRun(t, f, "deps", "sync", "--locked", "--yes", "--format", "json")
			binding := dependencyBinding(t, f)
			if binding.Activation != spec.activation {
				t.Fatalf("activation: got %s want %s", binding.Activation, spec.activation)
			}
			firstDigest := binding.TreeDigest
			// A repeat may observe native state, but must not redownload, mutate a native
			// registry, reopen ACP, or change any manifest/lock/state bytes.
			beforeState, beforeLock, beforeManifest := dependencyRead(t, f.store.Path), dependencyRead(t, lp), dependencyRead(t, mp)
			downloads, lookups, mutations, duplex, probes := source.downloads, source.lookups, runner.mutations, runner.duplex, runner.probes
			dependencyRun(t, f, "deps", "sync", "--locked", "--yes", "--format", "json")
			if source.downloads != downloads || source.lookups != lookups || runner.mutations != mutations || runner.duplex != duplex || runner.probes != probes {
				t.Fatalf("repeat performed acquisition/lifecycle work: source=%+v runner=%+v", source, runner)
			}
			if beforeState != dependencyRead(t, f.store.Path) || beforeLock != dependencyRead(t, lp) || beforeManifest != dependencyRead(t, mp) {
				t.Fatal("repeat changed state or environment bytes")
			}
			status := dependencyRun(t, f, "deps", "status", "--format", "json")
			if strings.Contains(status, "mismatch") || strings.Contains(status, "not_synced") {
				t.Fatal(status)
			}
			if spec.activation == "manual_activation_required" {
				if !strings.Contains(status, "activation_manual") || !strings.Contains(status, "attest_activation") {
					t.Fatal("manual readiness hidden: " + status)
				}
				// The composed provider accepts explicit activation attestation before
				// dispatching to the adapter. This is not native runtime observation.
				dependencyRun(t, f, "add", githubIdentity("acme/demo", binding.Revision, ""), "--target", string(spec.id), "--activation-complete", "--format", "json")
				attested := dependencyBinding(t, f)
				if attested.Activation != "active" || attested.Verification != "installation_verified" {
					t.Fatalf("attestation failed: %+v", attested)
				}
				retry, facts := environment.LifecycleFacts(attested)
				if retry {
					t.Fatalf("attested activation retried: %+v %v", attested, facts)
				}
				if attested.Authentication == string(domain.AuthenticationNotChecked) && (len(facts) != 1 || facts[0] != "auth_unverified") {
					t.Fatalf("authentication review fact lost: %+v %v", attested, facts)
				}
				dependencyRun(t, f, "deps", "sync", "--locked", "--yes", "--format", "json")
			}
			if !filepath.IsAbs(binding.DataRoot) {
				t.Fatalf("package did not allocate PLUGIN_DATA: %+v", binding)
			}
			relative, err := filepath.Rel(f.root, binding.DataRoot)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				t.Fatalf("PLUGIN_DATA escaped disposable fixture: %s", binding.DataRoot)
			}
			marker := filepath.Join(binding.DataRoot, "retained.txt")
			dependencyWrite(t, marker, "keep data")
			dependencyWrite(t, filepath.Join(repo, "revision.txt"), "next revision")
			dependencyGit(t, repo, "add", ".")
			dependencyGit(t, repo, "commit", "-m", "test: advance disposable dependency")
			beforeLookups := source.lookups
			dependencyRun(t, f, "deps", "update", "demo", "--format", "json")
			dependencyRun(t, f, "deps", "sync", "--locked", "--yes", "--format", "json")
			binding = dependencyBinding(t, f)
			if binding.TreeDigest == firstDigest || source.lookups != beforeLookups+1 {
				t.Fatal("targeted update did not advance exactly one pin")
			}
			if dependencyRead(t, marker) != "keep data" {
				t.Fatal("update lost PLUGIN_DATA")
			}
			drift := filepath.Join(binding.TargetPath, "user-edit.txt")
			dependencyWrite(t, drift, "preserve")
			beforeState = dependencyRead(t, f.store.Path)
			if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil {
				t.Fatal("sync overwrote drift")
			}
			if dependencyRead(t, drift) != "preserve" || beforeState != dependencyRead(t, f.store.Path) {
				t.Fatal("drift refusal mutated binding")
			}
			state, err := f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			id := state.EnvironmentReceipts[0].InstallationID
			dependencyRun(t, f, "repair", id, "--target", string(spec.id), "--format", "json")
			repaired := dependencyBinding(t, f)
			if repaired.TreeDigest != binding.TreeDigest || repaired.ProjectionStatus != "" {
				t.Fatalf("repair changed pin or retained drift: %+v", repaired)
			}
			if dependencyRead(t, marker) != "keep data" {
				t.Fatal("repair lost PLUGIN_DATA")
			}
			// Transfer only the portable agreement, then use real immutable Git fetch.
			f2, _, _ := dependencySpecHarness(t, repo, spec)
			mp2, lp2 := f2.app.dependencyPaths()
			dependencyWrite(t, mp2, dependencyRead(t, mp))
			dependencyWrite(t, lp2, dependencyRead(t, lp))
			dependencyRun(t, f2, "deps", "sync", "--locked", "--yes", "--format", "json")
			if transferred := dependencyBinding(t, f2); transferred.TreeDigest != binding.TreeDigest {
				t.Fatal("transfer changed package digest")
			}
			if status := dependencyRun(t, f2, "deps", "status", "--format", "json"); strings.Contains(status, "mismatch") || strings.Contains(status, "not_synced") {
				t.Fatal(status)
			}
			dependencyWrite(t, mp, "schema_version = 1\n[targets]\n")
			dependencyRun(t, f, "deps", "lock", "--format", "json")
			if spec.externalUninstall {
				if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil {
					t.Fatal("missing external uninstall accepted")
				}
				_ = dependencyBinding(t, f)
				dependencyRun(t, f, "deps", "sync", "--locked", "--yes", "--external-uninstalled", "--format", "json")
			} else {
				dependencyRun(t, f, "deps", "sync", "--locked", "--yes", "--format", "json")
			}
			view, err := f.app.Installer.Inspect(context.Background())
			if err != nil || len(view.EnvironmentReceipts) != 0 {
				t.Fatalf("removed owner: %+v %v", view, err)
			}
			if _, err := os.Stat(binding.TargetPath); !os.IsNotExist(err) {
				t.Fatalf("removed projection remains: %v", err)
			}
			if dependencyRead(t, marker) != "keep data" {
				t.Fatal("remove lost data")
			}
			if _, err := f.app.dependencyStore().Get(context.Background(), domain.TreeDigestAlgorithm, binding.TreeDigest); err != nil {
				t.Fatalf("remove lost store: %v", err)
			}
		})
	}
}

// Windows deliberately cannot authorize automatic Kiro ACP containment or
// settlement. Exercise the real preflight refusal, before any installed effects,
// instead of advertising a successful fixture capability on that platform.
func dependencyKiroPlatformRefusal(t *testing.T, f cliFixture, runner *dependencyFixtureRunner, source string) {
	t.Helper()
	mp, lp := f.app.dependencyPaths()
	manifest := dependencyRead(t, mp)
	out, stderr, err := f.execute(false, "add", source, "--target", "kiro", "--format", "json")
	if err == nil || !strings.Contains(out+stderr+err.Error(), "containment preflight failed") {
		t.Fatalf("Kiro platform preflight did not refuse honestly: %v\n%s\n%s", err, out, stderr)
	}
	if runner.duplex != 0 || runner.mutations != 0 {
		t.Fatalf("unsupported Kiro platform performed native work: %+v", runner)
	}
	if dependencyRead(t, mp) != manifest {
		t.Fatal("Kiro platform refusal changed manifest")
	}
	for _, path := range []string{lp, f.store.Path, f.app.ManagedRoot, runner.claude.root} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("Kiro platform refusal wrote installed effect %s: %v", path, err)
		}
	}
}

type dependencyFixtureRunner struct {
	id                        domain.ClientID
	claude                    dependencyClaudeListing
	copilot                   liveCopilotRunner
	grokPath                  string
	mutations, duplex, probes int
}

func (r *dependencyFixtureRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	args := command.Argv[1:]
	if len(args) >= 2 && args[0] == "plugin" && args[1] != "list" {
		r.mutations++
	}
	switch r.id {
	case domain.ClientCodex, domain.ClientClaude:
		return r.claude.Run(ctx, command)
	case domain.ClientCopilot:
		if len(args) >= 2 && args[0] == "plugin" && args[1] == "uninstall" {
			r.copilot.spec = ""
			r.copilot.path = ""
		}
		if len(args) >= 2 && args[0] == "plugin" && args[1] == "list" && r.copilot.spec == "" {
			return legacyports.CommandResult{Stdout: []byte("No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n")}, nil
		}
		return r.copilot.Run(ctx, command)
	case domain.ClientGrok:
		if len(args) >= 2 && args[0] == "plugin" {
			switch args[1] {
			case "install":
				r.grokPath = args[len(args)-1]
			case "uninstall":
				r.grokPath = ""
			case "list":
				entries := []map[string]string{}
				if r.grokPath != "" {
					entries = append(entries, map[string]string{"name": "demo", "status": "installed", "source": r.grokPath, "version": "1.0.0"})
				}
				body, err := json.Marshal(entries)
				return legacyports.CommandResult{Stdout: body}, err
			}
			return legacyports.CommandResult{}, nil
		}
	}
	return legacyports.CommandResult{}, fmt.Errorf("unexpected native fixture command: %v", command.Argv)
}
func (*dependencyFixtureRunner) DuplexCapability() error {
	if runtime.GOOS == "windows" {
		// This production preflight is read-only and never starts a process on
		// Windows. Unix uses the fixture's real deadline-capable pipe exchange.
		return (process.OS{}).DuplexCapability()
	}
	return nil
}
func (r *dependencyFixtureRunner) RunDuplexWithPlannedShutdown(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	r.duplex++
	// Keep a real pipe writer alive through Kiro's post-success settlement.
	// io.Pipe has no read deadlines and would hang the observer indefinitely.
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = reader.Close(); _ = writer.Close() })
	defer stop()
	if err := reader.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	output := `{"jsonrpc":"2.0","id":0,"result":{"protocolVersion":1}}` + "\n" +
		`{"jsonrpc":"2.0","id":1,"result":{"sessionId":"s"}}` + "\n" +
		`{"jsonrpc":"2.0","method":"_kiro/mcp/status","params":{"sessionId":"s","serverName":"demo","status":"connected","tools":[{"name":"tool","disabled":false}]}}` + "\n"
	if _, err := io.WriteString(writer, output); err != nil {
		return err
	}
	err = exchange(cliDiscardWriteCloser{Writer: io.Discard}, reader)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
