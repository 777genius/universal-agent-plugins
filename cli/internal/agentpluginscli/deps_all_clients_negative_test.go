package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDependencyAllClientsManualAndDetectionRefusals(t *testing.T) {
	repo := dependencyRepo(t)
	f, source, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientCline})
	status := dependencyRun(t, f, "deps", "status", "--format", "json")
	var report dependencyStatus
	if err := json.Unmarshal([]byte(status), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.SupportedTargets) != 11 || len(report.ExcludedTargets) != 2 || report.ExcludedTargets[0].ID != "chatgpt" || report.ExcludedTargets[0].Reason != "manual_preparation_only" || report.ExcludedTargets[1].ID != "vscode" || report.ExcludedTargets[1].Reason != "dependency_management_deferred" {
		t.Fatal(status)
	}
	mp, lp := f.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\nchatgpt = [\"demo\"]\n")
	if _, _, err := f.execute(false, "deps", "lock", "--format", "json"); err == nil || !strings.Contains(err.Error(), "prepared manually") {
		t.Fatalf("manual target refusal: %v", err)
	}
	if _, _, err := f.execute(false, "add", "github:acme/demo@0123456789012345678901234567890123456789", "--target", "chatgpt,codex", "--format", "json"); err == nil || !strings.Contains(err.Error(), "separately") {
		t.Fatalf("mixed manual target refusal: %v", err)
	}
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\nvscode = [\"demo\"]\n")
	if _, _, err := f.execute(false, "deps", "lock", "--format", "json"); err == nil {
		t.Fatal("owner-deferred managed target accepted")
	}
	if _, _, err := f.execute(false, "add", "github:acme/demo@0123456789012345678901234567890123456789", "--target", "vscode", "--format", "json"); err == nil {
		t.Fatal("owner-deferred saved add accepted")
	}
	if source.downloads != 0 || source.lookups != 0 {
		t.Fatal("excluded target refusal acquired source")
	}

	for _, path := range []string{f.store.Path, lp, f.app.EnvironmentStoreRoot} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("manual refusal wrote %s: %v", path, err)
		}
	}
	// Same early refusal for an adapter that detection could not authorize.
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\ncline = [\"demo\"]\n")
	dependencyRun(t, f, "deps", "lock", "--format", "json")
	client := fixtureClient(t, domain.ClientCline)
	client.Status = domain.DetectionNotDetected
	f.app.Detector = staticDetector{clients: []domain.DetectedClient{client}}
	beforeLock := dependencyRead(t, lp)
	if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil || !strings.Contains(err.Error(), "not detected") {
		t.Fatalf("undetected target refusal: %v", err)
	}
	client.Status = domain.DetectionDetected
	client.DetectionError = errors.New("profile discovery failed")
	f.app.Detector = staticDetector{clients: []domain.DetectedClient{client}}
	if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil || !strings.Contains(err.Error(), "not detected") {
		t.Fatalf("failed detection accepted: %v", err)
	}
	if dependencyRead(t, lp) != beforeLock {
		t.Fatal("detection refusal changed lock")
	}
	if _, err := os.Stat(f.store.Path); !os.IsNotExist(err) {
		t.Fatal("detection refusal wrote state")
	}
	if _, err := os.Stat(client.ConfigRoot); !os.IsNotExist(err) {
		t.Fatal("detection refusal wrote profile")
	}
}

// Unsupported-only packages cannot create an owned binding. Mixed packages
// retain their supported skill and expose the unsupported transport decision.
func TestDependencyAllClientsComponentSupport(t *testing.T) {
	for _, id := range []domain.ClientID{domain.ClientKimi, domain.ClientOpenCode, domain.ClientGrok} {
		t.Run(string(id), func(t *testing.T) {
			repo := writeCLIPlugin(t)
			dependencyWrite(t, filepath.Join(repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"events":{"type":"sse","url":"https://example.test/events"}}}`)
			dependencyGit(t, repo, "init", "-b", "main")
			dependencyGit(t, repo, "add", ".")
			dependencyGit(t, repo, "commit", "-m", "test: unsupported SSE fixture")
			f, _, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: id, executable: id != domain.ClientKimi})
			mp, lp := f.app.dependencyPaths()
			dependencyWrite(t, mp, "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\n"+string(id)+" = [\"demo\"]\n")
			dependencyRun(t, f, "deps", "lock", "--format", "json")
			beforeLock := dependencyRead(t, lp)
			if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil {
				t.Fatal("unsupported-only package accepted")
			}
			if dependencyRead(t, lp) != beforeLock {
				t.Fatal("unsupported preflight changed lock")
			}
			for _, path := range []string{f.store.Path} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("unsupported preflight wrote %s: %v", path, err)
				}
			}
			// Immutable acquisition cache may already have been populated by lock.
			// Validate retained bytes against the exact agreement; it is not an install.
			requirements, err := readDependencyManifest(mp)
			if err != nil {
				t.Fatal(err)
			}
			locked, err := readDependencyLock(lp, requirements)
			if err != nil {
				t.Fatal(err)
			}
			pin := locked.Packages[0]
			if _, err := f.app.dependencyStore().Get(context.Background(), pin.DigestAlgorithm, pin.TreeDigest); err != nil {
				t.Fatalf("retained acquisition cache differs from exact lock: %v", err)
			}
			detected, _ := f.app.Detector.Detect(context.Background())
			if _, err := os.Stat(detected[0].ConfigRoot); !os.IsNotExist(err) {
				t.Fatal("unsupported preflight changed profile")
			}
		})
	}
	t.Run("partial_kimi", func(t *testing.T) {
		repo := dependencyRepo(t)
		dependencyWrite(t, filepath.Join(repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"events":{"type":"sse","url":"https://example.test/events"}}}`)
		dependencyGit(t, repo, "add", ".")
		dependencyGit(t, repo, "commit", "-m", "test: mixed skill and unsupported SSE")
		f, source, runner := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientKimi})
		mp, _ := f.app.dependencyPaths()
		dependencyWrite(t, mp, "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\nkimi = [\"demo\"]\n")
		dependencyRun(t, f, "deps", "lock", "--format", "json")
		for _, dry := range []bool{true, false} {
			args := []string{"deps", "sync", "--locked", "--yes", "--format", "json"}
			if dry {
				args = append(args, "--dry-run")
			}
			out := dependencyRun(t, f, args...)
			var document struct {
				Actions []dependencyActionDTO `json:"actions"`
			}
			if err := json.Unmarshal([]byte(out), &document); err != nil {
				t.Fatal(err)
			}
			if len(document.Actions) != 1 {
				t.Fatal(out)
			}
			foundSkill, foundSSE := false, false
			for _, decision := range document.Actions[0].ComponentSupport {
				if decision.Kind == domain.ComponentSkill && decision.Support == domain.SupportNative {
					foundSkill = true
				}
				if decision.Kind == domain.ComponentMCPServer && decision.Name == "events" && decision.Support == domain.SupportUnsupported {
					foundSSE = true
				}
			}
			if !foundSkill || !foundSSE {
				t.Fatalf("component decisions lost: %s", out)
			}
		}
		binding := dependencyBinding(t, f)
		if binding.Activation != "active" {
			t.Fatalf("supported skill not activated: %+v", binding)
		}
		downloads, mutations := source.downloads, runner.mutations
		dependencyRun(t, f, "deps", "sync", "--locked", "--yes", "--format", "json")
		if downloads != source.downloads || mutations != runner.mutations {
			t.Fatal("partial components caused lifecycle retry")
		}
	})
}

func TestDependencyStatusManualNextActions(t *testing.T) {
	actions := dependencyNextActions(dependencyStatusFact{ClientID: "codex", Facts: []string{"activation_manual", "auth_failed"}})
	if len(actions) != 2 || actions[0].Kind != "attest_activation" || !strings.Contains(actions[0].Reason, "--activation-complete") || actions[1].Kind != "attest_authentication" {
		t.Fatalf("manual/authentication guidance: %+v", actions)
	}
	cursor := dependencyNextActions(dependencyStatusFact{ClientID: "cursor", Facts: []string{"activation_manual"}})
	if len(cursor) != 1 || cursor[0].Kind != "attest_activation" || !strings.Contains(cursor[0].Reason, "--activation-complete") {
		t.Fatalf("Cursor guidance lost composed-provider attestation: %+v", cursor)
	}
}

// An imperative native binding is foreign to the saved environment even when
// its source and bytes are identical. Saved sync must never adopt it.
func TestDependencyAllClientsNativeForeignBinding(t *testing.T) {
	repo := dependencyRepo(t)
	f, _, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientGemini})
	headCommand := exec.CommandContext(context.Background(), "git", "rev-parse", "HEAD")
	headCommand.Dir = repo
	head, err := headCommand.Output()
	if err != nil {
		t.Fatal(err)
	}
	dependencyRun(t, f, "add", githubIdentity("acme/demo", strings.TrimSpace(string(head)), ""), "--target", "gemini", "--no-save", "--format", "json")
	mp, _ := f.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\ngemini = [\"demo\"]\n")
	dependencyRun(t, f, "deps", "lock", "--format", "json")
	before := dependencyRead(t, f.store.Path)
	if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("adopted foreign native binding")
	}
	if before != dependencyRead(t, f.store.Path) {
		t.Fatal("foreign refusal changed state")
	}
	status := dependencyRun(t, f, "deps", "status", "--format", "json")
	if !strings.Contains(status, "foreign") {
		t.Fatal(status)
	}
	view, err := f.app.Installer.Inspect(context.Background())
	if err != nil || len(view.EnvironmentReceipts) != 0 {
		t.Fatalf("foreign binding gained owner: %+v %v", view, err)
	}
}

func TestSavedAddPreservesManualAndLocalRoutes(t *testing.T) {
	repo := dependencyRepo(t)
	f, source, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientGemini})
	opts := &options{scope: "user", format: "json", target: "chatgpt"}
	routed, err := routeSavedAdd(&cobra.Command{}, f.app, opts, "github:acme/demo@0123456789012345678901234567890123456789", false, false)
	if err != nil || routed || opts.environmentReason != "chatgpt_preparation_not_saved" {
		t.Fatalf("manual route changed: %v %v %+v", routed, err, opts)
	}
	if source.downloads != 0 || source.lookups != 0 {
		t.Fatal("manual routing acquired source")
	}
	client := fixtureClient(t, domain.ClientGemini)
	client.ExecutablePath = filepath.Join(t.TempDir(), "gemini")
	loaded := loadedPackage{origin: domain.OriginModeDirect, envelope: domain.PackageEnvelope{FormatID: domain.FormatIDAgentPluginsV1, SnapshotRoot: repo, Source: domain.SourceIdentity{SourceBindingHint: "direct-local"}}}
	if useInstallerFacadeForLocalAdd(f.app, &options{scope: "user", format: "json"}, loaded, []domain.ClientID{domain.ClientGemini}, false, false, false, []domain.DetectedClient{client}) {
		t.Fatal("saved support silently broadened the qualified local-file route")
	}
}

// Existing Codex E2E covers mixed-owner recovery and interrupted multi-target
// retries. This native representative exercises the same recovery boundary
// with genuine native config receipts, without repeating it for every adapter.
func TestDependencyAllClientsNativeRecovery(t *testing.T) {
	repo := dependencyRepo(t)
	f, _, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientGemini})
	mp, _ := f.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\ngemini = [\"demo\"]\n")
	dependencyRun(t, f, "deps", "lock", "--format", "json")
	dependencyRun(t, f, "deps", "sync", "--locked", "--yes", "--format", "json")
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		if len(binding.Receipts) == 0 {
			t.Fatal("native binding has no transaction receipt")
		}
		binding.Receipts[len(binding.Receipts)-1].Phase = "state_committed"
		state.Installations[0].Clients[key] = binding
	}
	if err := f.store.Save(state); err != nil {
		t.Fatal(err)
	}
	view, err := f.app.Installer.Inspect(context.Background())
	if err != nil || !view.Recovery.Required {
		t.Fatalf("native pending recovery missing: %+v %v", view, err)
	}
	dependencyRun(t, f, "deps", "recover", "--yes", "--format", "json")
	view, err = f.app.Installer.Inspect(context.Background())
	if err != nil || view.Recovery.Required {
		t.Fatalf("native recovery incomplete: %+v %v", view, err)
	}
}
