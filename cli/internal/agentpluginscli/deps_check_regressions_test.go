package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDependencyTransferFilePreservesAdmissionClassification(t *testing.T) {
	for _, tc := range []struct{ name, stage string }{{"uap.toml", "manifest"}, {"uap.lock", "lock"}} {
		t.Run(tc.stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			file, err := readDependencyTransferFile(path)
			if !errors.Is(err, atomicfile.ErrUnsafeFile) {
				t.Fatalf("agreement wrapper replaced strict admission identity: %v", err)
			}
			issue := dependencyDocumentReadIssue(t.Context(), tc.stage, path, file, err)
			if issue == nil || issue.Outcome != dependencyCheckInvalid || issue.Code != tc.stage+"_invalid" {
				t.Fatalf("unsafe agreement classification: %+v", issue)
			}
			// Operational platform errors must retain a different verdict when
			// the same shared reader cannot inspect the agreement's bytes.
			unavailable := &os.PathError{Op: "open", Path: path, Err: os.ErrPermission}
			issue = dependencyDocumentReadIssue(t.Context(), tc.stage, path, file, unavailable)
			if issue == nil || issue.Outcome != dependencyCheckError || issue.Code != tc.stage+"_unreadable" {
				t.Fatalf("operational agreement classification: %+v", issue)
			}
		})
	}
}

func TestDependencyCheckMissingPackageContentAndTemporaryStorage(t *testing.T) {
	t.Run("required_manifest_is_absent", func(t *testing.T) {
		f := newDependencyCheckFixture(t)
		pin := seedDependencyCheckPackage(t, f, "demo", "")
		raw := t.TempDir()
		dependencyWrite(t, filepath.Join(raw, "README.md"), "not a standard package\n")
		snapshot, err := (packagedigest.Builder{}).SnapshotWithExecutables(t.Context(), raw, identityFromLocked(pin), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = packagedigest.Remove(snapshot) }()
		if _, err := f.app.dependencyStore().Put(t.Context(), snapshot); err != nil {
			t.Fatal(err)
		}
		pin.TreeDigest = snapshot.TreeDigest
		writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
		report, _ := executeDependencyCheck(t, f, dependencyCheckInvalid)
		requireDependencyCheckCode(t, report, "package_invalid")
	})
	t.Run("temporary_directory_is_absent", func(t *testing.T) {
		f := newDependencyCheckFixture(t)
		pin := seedDependencyCheckPackage(t, f, "demo", "")
		writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
		missing := filepath.Join(t.TempDir(), "missing")
		for _, variable := range []string{"TMPDIR", "TMP", "TEMP"} {
			t.Setenv(variable, missing)
		}
		report, _ := executeDependencyCheck(t, f, dependencyCheckError)
		requireDependencyCheckCode(t, report, "store_unreadable")
	})
}

// Cancel immediately after the agreement read checkpoint has observed an active
// context, modeling cancellation arriving while the subsequent decode runs.
type dependencyCheckCancelAfterRead struct {
	context.Context
	cancel context.CancelFunc
}

func (ctx dependencyCheckCancelAfterRead) Err() error {
	err := ctx.Context.Err()
	ctx.cancel()
	return err
}

func TestDependencyCheckCancellationDuringLockDecode(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprintf("invalid=%t", invalid), func(t *testing.T) {
			f := newDependencyCheckFixture(t)
			writeDependencyCheckAgreement(t, f, nil)
			manifest, lock := f.app.dependencyPaths()
			requirements, err := readDependencyManifest(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if invalid {
				dependencyWrite(t, lock, `{"lock_version":`)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			_, _, issue := inspectDependencyLockFile(dependencyCheckCancelAfterRead{Context: ctx, cancel: cancel}, lock, requirements)
			if issue == nil || issue.Outcome != dependencyCheckCanceled || issue.Path != lock {
				t.Fatalf("cancellation during decode lost: %+v", issue)
			}
		})
	}
}

func TestDependencyCheckRegistryNameIndexPreservesAssociations(t *testing.T) {
	for _, tc := range []struct {
		name, short, alias, want string
		pins                     []environment.LockedPackage
	}{
		{name: "product_and_basename_are_one_candidate", short: "DEMO", alias: "root", want: "acme/demo", pins: []environment.LockedPackage{{Alias: "stable", Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{ProductID: "demo", DistributionID: "acme/demo"}}}}},
		{name: "unicode_simple_fold", short: "s", alias: "root", want: "acme/demo", pins: []environment.LockedPackage{{Alias: "stable", Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{ProductID: "ſ", DistributionID: "acme/demo"}}}}},
		{name: "alias_selects_collision", short: "demo", alias: "second", want: "other/demo", pins: []environment.LockedPackage{
			{Alias: "first", Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{ProductID: "demo", DistributionID: "acme/demo"}}},
			{Alias: "second", Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{ProductID: "another", DistributionID: "other/demo"}}},
		}},
		{name: "ambiguous_without_alias", short: "demo", alias: "root", pins: []environment.LockedPackage{
			{Alias: "first", Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{ProductID: "demo", DistributionID: "acme/demo"}}},
			{Alias: "second", Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{ProductID: "another", DistributionID: "other/demo"}}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requirements := environment.Requirements{Dependencies: map[string]environment.Requirement{tc.alias: {Alias: tc.alias, Registry: &environment.RegistrySource{Distribution: tc.short}}}}
			err := canonicalizeRequirementsFromLock(&requirements, environment.Lock{Packages: tc.pins})
			if tc.want == "" {
				if err == nil || !strings.Contains(err.Error(), "unambiguously") {
					t.Fatalf("ambiguous mapping accepted: %v", err)
				}
			} else if err != nil || requirements.Dependencies[tc.alias].Registry.Distribution != tc.want {
				t.Fatalf("offline association changed: %+v, %v", requirements, err)
			}
		})
	}
	// Every root shares a product-name bucket. Alias selection must remain
	// indexed even when that bucket contains a large historical graph.
	const count = 12000
	requirements := environment.Requirements{Dependencies: map[string]environment.Requirement{}}
	lock := environment.Lock{}
	for i := range count {
		alias := fmt.Sprintf("node%d", i)
		requirements.Dependencies[alias] = environment.Requirement{Alias: alias, Registry: &environment.RegistrySource{Distribution: "shared"}}
		lock.Packages = append(lock.Packages, environment.LockedPackage{Alias: alias, Source: environment.LockedSource{Registry: &environment.LockedRegistrySource{ProductID: "shared", DistributionID: "acme/" + alias}}})
	}
	if err := canonicalizeRequirementsFromLock(&requirements, lock); err != nil {
		t.Fatal(err)
	}
	for alias, requirement := range requirements.Dependencies {
		if requirement.Registry.Distribution != "acme/"+alias {
			t.Fatalf("large colliding index selected another alias: %+v", requirement)
		}
	}
}

func TestDependencyCheckSnapshotCleanupFailureClassification(t *testing.T) {
	cleanup := fmt.Errorf("%w: %w", errDependencySnapshotCleanup, &os.PathError{Op: "remove", Path: "snapshot", Err: os.ErrPermission})
	for _, initial := range []error{nil, errDependencyPinMismatch, errDependencyPackageContent} {
		issue := dependencyPackageCheckFailure("demo", errors.Join(initial, cleanup))
		if issue.Outcome != dependencyCheckError || issue.Code != "snapshot_cleanup_failed" {
			t.Fatalf("cleanup failure hidden by initial result: %+v", issue)
		}
	}
	issue := dependencyPackageCheckFailure("demo", fmt.Errorf("%w: output unavailable", packagedigest.ErrSnapshotStorage))
	if issue.Outcome != dependencyCheckError || issue.Code != "store_unreadable" {
		t.Fatalf("snapshot output failure became invalid package content: %+v", issue)
	}
}
