//go:build uappm_e2e

package agentpluginscli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// C7: one saved operation targets Codex and Gemini; only Gemini contains a
// foreign same-name Skill. The CLI must never describe divergent native targets
// as a complete success, nor adopt/delete the foreign Skill without receipts.
func pmAdversarialCrossTargetForeignSkill(t *testing.T) {
	graph := pmArchiveGraph{"a": nil}
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, true)
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{
		"codex":  {"a"},
		"gemini": {"a"},
	})
	expected := pmArchiveTargets{"a": {"codex", "gemini"}}
	fixture.run(t, "deps", "lock", "--format", "json")
	fixture.assertGraph(t, origin, graph, expected)

	foreignRoot := filepath.Join(fixture.clients["gemini"].ConfigRoot, "skills", "a")
	if err := os.MkdirAll(foreignRoot, 0700); err != nil {
		t.Fatal(err)
	}
	foreignSkill := filepath.Join(foreignRoot, "SKILL.md")
	foreignBytes := "---\nname: a\ndescription: Independently maintained personal Skill\n---\nNever adopt this foreign content.\n"
	dependencyWrite(t, foreignSkill, foreignBytes)
	userData := pmEvolutionSeedData(t, fixture)
	geminiSettings := filepath.Join(fixture.clients["gemini"].ConfigRoot, "settings.json")
	foreignBefore := freshRestoreFiles(t, foreignRoot, geminiSettings, userData)
	codexBefore := freshRestoreFiles(t, fixture.clients["codex"].ConfigRoot)
	mp, lp := fixture.app.dependencyPaths()
	portableBefore := freshRestoreFiles(t, mp, lp)

	_, err := fixture.executePM(t.Context(), t, "", false, "install", "--locked", "--yes", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "Gemini skill \"a\" already exists without agentplugins ownership") {
		t.Fatalf("cross-target foreign Skill must return a precise ownership refusal, got %v", err)
	}
	pmArchiveAssertEqual(t, "foreign Skill and unrelated Gemini/user files", foreignBefore,
		freshRestoreFiles(t, foreignRoot, geminiSettings, userData))
	pmArchiveAssertEqual(t, "immutable saved agreement on failed install", portableBefore,
		freshRestoreFiles(t, mp, lp))

	view, err := fixture.app.Installer.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatalf("failed multi-target operation left an uninspectable recovery state: %+v %v", view.Recovery, err)
	}
	codexActive := false
	for _, installation := range view.Installations {
		for _, binding := range installation.Bindings {
			if binding.Owner == nil || binding.Owner.Alias != "a" {
				t.Fatalf("unrelated or unowned binding after collision: %+v", binding)
			}
			switch binding.ClientID {
			case "gemini":
				if binding.Activation == "active" || binding.Verification == "installation_verified" {
					t.Fatalf("foreign Gemini Skill was falsely reported installed: %+v", binding)
				}
			case "codex":
				if binding.Activation == "active" && binding.Verification == "installation_verified" {
					codexActive = true
					if got := dependencyRead(t, filepath.Join(binding.TargetPath, "marker.txt")); got != pmArchiveMarker("a") {
						t.Fatalf("Codex partial binding has incorrect source bytes: %q", got)
					}
				} else {
					t.Fatalf("Codex binding was committed without a truthful activation outcome: %+v", binding)
				}
			default:
				t.Fatalf("unselected client acquired a binding: %+v", binding)
			}
		}
	}
	facts := readDependencyStatusTestFacts(t, fixture.run(t, "deps", "status", "--format", "json"))
	if len(view.EnvironmentReceipts) != 0 && !facts["partial"] {
		t.Fatalf("failed installation with persisted receipts was not marked partial: %v", facts)
	}
	if codexActive {
		owned := false
		for _, receipt := range view.EnvironmentReceipts {
			if receipt.ClientID == "codex" && receipt.Alias == "a" {
				owned = true
			}
		}
		if !owned {
			t.Fatal("Codex changed on partial success without a durable owner receipt")
		}
	} else {
		pmArchiveAssertEqual(t, "refusal without Codex binding must not change Codex files",
			codexBefore, freshRestoreFiles(t, fixture.clients["codex"].ConfigRoot))
	}

	// Resolve only the deliberately introduced foreign collision. Retain the
	// original lock and source identity across a real second CLI invocation.
	if err := os.RemoveAll(foreignRoot); err != nil {
		t.Fatal(err)
	}
	fixture.run(t, "install", "--locked", "--yes", "--format", "json")
	fixture.assertGraph(t, origin, graph, expected)
	bindings := fixture.assertInstalled(t, expected)
	if len(bindings) != 2 || bindings["a/codex"].BindingID == "" || bindings["a/gemini"].BindingID == "" {
		t.Fatalf("locked retry did not converge both client targets: %+v", bindings)
	}
	facts = readDependencyStatusTestFacts(t, fixture.run(t, "deps", "status", "--format", "json"))
	if facts["partial"] || facts["not_synced"] || facts.hasMismatch() {
		t.Fatalf("successful mixed-target retry still reports incomplete installation: %v", facts)
	}
	pmArchiveAssertEqual(t, "retry changed unrelated user data", freshRestoreFiles(t, userData),
		freshRestoreFiles(t, userData))
}
