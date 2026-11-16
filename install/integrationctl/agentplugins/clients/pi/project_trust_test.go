package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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
	return piTrustNodeAs(t, node, user, user, home, cwd)
}

// piTrustNodeAs runs Pi with agent as PI_CODING_AGENT_DIR, loading the hook
// through agent's extensions path while its bytes live in user's profile.
func piTrustNodeAs(t *testing.T, node, user, agent, home, cwd string) string {
	t.Helper()
	_, entry, _ := piTrustHookPaths(user)
	module := strings.TrimSuffix(entry, ".ts") + ".mjs"
	body, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	piTrustWrite(t, module, string(body))
	defer func() { _ = os.Remove(module) }()
	dir, _, _ := piTrustHookPaths(agent)
	module = filepath.Join(dir, filepath.Base(module))
	const runner = `import { pathToFileURL } from "node:url";
const [hook, cwd] = process.argv.slice(1);
const handlers = [];
(await import(pathToFileURL(hook).href)).default({ on: (name, handler) => { if (name === "project_trust") handlers.push(handler); } });
if (handlers.length !== 1) throw new Error("project_trust handler registration");
process.stdout.write(JSON.stringify(await handlers[0]({ type: "project_trust", cwd }, {})));`
	command := exec.CommandContext(t.Context(), node, "--input-type=module", "-e", runner, module, cwd)
	command.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "PI_CODING_AGENT_DIR="+agent)
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
// proves the owned-only, unchanged-content decision, and the Go explanation
// (with the same digests) must agree with it on every step.
func TestPiProjectTrustHookAnswersOnlyForOwnedUnchangedProjects(t *testing.T) {
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
	config := filepath.Join(project, ".pi")
	extension := filepath.Join(config, "extensions", artifact, "native", "index.ts")
	skill := filepath.Join(config, "skills", "owned")
	for _, dir := range []string{user, filepath.Join(other, ".pi", "extensions", artifact, "empty"), filepath.Join(skill, "empty")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Unicode names, nesting and diverse JSON numbers exercise Go/JS digest parity.
	ownedEntry := `{"command":"node","args":["s\u00e9rver.js","--port",8080,1.5,-0,1e21,1e-7],"env":{"Z":"1","\u00c4":null,"a":true},"nested":[{"b":false,"a":[]}]}`
	ownedMCP := `{"mcpServers":{"owned":` + ownedEntry + `}}`
	piTrustWrite(t, extension, "export default function owned() {}\n")
	piTrustWrite(t, filepath.Join(skill, "SKILL.md"), "---\nname: owned\ndescription: d\n---\n")
	piTrustWrite(t, filepath.Join(skill, "références", "ü.md"), "unicode\n")
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
	restore := func(path, body string) func() { return func() { piTrustWrite(t, path, body) } }
	write := func(path, body string) func() bool { return func() bool { piTrustWrite(t, path, body); return true } }
	drop := func(path string) func() bool { return func() bool { return os.RemoveAll(path) == nil } }
	// Replace the existing owned SKILL.md by a link to identical bytes, so only
	// the link checks can refuse it. Link creation can need privileges
	// (Windows symlinks); skip only that case.
	skillFile := filepath.Join(skill, "SKILL.md")
	skillBody := "---\nname: owned\ndescription: d\n---\n"
	identical := filepath.Join(root, "identical-skill.md")
	piTrustWrite(t, identical, skillBody)
	link := func(add func(string, string) error) func() bool {
		return func() bool { return os.Remove(skillFile) == nil && add(identical, skillFile) == nil }
	}
	unlink := func() { _ = os.Remove(skillFile); piTrustWrite(t, skillFile, skillBody) }
	harmless := filepath.Join(root, "harmless.json")
	piTrustWrite(t, harmless, "{}\n")
	symlink := func(path string) func() bool { return func() bool { return os.Symlink(harmless, path) == nil } }
	for _, change := range []struct {
		name  string
		add   func() bool
		after func()
	}{
		{"foreign project extension", write(filepath.Join(config, "extensions", "foreign.ts"), "export default () => {}\n"), remove(filepath.Join(config, "extensions", "foreign.ts"))},
		{"later foreign skill", write(filepath.Join(config, "skills", "foreign", "SKILL.md"), "x\n"), remove(filepath.Join(config, "skills", "foreign"))},
		{"foreign prompt", write(filepath.Join(config, "prompts", "foreign.md"), "x\n"), remove(filepath.Join(config, "prompts"))},
		{"foreign MCP server", write(filepath.Join(config, "mcp.json"), `{"mcpServers":{"owned":`+ownedEntry+`,"foreign":{"command":"true"}}}`), restore(filepath.Join(config, "mcp.json"), ownedMCP)},
		{"same-name extension content", write(extension, "export default function owned() { /* changed */ }\n"), restore(extension, "export default function owned() {}\n")},
		{"extra file in owned skill", write(filepath.Join(skill, "extra.md"), "x\n"), remove(filepath.Join(skill, "extra.md"))},
		{"missing file in owned skill", drop(filepath.Join(skill, "références")), restore(filepath.Join(skill, "références", "ü.md"), "unicode\n")},
		{"same-name MCP command", write(filepath.Join(config, "mcp.json"), strings.Replace(ownedMCP, `"command":"node"`, `"command":"sh"`, 1)), restore(filepath.Join(config, "mcp.json"), ownedMCP)},
		{"owned file replaced by symlink to identical bytes", link(os.Symlink), unlink},
		{"owned file replaced by hardlink to identical bytes", link(os.Link), unlink},
		{"project settings", write(filepath.Join(config, "settings.json"), "{}\n"), remove(filepath.Join(config, "settings.json"))},
		{"ancestor .agents skills", write(filepath.Join(work, ".agents", "skills", "x", "SKILL.md"), "x\n"), remove(filepath.Join(work, ".agents"))},
		{"saved Pi decision", write(filepath.Join(user, "trust.json"), `{"`+filepath.ToSlash(project)+`":false}`), remove(filepath.Join(user, "trust.json"))},
		{"Pi default policy", write(filepath.Join(user, "settings.json"), `{"defaultProjectTrust":"never"}`), remove(filepath.Join(user, "settings.json"))},
		{"symlinked Pi settings", symlink(filepath.Join(user, "settings.json")), remove(filepath.Join(user, "settings.json"))},
		{"symlinked Pi trust store", symlink(filepath.Join(user, "trust.json")), remove(filepath.Join(user, "trust.json"))},
	} {
		if !change.add() {
			t.Logf("%s: not supported by this filesystem", change.name)
			continue
		}
		expect(change.name, "undecided")
		change.after()
		expect(change.name+" withdrawn", "yes")
	}
	// Another profile sharing the extensions directory through a symlink
	// keeps its own saved "Do not trust" instead of borrowing this profile's.
	borrower := filepath.Join(root, "borrower", ".pi", "agent")
	if err := os.MkdirAll(borrower, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(user, "extensions"), filepath.Join(borrower, "extensions")); err != nil {
		t.Logf("symlinked profile: not supported by this filesystem: %v", err)
	} else {
		piTrustWrite(t, filepath.Join(borrower, "trust.json"), `{"`+filepath.ToSlash(project)+`":false}`)
		if got := piTrustNodeAs(t, node, user, borrower, home, project); got != "undecided" {
			t.Fatalf("symlinked profile borrowed trust: %q", got)
		}
		expect("symlinked profile withdrawn", "yes")
	}
	// A confirmed UAP update re-records the new content and keeps trust.
	piTrustWrite(t, extension, "export default function owned() { /* updated by UAP */ }\n")
	expect("update before confirmed activation", "undecided")
	if err := grantPiProjectTrust(env, request); err != nil {
		t.Fatal(err)
	}
	expect("confirmed update re-recorded content", "yes")
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
	if err := os.MkdirAll(user, 0o700); err != nil {
		t.Fatal(err)
	}
	// The owned content each confirmed activation has just installed.
	piTrustWrite(t, filepath.Join(projectA, ".pi", "extensions", "one-0123456789ab", "index.ts"), "one\n")
	piTrustWrite(t, filepath.Join(projectA, ".pi", "skills", "skill", "SKILL.md"), "skill\n")
	piTrustWrite(t, filepath.Join(projectA, ".pi", "extensions", "two-0123456789ab", "index.ts"), "two\n")
	piTrustWrite(t, filepath.Join(projectA, ".pi", "mcp.json"), `{"mcpServers":{"server":{"command":"node"}}}`)
	piTrustWrite(t, filepath.Join(projectB, ".pi", "extensions", "one-0123456789ab", "index.ts"), "b\n")
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
	tree := func(path string) string {
		t.Helper()
		digest, err := piTrustTreeDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		return digest
	}
	server, err := piTrustValueDigest(map[string]any{"command": "node"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]piTrustOwner{
		projectA: {
			"one-0123456789ab": {Extensions: map[string]string{"one-0123456789ab": tree(a1.Plan.ActivePath)}, Skills: map[string]string{"skill": tree(filepath.Join(projectA, ".pi", "skills", "skill"))}, MCPServers: map[string]string{}},
			"two-0123456789ab": {Extensions: map[string]string{"two-0123456789ab": tree(a2.Plan.ActivePath)}, Skills: map[string]string{}, MCPServers: map[string]string{"server": server}},
		},
		projectB: {"one-0123456789ab": {Extensions: map[string]string{"one-0123456789ab": tree(b1.Plan.ActivePath)}, Skills: map[string]string{}, MCPServers: map[string]string{}}},
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

// Concurrent first grants into one profile all succeed and record every
// project; a leftover atomic-write temporary never blocks grant or observe.
func TestPiProjectTrustConcurrentGrantsAndLeftoverTemporaries(t *testing.T) {
	root := piTrustTemp(t)
	user := filepath.Join(root, "home", ".pi", "agent")
	if err := os.MkdirAll(user, 0o700); err != nil {
		t.Fatal(err)
	}
	env := clients.Env{NativeConfig: nativeconfig.New()}
	requests := make([]domain.ActivationRequest, 24)
	for index := range requests {
		project := filepath.Join(root, fmt.Sprintf("project-%02d", index))
		piTrustWrite(t, filepath.Join(project, ".pi", "extensions", "owned-0123456789ab", "index.ts"), "owned\n")
		requests[index] = piTrustRequest(t, user, project, "owned-0123456789ab", nil, nil)
	}
	errs := make([]error, len(requests))
	var wait sync.WaitGroup
	for index := range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errs[index] = grantPiProjectTrust(env, requests[index])
		}()
	}
	wait.Wait()
	if err := errors.Join(errs...); err != nil {
		t.Fatal("concurrent first grants failed:", err)
	}
	_, _, record := piTrustHookPaths(user)
	piTrustWrite(t, record+".tmp-12345", "partial")
	body, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	document, err := decodePiTrust(nativeconfig.FileSnapshot{Body: body, Exists: true})
	if err != nil || len(document.Projects) != len(requests) {
		t.Fatalf("recorded %d of %d projects: %v", len(document.Projects), len(requests), err)
	}
	verify := requests[0]
	verify.VerifyOnly = true
	if recorded, err := observePiProjectTrust(env.NativeConfig, verify); err != nil || !recorded {
		t.Fatal("leftover temporary broke observation:", err)
	}
	if err := grantPiProjectTrust(env, requests[1]); err != nil {
		t.Fatal("leftover temporary broke a later grant:", err)
	}
}

func TestPiProjectTrustHookSourceIsBoundedAndMatchesNativeLocks(t *testing.T) {
	if len(piTrustHookSource) > 16<<10 || !strings.HasPrefix(piTrustHookSource, piTrustHookMarker) || strings.Contains(piTrustHookSource, "remember") {
		t.Fatal("generated Pi hook is unbounded, unmarked or asks Pi to persist trust")
	}
	for _, limit := range []string{
		fmt.Sprintf("const LIMIT = %d;", piTrustPiFileLimit),
		fmt.Sprintf("const TREE_ENTRIES = %d;", piTrustTreeEntryLimit),
		fmt.Sprintf("const TREE_BYTES = %d;", piTrustTreeByteLimit),
	} {
		if !strings.Contains(piTrustHookSource, limit) {
			t.Fatal("hook limit drifted from the Go mirror:", limit)
		}
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
	// The predicate reads nothing, but its roots must be absolute on every OS.
	root := piTrustTemp(t)
	user, project := filepath.Join(root, "home", ".pi", "agent"), filepath.Join(root, "work", "project")
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
