//go:build linux && grokpublicnative

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type grokSignedNativeCall struct {
	Phase     string
	Argv      []string
	TreeGrace time.Duration
	Result    legacyports.CommandResult
	Error     string
}

// Only the production Activator gets this delegating witness. Detector and
// registry-preflight reads cannot satisfy the fresh repeat assertion. Embedding
// OS and forwarding its original context/grace preserve containment capabilities.
type grokSignedNativeRunner struct {
	process.OS
	executable, home, managed, phase string
	calls                            []grokSignedNativeCall
}

func (runner *grokSignedNativeRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return runner.run(ctx, command, 0, false)
}

func (runner *grokSignedNativeRunner) RunWithTreeExitGrace(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	return runner.run(ctx, command, grace, true)
}

func (runner *grokSignedNativeRunner) run(ctx context.Context, command legacyports.Command, grace time.Duration, tree bool) (legacyports.CommandResult, error) {
	call := grokSignedNativeCall{Phase: runner.phase, Argv: append([]string(nil), command.Argv...), TreeGrace: grace}
	allowed := len(command.Argv) >= 4 && command.Argv[0] == runner.executable
	if allowed {
		tail := command.Argv[1:]
		switch {
		case slices.Equal(tail, []string{"plugin", "list", "--json"}):
		case runner.phase == "install" && len(tail) == 4 && slices.Equal(tail[:3], []string{"plugin", "install", "--trust"}):
			allowed = pathpolicy.RequireContainedChild(filepath.Join(runner.home, ".grok", "plugins"), tail[3]) == nil || pathpolicy.RequireContainedChild(runner.managed, tail[3]) == nil
		case runner.phase == "remove" && slices.Equal(tail, []string{"plugin", "uninstall", "--confirm", "tool"}):
		default:
			allowed = false
		}
	}
	if !allowed || len(runner.calls) >= 12 {
		err := fmt.Errorf("unexpected %s native activation command: %q", runner.phase, command.Argv)
		call.Error = err.Error()
		runner.calls = append(runner.calls, call)
		return legacyports.CommandResult{}, err
	}
	var result legacyports.CommandResult
	var err error
	if tree {
		result, err = runner.OS.RunWithTreeExitGrace(ctx, command, grace)
	} else {
		result, err = runner.OS.Run(ctx, command)
	}
	call.Result = result
	if err != nil {
		call.Error = err.Error()
	}
	runner.calls = append(runner.calls, call)
	return result, err
}

func (runner *grokSignedNativeRunner) phaseCalls(t *testing.T, phase string, lists, mutations int) []byte {
	t.Helper()
	var last []byte
	for _, call := range runner.calls {
		if call.Phase != phase {
			continue
		}
		if call.Error != "" || call.Result.ExitCode != 0 || !call.Result.CleanupComplete || len(call.Result.Stderr) != 0 {
			t.Fatalf("genuine activation command did not close cleanly: %+v", call)
		}
		if slices.Equal(call.Argv[1:], []string{"plugin", "list", "--json"}) {
			if call.TreeGrace <= 0 {
				t.Fatal("native listing lost its existing process-tree grace")
			}
			lists--
			last = call.Result.Stdout
		} else {
			mutations--
		}
	}
	if lists != 0 || mutations != 0 || len(last) == 0 {
		t.Fatalf("%s activation command counts differ: remaining lists=%d mutations=%d", phase, lists, mutations)
	}
	return last
}

func grokSignedUserDirectory(t *testing.T, journey projectJourney, source, revision string) (*directoryv1.Client, domain.PackageEnvelope) {
	t.Helper()
	captured, err := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(t.Context(), source, domain.SourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	envelope, loadErr := journey.app.PackageLoader.Load(t.Context(), domain.LoadInput{SnapshotRoot: captured.Root, TreeDigest: captured.TreeDigest, ExecutableFiles: captured.ExecutableFiles, Source: captured.Source})
	removeErr := packagedigest.Remove(captured)
	if loadErr != nil || removeErr != nil {
		t.Fatalf("load/remove signed source snapshot: %v / %v", loadErr, removeErr)
	}
	client, err := newConformanceDirectoryClient(writeConformanceFixture(t, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := client.Embedded.Verify(client.Trust)
	if err != nil {
		t.Fatal(err)
	}
	delivery, ok := domain.ExpectedDirectoryDelivery(domain.ClientGrok)
	if !ok {
		t.Fatal("Grok Directory delivery is missing")
	}
	bundle.Snapshot.Products[0].MinimumCapabilities = domain.DirectoryMinimumCapabilities{Skills: "required", MCP: "optional"}
	distribution := &bundle.Snapshot.Distributions[0]
	distribution.Kind = domain.DistributionCommunity
	release := &distribution.Releases[0]
	release.PackageSource = domain.DirectorySource{Repository: "fixture/grok-user", Revision: revision, Path: "plugin"}
	release.TreeDigest, release.ManifestDigest, release.Components = envelope.TreeDigest, envelope.ManifestDigest, []string{"skills"}
	distribution.ReleasePolicies[0].Targets = []domain.DirectoryTarget{{Client: domain.ClientGrok, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: delivery, Authentication: domain.AuthenticationRequirementNotRequired}}
	// Test-authored catalog authority is signed and verified through the existing
	// conformance fixture. It is not a published catalog or an auth attestation.
	bundle = transferSignedDirectoryBundle(t, bundle, false, client.Trust)
	client.Embedded = directoryv1.EmbeddedBundle{Snapshot: bundle.SnapshotBytes, Envelope: bundle.EnvelopeBytes}
	client.HTTPClient = &http.Client{Transport: conformanceFixtureTransport{sequence: bundle.Snapshot.Sequence, snapshot: bundle.SnapshotBytes, envelope: bundle.EnvelopeBytes}}
	client.Now = func() time.Time { return time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC) }
	t.Logf("GROK_USER_TEST_SIGNED_CATALOG digest=%s source_revision=%s tree=%s manifest=%s", bundle.Digest, revision, envelope.TreeDigest, envelope.ManifestDigest)
	return client, envelope
}

func grokSignedUserWire(t *testing.T, journey projectJourney, client *directoryv1.Client, witness *grokSignedNativeRunner, caFile string) projectJourney {
	t.Helper()
	source := func(root string) agentpluginscli.SourceAcquirer {
		return sourceacquisition.Acquirer{TempRoot: root, HTTPClient: journey.transport.server.Client(),
			Runner: signedV1ProjectGitRunner{delegate: sourceacquisition.OSRunner{}, caFile: caFile},
			URLForRepo: func(repo string) string {
				if repo == "fixture/grok-user" {
					return journey.transport.endpoint
				}
				return "https://invalid.example.test/refuse-unknown-repository"
			}}
	}
	acquisition := projectJourneyTemp(t)
	wire := func(app agentpluginscli.App) agentpluginscli.App {
		app.Version, app.DirectoryClient = "1.18.34", client
		app.SourceAcquirer, app.SourceAcquirerForDryRun = source(acquisition), source
		app.DirectoryClientForDryRun = func(string) (agentpluginscli.DirectoryClient, error) { return client, nil }
		activator, ok := app.Lifecycle.Activator.(providers.Activator)
		if !ok {
			t.Fatal("production lifecycle activator missing")
		}
		activator.Runner = witness
		app.Lifecycle.Activator = activator
		return app
	}
	journey.app = wire(journey.app)
	parent := journey.app.ForScope
	journey.app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		app, err := parent(scope, root)
		if err != nil {
			return app, err
		}
		return wire(app), nil
	}
	return journey
}

func grokSignedUserPublic(t *testing.T, journey projectJourney, args ...string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	app := journey.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &stdout, &stderr, false
	command := agentpluginscli.NewRoot(app)
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs(append(args, "--format", "json"))
	err := command.ExecuteContext(t.Context())
	if stdout.Len()+stderr.Len() > 128<<10 {
		t.Fatal("public Grok fixture transcript exceeds its finite bound")
	}
	t.Logf("GROK_USER_SIGNED_PUBLIC args=%q stdout=%s stderr=%s", args, stdout.Bytes(), stderr.Bytes())
	if err != nil {
		t.Fatal("public Grok lifecycle failed", err)
	}
	return stdout.Bytes()
}

type grokSignedUserOutput struct {
	Scope       string            `json:"scope"`
	Readiness   string            `json:"readiness"`
	NextActions []json.RawMessage `json:"next_actions"`
	Result      usecase.AddResult `json:"result"`
}

func grokSignedUserReady(t *testing.T, body []byte, unchanged bool) usecase.AddResult {
	t.Helper()
	var report struct {
		Result string `json:"result"`
		Data   struct {
			grokSignedUserOutput
			Batch          bool   `json:"batch"`
			Status         string `json:"status"`
			Succeeded      int    `json:"succeeded"`
			Failed         int    `json:"failed"`
			ActionRequired int    `json:"action_required"`
			Targets        []struct {
				Target, Status string
				Error          json.RawMessage
				Output         grokSignedUserOutput
			} `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatal(err)
	}
	output := report.Data.grokSignedUserOutput
	if unchanged {
		if report.Data.Batch || len(report.Data.Targets) != 0 {
			t.Fatal("existing User repeat did not use the single-binding public route")
		}
	} else {
		// A first explicit-target add is a one-target batch; only the repeat
		// has a saved binding and reaches the single-target resume route.
		if !report.Data.Batch || report.Data.Status != string(usecase.GroupPhaseCompleted) || report.Data.Succeeded != 1 || report.Data.Failed != 0 || report.Data.ActionRequired != 0 || len(report.Data.Targets) != 1 {
			t.Fatal("initial public User add was not one completed target", string(body))
		}
		target := report.Data.Targets[0]
		if target.Target != "grok" || target.Status != string(usecase.GroupTargetExternalCompleted) || len(target.Error) != 0 {
			t.Fatal("initial public batch did not complete exactly Grok", target)
		}
		output = target.Output
	}
	result := output.Result
	outcome := result.Activation
	if report.Result != "success" || output.Scope != "user" || output.Readiness != "ready" || len(output.NextActions) != 0 || result.Plan.ClientID != domain.ClientGrok || result.Plan.Scope != domain.ScopeUser || result.NoChange != unchanged || result.Mutated == unchanged || result.RequiresConfirmation ||
		outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || outcome.Authentication != domain.AuthenticationNotRequired || outcome.Policy != domain.PolicyAllowed || outcome.ActivationAttested || outcome.AuthenticationAttested {
		t.Fatalf("public User result is not the expected observed Ready state: %s", body)
	}
	return result
}

type grokSignedFile struct {
	info   os.FileInfo
	digest [32]byte
}

func grokSignedUserSnapshot(t *testing.T, roots ...string) map[string]grokSignedFile {
	t.Helper()
	result := map[string]grokSignedFile{}
	var size int64
	for _, root := range roots {
		if _, err := os.Lstat(root); os.IsNotExist(err) {
			result[root] = grokSignedFile{}
			continue
		}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return fmt.Errorf("unexpected fixture file kind at %s", path)
			}
			size += info.Size()
			if len(result) >= 4096 || size > 16<<20 {
				return fmt.Errorf("fixture snapshot exceeds finite bounds")
			}
			var body []byte
			if info.Mode().IsRegular() {
				body, err = os.ReadFile(path)
				if err != nil {
					return err
				}
			}
			result[path] = grokSignedFile{info: info, digest: sha256.Sum256(body)}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func grokSignedUserUnchanged(t *testing.T, before map[string]grokSignedFile, roots ...string) {
	t.Helper()
	after := grokSignedUserSnapshot(t, roots...)
	if len(before) != len(after) {
		t.Fatal("read-only User operation changed the protected inventory")
	}
	for path, old := range before {
		current, exists := after[path]
		if !exists || (old.info == nil) != (current.info == nil) {
			t.Fatal("protected path appeared or disappeared", path)
		}
		if old.info == nil {
			continue
		}
		left, right := old.info.Sys().(*syscall.Stat_t), current.info.Sys().(*syscall.Stat_t)
		if old.digest != current.digest || !os.SameFile(old.info, current.info) || old.info.Mode() != current.info.Mode() || old.info.Size() != current.info.Size() || !old.info.ModTime().Equal(current.info.ModTime()) ||
			left.Ctim != right.Ctim || left.Uid != right.Uid || left.Gid != right.Gid || left.Nlink != right.Nlink {
			t.Fatal("read-only User operation changed protected bytes or identity", path)
		}
	}
}

func grokSignedUserBinding(t *testing.T, journey projectJourney, result usecase.AddResult, source domain.PackageEnvelope, revision string) domain.ClientBinding {
	t.Helper()
	state := journey.state(t, "")
	if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("--no-save must retain one installer binding without a portable dependency: %+v", state)
	}
	installation := state.Installations[0]
	if installation.InstallationID != result.InstallationID || installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil || installation.Directory.DistributionID != "owner/tool" || installation.Source.ResolvedRevision != revision {
		t.Fatal("real signed Directory provenance was not committed", installation)
	}
	for _, binding := range installation.Clients {
		if binding.ClientID != string(domain.ClientGrok) || binding.Scope != "user" || !binding.LifecycleConverged() || binding.Authentication != domain.AuthenticationNotRequired || binding.Policy != domain.PolicyAllowed || binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil || binding.PackageRevision == nil {
			t.Fatal("initial public installation did not earn converged User state", binding)
		}
		revisionState := binding.PackageRevision
		if revisionState.TreeDigest != source.TreeDigest || revisionState.ManifestDigest != source.ManifestDigest || revisionState.ResolvedRevision != revision || revisionState.CatalogEvidence == nil || revisionState.CatalogEvidence.Compatibility["grok"].Authentication != domain.AuthenticationRequirementNotRequired {
			t.Fatal("saved package revision lacks exact verified catalog authority", revisionState)
		}
		digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), binding.TargetLocator)
		if err != nil || len(binding.NativeObjects) != 1 || !reflect.DeepEqual(binding.NativeObjects[0], domain.NativeObjectOwnership{ObjectID: "package:grok:" + binding.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: "tool", Path: binding.TargetLocator, ManagedDigest: digest, ProtectionClass: "managed"}) {
			t.Fatal("owned full package digest/receipt differs", err, binding.NativeObjects)
		}
		return binding
	}
	t.Fatal("committed binding missing")
	return domain.ClientBinding{}
}

func grokSignedUserInventory(t *testing.T, body []byte, journey projectJourney, binding domain.ClientBinding, present bool) {
	t.Helper()
	var rows []struct{ Name, Status, Source, Version, Path, RepoKey string }
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows {
		if row.Name != "tool" {
			continue
		}
		count++
		if !present || row.Status != "installed" || row.Source != binding.TargetLocator || row.Version != "1.0.0" || pathpolicy.RequireContainedChild(filepath.Join(journey.home, ".grok", "installed-plugins"), row.Path) != nil ||
			row.RepoKey != filepath.Base(row.Path) || !strings.HasPrefix(row.RepoKey, binding.PhysicalArtifact+"-") {
			t.Fatal("genuine User registration differs from the exact owned package", row)
		}
		var expected string
		for _, root := range []string{filepath.Join(journey.transport.repo, "plugin"), binding.TargetLocator, row.Path} {
			digest, err := shared.DigestSkillDirectory(filepath.Join(root, "skills", "docs"))
			if err != nil || expected != "" && digest != expected {
				t.Fatal("source/managed/native full Skill bytes differ", err)
			}
			expected = digest
		}
	}
	if count != 0 && !present || count != 1 && present {
		t.Fatalf("native inventory selected count=%d present=%t", count, present)
	}
}

func grokSignedUserForeignConfig(t *testing.T, home string) {
	t.Helper()
	var config struct {
		Marketplace struct {
			Purged *bool `toml:"default_skills_installs_purged"`
		}
		Features struct {
			Summary *bool `toml:"turn_summary"`
		}
		Plugins struct{ Enabled, Disabled []string }
	}
	if err := toml.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(home, ".grok", "config.toml"))), &config); err != nil || config.Marketplace.Purged == nil || !*config.Marketplace.Purged || config.Features.Summary == nil || *config.Features.Summary || !slices.Contains(config.Plugins.Enabled, "foreign-enabled") || !slices.Contains(config.Plugins.Disabled, "foreign-disabled") {
		t.Fatal("native lifecycle changed foreign configuration", err)
	}
}

// A real signed-catalog CLI install earns NotRequired. Repeating that same
// public source retains its catalog evidence and must freshly list through the
// Activator without native, managed, state or foreign writes. Literal SDK
// OnlinePreview remains separate controlled coverage; it is not claimed here.
func TestGrokNativeUserSignedRepeatIsFreshAndReadOnly(t *testing.T) {
	if !filepath.IsAbs(*grokPublicNative) || filepath.Clean(*grokPublicNative) != *grokPublicNative {
		t.Fatal("original absolute pinned Grok executable is required")
	}
	binary, err := os.ReadFile(*grokPublicNative)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(binary)) != "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9" {
		t.Fatal("original pinned Grok executable differs", err)
	}
	transport := newProjectJourneyTransport(t, "git")
	caFile := os.Getenv("GIT_SSL_CAINFO")
	if caFile == "" {
		t.Fatal("disposable signed Git fixture CA is missing")
	}
	source := filepath.Join(transport.repo, "plugin")
	projectJourneyWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"tool","version":"1.0.0"}`)
	projectJourneyWrite(t, filepath.Join(source, "skills", "docs", "SKILL.md"), projectJourneySkill)
	projectJourneyGit(t, transport.repo, "add", "plugin")
	projectJourneyGit(t, transport.repo, "commit", "-m", "pure Skill signed Grok User fixture")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	projectJourneyGit(t, transport.remote, "config", "uploadpack.allowTipSHA1InWant", "true")
	commit, err := exec.CommandContext(t.Context(), "git", "-C", transport.repo, "rev-parse", "HEAD").Output()
	revision := strings.TrimSpace(string(commit))
	if err != nil || len(revision) != 40 {
		t.Fatal("signed source commit is unavailable", err)
	}
	journey := newGrokNativePublicJourney(t, transport)
	if string(grokNativeCommand(t, journey.home, journey.a, *grokPublicNative, "--version")) != "grok 1.0.70 (f50904eaee6c)\n" {
		t.Fatal("genuine Grok version differs")
	}
	client, envelope := grokSignedUserDirectory(t, journey, source, revision)
	witness := &grokSignedNativeRunner{executable: *grokPublicNative, home: journey.home, managed: journey.app.ManagedRoot, phase: "install"}
	t.Cleanup(func() {
		body, err := json.Marshal(witness.calls)
		if err != nil || len(body) > 256<<10 {
			t.Fatal("native transcript exceeds its finite bound", err)
		}
		t.Logf("GROK_USER_ACTIVATOR_TRANSCRIPT %s", body)
	})
	journey = grokSignedUserWire(t, journey, client, witness, caFile)
	foreignRoots := []string{source, journey.a, journey.b, filepath.Join(journey.home, ".grok", "keep-user"), filepath.Join(journey.home, ".grok", "plugins", "foreign-disabled")}
	foreign := grokSignedUserSnapshot(t, foreignRoots...)
	// --no-save is the ordinary direct add route, not a synthetic Service call.
	// It commits installer ownership while avoiding dependency-sync short cuts.
	args := []string{"add", "tool", "--target", "grok", "--no-save", "--yes"}
	first := grokSignedUserReady(t, grokSignedUserPublic(t, journey, args...), false)
	binding := grokSignedUserBinding(t, journey, first, envelope, revision)
	grokSignedUserInventory(t, witness.phaseCalls(t, "install", 2, 1), journey, binding, true)
	grokSignedUserForeignConfig(t, journey.home)
	grokSignedUserUnchanged(t, foreign, foreignRoots...)
	state := journey.state(t, "")
	protectedRoots := []string{journey.home, journey.app.ManagedRoot, binding.TargetLocator, filepath.Join(journey.cache, "state-v2.json"), source, journey.a, journey.b}
	protected := grokSignedUserSnapshot(t, protectedRoots...)
	witness.phase = "repeat"
	repeat := grokSignedUserReady(t, grokSignedUserPublic(t, journey, args...), true)
	if repeat.InstallationID != first.InstallationID || !reflect.DeepEqual(state, journey.state(t, "")) {
		t.Fatal("unchanged User repeat rewrote committed ownership/state")
	}
	grokSignedUserInventory(t, witness.phaseCalls(t, "repeat", 1, 0), journey, binding, true)
	grokSignedUserUnchanged(t, protected, protectedRoots...)
	witness.phase = "remove"
	grokSignedUserPublic(t, journey, "remove", first.InstallationID, "--target", "grok", "--yes")
	grokSignedUserInventory(t, witness.phaseCalls(t, "remove", 2, 1), journey, binding, false)
	for _, installation := range journey.state(t, "").Installations {
		if len(installation.Clients) != 0 {
			t.Fatal("public remove retained a client binding")
		}
	}
	if _, err := os.Lstat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatal("public remove retained owned package bytes", err)
	}
	grokSignedUserUnchanged(t, foreign, foreignRoots...)
	grokSignedUserForeignConfig(t, journey.home)
	t.Log("GROK_NATIVE_USER_SIGNED_REPEAT_PASS")
}
