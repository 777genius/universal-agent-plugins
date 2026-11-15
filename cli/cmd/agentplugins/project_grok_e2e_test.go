package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type projectGrokDetector struct{ user, executable, version string }

func (d projectGrokDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientGrok, ConfigRoot: d.user, ExecutablePath: d.executable, Version: d.version, Status: domain.DetectionDetected}}, nil
}

type projectGrokRunner struct {
	t          *testing.T
	calls      int
	executable string
	version    string
}

func (r *projectGrokRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	if !reflect.DeepEqual(command.Argv, []string{r.executable, "inspect", "--json"}) || command.Dir == "" {
		r.t.Fatal("unexpected global/native command", command)
	}
	plugins := []any{}
	root := filepath.Join(command.Dir, ".grok", "plugins")
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		return legacyports.CommandResult{}, err
	}
	for _, entry := range entries {
		manifest := filepath.Join(root, entry.Name(), ".grok-plugin", "plugin.json")
		body, err := os.ReadFile(manifest)
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		var object map[string]any
		if err := json.Unmarshal(body, &object); err != nil {
			return legacyports.CommandResult{}, err
		}
		mcpCount := 0
		if object["mcpServers"] == "./.mcp.json" {
			var sidecar struct {
				Servers map[string]json.RawMessage `json:"mcpServers"`
			}
			raw, readErr := os.ReadFile(filepath.Join(root, entry.Name(), ".mcp.json"))
			if readErr == nil && json.Unmarshal(raw, &sidecar) == nil {
				mcpCount = len(sidecar.Servers)
			}
		}
		plugins = append(plugins, map[string]any{"name": object["name"], "scope": "project", "path": filepath.Join(root, entry.Name()), "enabled": false, "provides": map[string]any{"mcpServers": mcpCount}})
	}
	version := r.version
	if version == "" {
		version = "1.0.67"
	}
	body, _ := json.Marshal(map[string]any{"grokVersion": version, "cwd": command.Dir, "projectRoot": command.Dir + string(filepath.Separator), "projectTrusted": false, "skills": []any{}, "plugins": plugins})
	return legacyports.CommandResult{Stdout: body}, nil
}
func configureProjectGrokJourney(app agentpluginscli.App, user string, runner *projectGrokRunner) agentpluginscli.App {
	executable, err := os.Executable()
	if err != nil {
		runner.t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: filepath.Dir(app.ManagedRoot), ManagedRoot: app.ManagedRoot, ProjectRoot: app.ProjectRoot, Registry: app.ClientRegistry, Runner: runner, EnableNativeObserver: true, HelperExecutable: executable})
	if err != nil {
		runner.t.Fatal(err)
	}
	app.Installer = engine
	version := runner.version
	if version == "" {
		version = "1.0.67"
	}
	app.Detector = projectGrokDetector{user: user, executable: runner.executable, version: version}
	app.Lifecycle.Activator = providers.Activator{Registry: app.ClientRegistry, Runner: runner}
	app.Lifecycle.NativeObserver = providers.NativeIdentityObserver{Registry: app.ClientRegistry, Runner: runner, Stager: app.Lifecycle.Stager}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectGrokJourney(selected, user, runner), nil
	}
	return app
}

// Real TLS Git acquisition, production saved CLI/planner/staging/store/state
// and exact directory ownership, with inert inspect-only client transport.
func TestProjectGrokComposedSavedLifecycle(t *testing.T) {
	for _, withMCP := range []bool{false, true} {
		name := "skills-only"
		if withMCP {
			name = "skills-and-mcp"
		}
		t.Run(name, func(t *testing.T) { testProjectGrokComposedSavedLifecycle(t, withMCP) })
	}
}

func testProjectGrokComposedSavedLifecycle(t *testing.T, withMCP bool) {
	transport := newProjectJourneyTransport(t, "git")
	if withMCP {
		projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://inert.test/mcp"}}}`)
	} else if err := os.Remove(filepath.Join(transport.repo, "mcp.json")); err != nil {
		t.Fatal(err)
	}
	// The shared journey fixture contains MCP by default; remove it for a
	// genuinely Skills-only Grok project before publishing the Git source.
	projectJourneyGit(t, transport.repo, "add", "-A")
	projectJourneyGit(t, transport.repo, "commit", "-m", "select Grok project fixture components")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".grok")
	projectJourneyWrite(t, filepath.Join(user, "keep"), "original user\n")
	runner := &projectGrokRunner{t: t, executable: filepath.Join(journey.home, "grok-not-executed")}
	if withMCP {
		runner.version = "1.0.70"
	}
	journey.app = configureProjectGrokJourney(journey.app, user, runner)
	userBefore := securityFixtureTree(t, user)
	for _, project := range []string{journey.a, journey.b} {
		journey.run(t, project, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
		projectGrokOwned(t, journey, project, projectJourneySkill, withMCP)
	}
	bBefore := securityFixtureTree(t, journey.b)
	installation, binding := journey.owned(t, journey.a)
	if len(installation.DataReceipts) != 0 {
		t.Fatal("skills-only projection invented MCP persistent data")
	}
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	beforeAttest := securityFixtureTree(t, filepath.Join(journey.a, ".grok"))
	if _, err := journey.execute(journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok", "--activation-complete"); err == nil {
		t.Fatal("persisted prepared Grok binding accepted activation attestation")
	}
	if !reflect.DeepEqual(beforeAttest, securityFixtureTree(t, filepath.Join(journey.a, ".grok"))) {
		t.Fatal("refused attestation changed native project bytes")
	}
	_, refused := journey.owned(t, journey.a)
	if refused.Activation == domain.ActivationActive || refused.Verification == domain.VerificationInstalled || refused.TargetLocator != binding.TargetLocator {
		t.Fatal("refused attestation promoted loading or changed artifact ownership")
	}
	// A rejected attestation records a failed attempt in existing core state.
	// An explicit ordinary add retries that attempt without native trust or loading.
	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	projectGrokOwned(t, journey, journey.a, projectJourneySkill, withMCP)
	projectJourneyWrite(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"), "TEST immutable drift\n")
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "advance disposable Grok source")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "grok")
	projectGrokOwned(t, journey, journey.a, projectJourneySkill, withMCP)
	journey.run(t, journey.a, "update", "docs", "--target", "grok")
	updated, current := projectGrokOwned(t, journey, journey.a, projectJourneyAdvancedSkill, withMCP)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
		t.Fatal("Grok update lost ownership or exact pin")
	}
	status := journey.run(t, journey.a, "deps", "status")
	if !strings.Contains(status, `"activation_prepared"`) || strings.Contains(status, `"activation_active"`) {
		t.Fatal("status promoted unqualified native loading", status)
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "grok")
	projectJourneyAbsent(t, current.TargetLocator)
	if !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
		t.Fatal("Grok A lifecycle changed original user or B")
	}
	if runner.calls == 0 {
		t.Fatal("composed contract did not exercise read-only source discovery")
	}
}
func projectGrokOwned(t *testing.T, journey projectJourney, project, expected string, withMCP bool) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := journey.owned(t, project)
	if binding.ClientID != "grok" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(project, ".grok") || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationPackageValid {
		t.Fatalf("Grok binding lost conservative project authority %+v", binding)
	}
	if projectJourneyRead(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")) != expected {
		t.Fatal("Grok exact immutable skill differs")
	}
	portable := filepath.Join(binding.TargetLocator, "mcp.json")
	if withMCP {
		if _, err := os.Stat(portable); err != nil {
			t.Fatal("managed project is missing its portable source MCP", err)
		}
	} else if _, err := os.Stat(portable); !os.IsNotExist(err) {
		t.Fatal("skills-only Grok retained a portable MCP", err)
	}
	sidecar := filepath.Join(binding.TargetLocator, ".mcp.json")
	if !withMCP {
		if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
			t.Fatal("skills-only Grok projected MCP", err)
		}
	} else {
		body, err := os.ReadFile(sidecar)
		if err != nil {
			t.Fatal("managed Grok project MCP sidecar missing", err)
		}
		var document struct {
			Servers map[string]map[string]any `json:"mcpServers"`
		}
		if err := json.Unmarshal(body, &document); err != nil || len(document.Servers) != 1 ||
			document.Servers["docs"]["type"] != "http" || document.Servers["docs"]["url"] != "https://inert.test/mcp" {
			t.Fatalf("unexpected managed Grok MCP: %s %v", body, err)
		}
	}
	return installation, binding
}

func TestProjectGrokMCPOnlyComposedSavedLifecycle(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	if err := os.Remove(filepath.Join(transport.repo, "skills", "docs", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	mcpSource := filepath.Join(transport.repo, "mcp.json")
	projectJourneyWrite(t, mcpSource, grokMCPOnlySource("https://inert.test/mcp"))
	projectJourneyGit(t, transport.repo, "add", "-A")
	projectJourneyGit(t, transport.repo, "commit", "-m", "replace source Skill with MCP-only server")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".grok")
	projectJourneyWrite(t, filepath.Join(user, "keep"), "original user\n")
	runner := &projectGrokRunner{t: t, executable: filepath.Join(journey.home, "grok-not-executed"), version: "1.0.70"}
	journey.app = configureProjectGrokJourney(journey.app, user, runner)
	userBefore, bBefore := securityFixtureTree(t, user), securityFixtureTree(t, journey.b)

	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	first, binding := projectGrokMCPOnlyOwned(t, journey, "https://inert.test/mcp")
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	projectGrokMCPOnlyOwned(t, journey, "https://inert.test/mcp")

	projectJourneyWrite(t, filepath.Join(binding.TargetLocator, ".mcp.json"), `{"broken":true}`)
	journey.run(t, journey.a, "repair", first.InstallationID, "--target", "grok")
	projectGrokMCPOnlyOwned(t, journey, "https://inert.test/mcp")

	projectJourneyWrite(t, mcpSource, grokMCPOnlySource("https://inert.test/mcp-v2"))
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "advance inert Grok MCP source")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey.run(t, journey.a, "update", "docs", "--target", "grok")
	updated, current := projectGrokMCPOnlyOwned(t, journey, "https://inert.test/mcp-v2")
	if updated.Source.TreeDigest == first.Source.TreeDigest ||
		current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
		t.Fatal("MCP-only Grok update changed ownership or failed to advance pin")
	}
	// A client downgrade must not prevent safe removal of the owned package.
	runner.version = "1.0.67"
	journey.run(t, journey.a, "remove", "docs", "--target", "grok")
	projectJourneyAbsent(t, current.TargetLocator)
	if !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) ||
		!reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) || runner.calls == 0 {
		t.Fatal("MCP-only Grok lifecycle changed user/B or skipped native discovery")
	}
}

func grokMCPOnlySource(url string) string {
	return `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"` + url + `"}}}`
}

func projectGrokMCPOnlyOwned(t *testing.T, journey projectJourney, url string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := journey.owned(t, journey.a)
	if binding.ClientID != "grok" || binding.Scope != "project" || binding.Activation != domain.ActivationPrepared ||
		binding.Verification != domain.VerificationPackageValid ||
		binding.NativeProfileRoot != filepath.Join(journey.a, ".grok") {
		t.Fatalf("Grok MCP-only binding must remain prepared: %+v", binding)
	}
	if _, err := os.Stat(filepath.Join(binding.TargetLocator, "skills")); !os.IsNotExist(err) {
		t.Fatalf("MCP-only package unexpectedly projected Skills: %v", err)
	}
	if _, err := os.Stat(filepath.Join(binding.TargetLocator, "mcp.json")); err != nil {
		t.Fatalf("MCP-only package lost the sanitized portable MCP: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(binding.TargetLocator, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &document); err != nil || len(document.Servers) != 1 ||
		document.Servers["docs"]["type"] != "http" || document.Servers["docs"]["url"] != url {
		t.Fatalf("MCP-only Grok sidecar differs: %s %v", body, err)
	}
	return installation, binding
}

func TestProjectGrokMCPOnlyStdioKeepsOwnedData(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	if err := os.Remove(filepath.Join(transport.repo, "skills", "docs", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	// Keep the shared source's real stdio server with a PLUGIN_DATA placeholder.
	projectJourneyGit(t, transport.repo, "add", "-A")
	projectJourneyGit(t, transport.repo, "commit", "-m", "MCP-only stdio source")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".grok")
	projectJourneyWrite(t, filepath.Join(user, "keep"), "original user\n")
	runner := &projectGrokRunner{t: t, executable: filepath.Join(journey.home, "grok-not-executed"), version: "1.0.70"}
	journey.app = configureProjectGrokJourney(journey.app, user, runner)
	userBefore, bBefore := securityFixtureTree(t, user), securityFixtureTree(t, journey.b)

	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	installation, binding := journey.owned(t, journey.a)
	receipt := installation.DataReceipts[binding.DataReceiptID]
	expectedData := filepath.Join(journey.a, ".uap", "plugin-data", binding.PhysicalArtifact)
	if len(installation.DataReceipts) != 1 || receipt.DataReceiptID == "" ||
		receipt.Scope != "project" || receipt.State != domain.DataReceiptOwned ||
		receipt.Locator != expectedData || receipt.PhysicalBackend != binding.PhysicalArtifact ||
		binding.Activation != domain.ActivationPrepared {
		t.Fatalf("Grok stdio MCP lost owned data or prepared state: %+v %+v", receipt, binding)
	}
	body, err := os.ReadFile(filepath.Join(binding.TargetLocator, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var projected struct {
		Servers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			CWD     *string           `json:"cwd"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &projected); err != nil || len(projected.Servers) != 1 {
		t.Fatalf("Grok stdio native MCP projection differs: %s %v", body, err)
	}
	server, present := projected.Servers["docs"]
	helper := filepath.Join(binding.TargetLocator, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	authority := []string{managedstdio.Mode, binding.TargetLocator, expectedData, binding.TargetLocator, "plugin"}
	if !present || server.Command != helper || server.CWD != nil || len(server.Args) != 8 || server.Args[5] != "--argv-base64-v1" ||
		!reflect.DeepEqual(server.Args[:5], authority) || server.Env["DATA"] != expectedData {
		t.Fatalf("Grok stdio launcher lost package/data/cwd authority: %+v", server)
	}
	child, err := managedstdio.DecodeCommandArguments(server.Args[5], server.Args[6:])
	if err != nil || !reflect.DeepEqual(child, []string{"go", "version"}) {
		t.Fatalf("Grok stdio launcher changed the original child command: %v %v", child, err)
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "grok")
	projectJourneyAbsent(t, binding.TargetLocator)
	if !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) ||
		!reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) || runner.calls == 0 {
		t.Fatal("Grok stdio Project removal changed User/B or skipped native observation")
	}
}

func TestProjectGrokUnqualifiedMCPVersionRefusesBeforeOwnedMutation(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".grok")
	projectJourneyWrite(t, filepath.Join(user, "keep"), "original user\n")
	runner := &projectGrokRunner{t: t, executable: filepath.Join(journey.home, "grok-not-executed"), version: "1.0.67"}
	journey.app = configureProjectGrokJourney(journey.app, user, runner)
	userBefore, otherBefore := securityFixtureTree(t, user), securityFixtureTree(t, journey.b)
	if _, err := journey.execute(journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok"); err == nil ||
		!strings.Contains(err.Error(), "requires qualified Grok 1.0.70") {
		t.Fatalf("unqualified MCP version was not refused: %v", err)
	}
	projectJourneyAbsent(t, filepath.Join(journey.a, ".grok"))
	if !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) ||
		!reflect.DeepEqual(otherBefore, securityFixtureTree(t, journey.b)) || runner.calls == 0 {
		t.Fatal("version refusal changed user/B or skipped read-only inspect")
	}
}

// This explicit predecessor seam creates the actual old direct projection via
// the original shared encoder before its genuine immutable receipt is saved.
// No installed file or receipt is edited to simulate a legacy installation.
type legacyProjectGrok struct{ *grok.Adapter }

func (adapter legacyProjectGrok) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	manifest := shared.ManifestFromEnvelope(in.Envelope, shared.WithAuthorNameEmail())
	manifest["skills"] = "./skills/"
	manifest["mcpServers"] = "./.mcp.json"
	if err := shared.WriteJSON(filepath.Join(in.StagingPath, ".grok-plugin", "plugin.json"), manifest); err != nil {
		return nil, err
	}
	return []domain.NativeObjectOwnership{}, shared.ProjectMCPServers(shared.MCPProjection{Root: in.StagingPath, Envelope: in.Envelope, Names: shared.SupportedMCPNames(in.Plan), Dialect: shared.MCPDialectGrok, PluginRoot: in.Plan.ActivePath, DataPath: in.PluginDataPath})
}
func projectGrokAdapterApp(t *testing.T, app agentpluginscli.App, adapter clients.Adapter) agentpluginscli.App {
	t.Helper()
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	app.ClientRegistry = registry
	stager, ok := app.Lifecycle.Stager.(providers.Stager)
	if !ok {
		t.Fatal("fixture requires composed production stager")
	}
	stager.Registry = registry
	app.Lifecycle.Stager = stager
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return projectGrokAdapterApp(t, selected, adapter), nil
	}
	return app
}
func TestProjectGrokLegacyStdioRequiresExplicitProjectionRefresh(t *testing.T) {
	transport := newProjectJourneyTransport(t, "zip")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".grok")
	projectJourneyWrite(t, filepath.Join(user, "keep"), "foreign User")
	normal := journey.app
	runner := &projectGrokRunner{t: t, executable: filepath.Join(journey.home, "grok-not-executed"), version: "1.0.70"}
	journey.app = configureProjectGrokJourney(projectGrokAdapterApp(t, normal, legacyProjectGrok{grok.New()}), user, runner)
	journey.run(t, journey.a, "add", transport.endpoint, "--target", "grok", "--no-save")
	initial, binding := projectGrokUnsavedOwned(t, journey)
	before := securityFixtureTree(t, filepath.Join(journey.a, ".grok"))
	foreignUser, foreignB := securityFixtureTree(t, user), securityFixtureTree(t, journey.b)
	journey.app = configureProjectGrokJourney(normal, user, runner)
	_, err := journey.execute(journey.a, "add", transport.endpoint, "--target", "grok", "--no-save")
	if !errors.Is(err, domain.ErrProjectionRefreshRequired) {
		t.Fatal("same revision legacy direct projection was accepted", err)
	}
	if !reflect.DeepEqual(before, securityFixtureTree(t, filepath.Join(journey.a, ".grok"))) {
		t.Fatal("refused repeat rewrote old immutable projection")
	}
	journey.run(t, journey.a, "update", initial.InstallationID, "--target", "grok")
	refreshed, current := projectGrokUnsavedOwned(t, journey)
	if refreshed.Source.TreeDigest != initial.Source.TreeDigest || current.TargetLocator != binding.TargetLocator || current.ClientBindingID != binding.ClientBindingID || current.Activation != domain.ActivationPrepared || current.Verification != domain.VerificationPackageValid {
		t.Fatal("projection refresh changed source authority or promoted activation", current)
	}
	body := projectJourneyRead(t, filepath.Join(current.TargetLocator, ".mcp.json"))
	if !strings.Contains(body, "--argv-base64-v1") || strings.Contains(body, `"cwd"`) {
		t.Fatal("explicit update did not install managed cwd launcher", body)
	}
	journey.run(t, journey.a, "add", transport.endpoint, "--target", "grok", "--no-save")
	if !reflect.DeepEqual(foreignUser, securityFixtureTree(t, user)) || !reflect.DeepEqual(foreignB, securityFixtureTree(t, journey.b)) {
		t.Fatal("legacy refresh changed B or User")
	}
}

func projectGrokUnsavedOwned(t *testing.T, journey projectJourney) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	state := journey.state(t, journey.a)
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatal("legacy unsaved fixture authority differs", state)
	}
	installation := state.Installations[0]
	for _, binding := range installation.Clients {
		return installation, binding
	}
	t.Fatal("legacy binding missing")
	return domain.Installation{}, domain.ClientBinding{}
}

func projectGrokNoHelperApp(t *testing.T, app agentpluginscli.App) agentpluginscli.App {
	t.Helper()
	stager, ok := app.Lifecycle.Stager.(providers.Stager)
	if !ok {
		t.Fatal("fixture requires composed production stager")
	}
	stager.LauncherSource = nil
	app.Lifecycle.Stager = stager
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return projectGrokNoHelperApp(t, selected), nil
	}
	return app
}
func TestProjectGrokHelperReadinessPrecedesProjectionEffects(t *testing.T) {
	transport := newProjectJourneyTransport(t, "zip")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".grok")
	projectJourneyWrite(t, filepath.Join(user, "keep"), "foreign User")
	runner := &projectGrokRunner{t: t, executable: filepath.Join(journey.home, "grok-not-executed"), version: "1.0.70"}
	journey.app = projectGrokNoHelperApp(t, configureProjectGrokJourney(journey.app, user, runner))
	beforeA, beforeB, beforeUser := securityFixtureTree(t, journey.a), securityFixtureTree(t, journey.b), securityFixtureTree(t, user)
	output, err := journey.execute(journey.a, "add", transport.endpoint, "--target", "grok", "--no-save", "--dry-run")
	if err != nil || !strings.Contains(output, "managed_stdio_helper_unavailable") {
		t.Fatal("planning did not report unavailable trusted helper", output, err)
	}
	if !reflect.DeepEqual(beforeA, securityFixtureTree(t, journey.a)) || !reflect.DeepEqual(beforeB, securityFixtureTree(t, journey.b)) || !reflect.DeepEqual(beforeUser, securityFixtureTree(t, user)) {
		t.Fatal("readiness preview changed project or foreign roots")
	}
}

func TestProjectGrokSavedLegacyStdioRequiresPinChange(t *testing.T) {
	transport := newProjectJourneyTransport(t, "zip")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".grok")
	projectJourneyWrite(t, filepath.Join(user, "keep"), "foreign User")
	normal := journey.app
	runner := &projectGrokRunner{t: t, executable: filepath.Join(journey.home, "grok-not-executed"), version: "1.0.70"}
	journey.app = configureProjectGrokJourney(projectGrokAdapterApp(t, normal, legacyProjectGrok{grok.New()}), user, runner)
	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	initial, binding := journey.owned(t, journey.a)
	before := securityFixtureTree(t, filepath.Join(journey.a, ".grok"))
	stateBefore := projectJourneyRead(t, filepath.Join(journey.namespace(journey.a), "state-v2.json"))
	foreignUser, foreignB := securityFixtureTree(t, user), securityFixtureTree(t, journey.b)
	journey.app = configureProjectGrokJourney(normal, user, runner)
	_, err := journey.execute(journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	if !errors.Is(err, domain.ErrProjectionPinChangeRequired) {
		t.Fatal("saved stale pin was accepted or offered an unauthorized refresh", err)
	}
	if !reflect.DeepEqual(before, securityFixtureTree(t, filepath.Join(journey.a, ".grok"))) || stateBefore != projectJourneyRead(t, filepath.Join(journey.namespace(journey.a), "state-v2.json")) {
		t.Fatal("refused saved pin changed immutable package or ownership state")
	}
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	transport.mu.Lock()
	transport.archive = projectJourneyArchive(t, transport.repo)
	transport.mu.Unlock()
	journey.run(t, journey.a, "update", "docs", "--target", "grok")
	updated, current := journey.owned(t, journey.a)
	if updated.Source.TreeDigest == initial.Source.TreeDigest || current.TargetLocator != binding.TargetLocator || current.ClientBindingID != binding.ClientBindingID || current.Activation != domain.ActivationPrepared || current.Verification != domain.VerificationPackageValid {
		t.Fatal("saved pin update lost ownership or promoted activation", current)
	}
	body := projectJourneyRead(t, filepath.Join(current.TargetLocator, ".mcp.json"))
	if !strings.Contains(body, "--argv-base64-v1") || strings.Contains(body, `"cwd"`) {
		t.Fatal("new pin did not install managed cwd launcher", body)
	}
	stateBefore = projectJourneyRead(t, filepath.Join(journey.namespace(journey.a), "state-v2.json"))
	packageBefore := securityFixtureTree(t, current.TargetLocator)
	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	if !reflect.DeepEqual(packageBefore, securityFixtureTree(t, current.TargetLocator)) || stateBefore != projectJourneyRead(t, filepath.Join(journey.namespace(journey.a), "state-v2.json")) {
		t.Fatal("current saved pin repeat changed materialization or receipt")
	}
	if !reflect.DeepEqual(foreignUser, securityFixtureTree(t, user)) || !reflect.DeepEqual(foreignB, securityFixtureTree(t, journey.b)) {
		t.Fatal("saved pin admission changed User or B")
	}
}
