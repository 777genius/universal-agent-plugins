package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDependencyStatusHumanGuidancePreservesTechnicalFacts(t *testing.T) {
	status := dependencyStatus{
		Scope:            "project",
		ManifestPath:     "/private/project/uap.toml",
		LockPath:         "/private/project/uap.lock",
		SupportedTargets: []string{"codex"},
		PendingRecovery:  true,
		Facts: []dependencyStatusFact{
			{Alias: "review", ClientID: "codex", Facts: []string{"activation_prepared", "not_synced"}},
		},
	}
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := renderDependencyStatus(cmd, &options{}, status); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, expected := range []string{
		"Dependency environment (project)",
		"review/codex: activation_prepared,not_synced",
		"Recovery is pending. Review it first: agentplugins deps recover --dry-run",
		"agentplugins deps tree",
		"agentplugins outdated --all",
		"agentplugins install --locked",
		"agentplugins deps update <alias>, then agentplugins deps sync --locked",
		"add --scope project and the same --project-root",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("human guidance must retain %q:\n%s", expected, text)
		}
	}
	for _, forbidden := range []string{"Automatically activated", "Fully ready", "No dependencies installed"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unverified claim %q in guidance: %s", forbidden, text)
		}
	}

	output.Reset()
	if err := renderDependencyStatus(cmd, &options{format: "json"}, status); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("status JSON changed into prose: %v: %s", err, output.String())
	}
	if decoded["scope"] != "project" || decoded["pending_recovery"] != true {
		t.Fatalf("status JSON lost existing facts: %+v", decoded)
	}
	for _, forbidden := range []string{"Manage saved dependencies", "agentplugins deps tree", "Recovery is pending"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("status JSON contains human guidance: %s", output.String())
		}
	}
}

func TestDependencyStatusMissingManifestDoesNotSuggestLockedInstall(t *testing.T) {
	status := dependencyStatus{
		Scope:            "user",
		SupportedTargets: []string{},
		Facts:            []dependencyStatusFact{{Facts: []string{"manifest_missing"}}},
	}
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := renderDependencyStatus(cmd, &options{}, status); err != nil {
		t.Fatal(err)
	}
	s := output.String()
	for _, want := range []string{"manifest_missing", "No uap.toml is recorded", "agentplugins add <source> --target <agent>"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing manifest guidance lacks %q: %s", want, s)
		}
	}
	if strings.Contains(s, "agentplugins install --locked") || strings.Contains(s, "add --scope project") {
		t.Fatalf("missing user manifest gave inapplicable guidance: %s", s)
	}
}

func TestDependencyTreeHumanLabelsDoNotAlterGraph(t *testing.T) {
	tree := dependencyTree{
		Roots: []string{"main"},
		Packages: []environment.LockedPackage{
			{Alias: "main", Version: "1.0.0"},
			{Alias: "helper", Version: "2.0.0", RequiredBy: []environment.RequiredBy{{Alias: "main"}}},
			{Alias: "shared", Version: "3.1.0", RequiredBy: []environment.RequiredBy{{Alias: "main"}, {Alias: "helper"}}},
		},
	}
	var output bytes.Buffer
	if err := renderDependencyTree(&output, tree); err != nil {
		t.Fatal(err)
	}
	s := output.String()
	for _, want := range []string{"Saved dependencies (exact locked versions)", "main 1.0.0", "  helper 2.0.0", "    shared 3.1.0", "shared 3.1.0 (reference)", "not installed twice"} {
		if !strings.Contains(s, want) {
			t.Fatalf("graph lacks %q: %s", want, s)
		}
	}
	if count := strings.Count(s, "shared 3.1.0"); count != 2 {
		t.Fatalf("shared lock node should be rendered once with a reference, got %d: %s", count, s)
	}
	if len(tree.Packages) != 3 || len(tree.Packages[2].RequiredBy) != 2 {
		t.Fatal("rendering changed the graph input")
	}
}

func TestDependencyTreeEmptyLockIsExplicit(t *testing.T) {
	var output bytes.Buffer
	if err := renderDependencyTree(&output, dependencyTree{Roots: []string{}, Packages: []environment.LockedPackage{}}); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "No packages are recorded in this lock.\n" {
		t.Fatalf("empty graph should be unambiguous: %q", got)
	}
}
