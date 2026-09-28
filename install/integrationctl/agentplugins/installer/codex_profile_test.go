package installer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This executes a local Go subprocess fixture through the real process runner.
// It exercises the installer, not Codex authentication or a live native release.
func TestCustomCodexHomeDisposableAddRepeatRemove(t *testing.T) {
	skipWindowsLauncherExecuteBit(t)
	ctx := testCtx(t)
	helper, executable := buildProbe(t), buildCodexProbe(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil { t.Fatal(err) }
	home, project := filepath.Join(base, "home"), filepath.Join(base, "project")
	profile, defaultProfile := filepath.Join(project, "custom"), filepath.Join(home, ".codex")
	for _, dir := range []string{profile, defaultProfile} { if err := os.MkdirAll(dir, 0700); err != nil { t.Fatal(err) } }
	sentinel := []byte("default profile must survive\n")
	if err := os.WriteFile(filepath.Join(defaultProfile, "keep"), sentinel, 0600); err != nil { t.Fatal(err) }
	unrelated := filepath.Join(base, "unrelated-package")
	fixtureState, err := json.Marshal(map[string]any{"markets": map[string]string{"unrelated": unrelated}, "plugins": map[string]bool{"sample-notify@unrelated": true}})
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(profile, "fixture-registry.json"), fixtureState, 0600); err != nil { t.Fatal(err) }
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CODEX_HOME", "custom")
	t.Chdir(project)
	detector := clientdetect.NewOS(home)
	detector.Registry, err = clients.NewRegistry(codex.New())
	if err != nil { t.Fatal(err) }
	detector.LookPath = func(string) (string, error) { return executable, nil }
	t.Chdir(home)
	detected, err := detector.DetectTargetsWithVersionProbe(ctx, []domain.ClientID{domain.ClientCodex})
	if err != nil || len(detected) != 1 || detected[0].ConfigRoot != profile || detected[0].Version != "1.2.3" { t.Fatalf("detection: %+v %v", detected, err) }
	// Later environment and cwd changes must not reinterpret the selected root.
	t.Setenv("CODEX_HOME", defaultProfile)
	pkg := filepath.Join(project, "package")
	writePackage(t, pkg, helper)
	eng, err := newTestEngine(t, Config{StateRoot: filepath.Join(base, "state"), HelperExecutable: helper, Runner: processadapter.OS{}, EnableNativeObserver: true})
	if err != nil { t.Fatal(err) }
	id := "00000000-0000-4000-8000-000000000094"
	request := Request{Operation: OpInstall, PackageRoot: pkg, ClientID: "codex", ClientConfigRoot: detected[0].ConfigRoot, ClientExecutable: executable, InstallationID: id, RequiredComponents: []string{"mcp", "skills"}}
	for _, operation := range []string{"add", "repeat"} {
		request.OperationID = operation
		prepared, err := eng.Prepare(ctx, request)
		if err != nil { t.Fatal(err) }
		if prepared.Plan().ConfigRoot != profile { t.Fatalf("plan changed profile: %+v", prepared.Plan()) }
		result, err := eng.Apply(ctx, prepared, Decision{Confirmed: true})
		_ = prepared.Close()
		if err != nil || result.Client.Verification != string(domain.VerificationInstalled) { t.Fatalf("%s: %+v %v", operation, result, err) }
		if operation == "repeat" && result.Outcome != OutcomeUnchanged { t.Fatalf("repeat mutated: %+v", result) }
	}
	remove, err := eng.Prepare(ctx, Request{Operation: OpRemove, ClientID: "codex", ClientConfigRoot: profile, ClientExecutable: executable, InstallationID: id, OperationID: "remove", ExternalUninstalled: true})
	if err != nil { t.Fatal(err) }
	removed, err := eng.Apply(ctx, remove, Decision{Confirmed: true})
	_ = remove.Close()
	if err != nil || removed.Outcome != OutcomeCompleted { t.Fatalf("remove: %+v %v", removed, err) }
	body, err := os.ReadFile(filepath.Join(profile, "fixture-registry.json"))
	if err != nil { t.Fatal(err) }
	var remaining struct { Markets map[string]string; Plugins map[string]bool }
	if err := json.Unmarshal(body, &remaining); err != nil { t.Fatal(err) }
	if len(remaining.Markets) != 1 || remaining.Markets["unrelated"] != unrelated || len(remaining.Plugins) != 1 || !remaining.Plugins["sample-notify@unrelated"] { t.Fatalf("unrelated records changed: %s", body) }
	entries, err := os.ReadDir(defaultProfile)
	if err != nil || len(entries) != 1 || entries[0].Name() != "keep" { t.Fatalf("default profile mutated: %v %v", entries, err) }
	body, err = os.ReadFile(filepath.Join(defaultProfile, "keep"))
	if err != nil || !bytes.Equal(body, sentinel) { t.Fatal("default sentinel changed") }
	body, err = os.ReadFile(filepath.Join(profile, "fixture-commands.jsonl"))
	if err != nil { t.Fatal(err) }
	seen := map[string]int{}
	for _, line := range bytes.Split(bytes.TrimSpace(body), []byte("\n")) {
		var call struct { Root string; Args []string }
		if err := json.Unmarshal(line, &call); err != nil { t.Fatal(err) }
		if call.Root != profile { t.Fatalf("child changed profile: %s", line) }
		key, _ := json.Marshal(call.Args[:min(2, len(call.Args))])
		seen[string(key)]++
	}
	for _, command := range []string{`["--version"]`, `["plugin","add"]`, `["plugin","list"]`, `["plugin","remove"]`, `["plugin","marketplace"]`} {
		if seen[command] == 0 { t.Fatalf("missing command %s in %s", command, body) }
	}
	if seen[`["plugin","add"]`] != 1 { t.Fatalf("repeat issued native add: %s", body) }
	t.Logf("profile=%s; one native add; repeat unchanged; remove preserved unrelated marketplace/plugin; default sentinel unchanged; fixture only", profile)
}

func TestExplicitCodexProfileFacadeCanonicalization(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil { t.Fatal(err) }
	eng, err := newTestEngine(t, Config{StateRoot: filepath.Join(base, "state")})
	if err != nil { t.Fatal(err) }
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil { t.Fatal(err) }
	for _, root := range []string{"", "relative", base + string(filepath.Separator), file, filepath.Join(file, "child")} {
		if _, err := eng.detectedClient(Request{Operation: OpRemove, ClientID: "codex", ClientConfigRoot: root}); err == nil { t.Fatalf("accepted invalid explicit root %q", root) }
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(base, alias); err != nil { t.Skipf("symlinks unavailable: %v", err) }
	client, err := eng.detectedClient(Request{Operation: OpRemove, ClientID: "codex", ClientConfigRoot: alias})
	if err != nil || client.ConfigRoot != base { t.Fatalf("canonical facade profile: %+v %v", client, err) }
}
