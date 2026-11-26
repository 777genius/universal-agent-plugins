package environment

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// The shared predicate reports every foreign binding, including clients outside
// the selected mutation targets, without suggesting removal as a migration.
func TestForeignBindingErrorNamesEveryUnownedClient(t *testing.T) {
	pkg := LockedPackage{Alias: "demo", SourceKey: "git:demo"}
	duplicate := BindingObservation{InstallationID: "inst-b", ClientID: "cursor", SourceKey: pkg.SourceKey}
	observed := []BindingObservation{
		duplicate,
		{InstallationID: "inst-a", ClientID: "codex", SourceKey: pkg.SourceKey},
		{InstallationID: "inst-a", ClientID: "claude", SourceKey: pkg.SourceKey},
		{InstallationID: "inst-c", ClientID: "gemini", SourceKey: "git:other"},
		{InstallationID: "inst-d", ClientID: "kiro", SourceKey: pkg.SourceKey, Managed: true, Alias: pkg.Alias},
		{InstallationID: "inst-e", ClientID: "codex", SourceKey: pkg.SourceKey, OwnerEnvironmentID: "other-environment"},
		duplicate,
	}
	_, err := validateDesiredBinding(pkg, "codex", BindingObservation{}, observed)
	var foreign *ForeignBindingError
	if !errors.As(fmt.Errorf("wrapped: %w", err), &foreign) {
		t.Fatalf("error = %v", err)
	}
	want := []ForeignBinding{
		{InstallationID: "inst-a", ClientID: "claude", Reason: "unmanaged_binding"},
		{InstallationID: "inst-a", ClientID: "codex", Reason: "unmanaged_binding"},
		{InstallationID: "inst-b", ClientID: "cursor", Reason: "unmanaged_binding"},
		{InstallationID: "inst-e", ClientID: "codex", Reason: "foreign_environment", OwnerEnvironmentID: "other-environment"},
	}
	if foreign.Alias != pkg.Alias || foreign.SourceKey != pkg.SourceKey || !reflect.DeepEqual(foreign.Bindings, want) {
		t.Fatalf("foreign = %+v, want bindings %+v", foreign, want)
	}
	if msg := err.Error(); strings.Contains(msg, "remove") || strings.Contains(msg, "inst-c") {
		t.Fatalf("unsafe guidance or unrelated source: %s", msg)
	}
	for left, right := 0, len(observed)-1; left < right; left, right = left+1, right-1 {
		observed[left], observed[right] = observed[right], observed[left]
	}
	var reordered *ForeignBindingError
	if !errors.As(CheckSourceOwnership(pkg, observed), &reordered) || !reflect.DeepEqual(reordered, foreign) {
		t.Fatalf("input order changed refusal: %+v, want %+v", reordered, foreign)
	}
	if err := CheckSourceOwnership(pkg, []BindingObservation{{SourceKey: pkg.SourceKey, Managed: true, Alias: pkg.Alias}}); err != nil {
		t.Fatalf("owned source blocked: %v", err)
	}
}

func TestCheckSourceOwnershipNeedsKnownIdentity(t *testing.T) {
	if err := CheckSourceOwnership(LockedPackage{}, []BindingObservation{{InstallationID: "unknown"}}); err != nil {
		t.Fatalf("empty identity inferred: %v", err)
	}
	if err := CheckSourceOwnership(LockedPackage{SourceKey: "git:known"}, []BindingObservation{{SourceKey: "git:other"}}); err != nil {
		t.Fatalf("unrelated source blocked: %v", err)
	}
	var foreign *ForeignBindingError
	if !errors.As(CheckSourceOwnership(LockedPackage{SourceKey: "git:known"}, []BindingObservation{{SourceKey: "git:known"}}), &foreign) || foreign.Alias != "" {
		t.Fatalf("preview requires or invents alias: %+v", foreign)
	}
}

func TestForeignBindingCoordinatorDependencyRoot(t *testing.T) {
	for _, tc := range []struct {
		name, wantRoot string
		roots          []string
		packages       []LockedPackage
	}{
		{name: "direct root", roots: []string{"a"}, packages: []LockedPackage{removalPackage("a")}},
		{name: "direct root also dependency", roots: []string{"a", "c"}, packages: []LockedPackage{removalPackage("a"), removalPackage("c", "a")}},
		{name: "transitive dependency", roots: []string{"a"}, packages: []LockedPackage{removalPackage("a"), removalPackage("b", "a"), removalPackage("c", "b")}, wantRoot: "a"},
		{name: "ambiguous roots", roots: []string{"a", "b"}, packages: []LockedPackage{removalPackage("a"), removalPackage("b"), removalPackage("c", "a", "b")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := removalRequirements(tc.roots...)
			lock := removalLock(t, req, tc.packages...)
			conflict := lock.Packages[len(lock.Packages)-1]
			observed := []BindingObservation{{SourceKey: conflict.SourceKey, InstallationID: "existing", ClientID: "claude"}}
			_, err := (Coordinator{}).PrepareSyncForTargets(req, lock, observed, lock.ClosureAliases(tc.roots), nil, []string{"codex"})
			var foreign *ForeignBindingError
			if !errors.As(err, &foreign) || foreign.RootAlias != tc.wantRoot || foreign.Alias != conflict.Alias {
				t.Fatalf("error = %+v (%v), want alias %s root %s", foreign, err, conflict.Alias, tc.wantRoot)
			}
		})
	}
}

func TestSelectedSyncDoesNotCheckUnrelatedLockedPackage(t *testing.T) {
	req := removalRequirements("a", "b")
	lock := removalLock(t, req, removalPackage("a"), removalPackage("b"))
	observed := []BindingObservation{{SourceKey: lock.Packages[1].SourceKey, InstallationID: "unrelated", ClientID: "claude"}}
	plan, err := (Coordinator{}).PrepareSyncForTargets(req, lock, observed, []string{"a"}, nil, []string{"codex"})
	if err != nil || len(plan.Bindings) != 1 || plan.Bindings[0].Alias != "a" {
		t.Fatalf("unrelated locked source affected selection: %+v, %v", plan, err)
	}
}

func TestForeignBindingPreservesPendingAndDriftRefusals(t *testing.T) {
	pkg := LockedPackage{Alias: "a", SourceKey: "git:a"}
	observed := []BindingObservation{{SourceKey: pkg.SourceKey, InstallationID: "foreign"}}
	_, err := validateDesiredBinding(pkg, "codex", BindingObservation{Pending: true}, observed)
	if !errors.Is(err, installer.ErrRecoveryRequired) {
		t.Fatalf("pending refusal changed: %v", err)
	}
	_, err = validateDesiredBinding(pkg, "codex", BindingObservation{Drift: true}, observed)
	var foreign *ForeignBindingError
	if err == nil || errors.As(err, &foreign) || !strings.Contains(err.Error(), "drift:") {
		t.Fatalf("drift refusal changed: %v", err)
	}
}
