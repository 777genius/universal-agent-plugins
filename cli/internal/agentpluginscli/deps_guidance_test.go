package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func renderDependencyStatusForTest(t *testing.T, status dependencyStatus, format string) string {
	t.Helper()
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := renderDependencyStatus(cmd, &options{format: format}, status); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestDependencyStatusPendingRecoveryNeverSuggestsOtherMutations(t *testing.T) {
	status := dependencyStatus{
		Scope: "project", ManifestPath: "/private/project/uap.toml",
		LockPath: "/private/project/uap.lock", SupportedTargets: []string{"codex"},
		PendingRecovery: true, ValidLock: true,
		Facts: []dependencyStatusFact{
			{Alias: "review", ClientID: "codex", Facts: []string{"activation_prepared", "not_synced"}},
			{Facts: []string{"pending_recovery"}},
		},
	}
	text := renderDependencyStatusForTest(t, status, "")
	for _, expected := range []string{
		"Dependency environment (project)", "review/codex: activation_prepared,not_synced",
		"Recovery is pending", "agentplugins deps recover --dry-run",
		"--scope project and the same --project-root",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("human guidance must retain %q:\n%s", expected, text)
		}
	}
	for _, forbidden := range []string{"agentplugins deps tree", "agentplugins install --locked", "Automatically activated"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("recovery state suggested unrelated/unsupported action %q:\n%s", forbidden, text)
		}
	}

	body := renderDependencyStatusForTest(t, status, "json")
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("status JSON changed into prose: %v: %s", err, body)
	}
	expectedFields := map[string]bool{
		"excluded_targets": true, "scope": true, "manifest_path": true,
		"lock_path": true, "supported_targets": true, "pending_recovery": true,
		"facts": true,
	}
	if len(decoded) != len(expectedFields) {
		t.Fatalf("public JSON shape changed: %+v", decoded)
	}
	for field := range decoded {
		if !expectedFields[field] {
			t.Fatalf("unexpected JSON field %q in status", field)
		}
	}
	var restored dependencyStatus
	if err := json.Unmarshal([]byte(body), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Scope != status.Scope || !restored.PendingRecovery || !reflect.DeepEqual(restored.Facts, status.Facts) {
		t.Fatalf("status JSON lost original facts: %+v", restored)
	}
}

func TestDependencyStatusMissingManifestDoesNotClaimNoInstallations(t *testing.T) {
	for _, scope := range []string{"user", "project"} {
		t.Run(scope, func(t *testing.T) {
			status := dependencyStatus{
				Scope: scope, SupportedTargets: []string{},
				Facts: []dependencyStatusFact{{Facts: []string{"manifest_missing"}}},
			}
			s := renderDependencyStatusForTest(t, status, "")
			for _, want := range []string{"manifest_missing", "No saved uap.toml was found",
				"does not mean other plugins are absent", "supported saved-dependency target", "agentplugins add <source> --target <agent>"} {
				if !strings.Contains(s, want) {
					t.Fatalf("missing manifest guidance lacks %q: %s", want, s)
				}
			}
			if strings.Contains(s, "agentplugins install --locked") || strings.Contains(s, "agentplugins deps tree") {
				t.Fatalf("missing manifest suggested an unavailable lock: %s", s)
			}
			if scope == "project" && !strings.Contains(s, "--scope project and the same --project-root") {
				t.Fatalf("project guidance lost authority: %s", s)
			}
			if scope == "user" && !strings.Contains(s, "--scope user") {
				t.Fatalf("user guidance could act on a cwd project: %s", s)
			}
		})
	}
}

func TestDependencyStatusGuidanceRequiresValidatedLock(t *testing.T) {
	status := dependencyStatus{
		Scope: "user", SupportedTargets: []string{"codex"},
		Facts: []dependencyStatusFact{{Alias: "review", Facts: []string{"not_synced"}}},
	}
	missing := renderDependencyStatusForTest(t, status, "")
	if !strings.Contains(missing, "No valid uap.lock was found") ||
		!strings.Contains(missing, "Inspect uap.toml and uap.lock") ||
		strings.Contains(missing, "agentplugins deps tree") ||
		strings.Contains(missing, "agentplugins install --locked") {
		t.Fatalf("invalid lock was described as usable: %s", missing)
	}
	status.ValidLock = true
	valid := renderDependencyStatusForTest(t, status, "")
	if !strings.Contains(valid, "agentplugins deps tree") || !strings.Contains(valid, "--scope user") {
		t.Fatalf("valid lock did not surface a read-only scoped inspection: %s", valid)
	}
	if strings.Contains(valid, "No valid uap.lock") || strings.Contains(valid, "agentplugins install") {
		t.Fatalf("valid lock suggested an unsafe mutation: %s", valid)
	}
}

// Exercise the collector as well as the human renderer: an absent or corrupt
// lock must never be presented as safe to inspect or replay.
func TestDependencyStatusCollectorRejectsUnusableLocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		lock []byte
	}{
		{name: "missing"},
		{name: "malformed", lock: []byte("{not valid JSON")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCLIFixture(t, nil)
			f.app.DependenciesRoot = filepath.Join(t.TempDir(), "dependencies")
			var err error
			f.app.Installer, err = installer.New(installer.Config{
				StateRoot: f.app.DependenciesRoot, Registry: f.app.ClientRegistry,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := f.execute(false, "deps", "init"); err != nil {
				t.Fatalf("initialize saved-dependency manifest: %v", err)
			}
			_, lockPath := f.app.dependencyPaths()
			if tc.lock != nil {
				if err := os.WriteFile(lockPath, tc.lock, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			output, _, err := f.execute(false, "deps", "status")
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"No valid uap.lock was found", "Inspect uap.toml and uap.lock"} {
				if !strings.Contains(output, want) {
					t.Fatalf("collector omitted invalid-lock guidance %q: %s", want, output)
				}
			}
			for _, unsafe := range []string{"agentplugins deps tree", "agentplugins install --locked"} {
				if strings.Contains(output, unsafe) {
					t.Fatalf("collector suggested %q for %s lock: %s", unsafe, tc.name, output)
				}
			}
		})
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
	for _, want := range []string{"Saved dependencies (exact locked versions)", "main 1.0.0",
		"  helper 2.0.0", "    shared 3.1.0", "shared 3.1.0 (reference)",
		"same locked package was already shown"} {
		if !strings.Contains(s, want) {
			t.Fatalf("graph lacks %q: %s", want, s)
		}
	}
	if count := strings.Count(s, "shared 3.1.0"); count != 2 {
		t.Fatalf("shared lock node should be displayed with a reference, got %d: %s", count, s)
	}
	if len(tree.Packages) != 3 || len(tree.Packages[2].RequiredBy) != 2 {
		t.Fatal("rendering changed graph input")
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

func TestDependencyTreeIndentedPackageCanAlsoBeDirectRoot(t *testing.T) {
	tree := dependencyTree{
		Roots: []string{"a", "b"},
		Packages: []environment.LockedPackage{
			{Alias: "a", Version: "1.0.0"},
			{Alias: "b", Version: "2.0.0", RequiredBy: []environment.RequiredBy{{Alias: "a"}}},
		},
	}
	var out bytes.Buffer
	if err := renderDependencyTree(&out, tree); err != nil {
		t.Fatal(err)
	}
	human := out.String()
	for _, want := range []string{
		"a 1.0.0\n  b 2.0.0\nb 2.0.0 (reference)\n",
		"an indented package may also be a direct dependency",
	} {
		if !strings.Contains(human, want) {
			t.Fatalf("missing direct-and-indirect package relationship %q: %s", want, human)
		}
	}
}
