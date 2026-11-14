package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type devinProjectDetector struct{ root string }

func (detector devinProjectDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientWindsurf, ConfigRoot: "", ExecutablePath: filepath.Join(detector.root, "devin"), Version: "3000.11.205", Status: domain.DetectionDetected}}, nil
}

// Reuse the core journey's production CLI/state helpers and real TLS Git/ZIP
// transports. Devin supplies its own detected profile and native layout.
type devinProjectJourney struct{ projectJourney }

type devinLifecycleJourney struct {
	devinProjectJourney
	profile   string
	client    domain.DetectedClient
	native    *devinNativeRejectCounter
	protected map[string]devinProtectedTree
	foreign   map[string]map[string]any
}

type devinNativeRejectCounter struct{ calls int }

func (counter *devinNativeRejectCounter) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	counter.calls++
	return legacyports.CommandResult{}, fmt.Errorf("unexpected Devin or MCP process: %v", command.Argv)
}

type devinProtectedTree struct {
	entries    map[string]string
	identities map[string]os.FileInfo
}

func newDevinProjectJourney(t *testing.T, transport *projectJourneyTransport) devinProjectJourney {
	t.Helper()
	return newDevinJourney(t, transport, false, false).devinProjectJourney
}

func newDevinJourney(t *testing.T, transport *projectJourneyTransport, lifecycle, user bool) devinLifecycleJourney {
	t.Helper()
	f := devinLifecycleJourney{devinProjectJourney: devinProjectJourney{projectJourney: projectJourney{home: devinJourneyTemp(t), a: devinJourneyTemp(t), b: devinJourneyTemp(t), transport: transport}}}
	f.cache = filepath.Join(f.home, "uap")
	if lifecycle {
		f.profile = filepath.Join(f.home, ".config", "devin")
		executable := filepath.Join(f.home, "never-start", "devin")
		if runtime.GOOS == "windows" {
			f.profile = filepath.Join(f.home, "AppData", "Roaming", "devin")
			executable += ".exe"
		}
		f.client = domain.DetectedClient{ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected,
			ConfigRoot: filepath.Join(f.home, ".codeium", "windsurf"), NativeConfigRoot: f.profile,
			ExecutablePath: executable, Version: "3000.11.205"}
		if user {
			f.client.Version = "3000.12.57"
		}
		f.native, f.protected, f.foreign = &devinNativeRejectCounter{}, map[string]devinProtectedTree{}, map[string]map[string]any{}
	}
	for _, root := range []string{"", f.a, f.b} {
		native := f.devinRoot(root)
		projectJourneyWrite(t, filepath.Join(native, "mcp_config.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`)
		projectJourneyWrite(t, filepath.Join(native, "skills", "foreign", "SKILL.md"), "foreign skill\n")
		projectJourneyWrite(t, filepath.Join(native, "hooks.json"), "foreign hooks\n")
	}
	if lifecycle {
		f.prepareDevinProtection(t, user)
	}
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(f.home, f.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	f.app = configureDevinProjectJourney(t, app, f.home, transport, f.client, f.native)
	return f
}

// Preserve the production scoped composer, installer, ownership kernel, store,
// registry and solver. Only host discovery and security assessment are inert.
func configureDevinProjectJourney(t *testing.T, app agentpluginscli.App, home string, transport *projectJourneyTransport, client domain.DetectedClient, counter *devinNativeRejectCounter) agentpluginscli.App {
	t.Helper()
	app.Detector = devinProjectDetector{root: filepath.Join(home, ".devin")}
	if counter != nil {
		// These accepted-version discovery inputs are inert. Neither a vendor
		// executable nor version stdout is fabricated or executed.
		app.Detector = devinUnqualifiedDetector{client: client}
		helper, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		stateRoot := filepath.Dir(app.ManagedRoot)
		engine, err := installer.New(installer.Config{StateRoot: stateRoot, ManagedRoot: app.ManagedRoot, ProjectRoot: app.ProjectRoot,
			OperationsDir: filepath.Join(stateRoot, "operations-v2"), TempRoot: filepath.Join(stateRoot, "installer-tmp"),
			Registry: app.ClientRegistry, Runner: counter, EnableNativeObserver: true, HelperExecutable: helper, HelperVersion: version})
		if err != nil {
			t.Fatal(err)
		}
		app.Installer = engine
		activator := app.Lifecycle.Activator.(providers.Activator)
		activator.Runner = counter
		app.Lifecycle.Activator = activator
		observer := app.Lifecycle.NativeObserver.(providers.NativeIdentityObserver)
		observer.Runner = counter
		app.Lifecycle.NativeObserver = observer
	}
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.SecurityEvaluator = projectJourneySecurity{}
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
	if counter != nil && transport.diagnostic == nil {
		transport.diagnostic = &projectGitDiagnostic{counts: map[string]uint16{}, endpoint: transport.server.URL + "/repo.git", log: func(record string) { t.Log(record) }}
	}
	sourceRunner := func(root string) sourceacquisition.Runner {
		if counter == nil {
			return nil
		}
		return projectGitDiagnosticRunner{recorder: transport.diagnostic, root: root}
	}
	app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: home, HTTPClient: transport.server.Client(), Runner: sourceRunner(home)}
	app.SourceAcquirerForDryRun = func(root string) agentpluginscli.SourceAcquirer {
		return sourceacquisition.Acquirer{TempRoot: root, HTTPClient: transport.server.Client(), Runner: sourceRunner(root)}
	}
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureDevinProjectJourney(t, selected, home, transport, client, counter), nil
	}
	return app
}

func (f devinProjectJourney) devinRoot(project string) string {
	return (devinLifecycleJourney{devinProjectJourney: f}).devinRoot(project)
}

func (f devinLifecycleJourney) devinRoot(project string) string {
	if project == "" {
		if f.profile != "" {
			return f.profile
		}
		return filepath.Join(f.home, ".devin")
	}
	return filepath.Join(project, ".devin")
}

func (f devinLifecycleJourney) prepareDevinProtection(t *testing.T, user bool) {
	t.Helper()
	projectJourneyWrite(t, filepath.Join(f.client.ConfigRoot, "mcp_config.json"), `{"mcpServers":{"docs":{"serverUrl":"https://cascade.invalid/mcp","disabled":true}},"legacy":"keep"}`)
	peers, selected := []string{f.devinRoot("")}, []string{f.devinRoot(f.a), f.devinRoot(f.b)}
	if user {
		peers, selected = selected, peers
	}
	for _, root := range peers {
		projectJourneyWrite(t, filepath.Join(root, "mcp_config.json"), `{"mcpServers":{"docs":{"url":"https://peer.invalid/mcp","disabled":true},"docs_data":{"url":"https://data-peer.invalid/mcp","disabled":true}},"peer":"keep"}`)
		projectJourneyWrite(t, filepath.Join(root, "skills", "docs", "SKILL.md"), "original same-name peer Skill\n")
		f.preserveDevin(t, root)
	}
	for _, root := range selected {
		var document map[string]any
		if err := json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(root, "mcp_config.json"))), &document); err != nil {
			t.Fatal(err)
		}
		f.foreign[root] = document
		f.preserveDevin(t, filepath.Join(root, "skills", "foreign"), filepath.Join(root, "hooks.json"))
	}
	f.preserveDevin(t, f.client.ConfigRoot)
}

func (f devinLifecycleJourney) preserveDevin(t *testing.T, roots ...string) {
	t.Helper()
	if err := f.checkDevinProtection(t); err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		if _, exists := f.protected[root]; exists {
			t.Fatal("cannot renew original Devin authority")
		}
		proof := devinProtectedTree{entries: securityFixtureTree(t, root), identities: map[string]os.FileInfo{}}
		for relative := range proof.entries {
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
			proof.identities[relative] = original
		}
		f.protected[root] = proof
	}
}

func (f devinLifecycleJourney) checkDevinProtection(t *testing.T) error {
	t.Helper()
	if f.native == nil {
		return nil
	}
	if f.native.calls != 0 {
		return fmt.Errorf("prepared file lifecycle executed a native/MCP process")
	}
	for root, original := range f.foreign {
		var current map[string]any
		if err := json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(root, "mcp_config.json"))), &current); err != nil {
			return err
		}
		servers, ok := current["mcpServers"].(map[string]any)
		if !ok {
			return fmt.Errorf("foreign Devin config shape changed")
		}
		delete(servers, "docs")
		delete(servers, "docs_data")
		if !reflect.DeepEqual(current, original) {
			return fmt.Errorf("foreign Devin config fields changed")
		}
	}
	for root, original := range f.protected {
		if !reflect.DeepEqual(original.entries, securityFixtureTree(t, root)) {
			return fmt.Errorf("original Devin tree bytes or membership changed")
		}
		for relative, held := range original.identities {
			current, err := os.Lstat(filepath.Join(root, relative))
			if err != nil || !os.SameFile(held, current) || held.Mode() != current.Mode() {
				return fmt.Errorf("original Devin tree identity changed")
			}
		}
	}
	return nil
}

func (f devinLifecycleJourney) run(t *testing.T, project string, args ...string) {
	t.Helper()
	if err := f.checkDevinProtection(t); err != nil {
		t.Fatal(err)
	}
	f.assertDevinPreparedState(t, project)
	output := f.projectJourney.run(t, project, args...)
	if err := f.checkDevinProtection(t); err != nil {
		t.Fatal(err)
	}
	if f.native != nil {
		var value any
		if err := json.NewDecoder(strings.NewReader(output)).Decode(&value); err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(value)
		if err != nil || strings.Contains(string(body), `"activation_attested":true`) || strings.Contains(string(body), `"authentication_attested":true`) {
			t.Fatal("public Devin response attested unobserved native activity")
		}
	}
	f.assertDevinPreparedState(t, project)
}

func (f devinLifecycleJourney) assertDevinPreparedState(t *testing.T, project string) {
	t.Helper()
	if f.native == nil {
		return
	}
	for _, installation := range f.state(t, project).Installations {
		for _, binding := range installation.Clients {
			if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || binding.Authentication == domain.AuthenticationComplete || binding.PendingNativeIntent != nil || binding.NativeActivationAttempt != "" {
				t.Fatal("public Devin lifecycle lost truthful Prepared state")
			}
		}
	}
}

func (f devinProjectJourney) assertDevinOwned(t *testing.T, project, skill, endpoint string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	return (devinLifecycleJourney{devinProjectJourney: f}).assertDevinOwned(t, project, skill, endpoint)
}

func (f devinLifecycleJourney) assertDevinOwned(t *testing.T, project, skill, endpoint string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := f.owned(t, project)
	scope := domain.ScopeProject
	if project == "" {
		scope = domain.ScopeUser
	}
	if binding.Scope != string(scope) || binding.NativeProfileRoot != f.devinRoot(project) || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("Devin project lifecycle lost its exact scope or conservative observation: %+v", binding)
	}
	if err := pathpolicy.RequireContainedChild(f.namespace(project), binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{binding.TargetLocator, f.devinRoot(project)} {
		if projectJourneyRead(t, filepath.Join(root, "skills", "docs", "SKILL.md")) != skill {
			t.Fatal("Devin managed/native skill bytes differ from the selected pin")
		}
	}
	settings := projectJourneyRead(t, filepath.Join(f.devinRoot(project), "mcp_config.json"))
	for _, text := range []string{endpoint, "foreign.test", "foreignPreference", `"disabled":true`} {
		if !strings.Contains(strings.ReplaceAll(settings, " ", ""), text) {
			t.Fatalf("Devin settings lost selected or foreign member %q: %s", text, settings)
		}
	}
	if f.native != nil {
		f.assertDevinSourcePackage(t, installation, binding)
		if err := windsurf.New().ValidateBindingProfile(binding.NativeProfileRoot, binding); err != nil {
			t.Fatal(err)
		}
		wantKind := windsurf.DevinProjectMCPKind
		if project == "" {
			wantKind = windsurf.DevinGlobalMCPKind
		}
		count := 0
		for _, object := range binding.NativeObjects {
			if object.Kind == windsurf.WindsurfMCPObjectKind {
				t.Fatal("Cascade receipt cannot prove Devin lifecycle")
			}
			if object.Kind == wantKind {
				count++
			}
		}
		receipt, ok := installation.DataReceipts[binding.DataReceiptID]
		if count != 2 || !ok || len(installation.DataReceipts) != 1 || receipt.Scope != string(scope) || receipt.PhysicalBackend != binding.PhysicalArtifact {
			t.Fatal("Devin selected components or data receipt differs")
		}
		if err := pathpolicy.RequireContainedChild(filepath.Join(f.namespace(project), "plugin-data"), receipt.Locator); err != nil {
			t.Fatal(err)
		}
		if err := (providers.PluginDataManager{Base: filepath.Join(f.namespace(project), "plugin-data")}).ValidateData(t.Context(), receipt); err != nil {
			t.Fatal(err)
		}
		var document struct {
			Servers map[string]struct {
				Env map[string]string `json:"env"`
			} `json:"mcpServers"`
		}
		if json.Unmarshal([]byte(settings), &document) != nil || document.Servers["docs_data"].Env["DATA"] != receipt.Locator {
			t.Fatal("Devin stdio projection lost exact data authority")
		}
	}
	return installation, binding
}

func (f devinLifecycleJourney) assertDevinSourcePackage(t *testing.T, installation domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	kind := sourcespec.KindArchive
	if strings.HasSuffix(f.transport.endpoint, "/repo.git") {
		kind = sourcespec.KindGit
	}
	if installation.Source.SourceKind != string(kind) || installation.Source.SourceURL != f.transport.endpoint || installation.Source.RequestedSource != f.transport.endpoint ||
		installation.Source.CanonicalSource != sourcespec.Key(kind, f.transport.endpoint, "") || installation.Source.PackageSubpath != "" {
		t.Fatal("Devin source differs from original authored transport")
	}
	manifest := projectJourneyRead(t, filepath.Join(f.transport.repo, "plugin.json"))
	if installation.Package.ManifestDigest != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(manifest))) || binding.PackageRevision == nil ||
		binding.PackageRevision.TreeDigest != installation.Source.TreeDigest || binding.PackageRevision.ManifestDigest != installation.Package.ManifestDigest {
		t.Fatal("Devin installed revision does not join its exact source manifest")
	}
	count, digest := 0, ""
	for _, object := range binding.NativeObjects {
		if object.Kind == "managed_package_directory" {
			count++
			if object.Path != binding.TargetLocator || object.LogicalName != installation.DeclaredName || object.ProtectionClass != "managed" || object.UserModified {
				t.Fatal("Devin package ownership differs")
			}
			digest = object.ManagedDigest
		}
	}
	var latest domain.MutationReceipt
	for _, receipt := range binding.Receipts {
		if receipt.Sequence > latest.Sequence {
			latest = receipt
		}
	}
	if count != 1 || digest == "" || latest.MutationType != "directory_swap" || latest.Phase != "committed" ||
		latest.ClientBindingID != binding.ClientBindingID || latest.ActivePath != binding.TargetLocator || latest.AfterDigest != digest {
		t.Fatal("Devin package digest lacks its committed directory receipt")
	}
	if err := f.app.Lifecycle.Stager.Verify(t.Context(), binding.TargetLocator, digest); err != nil {
		t.Fatal(err)
	}
}

// Preserve the original HTTP-only source used by the existing version/cache
// contracts. Only the full lifecycle explicitly adds a never-executed stdio
// component to exercise genuine production data allocation and retention.
func devinProjectCommitSource(t *testing.T, transport *projectJourneyTransport, skill, endpoint string) {
	t.Helper()
	devinCommitSource(t, transport, skill, endpoint, false)
}

func devinCommitSource(t *testing.T, transport *projectJourneyTransport, skill, endpoint string, data bool) {
	t.Helper()
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), skill)
	stdio := ""
	if data {
		stdio = `,"docs_data":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}`
	}
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"`+endpoint+`"}`+stdio+`}}`)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "select inert Devin project components")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.archive = projectJourneyArchive(t, transport.repo)
}

func devinJourneyTemp(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
