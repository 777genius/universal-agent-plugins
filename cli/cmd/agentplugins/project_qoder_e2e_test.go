package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type projectQoderDetector struct{ root string }

func (d projectQoderDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientQoder, ConfigRoot: d.root, Status: domain.DetectionDetected}}, nil
}

type projectQoderNoNative struct{ calls int }

func (r *projectQoderNoNative) Run(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, fmt.Errorf("unexpected Qoder native command %v", c.Argv)
}

func configureProjectQoder(t *testing.T, app agentpluginscli.App, user string, runner *projectQoderNoNative) agentpluginscli.App {
	t.Helper()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: filepath.Dir(app.ManagedRoot), ManagedRoot: app.ManagedRoot, ProjectRoot: app.ProjectRoot, Registry: app.ClientRegistry, Runner: runner, EnableNativeObserver: true, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	app.Installer = engine
	app.Detector = projectQoderDetector{user}
	// Retain the composition root's explicit native file kernel. This test
	// substitutes only command execution and the selected client identity.
	activator, ok := app.Lifecycle.Activator.(providers.Activator)
	if !ok || activator.NativeConfig == nil {
		t.Fatal("Qoder fixture requires the composed native config kernel")
	}
	activator.Registry, activator.Runner = app.ClientRegistry, runner
	app.Lifecycle.Activator = activator
	observer, ok := app.Lifecycle.NativeObserver.(providers.NativeIdentityObserver)
	if !ok || observer.NativeConfig == nil {
		t.Fatal("Qoder fixture requires the composed native observer kernel")
	}
	observer.Stager, observer.Registry, observer.Runner = app.Lifecycle.Stager, app.ClientRegistry, runner
	app.Lifecycle.NativeObserver = observer
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectQoder(t, selected, user, runner), nil
	}
	return app
}

// Actual saved CLI/TLS Git, immutable pin, typed state and directory
// transactions. Every native-client command is forbidden by the injected runner.
func TestProjectQoderSavedLifecycleSeparatesUserAndProjects(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".qoder")
	projectJourneyWrite(t, filepath.Join(user, "plugins", "installed_plugins_v2.json"), "original unrelated global registry")
	// Keep an actual original user installation/state as an independent sentinel.
	// Qoder user registry mutation is intentionally outside this project slice.
	journey.run(t, "", "add", transport.endpoint, "--alias", "user-docs", "--target", "gemini")
	userGemini := securityFixtureTree(t, filepath.Join(journey.home, ".gemini"))
	runner := &projectQoderNoNative{}
	journey.app = configureProjectQoder(t, journey.app, user, runner)
	userBefore := securityFixtureTree(t, user)
	userStateBefore := projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json"))
	for _, project := range []string{journey.a, journey.b} {
		journey.run(t, project, "add", transport.endpoint, "--alias", "docs", "--target", "qoder")
		projectQoderOwned(t, journey, project, projectJourneySkill)
	}
	bBefore := securityFixtureTree(t, journey.b)
	installation, binding := journey.owned(t, journey.a)
	for _, args := range [][]string{
		{"add", transport.endpoint, "--alias", "docs", "--target", "qoder"},
		{"update", "docs", "--target", "qoder"},
		{"repair", installation.InstallationID, "--target", "qoder"},
	} {
		state := projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json"))
		if _, err := journey.execute(journey.a, append(append([]string(nil), args...), "--activation-complete")...); err == nil {
			t.Fatal("saved Qoder request accepted attestation")
		}
		if state != projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json")) {
			t.Fatal("refused attestation changed state")
		}
		output := journey.run(t, journey.a, args...)
		if strings.Contains(output, `"mutated":true`) || state != projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json")) {
			t.Fatal("intact saved Prepared operation changed state", output)
		}
	}
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	view := filepath.Join(binding.TargetLocator, ".agentplugins-qoder-native")
	projectJourneyWrite(t, filepath.Join(view, "skills", "docs", "SKILL.md"), "managed view drift")
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "advance disposable Qoder source")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "qoder")
	projectQoderOwned(t, journey, journey.a, projectJourneySkill)
	if err := os.RemoveAll(binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "qoder")
	projectQoderOwned(t, journey, journey.a, projectJourneySkill)
	journey.run(t, journey.a, "update", "docs", "--target", "qoder")
	updated, current := projectQoderOwned(t, journey, journey.a, projectJourneyAdvancedSkill)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
		t.Fatal("saved update lost immutable source or binding authority")
	}
	calls := runner.calls
	status := journey.run(t, journey.a, "deps", "status")
	if runner.calls != calls || !strings.Contains(status, "activation_prepared") || strings.Contains(status, "activation_active") {
		t.Fatal("status called native or promoted activation", status)
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "qoder")
	projectJourneyAbsent(t, current.TargetLocator)
	if runner.calls != 0 || !reflect.DeepEqual(userGemini, securityFixtureTree(t, filepath.Join(journey.home, ".gemini"))) || !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) || userStateBefore != projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json")) {
		t.Fatal("A lifecycle changed original user state/native registry or B")
	}
}

// Both project paths must hold exactly the selected immutable package, MCP
// member and Skill directory; no inferred user/global ownership is allowed.
func projectQoderAssertNativeReceipts(t *testing.T, b domain.ClientBinding, project string, includeHTTP bool) {
	t.Helper()
	objects := b.NativeObjects
	wantCount, skillIndex := 3, 2
	if includeHTTP {
		wantCount, skillIndex = 4, 3
	}
	if len(objects) != wantCount ||
		objects[0].Kind != "managed_package_directory" || objects[0].Path != b.TargetLocator ||
		objects[1].ObjectID != "qoder_project_mcp_server:docs" ||
		objects[1].Kind != "qoder_project_mcp_server" ||
		objects[1].Path != filepath.Join(project, ".mcp.json") ||
		objects[skillIndex].ObjectID != "qoder_project_skill_directory:docs" ||
		objects[skillIndex].Kind != "qoder_project_skill_directory" ||
		objects[skillIndex].Path != filepath.Join(project, ".qoder", "skills", "docs") ||
		objects[skillIndex].SourceRelative != "skills/docs" {
		t.Fatalf("Qoder project component receipt authority differs: %+v", objects)
	}
	if includeHTTP {
		http := objects[2]
		if http.ObjectID != "qoder_project_mcp_server:remote" || http.Kind != "qoder_project_mcp_server" ||
			http.Path != filepath.Join(project, ".mcp.json") {
			t.Fatalf("Qoder HTTP MCP is missing its exact receipt: %+v", http)
		}
	}
	for _, object := range objects {
		if !strings.HasPrefix(object.ManagedDigest, "sha256:") || object.ProtectionClass != "managed" {
			t.Fatalf("Qoder project receipt does not carry exact owned digest: %+v", object)
		}
	}
}

func projectQoderOwned(t *testing.T, j projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, b := j.owned(t, project)
	if b.ClientID != "qoder" || b.Scope != "project" || b.NativeProfileRoot != filepath.Join(project, ".uap", "managed", "clients", "qoder") || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationInstalled {
		t.Fatalf("Qoder lost exact project native authority %+v", b)
	}
	projectQoderAssertNativeReceipts(t, b, project, false)
	if projectJourneyRead(t, filepath.Join(project, ".qoder", "skills", "docs", "SKILL.md")) != expected ||
		projectJourneyRead(t, filepath.Join(b.TargetLocator, ".agentplugins-qoder-native", "skills", "docs", "SKILL.md")) != expected {
		t.Fatal("native import view differs from selected immutable skill")
	}
	for _, surface := range []string{"mcp.json", ".mcp.json", "hooks", "agents"} {
		if _, err := os.Lstat(filepath.Join(b.TargetLocator, ".agentplugins-qoder-native", surface)); !os.IsNotExist(err) {
			t.Fatal("unqualified native resource projected", surface, err)
		}
	}
	return installation, b
}

// Exercise the same direct local, explicit-target CLI group route as the native
// cycle. No Qoder process is permitted; repair and retention use production APIs.
func TestProjectQoderDirectGroupRepairAndDefaultRemovalKeepExactAuthority(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	// Extend the existing real Git fixture with one portable Streamable HTTP
	// peer; do not create a second Qoder E2E package or native harness.
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}},"remote":{"type":"streamable-http","url":"https://example.test/mcp"}}}`)
	projectJourneyGit(t, transport.repo, "add", "mcp.json")
	projectJourneyGit(t, transport.repo, "commit", "-m", "add disposable HTTP peer to Qoder fixture")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	j := newProjectJourney(t, transport)
	projectJourneyWrite(t, filepath.Join(j.a, ".mcp.json"), `{"mcpServers":{"foreign":{"command":"foreign"}},"note":"keep"}`)
	user := filepath.Join(j.home, ".qoder")
	projectJourneyWrite(t, filepath.Join(user, "plugins", "installed_plugins_v2.json"), "original registry\n")
	runner := &projectQoderNoNative{}
	j.app = configureProjectQoder(t, j.app, user, runner)
	beforeUser := securityFixtureTree(t, user)
	for _, project := range []string{j.a, j.b} {
		j.run(t, project, "add", transport.repo, "--target", "qoder", "--no-save")
	}
	installed, binding := projectQoderDirectOwned(t, j, projectJourneySkill)
	beforeB := securityFixtureTree(t, j.b)
	// This selected package includes an stdio MCP server with ${PLUGIN_DATA}.
	// Native Project MCP support therefore requires one exact owned data
	// receipt; it is not the historical Skills-only prepared-import route.
	if len(installed.DataReceipts) != 1 || installed.DataRetained || binding.DataReceiptID == "" {
		t.Fatalf("selected MCP did not allocate one exact data receipt: %+v %+v", installed.DataReceipts, binding)
	}
	allocated, ok := installed.DataReceipts[binding.DataReceiptID]
	if !ok || allocated.DataReceiptID != binding.DataReceiptID ||
		allocated.Scope != "project" || allocated.State != domain.DataReceiptOwned ||
		allocated.PhysicalBackend != binding.PhysicalArtifact ||
		allocated.Locator != filepath.Join(j.a, ".uap", "plugin-data", binding.PhysicalArtifact) ||
		!strings.HasPrefix(allocated.OwnershipDigest, "sha256:") {
		t.Fatalf("Qoder selected stdio PLUGIN_DATA ownership is not exact: %+v", allocated)
	}
	stateBefore := projectJourneyRead(t, filepath.Join(j.a, ".uap", "state-v2.json"))
	for _, args := range [][]string{{"add", transport.repo, "--target", "qoder", "--no-save"}, {"update", installed.DeclaredName, "--target", "qoder"}} {
		output := j.run(t, j.a, args...)
		if !strings.Contains(output, `"mutated":false`) || stateBefore != projectJourneyRead(t, filepath.Join(j.a, ".uap", "state-v2.json")) {
			t.Fatal("direct unchanged group changed state", output)
		}
	}
	output := j.run(t, j.a, "repair", installed.InstallationID, "--target", "qoder")
	intact, current := projectQoderDirectOwned(t, j, projectJourneySkill)
	if !strings.Contains(output, `"mutated":true`) || intact.InstallationID != installed.InstallationID || current.ClientBindingID != binding.ClientBindingID || intact.Source.TreeDigest != installed.Source.TreeDigest {
		t.Fatal("explicit direct CLI group repair lost exact restaging authority", output)
	}
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	j.run(t, j.a, "update", transport.repo, "--target", "qoder")
	updated, current := projectQoderDirectOwned(t, j, projectJourneyAdvancedSkill)
	if updated.Source.TreeDigest == installed.Source.TreeDigest || current.TargetLocator != binding.TargetLocator {
		t.Fatal("same-version direct update lost source or physical authority")
	}
	// This inert CLI contract owns the complete selected package. The hosted
	// cycle separately authorizes each leaf before nonrecursive fault removal.
	if err := os.RemoveAll(current.TargetLocator); err != nil {
		t.Fatal(err)
	}
	j.run(t, j.a, "repair", installed.InstallationID, "--target", "qoder")
	repaired, _ := projectQoderDirectOwned(t, j, projectJourneyAdvancedSkill)
	if repaired.Source.TreeDigest != updated.Source.TreeDigest {
		t.Fatal("frozen local repair did not restore its exact recorded digest")
	}
	j.run(t, j.a, "remove", installed.DeclaredName, "--target", "qoder")
	projectJourneyAbsent(t, current.TargetLocator)
	mcpAfterRemove := projectJourneyRead(t, filepath.Join(j.a, ".mcp.json"))
	if strings.Contains(mcpAfterRemove, `"docs"`) || strings.Contains(mcpAfterRemove, `"remote"`) ||
		!strings.Contains(mcpAfterRemove, `"foreign"`) || !strings.Contains(mcpAfterRemove, `"note":"keep"`) {
		t.Fatal("Qoder mixed removal retained selected server or lost foreign members", mcpAfterRemove)
	}
	state := j.state(t, j.a)
	if len(state.Installations) != 1 || !state.Installations[0].DataRetained ||
		len(state.Installations[0].Clients) != 0 || len(state.Installations[0].DataReceipts) != 1 {
		t.Fatal("default Qoder MCP removal did not retain its exact owner allocation")
	}
	if retained, ok := state.Installations[0].DataReceipts[allocated.DataReceiptID]; !ok ||
		retained.DataReceiptID != allocated.DataReceiptID || retained.OwnershipDigest != allocated.OwnershipDigest ||
		retained.Locator != allocated.Locator {
		t.Fatal("Qoder removal changed retained plugin data authority")
	}
	if runner.calls != 0 || !reflect.DeepEqual(beforeUser, securityFixtureTree(t, user)) || !reflect.DeepEqual(beforeB, securityFixtureTree(t, j.b)) {
		t.Fatal("direct A lifecycle invoked native or changed original user/B authority")
	}
}

func projectQoderDirectOwned(t *testing.T, j projectJourney, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	state := j.state(t, j.a)
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatal("direct --no-save route lost exact unsaved state")
	}
	installation := state.Installations[0]
	for _, binding := range installation.Clients {
		if binding.ClientID != "qoder" || binding.Scope != "project" || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || binding.NativeProfileRoot != filepath.Join(j.a, ".uap", "managed", "clients", "qoder") {
			t.Fatal("direct native Skill receipt ownership differs")
		}
		projectQoderAssertNativeReceipts(t, binding, j.a, true)
		mcp := projectJourneyRead(t, filepath.Join(j.a, ".mcp.json"))
		if !strings.Contains(mcp, `"docs"`) || !strings.Contains(mcp, `"remote"`) ||
			!strings.Contains(mcp, `"type":"http"`) || !strings.Contains(mcp, `"foreign"`) ||
			!strings.Contains(mcp, `"note":"keep"`) {
			t.Fatal("mixed Qoder stdio/HTTP native MCP or foreign neighbor differs", mcp)
		}
		if projectJourneyRead(t, filepath.Join(j.a, ".qoder", "skills", "docs", "SKILL.md")) != expected ||
			projectJourneyRead(t, filepath.Join(binding.TargetLocator, ".agentplugins-qoder-native", "skills", "docs", "SKILL.md")) != expected {
			t.Fatal("direct Qoder native view does not match frozen source")
		}
		return installation, binding
	}
	t.Fatal("direct binding missing")
	return domain.Installation{}, domain.ClientBinding{}
}
