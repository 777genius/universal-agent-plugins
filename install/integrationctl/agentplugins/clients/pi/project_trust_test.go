package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func piTrustTemp(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func piTrustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func piTrustRequest(t *testing.T, user, project, artifact string, skills, servers []string) domain.ActivationRequest {
	t.Helper()
	target, err := domain.NewTargetContext(domain.ScopeProject, project, user)
	if err != nil {
		t.Fatal(err)
	}
	anchor := filepath.Join(project, ".pi")
	native := filepath.Join(anchor, "extensions")
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientPi, Scope: domain.ScopeProject, TargetContext: target, PackageMode: domain.PackageNative,
		InstallIntent: domain.InstallIntentAutomatic, NativeRegistryRoot: anchor, TargetAnchor: anchor, TargetRoot: native,
		PhysicalArtifactID: artifact, ActivePath: filepath.Join(native, artifact),
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(skills)+len(servers))
	for _, name := range skills {
		objects = append(objects, domain.NativeObjectOwnership{Kind: piProjectSkillKind, LogicalName: name, Path: filepath.Join(anchor, "skills", name)})
	}
	for _, name := range servers {
		objects = append(objects, domain.NativeObjectOwnership{Kind: piProjectMCPKind, LogicalName: name, Path: filepath.Join(anchor, "mcp.json")})
	}
	return domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientPi, ConfigRoot: user}, Plan: plan,
		Delivery: domain.StagedDelivery{ClientID: domain.ClientPi, ActivePath: plan.ActivePath, ArtifactDigest: "sha256:test", NativeObjects: objects},
	}
}

func piTrustRemoval(request domain.ActivationRequest) domain.DeactivationRequest {
	return domain.DeactivationRequest{
		TargetContext: request.Plan.TargetContext, NativeRegistryRoot: request.Plan.NativeRegistryRoot, Client: request.Client,
		Confirmed: true, PhysicalArtifactID: request.Plan.PhysicalArtifactID, ManagedArtifactPath: request.Plan.ActivePath,
	}
}

// piTrustNode executes the generated hook bytes exactly as Pi's extension
// factory would and returns its project_trust answer for cwd.
func piTrustNode(t *testing.T, node, user, home, cwd string) string {
	t.Helper()
	_, entry, _ := piTrustHookPaths(user)
	module := strings.TrimSuffix(entry, ".ts") + ".mjs"
	body, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	piTrustWrite(t, module, string(body))
	defer func() { _ = os.Remove(module) }()
	const runner = `import { pathToFileURL } from "node:url";
const [hook, cwd] = process.argv.slice(1);
const handlers = [];
(await import(pathToFileURL(hook).href)).default({ on: (name, handler) => { if (name === "project_trust") handlers.push(handler); } });
if (handlers.length !== 1) throw new Error("project_trust handler registration");
process.stdout.write(JSON.stringify(await handlers[0]({ type: "project_trust", cwd }, {})));`
	command := exec.CommandContext(t.Context(), node, "--input-type=module", "-e", runner, module, cwd)
	command.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("execute generated Pi hook: %v", err)
	}
	var answer map[string]any
	if err := json.Unmarshal(output, &answer); err != nil || len(answer) != 1 {
		t.Fatalf("hook answer %q: %v", output, err)
	}
	trusted, _ := answer["trusted"].(string)
	return trusted
}

// The generated hook is the runtime authority. Executing its exact bytes
// proves the owned-only decision, and the Go explanation must agree with it.
func TestPiProjectTrustHookAnswersOnlyForOwnedOnlyProjects(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required to execute the generated Pi hook")
	}
	root := piTrustTemp(t)
	home := filepath.Join(root, "home")
	user := filepath.Join(home, ".pi", "agent")
	work := filepath.Join(root, "work")
	project, other := filepath.Join(work, "project"), filepath.Join(root, "other")
	artifact := "owned-0123456789ab"
	for _, dir := range []string{user, filepath.Join(project, ".pi", "extensions", artifact), filepath.Join(other, ".pi", "extensions", artifact)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	config := filepath.Join(project, ".pi")
	ownedMCP := `{"mcpServers":{"owned":{"command":"true"}}}`
	piTrustWrite(t, filepath.Join(config, "skills", "owned", "SKILL.md"), "---\nname: owned\ndescription: d\n---\n")
	piTrustWrite(t, filepath.Join(config, "mcp.json"), ownedMCP)
	piTrustWrite(t, filepath.Join(config, "mcp.json.agentplugins.lock"), "")
	env := clients.Env{NativeConfig: nativeconfig.New()}
	request := piTrustRequest(t, user, project, artifact, []string{"owned"}, []string{"owned"})
	for _, grant := range []domain.ActivationRequest{request, piTrustRequest(t, user, other, artifact, nil, nil)} {
		if err := grantPiProjectTrust(env, grant); err != nil {
			t.Fatal(err)
		}
	}
	expect := func(stage, want string) {
		t.Helper()
		got := piTrustNode(t, node, user, home, project)
		blocker, err := piTrustBlocker(env.NativeConfig, user, project)
		if got != want || err != nil || (blocker == "") != (want == "yes") {
			t.Fatalf("%s: hook %q, want %q; Go explanation %q, %v", stage, got, want, blocker, err)
		}
	}
	expect("owned resources only", "yes")
	remove := func(path string) func() { return func() { _ = os.RemoveAll(path) } }
	for _, foreign := range []struct {
		name       string
		add, after func()
	}{
		{"foreign project extension", func() {
			piTrustWrite(t, filepath.Join(config, "extensions", "foreign.ts"), "export default () => {}\n")
		}, remove(filepath.Join(config, "extensions", "foreign.ts"))},
		{"later foreign skill", func() { piTrustWrite(t, filepath.Join(config, "skills", "foreign", "SKILL.md"), "x\n") }, remove(filepath.Join(config, "skills", "foreign"))},
		{"foreign prompt", func() { piTrustWrite(t, filepath.Join(config, "prompts", "foreign.md"), "x\n") }, remove(filepath.Join(config, "prompts"))},
		{"foreign MCP server", func() {
			piTrustWrite(t, filepath.Join(config, "mcp.json"), `{"mcpServers":{"owned":{"command":"true"},"foreign":{"command":"true"}}}`)
		}, func() { piTrustWrite(t, filepath.Join(config, "mcp.json"), ownedMCP) }},
		{"project settings", func() { piTrustWrite(t, filepath.Join(config, "settings.json"), "{}\n") }, remove(filepath.Join(config, "settings.json"))},
		{"ancestor .agents skills", func() { piTrustWrite(t, filepath.Join(work, ".agents", "skills", "x", "SKILL.md"), "x\n") }, remove(filepath.Join(work, ".agents"))},
		{"saved Pi decision", func() { piTrustWrite(t, filepath.Join(user, "trust.json"), `{"`+filepath.ToSlash(project)+`":false}`) }, remove(filepath.Join(user, "trust.json"))},
		{"Pi default policy", func() { piTrustWrite(t, filepath.Join(user, "settings.json"), `{"defaultProjectTrust":"never"}`) }, remove(filepath.Join(user, "settings.json"))},
	} {
		foreign.add()
		expect(foreign.name, "undecided")
		foreign.after()
		expect(foreign.name+" withdrawn", "yes")
	}
	if err := revokePiProjectTrust(env, piTrustRemoval(request)); err != nil {
		t.Fatal(err)
	}
	if got := piTrustNode(t, node, user, home, project); got != "undecided" {
		t.Fatalf("revoked project still trusted: %q", got)
	}
	if got := piTrustNode(t, node, user, home, other); got != "yes" {
		t.Fatalf("revoking one project changed another: %q", got)
	}
}

type piTrustFailingFiles struct{ fail string }

func (piTrustFailingFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, 0o600, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	body, err := os.ReadFile(path)
	return body, info.Mode().Perm(), err == nil, err
}

func (files piTrustFailingFiles) WriteAtomic(path string, body []byte, mode os.FileMode) error {
	if path == files.fail {
		return errors.New("injected hook write failure")
	}
	return os.WriteFile(path, body, mode)
}

func (piTrustFailingFiles) RemoveNoFollow(path string) error { return os.Remove(path) }

func TestPiProjectTrustRecordGrantRevokeAndRollback(t *testing.T) {
	root := piTrustTemp(t)
	user := filepath.Join(root, "home", ".pi", "agent")
	projectA, projectB := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, dir := range []string{user, projectA, projectB} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	env := clients.Env{NativeConfig: nativeconfig.New()}
	dir, entry, record := piTrustHookPaths(user)
	a1 := piTrustRequest(t, user, projectA, "one-0123456789ab", []string{"skill"}, nil)
	a2 := piTrustRequest(t, user, projectA, "two-0123456789ab", nil, []string{"server"})
	b1 := piTrustRequest(t, user, projectB, "one-0123456789ab", nil, nil)
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	absent := func(stage string, paths ...string) {
		t.Helper()
		for _, path := range paths {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("%s: %s exists: %v", stage, path, err)
			}
		}
	}

	// Read-only verification observes and reports, but never grants.
	verify := a1
	verify.VerifyOnly = true
	if action := piProjectTrustAction(env, verify); action != piProjectTrustPendingAction {
		t.Fatalf("verification action: %q", action)
	}
	if err := grantPiProjectTrust(env, verify); err == nil {
		t.Fatal("verification granted project trust")
	}
	absent("verification", dir)

	// A failed hook write restores the exact earlier record.
	failing := clients.Env{NativeConfig: nativeconfig.NewWithFileIO(piTrustFailingFiles{fail: entry})}
	if err := grantPiProjectTrust(failing, a1); err == nil {
		t.Fatal("injected hook failure was ignored")
	}
	absent("first failed grant", entry, record)
	for _, request := range []domain.ActivationRequest{a1, b1} {
		if err := grantPiProjectTrust(env, request); err != nil {
			t.Fatal(err)
		}
	}
	before := read(record)
	piTrustWrite(t, entry, piTrustHookMarker+" from an earlier revision\n")
	if err := grantPiProjectTrust(failing, a2); err == nil {
		t.Fatal("injected hook failure was ignored")
	}
	if read(record) != before || read(entry) != piTrustHookMarker+" from an earlier revision\n" {
		t.Fatal("failed grant did not restore the exact prior record and hook")
	}
	if err := grantPiProjectTrust(env, a2); err != nil {
		t.Fatal(err)
	}
	if read(entry) != piTrustHookSource {
		t.Fatal("earlier generated hook was not replaced")
	}
	var document piTrustDocument
	if err := json.Unmarshal([]byte(read(record)), &document); err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]piTrustOwner{
		projectA: {
			"one-0123456789ab": {Extensions: []string{"one-0123456789ab"}, Skills: []string{"skill"}, MCPServers: []string{}},
			"two-0123456789ab": {Extensions: []string{"two-0123456789ab"}, Skills: []string{}, MCPServers: []string{"server"}},
		},
		projectB: {"one-0123456789ab": {Extensions: []string{"one-0123456789ab"}, Skills: []string{}, MCPServers: []string{}}},
	}
	if document.SchemaVersion != piTrustSchemaVersion || !reflect.DeepEqual(document.Projects, want) {
		t.Fatalf("recorded projects: %+v", document)
	}
	for _, request := range []domain.ActivationRequest{a1, a2, b1} {
		request.VerifyOnly = true
		if recorded, err := observePiProjectTrust(env.NativeConfig, request); err != nil || !recorded {
			t.Fatalf("granted installation not observed: %v", err)
		}
	}

	// Removal revokes exactly one installation; the last one deletes both files.
	if err := revokePiProjectTrust(env, piTrustRemoval(a1)); err != nil {
		t.Fatal(err)
	}
	if recorded, _ := observePiProjectTrust(env.NativeConfig, a1); recorded {
		t.Fatal("removed installation is still recorded")
	}
	if recorded, _ := observePiProjectTrust(env.NativeConfig, a2); !recorded {
		t.Fatal("removing one installation revoked another in the same project")
	}
	for _, request := range []domain.ActivationRequest{a2, b1} {
		if err := revokePiProjectTrust(env, piTrustRemoval(request)); err != nil {
			t.Fatal(err)
		}
	}
	absent("last revocation", entry, record)
	if err := revokePiProjectTrust(env, piTrustRemoval(a1)); err != nil {
		t.Fatal("idempotent revocation", err)
	}

	// Foreign bytes at the reserved hook path are neither adopted nor removed.
	piTrustWrite(t, entry, "export default function foreign() {}\n")
	if err := grantPiProjectTrust(env, a1); err == nil {
		t.Fatal("foreign hook adopted")
	}
	if err := revokePiProjectTrust(env, piTrustRemoval(a1)); err != nil {
		t.Fatal(err)
	}
	if read(entry) != "export default function foreign() {}\n" {
		t.Fatal("foreign hook changed")
	}
	absent("foreign hook", record)
}

func TestPiProjectTrustHookSourceIsBoundedAndMatchesNativeLocks(t *testing.T) {
	if len(piTrustHookSource) > 16<<10 || !strings.HasPrefix(piTrustHookSource, piTrustHookMarker) || strings.Contains(piTrustHookSource, "remember") {
		t.Fatal("generated Pi hook is unbounded, unmarked or asks Pi to persist trust")
	}
	locks, err := nativeconfig.WriterLockPaths(nativeconfig.Paths{JSON: filepath.Join(piTrustTemp(t), ".pi", "mcp.json")}, nativeconfig.CodecMCPServers)
	if err != nil || len(locks) != 1 || !strings.Contains(piTrustHookSource, `"`+filepath.Base(locks[0])+`"`) {
		t.Fatal("hook does not admit the native MCP writer lock", locks, err)
	}
	if !bytes.Contains([]byte(piTrustHookSource), []byte(`"`+piTrustHookName+`"`)) || !bytes.Contains([]byte(piTrustHookSource), []byte(`"`+piTrustProjectsName+`"`)) {
		t.Fatal("hook location constants drifted from the Go writer")
	}
}

func TestPiProjectCanCompletePreparedSetupIsConservative(t *testing.T) {
	user, project := filepath.Join(string(filepath.Separator), "home", ".pi", "agent"), filepath.Join(string(filepath.Separator), "work", "project")
	valid := piTrustRequest(t, user, project, "owned-0123456789ab", nil, nil)
	pending := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, UserActions: []string{piProjectTrustPendingAction}}
	adapter := NewProject()
	if !adapter.CanCompletePreparedSetup(valid, pending) {
		t.Fatal("missing trust record of an intact project installation was not admitted")
	}
	for name, mutate := range map[string]func(*domain.ActivationRequest, *domain.ActivationOutcome){
		"trust ready": func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) {
			o.UserActions = []string{piProjectTrustReadyAction}
		},
		"foreign resource": func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) {
			o.UserActions = []string{"Pi will still ask"}
		},
		"active":   func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.Activation = domain.ActivationActive },
		"attested": func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.ActivationAttested = true },
		"not installed": func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) {
			o.Verification = domain.VerificationPackageValid
		},
		"user scope":   func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.Scope = domain.ScopeUser },
		"other client": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Client.ClientID = domain.ClientCodex },
		"other profile": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Client.ConfigRoot = filepath.Dir(user)
		},
		"other delivery": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Delivery.ActivePath += "-other" },
		"other registry": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.NativeRegistryRoot = filepath.Join(filepath.Dir(project), ".pi")
		},
	} {
		request, observed := valid, pending
		observed.UserActions = append([]string(nil), pending.UserActions...)
		mutate(&request, &observed)
		if adapter.CanCompletePreparedSetup(request, observed) {
			t.Fatalf("%s admitted prepared setup completion", name)
		}
	}
}
