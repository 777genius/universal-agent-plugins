//go:build !windows

package agentpluginscli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDependencyCheckSnapshotCleanupFailureCLI(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(fmt.Sprintf("pin_mismatch=%t", mismatch), func(t *testing.T) {
			f := newDependencyCheckFixture(t)
			pin := seedDependencyCheckPackage(t, f, "demo", "")
			if mismatch {
				pin.Version = "9.0.0"
			}
			writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
			parent := t.TempDir()
			probe := filepath.Join(parent, "permission-probe")
			if err := os.Mkdir(probe, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(parent, 0500); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Chmod(parent, 0700) }()
			removeErr := os.Remove(probe)
			if err := os.Chmod(parent, 0700); err != nil {
				t.Fatal(err)
			}
			if removeErr == nil {
				t.Skip("host bypasses directory write permissions")
			}
			if !errors.Is(removeErr, os.ErrPermission) {
				t.Fatal(removeErr)
			}
			t.Setenv("TMPDIR", parent)
			var snapshotRoot string
			f.app.PackageLoader = dependencyCheckHookLoader{delegate: f.app.PackageLoader, hook: func(input domain.LoadInput) {
				snapshotRoot = input.SnapshotRoot
				if err := os.Chmod(parent, 0500); err != nil {
					t.Fatal(err)
				}
			}}
			report, _ := executeDependencyCheck(t, f, dependencyCheckError)
			requireDependencyCheckCode(t, report, "snapshot_cleanup_failed")
			if snapshotRoot == "" {
				t.Fatal("snapshot was not created")
			}
			if _, err := os.Stat(filepath.Dir(snapshotRoot)); err != nil {
				t.Fatalf("test did not cause a snapshot cleanup failure: %v", err)
			}
			if err := os.Chmod(parent, 0700); err != nil {
				t.Fatal(err)
			}
			_, err := describePinnedDependency(t.Context(), f.app, pin)
			if !errors.Is(err, errDependencySnapshotCleanup) || mismatch && !errors.Is(err, errDependencyPinMismatch) {
				t.Fatalf("cleanup and original error were not preserved: %v", err)
			}
		})
	}
}
