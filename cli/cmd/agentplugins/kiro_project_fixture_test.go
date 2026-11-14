package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type kiroProjectDetector struct{ root, executable string }

func (detector kiroProjectDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientKiro, ConfigRoot: detector.root, ExecutablePath: detector.executable, Status: domain.DetectionDetected}}, nil
}

type kiroNoNativeRunner struct{ calls int }

func (runner *kiroNoNativeRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	return legacyports.CommandResult{}, fmt.Errorf("unexpected Kiro process: %v", command.Argv)
}

// Reuse the core journey's production CLI/state helpers and real TLS Git/ZIP
// transports. Kiro supplies its own detected profile and native layout.
type kiroProjectJourney struct {
	projectJourney
	protected  map[string]kiroProtectedTree
	native     *kiroNoNativeRunner
	foreignMCP map[string]any
}

type kiroProtectedTree struct {
	entries    map[string]string
	identities map[string]os.FileInfo
}

func newKiroProjectJourney(t *testing.T, transport *projectJourneyTransport) kiroProjectJourney {
	t.Helper()
	return newKiroJourney(t, transport, false)
}

func newKiroJourney(t *testing.T, transport *projectJourneyTransport, userScope bool) kiroProjectJourney {
	t.Helper()
	f := kiroProjectJourney{projectJourney: projectJourney{home: projectJourneyTemp(t), a: projectJourneyTemp(t), b: projectJourneyTemp(t), transport: transport},
		protected: map[string]kiroProtectedTree{}, native: &kiroNoNativeRunner{}}
	f.cache = filepath.Join(f.home, "uap")
	for index, root := range []string{"", f.a, f.b} {
		native := f.kiroRoot(root)
		projectJourneyWrite(t, filepath.Join(native, "settings", "mcp.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`)
		projectJourneyWrite(t, filepath.Join(native, "skills", "foreign", "SKILL.md"), "foreign skill\n")
		projectJourneyWrite(t, filepath.Join(native, "hooks.json"), "foreign hooks\n")
		label := []string{"user", "project-a", "project-b"}[index]
		projectJourneyWrite(t, filepath.Join(native, "agents", "preserved.json"), `{"name":"preserved","includeMcpJson":false,"resources":["file://`+label+`.md"],"mcpServers":{"docs":{"url":"https://`+label+`.invalid/mcp","disabled":true}}}`)
	}
	user := f.kiroRoot("")
	projectJourneyWrite(t, filepath.Join(user, "settings", "mcp.json"), `{"mcpServers":{"docs":{"url":"https://user-peer.invalid/mcp","disabled":true},"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`)
	projectJourneyWrite(t, filepath.Join(user, "skills", "docs", "SKILL.md"), "Original same-name User skill.\n")
	if userScope {
		for _, project := range []string{f.a, f.b} {
			projectJourneyWrite(t, filepath.Join(f.kiroRoot(project), "settings", "mcp.json"), `{"mcpServers":{"uap_user_probe":{"url":"https://project-peer.invalid/mcp","disabled":true},"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`)
			projectJourneyWrite(t, filepath.Join(f.kiroRoot(project), "skills", "uap-user-docs", "SKILL.md"), "Original same-name project Skill.\n")
		}
		if err := json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(user, "settings", "mcp.json"))), &f.foreignMCP); err != nil {
			t.Fatal(err)
		}
		f.preserve(t, f.a, f.b, filepath.Join(user, "agents"), filepath.Join(user, "skills", "docs"), filepath.Join(user, "skills", "foreign"), filepath.Join(user, "hooks.json"))
	} else {
		f.preserve(t, user, filepath.Join(f.kiroRoot(f.a), "agents"), filepath.Join(f.kiroRoot(f.b), "agents"))
	}
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(f.home, f.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	f.app = configureKiroProjectJourney(t, app, f.home, transport, f.native)
	return f
}

// These are preserved opaque policy files, never evidence of active agent selection.
func (f kiroProjectJourney) preserve(t *testing.T, roots ...string) {
	t.Helper()
	f.assertProtected(t)
	for _, root := range roots {
		if _, exists := f.protected[root]; exists {
			t.Fatal("cannot rebaseline original Kiro policy or peer")
		}
		snapshot := kiroProtectedTree{entries: securityFixtureTree(t, root), identities: map[string]os.FileInfo{}}
		for relative := range snapshot.entries {
			held, err := os.Open(filepath.Join(root, relative))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := held.Close(); err != nil {
					t.Error(err)
				}
			})
			original, err := held.Stat()
			if err != nil {
				t.Fatal(err)
			}
			snapshot.identities[relative] = original
		}
		f.protected[root] = snapshot
	}
	f.assertProtected(t)
}

func (f kiroProjectJourney) assertProtected(t *testing.T) {
	t.Helper()
	if f.native != nil {
		if f.native.calls != 0 {
			t.Fatal("file-only Kiro lifecycle executed a native or MCP process")
		}
	}
	if f.foreignMCP != nil {
		var current map[string]any
		if err := json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(f.kiroRoot(""), "settings", "mcp.json"))), &current); err != nil {
			t.Fatal(err)
		}
		servers, ok := current["mcpServers"].(map[string]any)
		if !ok {
			t.Fatal("Kiro foreign MCP mapping changed shape")
		}
		delete(servers, "uap_user_probe")
		if !reflect.DeepEqual(current, f.foreignMCP) {
			t.Fatal("Kiro changed an original foreign User MCP field")
		}
	}
	for root, snapshot := range f.protected {
		if !reflect.DeepEqual(snapshot.entries, securityFixtureTree(t, root)) {
			t.Fatalf("Kiro changed original policy or peer bytes/membership: %s", root)
		}
		for relative, original := range snapshot.identities {
			current, err := os.Lstat(filepath.Join(root, relative))
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(original, current) || original.Mode() != current.Mode() {
				t.Fatalf("Kiro replaced original policy or peer identity: %s", root)
			}
		}
	}
}

func (f kiroProjectJourney) run(t *testing.T, project string, args ...string) {
	t.Helper()
	f.assertProtected(t)
	if f.native != nil {
		f.assertPreparedState(t, project)
	}
	output := f.projectJourney.run(t, project, args...)
	f.assertProtected(t)
	if f.native != nil {
		var value any
		// execute joins stdout and diagnostic stderr; decode the public object
		// before the separately rendered preparation guidance.
		if err := json.NewDecoder(strings.NewReader(output)).Decode(&value); err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(value)
		if err != nil || strings.Contains(string(body), `"activation_attested":true`) || strings.Contains(string(body), `"authentication_attested":true`) {
			t.Fatal("Kiro public output asserted unobserved activation or authentication")
		}
	}
	f.assertPreparedState(t, project)
}

func (f kiroProjectJourney) assertPreparedState(t *testing.T, project string) {
	t.Helper()
	for _, installation := range f.state(t, project).Installations {
		for _, binding := range installation.Clients {
			verification := domain.VerificationInstalled
			if project == "" {
				verification = domain.VerificationPackageValid
				if binding.InstallIntent != domain.InstallIntentPrepare {
					t.Fatal("Kiro User preparation was not selected by the public lifecycle")
				}
			}
			if binding.Activation != domain.ActivationPrepared || binding.Verification != verification {
				t.Fatal("Kiro policy/peer fixture upgraded prepared file verification")
			}
		}
	}
}

func TestKiroUserArchivePreservesAuthoredBytesWithAutocrlf(t *testing.T) {
	transport := newProjectJourneyTransport(t, "zip")
	// Exercise the real archive consumer with a task-owned repository setting.
	// The authoring and archive commands must preserve the same literal bytes.
	projectJourneyGit(t, transport.repo, "config", "core.autocrlf", "true")
	skill := kiroUserCommitSource(t, transport, "literal User archive pin")
	f := newKiroJourney(t, transport, true)
	f.run(t, "", "add", transport.endpoint, "--alias", "user-docs", "--target", "kiro", "--scope", "user", "--yes")
	f.assertKiroUserOwned(t, skill)
}

func (f kiroProjectJourney) assertKiroUserOwned(t *testing.T, skill string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := f.owned(t, "")
	f.assertKiroSourceData(t, "", installation, binding)
	// User preparation owns exact native file receipts; the historical User
	// trait does not record a native registration profile root.
	if binding.Scope != string(domain.ScopeUser) || binding.NativeProfileRoot != "" || binding.InstallIntent != domain.InstallIntentPrepare ||
		binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationPackageValid || len(kiro.NativeObjects(binding.NativeObjects)) != 2 ||
		binding.PendingNativeIntent != nil || binding.NativeActivationAttempt != "" {
		t.Fatalf("Kiro User preparation lost scope or component receipts: %+v", binding)
	}
	if err := pathpolicy.RequireContainedChild(f.cache, binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	if err := kiro.VerifyNativeObjects(f.kiroRoot(""), binding.NativeObjects, false); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{binding.TargetLocator, f.kiroRoot("")} {
		if projectJourneyRead(t, filepath.Join(root, "skills", "uap-user-docs", "SKILL.md")) != skill {
			t.Fatal("Kiro User selected Skill differs from its owned package")
		}
	}
	receipt, ok := installation.DataReceipts[binding.DataReceiptID]
	if !ok || len(installation.DataReceipts) != 1 || receipt.DataReceiptID != binding.DataReceiptID || receipt.Scope != string(domain.ScopeUser) {
		t.Fatal("Kiro User data authority missing")
	}
	if err := pathpolicy.RequireContainedChild(filepath.Join(f.cache, "plugin-data"), receipt.Locator); err != nil {
		t.Fatal(err)
	}
	servers, _, _, _, err := kiro.ReadMCPConfig(filepath.Join(f.kiroRoot(""), "settings", "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	server, ok := servers["uap_user_probe"].(map[string]any)
	if !ok {
		t.Fatal("Kiro User selected MCP server missing")
	}
	env, ok := server["env"].(map[string]any)
	if !ok || env["DATA"] != receipt.Locator {
		t.Fatal("Kiro User MCP projection lost its original data receipt")
	}
	return installation, binding
}

// Preserve the production scoped composer, installer, ownership kernel, store,
// registry and solver. Only host discovery and security assessment are inert.
func configureKiroProjectJourney(t *testing.T, app agentpluginscli.App, home string, transport *projectJourneyTransport, runner *kiroNoNativeRunner) agentpluginscli.App {
	t.Helper()
	app.Detector = kiroProjectDetector{root: filepath.Join(home, ".kiro")}
	if runner != nil {
		app.Detector = kiroProjectDetector{root: filepath.Join(home, ".kiro"), executable: filepath.Join(home, "never-start", "kiro-cli")}
		helper, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		stateRoot := filepath.Dir(app.ManagedRoot)
		engine, err := installer.New(installer.Config{StateRoot: stateRoot, ManagedRoot: app.ManagedRoot, ProjectRoot: app.ProjectRoot,
			OperationsDir: filepath.Join(stateRoot, "operations-v2"), TempRoot: filepath.Join(stateRoot, "installer-tmp"),
			Registry: app.ClientRegistry, Runner: runner, EnableNativeObserver: true, HelperExecutable: helper, HelperVersion: version})
		if err != nil {
			t.Fatal(err)
		}
		app.Installer = engine
		activator, ok := app.Lifecycle.Activator.(providers.Activator)
		if !ok {
			t.Fatal("production Kiro activator missing")
		}
		activator.Runner = runner
		app.Lifecycle.Activator = activator
		observer, ok := app.Lifecycle.NativeObserver.(providers.NativeIdentityObserver)
		if !ok {
			t.Fatal("production Kiro observer missing")
		}
		observer.Runner = runner
		app.Lifecycle.NativeObserver = observer
	}
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.SecurityEvaluator = projectJourneySecurity{}
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
	app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: home, HTTPClient: transport.server.Client()}
	app.SourceAcquirerForDryRun = func(root string) agentpluginscli.SourceAcquirer {
		return sourceacquisition.Acquirer{TempRoot: root, HTTPClient: transport.server.Client()}
	}
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureKiroProjectJourney(t, selected, home, transport, runner), nil
	}
	return app
}

func (f kiroProjectJourney) kiroRoot(project string) string {
	if project == "" {
		return filepath.Join(f.home, ".kiro")
	}
	return filepath.Join(project, ".kiro")
}

func (f kiroProjectJourney) assertKiroOwned(t *testing.T, project, skill, endpoint string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := f.owned(t, project)
	f.assertKiroSourceData(t, project, installation, binding)
	if binding.Scope != string(domain.ScopeProject) || binding.NativeProfileRoot != f.kiroRoot(project) || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("Kiro project lifecycle lost its exact scope or conservative observation: %+v", binding)
	}
	if err := pathpolicy.RequireContainedChild(filepath.Join(project, ".uap"), binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{binding.TargetLocator, f.kiroRoot(project)} {
		if projectJourneyRead(t, filepath.Join(root, "skills", "docs", "SKILL.md")) != skill {
			t.Fatal("Kiro managed/native skill bytes differ from the selected pin")
		}
	}
	settings := projectJourneyRead(t, filepath.Join(f.kiroRoot(project), "settings", "mcp.json"))
	for _, text := range []string{endpoint, "foreign.test", "foreignPreference", `"disabled":true`} {
		if !strings.Contains(strings.ReplaceAll(settings, " ", ""), text) {
			t.Fatalf("Kiro settings lost selected or foreign member %q: %s", text, settings)
		}
	}
	return installation, binding
}

func (f kiroProjectJourney) assertKiroSourceData(t *testing.T, project string, installed domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	kind := sourcespec.KindArchive
	if strings.HasSuffix(f.transport.endpoint, "/repo.git") {
		kind = sourcespec.KindGit
	}
	if installed.Source.SourceKind != string(kind) || installed.Source.SourceURL != f.transport.endpoint || installed.Source.RequestedSource != f.transport.endpoint ||
		installed.Source.CanonicalSource != sourcespec.Key(kind, f.transport.endpoint, "") || installed.Source.PackageSubpath != "" {
		t.Fatal("Kiro source does not match its original authored transport")
	}
	if binding.PackageRevision == nil || binding.PackageRevision.TreeDigest != installed.Source.TreeDigest ||
		binding.PackageRevision.ResolvedRevision != installed.Source.ResolvedRevision || binding.PackageRevision.ManifestDigest != installed.Package.ManifestDigest {
		t.Fatal("Kiro binding revision is detached from its complete source and package")
	}
	receipt, ok := installed.DataReceipts[binding.DataReceiptID]
	if !ok || len(installed.DataReceipts) != 1 || receipt.DataReceiptID != binding.DataReceiptID ||
		receipt.Scope != binding.Scope || receipt.PhysicalBackend != binding.PhysicalArtifact || receipt.State != domain.DataReceiptOwned {
		t.Fatal("Kiro binding lacks one exact owned data receipt")
	}
	if err := (providers.PluginDataManager{Base: filepath.Join(f.namespace(project), "plugin-data")}).ValidateData(t.Context(), receipt); err != nil {
		t.Fatal(err)
	}
	var marker domain.DataReceipt
	body, err := atomicfile.ReadBoundedRegularFile(filepath.Join(receipt.Locator, ".agentplugins-data-owner.json"), 4096)
	if err != nil || json.Unmarshal(body, &marker) != nil || marker != receipt {
		t.Fatal("Kiro data marker differs from its complete original receipt", err)
	}
	if project != "" {
		if err := kiro.VerifyNativeObjects(f.kiroRoot(project), binding.NativeObjects, false); err != nil {
			t.Fatal(err)
		}
		servers, _, _, _, err := kiro.ReadMCPConfig(filepath.Join(f.kiroRoot(project), "settings", "mcp.json"))
		if err != nil {
			t.Fatal(err)
		}
		server, ok := servers["docs_data"].(map[string]any)
		if !ok {
			t.Fatal("Kiro Project data component missing")
		}
		env, ok := server["env"].(map[string]any)
		if !ok || env["DATA"] != receipt.Locator || env["PLUGIN_DATA"] != receipt.Locator {
			t.Fatal("Kiro Project projection lost exact original data authority")
		}
	}
	packages := 0
	for _, object := range binding.NativeObjects {
		if object.Kind == "managed_package_directory" {
			packages++
			if object.Path != binding.TargetLocator || object.LogicalName != installed.DeclaredName || object.ProtectionClass != "managed" || object.UserModified || object.ManagedDigest == "" {
				t.Fatal("Kiro managed package lacks its original ownership")
			}
			if err := f.app.Lifecycle.Stager.Verify(t.Context(), binding.TargetLocator, object.ManagedDigest); err != nil {
				t.Fatal(err)
			}
		}
	}
	if packages != 1 {
		t.Fatal("Kiro requires one exact managed package receipt")
	}
}

func (f kiroProjectJourney) rememberKiroData(t *testing.T, project string, installed domain.Installation) {
	t.Helper()
	projectJourneyDataMarker(t, f.projectJourney, project, installed)
	for _, receipt := range installed.DataReceipts {
		// Hold the original directory, owner marker and user bytes through every
		// later command; identical replacement bytes cannot establish continuity.
		f.preserve(t, receipt.Locator)
	}
}

// The remote component and never-executed stdio component exercise real data
// ownership. A rejecting runner prevents native or MCP execution in both scopes.
func kiroProjectCommitSource(t *testing.T, transport *projectJourneyTransport, skill, endpoint string) {
	t.Helper()
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), skill)
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"`+endpoint+`"},"docs_data":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "select inert Kiro project components")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.archive = projectJourneyArchive(t, transport.repo)
}
