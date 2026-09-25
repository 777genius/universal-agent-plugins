//go:build linux

package providers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestLinuxClaudePreparedIdentityRealHomeLayouts(t *testing.T) {
	t.Parallel()
	t.Run("relative-and-absolute-stale-links-with-valid-skill", func(t *testing.T) {
		t.Parallel()
		config := filepath.Join(t.TempDir(), "claude-config")
		root := filepath.Join(config, "skills")
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		linked := filepath.Join(t.TempDir(), "shared-review")
		if err := os.MkdirAll(linked, 0o700); err != nil {
			t.Fatal(err)
		}
		writeIdentityFile(t, filepath.Join(linked, "SKILL.md"), "# Linked skill\n")
		if err := os.Symlink(linked, filepath.Join(root, "shared-review")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("removed-skill", filepath.Join(root, "stale-relative")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(t.TempDir(), "removed-abs"), filepath.Join(root, "stale-absolute")); err != nil {
			t.Fatal(err)
		}
		plan := domain.DeliveryPlan{
			ClientID: domain.ClientClaude, DeclaredName: "demo", TargetAnchor: config, TargetRoot: root,
			ActivePath: filepath.Join(root, "managed-demo"),
		}
		observation, err := (testObserver(NativeIdentityObserver{})).ObservePreparedIdentity(
			context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: config}, plan, nil,
		)
		if err != nil || observation.State != domain.NativeIdentityAbsent {
			t.Fatalf("observation=%+v err=%v", observation, err)
		}
	})
	t.Run("dangling-skills-root-fails-closed", func(t *testing.T) {
		t.Parallel()
		config := filepath.Join(t.TempDir(), "claude-config")
		root := filepath.Join(config, "skills")
		if err := os.MkdirAll(config, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(t.TempDir(), "removed-skills"), root); err != nil {
			t.Fatal(err)
		}
		plan := domain.DeliveryPlan{
			ClientID: domain.ClientClaude, DeclaredName: "demo", TargetAnchor: config, TargetRoot: root,
			ActivePath: filepath.Join(root, "managed-demo"),
		}
		observation, err := (testObserver(NativeIdentityObserver{})).ObservePreparedIdentity(
			context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: config}, plan, nil,
		)
		if err == nil || observation.State != domain.NativeIdentityIndeterminate ||
			!strings.Contains(err.Error(), "symlink target does not exist") {
			t.Fatalf("observation=%+v err=%v", observation, err)
		}
	})
	t.Run("circular-unrelated-skill-fails-closed", func(t *testing.T) {
		t.Parallel()
		config := filepath.Join(t.TempDir(), "claude-config")
		root := filepath.Join(config, "skills")
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("loop", filepath.Join(root, "loop")); err != nil {
			t.Fatal(err)
		}
		plan := domain.DeliveryPlan{
			ClientID: domain.ClientClaude, DeclaredName: "demo", TargetAnchor: config, TargetRoot: root,
			ActivePath: filepath.Join(root, "managed-demo"),
		}
		observation, err := (testObserver(NativeIdentityObserver{})).ObservePreparedIdentity(
			context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: config}, plan, nil,
		)
		if err == nil || observation.State != domain.NativeIdentityIndeterminate {
			t.Fatalf("observation=%+v err=%v", observation, err)
		}
	})
	t.Run("fifo-skills-root-fails-closed", func(t *testing.T) {
		t.Parallel()
		config := filepath.Join(t.TempDir(), "claude-config")
		root := filepath.Join(config, "skills")
		if err := os.MkdirAll(config, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(root, 0o600); err != nil {
			t.Fatal(err)
		}
		plan := domain.DeliveryPlan{
			ClientID: domain.ClientClaude, DeclaredName: "demo", TargetAnchor: config, TargetRoot: root,
			ActivePath: filepath.Join(root, "managed-demo"),
		}
		type outcome struct {
			observation domain.NativeIdentityObservation
			err         error
		}
		done := make(chan outcome, 1)
		go func() {
			observation, err := (testObserver(NativeIdentityObserver{})).ObservePreparedIdentity(
				context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: config}, plan, nil,
			)
			done <- outcome{observation, err}
		}()
		select {
		case result := <-done:
			if result.err == nil || result.observation.State != domain.NativeIdentityIndeterminate ||
				!strings.Contains(result.err.Error(), "is not a directory") {
				t.Fatalf("observation=%+v err=%v", result.observation, result.err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("ObservePreparedIdentity hung on a FIFO Claude skills root")
		}
	})
}
