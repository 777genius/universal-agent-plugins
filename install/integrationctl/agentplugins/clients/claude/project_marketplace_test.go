package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func newClaudeMarketplaceRequest(t *testing.T) domain.ActivationRequest {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home, project, config := filepath.Join(base, "home"), filepath.Join(base, "project"), filepath.Join(base, "config")
	for _, path := range []string{home, project, config} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	target, err := domain.NewTargetContext(domain.ScopeProject, project, config)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(project, ".claude")
	active := filepath.Join(root, "skills", "demo-0123456789ab")
	plan := domain.DeliveryPlan{ClientID: domain.ClientClaude, Scope: domain.ScopeProject, TargetContext: target, PackageMode: domain.PackageProjection,
		TargetAnchor: root, TargetRoot: filepath.Join(root, "skills"), NativeRegistryRoot: root, ActivePath: active,
		PhysicalArtifactID: "demo-0123456789ab", DeclaredName: "demo", DeclaredVersion: "1.0.0",
		Authentication: domain.AuthenticationNotRequired,
		Components:     []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "first", Support: domain.SupportProjected}, {Kind: domain.ComponentSkill, Name: "second", Support: domain.SupportProjected}}}
	for _, skill := range []string{"first", "second", "ignored"} {
		claudeMarketplaceWrite(t, filepath.Join(active, "skills", skill, "SKILL.md"), "---\nname: "+skill+"\ndescription: Inert native fixture\n---\nFixture "+skill+".\n")
	}
	claudeMarketplaceWrite(t, filepath.Join(active, "hooks", "hooks.json"), "{\"hooks\":{}}")
	claudeMarketplaceWrite(t, filepath.Join(active, ".claude-plugin", "plugin.json"), "{\"name\":\"foreign-unselected\"}")
	envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0"}}
	objects, err := New().Project(t.Context(), clients.ProjectionInput{StagingPath: active, Envelope: envelope, Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), active)
	if err != nil {
		t.Fatal(err)
	}
	objects = append([]domain.NativeObjectOwnership{{ObjectID: "package:claude:" + plan.PhysicalArtifactID, Kind: "managed_package_directory", LogicalName: "demo", Path: active, ManagedDigest: digest, ProtectionClass: "managed"}}, objects...)
	return domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: config, ExecutablePath: "/test/claude"}, Plan: plan,
		Delivery: domain.StagedDelivery{ClientID: domain.ClientClaude, ActivePath: active, ArtifactDigest: digest, NativeObjects: objects}, DeclaredName: "demo", BackendExecutable: "/test/claude"}
}

func claudeMarketplaceWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func claudeMarketplaceRemoval(request domain.ActivationRequest) domain.DeactivationRequest {
	return domain.DeactivationRequest{Client: request.Client, TargetContext: request.Plan.TargetContext, NativeRegistryRoot: request.Plan.NativeRegistryRoot,
		DeclaredName: request.DeclaredName, Confirmed: true, PhysicalArtifactID: request.Plan.PhysicalArtifactID, BackendExecutable: request.BackendExecutable,
		ManagedArtifactPath: request.Plan.ActivePath, NativeObjects: request.PreviousNativeObjects, CurrentActivation: domain.ActivationActive}
}

func TestClaudeProjectMarketplaceProjectionAndCacheOwnership(t *testing.T) {
	request := newClaudeMarketplaceRequest(t)
	root := request.Plan.ActivePath
	for _, absent := range []string{".claude-plugin/plugin.json", "skills", "hooks", ".mcp.json", ".agentplugins-plugin/hooks", ".agentplugins-plugin/.agentplugins-runtime/skills/ignored"} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(absent))); !os.IsNotExist(err) {
			t.Fatalf("unplanned native surface %s: %v", absent, err)
		}
	}
	objects, err := New().ProjectActiveNative(t.Context(), root, domain.PackageEnvelope{}, request.Plan, "")
	if err != nil || !reflect.DeepEqual(objects, request.Delivery.NativeObjects[1:]) {
		t.Fatalf("projection drift: %+v %v", objects, err)
	}
	cache := filepath.Join(request.Client.ConfigRoot, filepath.FromSlash(objects[0].SourceRelative))
	if err := filetree.CopyDir(filepath.Join(root, claudeProjectBundleDirectory), cache); err != nil {
		t.Fatal(err)
	}
	verify := func() error {
		return verifyClaudeProjectCache(t.Context(), request.Client.ConfigRoot, request.DeclaredName, request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot, cache, objects, false)
	}
	if err := verify(); err != nil {
		t.Fatal(err)
	}
	claudeMarketplaceWrite(t, filepath.Join(cache, "user-file.txt"), "preserve user change")
	if err := verify(); err == nil {
		t.Fatal("adopted a modified native cache")
	}
	if _, err := New().ProjectActiveNative(t.Context(), t.TempDir(), domain.PackageEnvelope{}, request.Plan, ""); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
		t.Fatalf("legacy projection migration: %v", err)
	}
}

type claudeNativeAdapterRunner struct {
	t        *testing.T
	commands [][]string
	evidence string
}

func (r *claudeNativeAdapterRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.t.Helper()
	for _, value := range command.Env {
		key, _, _ := strings.Cut(value, "=")
		if strings.Contains(key, "TOKEN") || strings.Contains(key, "API_KEY") {
			r.t.Fatalf("credentials reached native fixture: %s", key)
		}
	}
	r.commands = append(r.commands, append([]string(nil), command.Argv[1:]...))
	cmd := exec.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	cmd.Dir, cmd.Env = command.Dir, command.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := legacyports.CommandResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if r.evidence != "" {
		body, _ := json.Marshal(map[string]any{"argv": command.Argv[1:], "exit_code": result.ExitCode, "stdout": stdout.String(), "stderr": stderr.String()})
		claudeMarketplaceWrite(r.t, filepath.Join(r.evidence, strings.Join(command.Argv[1:3], "-")+"-"+string(rune('a'+len(r.commands)))+".json"), string(body))
	}
	var exited *exec.ExitError
	if errors.As(err, &exited) {
		return result, nil
	}
	return result, err
}

// This opt-in test exercises the real adapter and official native executable.
// It uses an ordinary test exec runner, not the production containment runner;
// hosted public-installer native qualification remains a separate required gate.
func TestClaudeProjectMarketplaceNativeAdapterLifecycle(t *testing.T) {
	binary := os.Getenv("AGENTPLUGINS_CLAUDE_ADAPTER_NATIVE_BIN")
	if binary == "" {
		t.Skip("opt-in real Claude adapter integration")
	}
	request := newClaudeMarketplaceRequest(t)
	request.BackendExecutable, request.Client.ExecutablePath = binary, binary
	runner := &claudeNativeAdapterRunner{t: t, evidence: os.Getenv("AGENTPLUGINS_CLAUDE_ADAPTER_EVIDENCE_DIR")}
	env := clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}
	outcome, err := New().Activate(t.Context(), env, request)
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || outcome.NativeEffect != domain.NativeEffectCommitted {
		t.Fatalf("initial native activation: %+v %v", outcome, err)
	}
	request.Replacing, request.PreviousNativeObjects = true, outcome.NativeObjects
	before := len(runner.commands)
	repeated, err := New().Activate(t.Context(), env, request)
	if err != nil || repeated.NativeEffect != domain.NativeEffectUnchanged || repeated.Activation != domain.ActivationActive || len(runner.commands) != before+1 {
		t.Fatalf("read-only repeat: %+v %v commands=%v", repeated, err, runner.commands[before:])
	}
	probe, err := PrepareClaudeActivationProbe(request)
	if err != nil {
		t.Fatal(err)
	}
	details := probe.Command
	details.Argv = []string{binary, "plugin", "details", "demo@" + ProjectMarketplaceName(request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot)}
	native, err := runner.Run(t.Context(), details)
	if err != nil || native.ExitCode != 0 || !strings.Contains(string(native.Stdout), "Skills (2)") || !strings.Contains(string(native.Stdout), "first") || !strings.Contains(string(native.Stdout), "second") || strings.Contains(string(native.Stdout), "ignored") {
		t.Fatalf("native selected skill discovery: %s %s %v", native.Stdout, native.Stderr, err)
	}
	// Same-version content replacement must refresh Claude 2.1.263's cache too.
	changed := filepath.Join(request.Plan.ActivePath, claudeProjectBundleDirectory, ClaudeRuntimeDirectory, "skills", "first", "SKILL.md")
	claudeMarketplaceWrite(t, changed, "---\nname: first\ndescription: Updated native fixture\n---\nChanged selected skill.\n")
	projected, err := claudeProjectCacheProjection(t.Context(), request.Plan.ActivePath, request.Plan)
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.NativeObjects = append(request.Delivery.NativeObjects[:1:1], projected...)
	updated, err := New().Activate(t.Context(), env, request)
	if err != nil || updated.Activation != domain.ActivationActive || updated.NativeEffect != domain.NativeEffectCommitted {
		t.Fatalf("same-version update: %+v %v", updated, err)
	}
	request.PreviousNativeObjects = updated.NativeObjects
	cache := filepath.Join(request.Client.ConfigRoot, filepath.FromSlash(projected[0].SourceRelative))
	foreign := filepath.Join(cache, "foreign.txt")
	claudeMarketplaceWrite(t, foreign, "preserve foreign bytes")
	before = len(runner.commands)
	refused, err := New().Deactivate(t.Context(), env, claudeMarketplaceRemoval(request))
	if err == nil || refused.NativeEffect != domain.NativeEffectUnchanged || len(runner.commands) != before+1 {
		t.Fatalf("modified cache removal was not inert: %+v %v", refused, err)
	}
	if err := os.Remove(foreign); err != nil {
		t.Fatal(err)
	}
	removed, err := New().Deactivate(t.Context(), env, claudeMarketplaceRemoval(request))
	if err != nil || !removed.ExternalRemovalComplete || removed.NativeEffect != domain.NativeEffectCommitted {
		t.Fatalf("native removal: %+v %v", removed, err)
	}
	listed, err := runner.Run(t.Context(), probe.Command)
	if err != nil || observeClaudeMarketplacePlugin(listed.Stdout, request.DeclaredName, request.Plan.PhysicalArtifactID, request.Plan.ActivePath, request.Plan.TargetContext.ProjectRoot, request.Client.ConfigRoot, "") != projectListAbsent {
		t.Fatalf("negative native discovery: %s %v", listed.Stdout, err)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("current owned cache survived removal: %v", err)
	}
	// A saved raw skill reuses its installation ID after the old client binding
	// and its native receipts have been removed by the ordinary installer.
	request.Replacing, request.PreviousNativeObjects = false, nil
	readded, err := New().Activate(t.Context(), env, request)
	if err != nil || readded.Activation != domain.ActivationActive || readded.NativeEffect != domain.NativeEffectCommitted {
		t.Fatalf("same-identity add after removal: %+v %v", readded, err)
	}
	request.PreviousNativeObjects = readded.NativeObjects
	removed, err = New().Deactivate(t.Context(), env, claudeMarketplaceRemoval(request))
	if err != nil || !removed.ExternalRemovalComplete {
		t.Fatalf("remove re-added native installation: %+v %v", removed, err)
	}
}

func TestClaudeUserNativeAdapterLifecycle(t *testing.T) {
	binary := os.Getenv("AGENTPLUGINS_CLAUDE_ADAPTER_NATIVE_BIN")
	if binary == "" {
		t.Skip("opt-in real Claude adapter integration")
	}
	request := newClaudeMarketplaceRequest(t)
	active := filepath.Join(request.Client.ConfigRoot, "skills", "demo-user-owned")
	if err := filetree.CopyDir(filepath.Join(request.Plan.ActivePath, claudeProjectBundleDirectory, ClaudeRuntimeDirectory), active); err != nil {
		t.Fatal(err)
	}
	target, err := domain.NewTargetContext(domain.ScopeUser, "", request.Client.ConfigRoot)
	if err != nil {
		t.Fatal(err)
	}
	request.Plan.Scope, request.Plan.TargetContext = domain.ScopeUser, target
	request.Plan.TargetAnchor, request.Plan.TargetRoot = request.Client.ConfigRoot, filepath.Dir(active)
	request.Plan.NativeRegistryRoot, request.Plan.ActivePath = request.Client.ConfigRoot, active
	request.Delivery.ActivePath, request.Delivery.NativeObjects = active, nil
	request.BackendExecutable, request.Client.ExecutablePath = binary, binary
	_, err = New().Project(t.Context(), clients.ProjectionInput{StagingPath: active, Envelope: domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0"}}, Plan: request.Plan})
	if err != nil {
		t.Fatal(err)
	}
	runner := &claudeNativeAdapterRunner{t: t}
	if evidence := os.Getenv("AGENTPLUGINS_CLAUDE_ADAPTER_EVIDENCE_DIR"); evidence != "" {
		runner.evidence = filepath.Join(evidence, "user")
	}
	env := clients.Env{Runner: runner, NativeConfig: nativeconfig.New()}
	for range 2 {
		out, err := New().Activate(t.Context(), env, request)
		if err != nil || out.Activation != domain.ActivationActive || out.Verification != domain.VerificationInstalled {
			t.Fatalf("user native activation: %+v %v", out, err)
		}
	}
	probe, err := PrepareClaudeActivationProbe(request)
	if err != nil {
		t.Fatal(err)
	}
	details := probe.Command
	details.Argv = []string{binary, "plugin", "details", "demo@skills-dir"}
	native, err := runner.Run(t.Context(), details)
	if err != nil || native.ExitCode != 0 || !strings.Contains(string(native.Stdout), "Skills (2)") {
		t.Fatalf("user selected skill discovery: %s %s %v", native.Stdout, native.Stderr, err)
	}
	removed, err := New().Deactivate(t.Context(), env, claudeMarketplaceRemoval(request))
	if err != nil || !removed.ExternalRemovalComplete {
		t.Fatalf("user native removal admission: %+v %v", removed, err)
	}
	if err := os.RemoveAll(active); err != nil {
		t.Fatal(err)
	}
	listed, err := runner.Run(t.Context(), probe.Command)
	if err != nil || PluginStatusFromListForScope(listed.Stdout, "demo", active, domain.ScopeUser) != StatusAbsent {
		t.Fatalf("user negative discovery: %s %v", listed.Stdout, err)
	}
}
