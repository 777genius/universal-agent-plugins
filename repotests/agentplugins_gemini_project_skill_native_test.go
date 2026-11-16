//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const geminiPureSkillName = "uap-gemini-pure-skill"
const geminiPureSkillDescription = "Disposable pure Skill native discovery"

type geminiPureSkillFixture struct {
	f            *geminiProjectAttempt
	skillBody    []byte
	skillPath    string
	trustPath    string
	statePath    string
	installation domain.Installation
	binding      domain.ClientBinding
	foreignB     map[string]geminiCycleRecord
}

func TestAgentpluginsGeminiNativeProjectSkillReady(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_GEMINI_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in genuine hosted Gemini pure Skill lifecycle")
	}
	f := newGeminiProjectAttempt(t)
	f.cancel()
	f.ctx, f.cancel = context.WithDeadline(context.Background(), f.started.Add(60*time.Second))
	defer f.cancel()
	p := &geminiPureSkillFixture{f: f,
		skillPath: filepath.Join(f.base.Project, ".gemini", "skills", geminiPureSkillName, "SKILL.md"),
		trustPath: filepath.Join(f.base.Home, ".gemini", "trustedFolders.json"),
		statePath: filepath.Join(f.base.Project, ".uap", "state-v2.json")}
	defer p.capture(t)
	p.setup(t)
	p.listings(t, "baseline", false)
	p.requireTrust(t, false)
	var err error
	p.foreignB, err = geminiCycleTree(f.ctx, f.other)
	if err != nil {
		t.Fatal(err)
	}
	p.add(t, "add", true)
	p.requireTrust(t, true)
	trusted, err := projectNativeRuntimeRead(p.trustPath, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	f.keep(t, "pure-trust-after.json", trusted)
	f.protected[p.trustPath] = nativeSHA(t, p.trustPath)
	p.listings(t, "installed", true)
	p.repeat(t)
	p.remove(t)
	p.listings(t, "removed", false)
	p.check(t)
	if _, err := os.Lstat(f.base.Events); !os.IsNotExist(err) {
		t.Fatal("pure Skill lifecycle unexpectedly started MCP")
	}
	f.evidence["pure_skill_lifecycle_complete"] = true
}

func (p *geminiPureSkillFixture) setup(t *testing.T) {
	t.Helper()
	f := p.f
	f.evidence["provider_kind"] = "none"
	f.evidence["pure_skill_lifecycle_complete"] = false
	delete(f.evidence, "activation")
	delete(f.evidence, "qualification")
	delete(f.evidence, "runtime_attempt_complete")
	for _, project := range []struct{ label, path string }{{"a", f.base.Project}, {"b", f.other}} {
		f.command(t, "pure-git-"+project.label, project.path, f.env(), []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}, 10*time.Second)
	}
	settings, err := json.Marshal(map[string]any{
		"security": map[string]any{"folderTrust": map[string]any{"enabled": true}},
		"skills":   map[string]any{"disabled": []string{"foreign-user", "foreign-b"}},
		"privacy":  map[string]any{"usageStatisticsEnabled": false}, "telemetry": map[string]any{"enabled": false},
		"general": map[string]any{"enableAutoUpdate": false, "enableAutoUpdateNotification": false}})
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string][]byte{
		filepath.Join(f.base.Home, ".gemini", "settings.json"):                      settings,
		filepath.Join(f.base.Home, ".gemini", "skills", "foreign-user", "SKILL.md"): geminiCycleForeignSkill("user"),
		filepath.Join(f.other, ".gemini", "skills", "foreign-b", "SKILL.md"):        geminiCycleForeignSkill("b"),
		filepath.Join(f.other, "keep.txt"):                                          []byte("unselected project B\n"),
	} {
		projectNativeRuntimeWrite(t, path, body, 0600)
		f.protected[path] = nativeSHA(t, path)
	}
	trust, err := json.Marshal(map[string]string{f.other: "TRUST_FOLDER"})
	if err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, p.trustPath, trust, 0600)
	f.keep(t, "pure-trust-before.json", trust)
	p.requireTrust(t, false)
	manifest, err := json.Marshal(map[string]string{"$schema": domain.PluginSchemaV1, "name": geminiPureSkillName,
		"version": "1.0.0", "description": geminiPureSkillDescription})
	if err != nil {
		t.Fatal(err)
	}
	p.skillBody = []byte("---\nname: " + geminiPureSkillName + "\ndescription: " + geminiPureSkillDescription + "\n---\nPURE_SKILL_" + f.base.Nonce + "\n")
	for relative, body := range map[string][]byte{"plugin.json": manifest, "skills/" + geminiPureSkillName + "/SKILL.md": p.skillBody} {
		path := filepath.Join(f.base.Package, filepath.FromSlash(relative))
		projectNativeRuntimeWrite(t, path, body, 0600)
		f.immutable[path] = nativeSHA(t, path)
	}
	f.keep(t, "pure-source-manifest.json", manifest)
	f.keep(t, "pure-source-skill.md", p.skillBody)
	result := f.command(t, "pure-version", f.outside, f.env(), []string{f.node, f.client, "--version"}, 10*time.Second)
	if string(result.Stdout) != "0.65.0\n" || len(result.Stderr) != 0 {
		t.Fatal("pure Skill pinned Gemini version differs")
	}
}

func (p *geminiPureSkillFixture) requireTrust(t *testing.T, installed bool) {
	t.Helper()
	raw, err := projectNativeRuntimeRead(p.trustPath, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var mapping map[string]string
	if err := json.Unmarshal(raw, &mapping); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{p.f.other: "TRUST_FOLDER"}
	if installed {
		want[p.f.base.Project] = "TRUST_FOLDER"
	}
	if !reflect.DeepEqual(mapping, want) {
		t.Fatalf("pure Skill trust changed beyond the selected folder: %q", raw)
	}
}

func (p *geminiPureSkillFixture) check(t *testing.T) {
	t.Helper()
	p.f.check(t)
	if p.foreignB != nil {
		current, err := geminiCycleTree(p.f.ctx, p.f.other)
		if err != nil || !reflect.DeepEqual(current, p.foreignB) {
			t.Fatal("pure Skill operation changed unselected project B", err)
		}
	}
}

func (p *geminiPureSkillFixture) listings(t *testing.T, phase string, installed bool) {
	t.Helper()
	f := p.f
	for _, scope := range []struct{ label, path string }{{"a", f.base.Project}, {"b", f.other}, {"user", f.outside}} {
		want := map[string]geminiCycleSkillRow{}
		for name, sha := range geminiCycleBuiltinSHA {
			path := filepath.Join(filepath.Dir(f.client), "builtin", name, "SKILL.md")
			if f.immutable[path] != sha || nativeSHA(t, path) != sha {
				t.Fatal("pure Skill pinned builtin bytes differ")
			}
			want[name] = geminiCycleSkillRow{Description: geminiCycleBuiltinDescription[name], Path: path, Enabled: true, Builtin: true}
		}
		want["foreign-user"] = geminiCycleSkillRow{Description: "Foreign inert marker",
			Path: filepath.Join(f.base.Home, ".gemini", "skills", "foreign-user", "SKILL.md")}
		if scope.label == "b" {
			want["foreign-b"] = geminiCycleSkillRow{Description: "Foreign inert marker",
				Path: filepath.Join(f.other, ".gemini", "skills", "foreign-b", "SKILL.md")}
		}
		if scope.label == "a" && installed {
			body, err := projectNativeRuntimeRead(p.skillPath, 1<<20)
			if err != nil || !bytes.Equal(body, p.skillBody) {
				t.Fatal("pure Skill installed bytes differ", err)
			}
			if phase == "installed" {
				f.keep(t, "pure-installed-skill.md", body)
			}
			want[geminiPureSkillName] = geminiCycleSkillRow{Description: geminiPureSkillDescription, Path: p.skillPath, Enabled: true}
		}
		result := f.command(t, "pure-"+phase+"-"+scope.label, scope.path, f.env(), []string{f.node, f.client, "skills", "list", "--all"}, 10*time.Second)
		if !geminiCycleListingStderr(result.Stderr, scope.label, nil) {
			t.Fatal("pure Skill native inventory emitted unrecognized diagnostics")
		}
		if err := geminiCycleSkills(result.Stdout, want); err != nil {
			t.Fatal("pure Skill exact native inventory", phase, scope.label, err)
		}
	}
	p.check(t)
}

func (p *geminiPureSkillFixture) state(t *testing.T, label string) domain.Installation {
	t.Helper()
	raw, err := projectNativeRuntimeRead(p.statePath, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	p.f.keep(t, "pure-"+label+".state.json", raw)
	state, err := geminiCycleState(raw)
	if err != nil || len(state.Installations) != 1 {
		t.Fatal("pure Skill exact state", err)
	}
	installation := state.Installations[0]
	if installation.DeclaredName != geminiPureSkillName || installation.Source.CanonicalSource != p.f.base.Package ||
		installation.Source.TreeDigest == "" || installation.Source.SourceBindingID == "" ||
		p.installation.InstallationID != "" && installation.InstallationID != p.installation.InstallationID {
		t.Fatal("pure Skill installation source/identity differs")
	}
	if inventory := installation.Package.Inventory; len(inventory.Skills) != 1 || inventory.Skills[0] != geminiPureSkillName ||
		inventory.MCPPresent || inventory.MCPEnabled || len(inventory.MCPServers) != 0 {
		t.Fatal("pure Skill package unexpectedly contains other native components")
	}
	return installation
}

func (p *geminiPureSkillFixture) add(t *testing.T, label string, mutated bool) {
	t.Helper()
	f := p.f
	result := f.command(t, "pure-"+label, f.base.Project, f.env(),
		[]string{f.installer, "add", f.base.Package, "--no-save", "--scope", "project", "--project-root", f.base.Project,
			"--target", "gemini", "--format", "json", "--yes"}, 30*time.Second)
	if err := nativeValidateInstallerJSON(result.Stdout); err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(result.Stdout, &document); err != nil {
		t.Fatal(err)
	}
	document = codexProjectAssertionEnvelope(document)
	codexNativeProjectSkillsReady(t, document)
	commandsRequireMutated(t, label, document, mutated)
	outcome, err := commandsTargetOutcome(document)
	if err != nil {
		t.Fatal(err)
	}
	activation, _ := outcome["activation"].(map[string]any)
	if actions, ok := activation["user_actions"].([]any); ok && len(actions) != 0 {
		t.Fatalf("Ready pure Skill still asks for native/auth setup: %v", actions)
	}
	data, _ := document["data"].(map[string]any)
	targets, _ := data["targets"].([]any)
	target, _ := targets[0].(map[string]any)
	output, _ := target["output"].(map[string]any)
	for _, key := range []string{"manual_actions", "readiness_reasons"} {
		if actions, ok := output[key].([]any); ok && len(actions) != 0 {
			t.Fatalf("Ready pure Skill retains %s: %v", key, actions)
		}
	}
	if !mutated && outcome["no_change"] != true {
		t.Fatal("repeated pure Skill installation lacks no-change")
	}
	p.installation = p.state(t, label)
	p.requireBinding(t)
	p.check(t)
}

func (p *geminiPureSkillFixture) requireBinding(t *testing.T) {
	t.Helper()
	f := p.f
	if len(p.installation.Clients) != 1 {
		t.Fatal("pure Skill receipt has an unexpected target")
	}
	for _, binding := range p.installation.Clients {
		if binding.ClientID != "gemini" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(f.base.Project, ".gemini") ||
			binding.Activation != domain.ActivationActive || binding.Verification != domain.VerificationInstalled ||
			binding.Authentication != domain.AuthenticationNotRequired || binding.Policy != domain.PolicyAllowed ||
			binding.Materialization != domain.MaterializationMaterialized || binding.PendingNativeIntent != nil || binding.NativeActivationAttempt != "" {
			t.Fatalf("pure Skill saved readiness differs: %+v", binding)
		}
		p.requireSkillReceipt(t, binding)
		p.binding = binding
	}
}

func (p *geminiPureSkillFixture) requireSkillReceipt(t *testing.T, binding domain.ClientBinding) {
	t.Helper()
	nativeRequireContained(t, filepath.Join(p.f.base.Project, ".uap", "managed"), binding.TargetLocator)
	relative := "skills/" + geminiPureSkillName
	source, err := shared.DigestSkillDirectory(filepath.Join(p.f.base.Package, filepath.FromSlash(relative)))
	if err != nil || source == "" {
		t.Fatal("pure Skill complete source directory digest", err)
	}
	for _, path := range []string{filepath.Join(binding.TargetLocator, filepath.FromSlash(relative)), filepath.Dir(p.skillPath)} {
		digest, err := shared.DigestSkillDirectory(path)
		if err != nil || digest != source {
			t.Fatal("pure Skill complete source/managed/native directory bytes differ", path, err)
		}
	}
	owned := 0
	for _, object := range binding.NativeObjects {
		if object.Kind == "gemini_global_skill_directory" && object.LogicalName == geminiPureSkillName &&
			object.ObjectID == "gemini-skill:"+geminiPureSkillName && object.Path == filepath.Dir(p.skillPath) &&
			object.ManagedDigest == source && object.SourceRelative == relative && object.ProtectionClass == "managed" && !object.UserModified {
			owned++
		}
	}
	if owned != 1 {
		t.Fatal("pure Skill exact native ownership receipt differs from complete directory proof")
	}
	p.f.evidence["owned_skill_directory_digest"] = source
}

func (p *geminiPureSkillFixture) repeat(t *testing.T) {
	t.Helper()
	before, err := projectNativeRuntimeRead(p.statePath, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p.statePath)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := geminiCycleTree(p.f.ctx, filepath.Dir(filepath.Dir(filepath.Dir(p.skillPath))))
	if err != nil {
		t.Fatal(err)
	}
	p.add(t, "repeat", false)
	p.listings(t, "repeat", true)
	after, err := projectNativeRuntimeRead(p.statePath, 1<<20)
	afterInfo, statErr := os.Stat(p.statePath)
	current, treeErr := geminiCycleTree(p.f.ctx, filepath.Dir(filepath.Dir(filepath.Dir(p.skillPath))))
	if err != nil || statErr != nil || treeErr != nil || !bytes.Equal(before, after) ||
		!os.SameFile(info, afterInfo) || !reflect.DeepEqual(profile, current) {
		t.Fatal("pure Skill no-change altered state or native profile", err, statErr, treeErr)
	}
}

func (p *geminiPureSkillFixture) remove(t *testing.T) {
	t.Helper()
	f := p.f
	result := f.command(t, "pure-remove", f.base.Project, f.env(),
		[]string{f.installer, "remove", p.installation.InstallationID, "--scope", "project", "--project-root", f.base.Project,
			"--target", "gemini", "--format", "json"}, 30*time.Second)
	if err := geminiCyclePublicJSON(result.Stdout, true); err != nil {
		t.Fatal(err)
	}
	retained := p.state(t, "removed")
	if len(retained.Clients) != 0 || !retained.DataRetained {
		t.Fatal("pure Skill removal did not retain the installation/data without its client binding")
	}
	for _, path := range []string{filepath.Dir(p.skillPath), p.binding.TargetLocator} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("pure Skill owned directory remains after removal", path, err)
		}
	}
	p.requireTrust(t, true)
}

func (p *geminiPureSkillFixture) capture(t *testing.T) {
	t.Helper()
	p.f.evidence["test_failed"] = t.Failed()
	p.f.evidence["elapsed_ms"] = time.Since(p.f.started).Milliseconds()
	p.f.evidence["transcript_kind"] = "pure-skill-installation-no-model"
	if err := geminiProjectSecretFree(p.f.files, []string{p.f.secret}); err != nil {
		t.Error(err)
		return
	}
	body, err := json.Marshal(map[string]any{"evidence": p.f.evidence, "streams": p.f.files})
	// The containing runtime-test stdout has a 1 MiB limit. Keep this complete
	// encoded transcript below 256 KiB, leaving room for the mixed cycle logs.
	if err != nil || len(body) > 256<<10 {
		t.Error("pure Skill complete transcript exceeds its capture bound", err)
		return
	}
	t.Logf("Gemini pure Skill capture: %s", body)
}
