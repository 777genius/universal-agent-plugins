package installer_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func claudeSDKBytes(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestProjectClaudePublicInstallerLifecycleSeparatesUserAndProjects(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, pkg := filepath.Join(base, "user", ".claude"), filepath.Join(base, "package")
	for _, root := range []string{user, filepath.Join(base, "A", ".claude"), filepath.Join(base, "B", ".claude")} {
		claudeSDKWrite(t, filepath.Join(root, "settings.json"), `{"foreign":true}`)
		claudeSDKWrite(t, filepath.Join(root, "skills", "foreign", "SKILL.md"), "foreign skill\n")
	}
	claudeSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-claude","version":"1.0.0"}`)
	const original = "---\nname: docs\ndescription: Inert SDK fixture\n---\nOriginal pinned bytes.\n"
	claudeSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), original)
	claudeSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	claudeSDKWrite(t, filepath.Join(pkg, "server.js"), "// Inert SDK fixture; never executed.\n")
	fixtures := make([]*claudeSDKFixture, 0, 3)
	for _, project := range []string{"", filepath.Join(base, "A"), filepath.Join(base, "B")} {
		f := newClaudeSDKFixture(t, base, project, user, pkg)
		f.apply(t, installer.OpInstall, "install")
		if f.dataRoot == "" {
			t.Fatal("SDK omitted persistent data locator")
		}
		claudeSDKWrite(t, filepath.Join(f.dataRoot, "keep.txt"), "retained scoped data\n")
		binding := f.binding(t)
		wantActivation := domain.ActivationActive
		if project != "" {
			wantActivation = domain.ActivationPrepared
		}
		if binding.Activation != wantActivation || binding.Scope != string(f.request.Scope) {
			t.Fatalf("SDK scoped binding=%+v", binding)
		}
		fixtures = append(fixtures, f)
	}
	claudeSDKUpdateRepairRemove(t, fixtures, original)
}

func claudeSDKUpdateRepairRemove(t *testing.T, fixtures []*claudeSDKFixture, original string) {
	t.Helper()
	f := fixtures[1]
	userBinding, bBinding := fixtures[0].binding(t), fixtures[2].binding(t)
	beforeUser, beforeB := claudeSDKBytes(t, filepath.Join(fixtures[0].state, "state-v2.json")), claudeSDKBytes(t, filepath.Join(fixtures[2].state, "state-v2.json"))
	binding := f.binding(t)
	path := filepath.Join(binding.TargetLocator, claude.ClaudeRuntimeDirectory, "skills", "docs", "SKILL.md")
	if result := f.apply(t, installer.OpUpdate, "unchanged"); result.Outcome != installer.OutcomeUnchanged {
		t.Fatalf("unchanged SDK update=%+v", result)
	}
	claudeSDKWrite(t, path, "external drift\n")
	claudeSDKWrite(t, filepath.Join(f.packageRoot, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Advanced SDK fixture\n---\nMoving source bytes.\n")
	claudeSDKRepairPinned(t, f, original)
	if claudeSDKBytes(t, path) != original {
		t.Fatal("repair followed moving source")
	}
	claudeSDKInterruptedUpdateAndRestart(t, f)
	if claudeSDKBytes(t, path) == original {
		t.Fatal("update did not advance selected project")
	}
	view, err := f.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatalf("SDK status=%+v,%v", view, err)
	}
	f.apply(t, installer.OpRemove, "remove-selected-project")
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("SDK artifact survived remove: %v", err)
	}
	if !reflect.DeepEqual(userBinding, fixtures[0].binding(t)) || !reflect.DeepEqual(bBinding, fixtures[2].binding(t)) || beforeUser != claudeSDKBytes(t, filepath.Join(fixtures[0].state, "state-v2.json")) || beforeB != claudeSDKBytes(t, filepath.Join(fixtures[2].state, "state-v2.json")) {
		t.Fatal("project SDK lifecycle changed user/project B ownership")
	}
	for _, other := range []domain.ClientBinding{userBinding, bBinding} {
		if claudeSDKBytes(t, filepath.Join(other.TargetLocator, claude.ClaudeRuntimeDirectory, "skills", "docs", "SKILL.md")) != original {
			t.Fatal("project SDK lifecycle changed another native package")
		}
	}
	if claudeSDKBytes(t, filepath.Join(f.project, ".claude", "settings.json")) != `{"foreign":true}` {
		t.Fatal("SDK lifecycle modified foreign settings")
	}
	for _, selected := range fixtures {
		if claudeSDKBytes(t, filepath.Join(selected.dataRoot, "keep.txt")) != "retained scoped data\n" {
			t.Fatal("project SDK lifecycle consumed scoped data")
		}
	}
}

func claudeSDKInterruptedUpdateAndRestart(t *testing.T, f *claudeSDKFixture) {
	t.Helper()
	f.failAfterCommit = true
	f.request.Operation, f.request.OperationID = installer.OpUpdate, "update-listing-unavailable-after-commit"
	handle, err := f.engine.Prepare(t.Context(), f.request)
	if err != nil {
		t.Fatal(err)
	}
	result, applyErr := f.engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	if applyErr == nil || result.Outcome != installer.OutcomeIncomplete || !result.Mutated {
		t.Fatalf("postcommit listing failure=%+v,%v", result, applyErr)
	}
	if binding := f.binding(t); binding.Activation == domain.ActivationActive || binding.TargetLocator != result.Binding.TargetPath {
		t.Fatalf("failed native observation manufactured active or lost binding=%+v", binding)
	}
	fresh := newClaudeSDKFixture(t, filepath.Dir(f.executable), f.project, f.user, f.packageRoot)
	fresh.dataRoot = f.dataRoot
	*f = *fresh
	f.apply(t, installer.OpRepair, "repair-after-process-restart")
	if binding := f.binding(t); binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("fresh public SDK did not preserve blocked project discovery=%+v", binding)
	}
}

func claudeSDKRepairPinned(t *testing.T, f *claudeSDKFixture, original string) {
	t.Helper()
	pinned := t.TempDir()
	for _, name := range []string{"plugin.json", "mcp.json", "server.js"} {
		claudeSDKWrite(t, filepath.Join(pinned, name), claudeSDKBytes(t, filepath.Join(f.packageRoot, name)))
	}
	claudeSDKWrite(t, filepath.Join(pinned, "skills", "docs", "SKILL.md"), original)
	f.request.PackageRoot, f.request.SourceRoot = pinned, f.packageRoot
	f.apply(t, installer.OpRepair, "repair-installed-pin")
	f.request.PackageRoot, f.request.SourceRoot = f.packageRoot, ""
}

func TestProjectClaudePreparedInitialAddReplayStatusAndChangedBytes(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(base, "A")
	user := filepath.Join(base, "user", ".claude")
	pkg := filepath.Join(base, "package")
	claudeSDKWrite(t, filepath.Join(user, "settings.json"), `{"foreign":true}`)
	claudeSDKWrite(t, filepath.Join(project, ".claude", "settings.json"), `{"foreign":true}`)
	claudeSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"prepared-initial","version":"1.0.0"}`)
	sourceSkill := filepath.Join(pkg, "skills", "docs", "SKILL.md")
	claudeSDKWrite(t, sourceSkill, "---\nname: docs\ndescription: Inert project marker\n---\nVersion one.\n")
	f := newClaudeSDKFixture(t, base, project, user, pkg)
	f.request.RequiredComponents = []string{"skills"}
	f.apply(t, installer.OpInstall, "initial-untrusted-no-receipt")
	if binding := f.binding(t); binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("initial untrusted binding=%+v", binding)
	}
	calls := f.runner.calls
	view, err := f.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required || f.runner.calls != calls {
		t.Fatalf("host-free status=%+v,%v calls=%d/%d", view, err, f.runner.calls, calls)
	}
	replay := f.apply(t, installer.OpInstall, "repeat-blocked-add")
	if replay.Outcome != installer.OutcomeUnchanged || replay.Mutated || f.runner.calls <= calls {
		t.Fatalf("stable prepared replay=%+v calls=%d/%d", replay, f.runner.calls, calls)
	}
	binding := f.binding(t)
	installed := filepath.Join(binding.TargetLocator, claude.ClaudeRuntimeDirectory, "skills", "docs", "SKILL.md")
	before := claudeSDKBytes(t, installed)
	claudeSDKWrite(t, sourceSkill, "---\nname: docs\ndescription: Updated project marker\n---\nVersion two.\n")
	updated := f.apply(t, installer.OpUpdate, "changed-bytes-still-blocked")
	if updated.Outcome == installer.OutcomeUnchanged || !updated.Mutated || claudeSDKBytes(t, installed) == before {
		t.Fatalf("changed owned bytes called unchanged=%+v", updated)
	}
	if binding := f.binding(t); binding.Activation != domain.ActivationPrepared {
		t.Fatalf("changed bytes promoted discovery=%+v", binding)
	}
	f.apply(t, installer.OpRemove, "remove-inactive-owned-project")
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("inactive owned artifact survived=%v", err)
	}
}
