package installer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
)

// Public Prepare/Apply regression: update the selected Claude+Codex bindings
// while Pi stays live, unselected and exactly recorded. The genuine native
// Pi projection is exercised with disposable profiles; visible desktop/UI is
// outside this install-core E2E.
func TestPublicGroupUpdatePreflightsUnselectedPiWithoutMutatingIt(t *testing.T) {
	s := newBothClientSandbox(t)
	registry, err := clients.NewRegistry(codex.New(), claude.New(), pi.New())
	if err != nil {
		t.Fatal(err)
	}
	eng, err := newTestEngine(t, Config{StateRoot: s.engine.cfg.StateRoot, HelperExecutable: s.probe,
		Runner: s.runner, Registry: registry})
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Dir(s.pkg)
	id := "00000000-0000-4000-8000-000000000596"
	plugin := filepath.Join(s.pkg, "plugin.json")
	originalManifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sample-notify","version":"1.0.0","extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"native"}}}`
	updatedManifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sample-notify","version":"1.0.1","extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"native"}}}`
	if err := os.WriteFile(plugin, []byte(originalManifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(s.pkg, "native"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.pkg, "native", "index.ts"), []byte("export default function(pi) {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	piRoot := filepath.Join(base, "pi-profile")
	foreign := filepath.Join(piRoot, "extensions", "foreign.ts")
	if err := os.MkdirAll(filepath.Dir(foreign), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, []byte("// foreign retained\n"), 0600); err != nil {
		t.Fatal(err)
	}
	installBothClients(s.ctx, t, eng, s.pkg, s.probe, id, "group-base-install", bothClientTargets(s.codexConfig, s.claudeConfig, s.probe))
	piRequest := Request{Operation: OpInstall, PackageRoot: s.pkg, InstallationID: id,
		ClientID: "pi", ClientConfigRoot: piRoot, OperationID: "add-unselected-pi"}
	prepared, err := eng.Prepare(s.ctx, piRequest)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := eng.Apply(s.ctx, prepared, Decision{Confirmed: true})
	_ = prepared.Close()
	if err != nil || installed.Outcome != OutcomeCompleted {
		t.Fatalf("public Pi install: %+v %v", installed, err)
	}
	before, err := eng.Inspect(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	piBefore := inspectedBinding(t, before, "pi")
	claudeBefore := inspectedBinding(t, before, "claude")
	codexBefore := inspectedBinding(t, before, "codex")
	if err := os.WriteFile(plugin, []byte(updatedManifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.pkg, "skills", "sample-notify", "SKILL.md"),
		[]byte("---\nname: sample-notify\ndescription: Updated group fixture\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	request := Request{Operation: OpUpdate, PackageRoot: s.pkg, InstallationID: id, OperationID: "selected-group-update",
		RequiredComponents: []string{"mcp", "skills"}, ClientExecutable: s.probe,
		Targets:           bothClientTargets(s.codexConfig, s.claudeConfig, s.probe),
		KnownTargets: []TargetFacts{{ClientID: "pi", BindingID: piBefore.BindingID, ConfigRoot: piRoot, Executable: s.probe}},
	}
	precondition, err := os.ReadFile(eng.cfg.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []struct {
		name  string
		facts []TargetFacts
	}{
		{"missing", nil},
		{"stale-binding", []TargetFacts{{ClientID: "pi", BindingID: "not-the-pi-binding", ConfigRoot: piRoot, Executable: s.probe}}},
		{"wrong-profile", []TargetFacts{{ClientID: "pi", BindingID: piBefore.BindingID, ConfigRoot: filepath.Join(base, "wrong-pi"), Executable: s.probe}}},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			bad := request
			bad.KnownTargets = invalid.facts
			handle, err := eng.Prepare(s.ctx, bad)
			if err == nil {
				_ = handle.Close()
				t.Fatalf("unverified unselected Pi binding accepted")
			}
			current, readErr := os.ReadFile(eng.cfg.StateFile)
			if readErr != nil || !bytes.Equal(current, precondition) {
				t.Fatalf("refused preflight mutated registry: %v", readErr)
			}
		})
	}
	prepared, err = eng.Prepare(s.ctx, request)
	if err != nil {
		t.Fatalf("exact unselected Pi facts did not admit group: %v", err)
	}
	updated, err := eng.Apply(s.ctx, prepared, Decision{Confirmed: true})
	_ = prepared.Close()
	if err != nil || updated.Outcome != OutcomeCompleted || len(updated.Targets) != 2 {
		t.Fatalf("selected group update: %+v %v", updated, err)
	}
	for _, target := range updated.Targets {
		if target.ClientID == "pi" || target.BindingID == piBefore.BindingID {
			t.Fatalf("unselected Pi became a mutation target: %+v", updated.Targets)
		}
	}
	after, err := eng.Inspect(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	piAfter := inspectedBinding(t, after, "pi")
	if piBefore.BindingID != piAfter.BindingID || piBefore.TargetPath != piAfter.TargetPath ||
		piBefore.TreeDigest != piAfter.TreeDigest || piBefore.DataRoot != piAfter.DataRoot ||
		piBefore.Materialization != piAfter.Materialization || piBefore.Activation != piAfter.Activation {
		t.Fatalf("unselected Pi changed: before=%+v after=%+v", piBefore, piAfter)
	}
	if inspectedBinding(t, after, "claude").TreeDigest == claudeBefore.TreeDigest ||
		inspectedBinding(t, after, "codex").TreeDigest == codexBefore.TreeDigest {
		t.Fatalf("selected clients did not receive new revision: %+v", after)
	}
	if body, err := os.ReadFile(foreign); err != nil || string(body) != "// foreign retained\n" {
		t.Fatalf("foreign Pi extension changed: %s %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(piAfter.TargetPath, "native", "index.ts")); err != nil ||
		string(body) != "export default function(pi) {}\n" {
		t.Fatalf("unselected Pi native payload changed: %s %v", body, err)
	}
}
