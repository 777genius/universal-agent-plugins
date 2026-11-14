package agentpluginscli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestSavedAliasResolutionRequiresUniqueOwnership(t *testing.T) {
	requirements := environment.Requirements{Dependencies: map[string]environment.Requirement{"tools": {Alias: "tools"}}}
	state := domain.StateFileV2{Installations: []domain.Installation{{InstallationID: "owned-id", DeclaredName: "declared"}}, EnvironmentReceipts: []domain.EnvironmentReceipt{{EnvironmentID: environment.EnvironmentUserDefault, Alias: "tools", InstallationID: "owned-id", ClientID: "cursor"}}}
	for _, selector := range []string{"tools", "owned-id", "declared"} {
		alias, installation, err := resolveSavedAlias(state, requirements, selector)
		if err != nil || alias != "tools" || installation == nil || installation.InstallationID != "owned-id" {
			t.Fatalf("%s resolved %s %+v: %v", selector, alias, installation, err)
		}
	}
	state.Installations = append(state.Installations, domain.Installation{InstallationID: "unmanaged-id", DeclaredName: "tools"})
	if _, _, err := resolveSavedAlias(state, requirements, "tools"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("collision accepted: %v", err)
	}
	alias, _, err := resolveSavedAlias(state, requirements, "owned-id")
	if err != nil || alias != "tools" {
		t.Fatalf("exact ID did not disambiguate: %s %v", alias, err)
	}
	state.EnvironmentReceipts = append(state.EnvironmentReceipts, domain.EnvironmentReceipt{EnvironmentID: environment.EnvironmentUserDefault, Alias: "other", InstallationID: "owned-id"})
	if _, _, err := resolveSavedAlias(state, requirements, "owned-id"); err == nil {
		t.Fatal("multiple receipt aliases accepted")
	}
}

func TestSavedRemoveTargetsPreservesOtherAliasesAndReceiptOnlyRetry(t *testing.T) {
	original := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{"tools": {Alias: "tools"}, "other": {Alias: "other"}}, Targets: map[string][]string{"cursor": {"tools", "other"}, "claude": {"tools"}}}
	state := domain.StateFileV2{EnvironmentReceipts: []domain.EnvironmentReceipt{{EnvironmentID: environment.EnvironmentUserDefault, Alias: "tools", ClientID: "codex"}, {EnvironmentID: "other-environment", Alias: "tools", ClientID: "ignored"}}}
	if got := savedAliasTargets(original, state, "tools"); !reflect.DeepEqual(got, []string{"claude", "codex", "cursor"}) {
		t.Fatalf("targets=%v", got)
	}
	first := removeSavedTargets(original, "tools", []string{"cursor"})
	if got := first.Targets["cursor"]; !reflect.DeepEqual(got, []string{"other"}) {
		t.Fatalf("other assignment changed: %v", got)
	}
	if _, ok := first.Dependencies["tools"]; !ok {
		t.Fatal("removed dependency before its last target")
	}
	final := removeSavedTargets(first, "tools", []string{"claude"})
	if _, ok := final.Dependencies["tools"]; ok {
		t.Fatal("last target retained dependency")
	}
	if _, ok := final.Targets["claude"]; ok {
		t.Fatal("empty target key retained")
	}
	retry := removeSavedTargets(final, "tools", []string{"codex"})
	if !reflect.DeepEqual(final, retry) {
		t.Fatalf("receipt-only retry changed intent: %+v", retry)
	}
	if !reflect.DeepEqual(original.Targets["cursor"], []string{"tools", "other"}) {
		t.Fatal("input mutated")
	}
}

func TestUpdateAllOwnedSkipsBeforePackageOrDirectoryResolution(t *testing.T) {
	session := updateAllSession{ownedAliases: map[string][]string{"owned": {"tools"}}}
	session.classifyInstallation(domain.Installation{InstallationID: "owned", DeclaredName: "plugin", OriginMode: domain.OriginModeDirectory, Directory: &domain.DirectoryOrigin{}})
	if session.result.Skipped != 1 || session.result.Planned != 0 || session.preflightFailed {
		t.Fatalf("owned item was prepared: %+v", session.result)
	}
	if got := session.result.Installations[0]; got.Status != "skipped" || !strings.Contains(got.Reason, "agentplugins update tools") {
		t.Fatalf("skip lacks honest next action: %+v", got)
	}
}

func TestSavedLifecycleRoutesUninstalledAliasBeforeImperativeSelection(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	fixture.app.DependenciesRoot = t.TempDir()
	manifest := "schema_version = 1\n[dependencies.tools]\ngit = \"acme/tools\"\nref = \"main\"\n[targets]\ncursor = [\"tools\"]\n"
	if err := os.WriteFile(filepath.Join(fixture.app.DependenciesRoot, "uap.toml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"repair", "tools", "--format", "json"}, "run agentplugins deps sync"},
		{[]string{"remove", "tools", "--purge-data", "--format", "json"}, "remove owned bindings before purging"},
	} {
		_, _, err := fixture.execute(false, test.args...)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("%v: %v; want %q", test.args, err, test.want)
		}
	}
}
