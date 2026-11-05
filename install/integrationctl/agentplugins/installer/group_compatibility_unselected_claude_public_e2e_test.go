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

// Public Prepare/Apply: Codex+Pi update without selecting the third live Claude
// binding. The SDK may observe Claude compatibility but must not mutate it.
func TestPublicGroupUpdateCodexPiPreservesUnselectedClaude(t *testing.T) {
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
	manifest := filepath.Join(s.pkg, "plugin.json")
	initial := \`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sample-notify","version":"1.0.0","extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"native"}}}\`
	updated := \`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sample-notify","version":"1.0.1","extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"native"}}}\`
	if err := os.WriteFile(manifest, []byte(initial), 0600); err != nil {
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
	if err := os.WriteFile(foreign, []byte("// owner: foreign\n"), 0600); err != nil {
		t.Fatal(err)
	}

	id := "00000000-0000-4000-8000-000000000597"
	installBothClients(s.ctx, t, eng, s.pkg, s.probe, id, "initial-claude-codex", bothClientTargets(s.codexConfig, s.claudeConfig, s.probe))
	piReq := Request{Operation: OpInstall, PackageRoot: s.pkg, InstallationID: id,
		ClientID: "pi", ClientConfigRoot: piRoot, OperationID: "add-pi"}
	piHandle, err := eng.Prepare(s.ctx, piReq)
	if err != nil {
		t.Fatal(err)
	}
	piResult, err := eng.Apply(s.ctx, piHandle, Decision{Confirmed: true})
	_ = piHandle.Close()
	if err != nil || piResult.Outcome != OutcomeCompleted {
		t.Fatalf("install Pi: %+v %v", piResult, err)
	}

	before, err := eng.Inspect(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	oldClaude := inspectedBinding(t, before, "claude")
	oldCodex := inspectedBinding(t, before, "codex")
	oldPi := inspectedBinding(t, before, "pi")
	claudeSkillPath := filepath.Join(oldClaude.TargetPath, ".agentplugins-runtime", "skills", "sample-notify", "SKILL.md")
	oldSkill, err := os.ReadFile(claudeSkillPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(updated), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.pkg, "skills", "sample-notify", "SKILL.md"),
		[]byte("---\nname: sample-notify\ndescription: Selected new revision\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}

	req := Request{Operation: OpUpdate, PackageRoot: s.pkg, InstallationID: id,
		OperationID: "update-selected-codex-pi", ClientExecutable: s.probe,
		Targets: []ClientTarget{
			{ClientID: "codex", ClientConfigRoot: s.codexConfig, ClientExecutable: s.probe},
			{ClientID: "pi", ClientConfigRoot: piRoot, ClientExecutable: s.probe},
		},
		KnownTargets: []TargetFacts{{ClientID: "claude", BindingID: oldClaude.BindingID, ConfigRoot: s.claudeConfig, Executable: s.probe}},
	}
	initialState, err := os.ReadFile(eng.cfg.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, badFacts := range [][]TargetFacts{
		nil,
		{{ClientID: "claude", BindingID: "substituted-claude-binding", ConfigRoot: s.claudeConfig, Executable: s.probe}},
	} {
		bad := req
		bad.KnownTargets = badFacts
		if handle, err := eng.Prepare(s.ctx, bad); err == nil {
			_ = handle.Close()
			t.Fatalf("unverified sibling facts admitted: %+v", badFacts)
		}
		observed, err := os.ReadFile(eng.cfg.StateFile)
		if err != nil || !bytes.Equal(initialState, observed) {
			t.Fatalf("rejected plan changed UAP state: %v", err)
		}
	}

	handle, err := eng.Prepare(s.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := eng.Apply(s.ctx, handle, Decision{Confirmed: true})
	_ = handle.Close()
	if err != nil || result.Outcome != OutcomeCompleted || len(result.Targets) != 2 {
		t.Fatalf("selected group update: %+v %v", result, err)
	}
	for _, target := range result.Targets {
		if target.ClientID != "codex" && target.ClientID != "pi" || target.BindingID == oldClaude.BindingID {
			t.Fatalf("unselected Claude became mutation target: %+v", result.Targets)
		}
	}
	after, err := eng.Inspect(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	claudeAfter := inspectedBinding(t, after, "claude")
	if oldClaude.BindingID != claudeAfter.BindingID || oldClaude.TargetPath != claudeAfter.TargetPath ||
		oldClaude.TreeDigest != claudeAfter.TreeDigest || oldClaude.DataRoot != claudeAfter.DataRoot ||
		oldClaude.Materialization != claudeAfter.Materialization || oldClaude.Activation != claudeAfter.Activation {
		t.Fatalf("unselected Claude changed: before=%+v after=%+v", oldClaude, claudeAfter)
	}
	skillAfter, err := os.ReadFile(claudeSkillPath)
	if err != nil || !bytes.Equal(oldSkill, skillAfter) {
		t.Fatalf("Claude installed skill mutated: %v", err)
	}
	if inspectedBinding(t, after, "codex").TreeDigest == oldCodex.TreeDigest ||
		inspectedBinding(t, after, "pi").TreeDigest == oldPi.TreeDigest {
		t.Fatalf("selected client revisions did not change: %+v", after)
	}
	body, err := os.ReadFile(foreign)
	if err != nil || string(body) != "// owner: foreign\n" {
		t.Fatalf("foreign Pi extension changed: %v", err)
	}
}
